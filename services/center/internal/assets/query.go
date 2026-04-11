package assets

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	assetdto "mare/shared/contracts/dto/asset"
)

type Service struct {
	pool             *pgxpool.Pool
	now              func() time.Time
	executorResolver func(nodeType string) (mountPathExecutor, error)
}

type libraryModel struct {
	ID        string
	Name      string
	RootLabel string
}

type directoryModel struct {
	ID           string
	LibraryID    string
	RelativePath string
	Name         string
	ParentPath   *string
	Depth        int
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type assetModel struct {
	ID                  string
	LibraryID           string
	DirectoryID         string
	RelativePath        string
	Name                string
	Extension           *string
	SizeBytes           int64
	FileKind            string
	LifecycleState      string
	Rating              int16
	ColorLabel          string
	CanonicalModifiedAt *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type mountModel struct {
	ID       string
	Name     string
	NodeType string
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, now: time.Now, executorResolver: executorForNodeType}
}

func (s *Service) ListLibraries(ctx context.Context) ([]assetdto.LibraryRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, root_label
		FROM libraries
		WHERE status <> 'ARCHIVED'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]assetdto.LibraryRecord, 0)
	for rows.Next() {
		var row libraryModel
		if err := rows.Scan(&row.ID, &row.Name, &row.RootLabel); err != nil {
			return nil, err
		}
		totalAssets, err := s.countActiveAssets(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		health, err := s.computeLibraryHealth(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		endpointNames, storagePolicy, err := s.loadLibraryMountSummary(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, assetdto.LibraryRecord{
			ID:            row.ID,
			Name:          row.Name,
			RootLabel:     row.RootLabel,
			ItemCount:     formatCount(totalAssets),
			Health:        health,
			StoragePolicy: storagePolicy,
			EndpointNames: endpointNames,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) BrowseLibrary(ctx context.Context, libraryID string, query assetdto.BrowseQuery) (assetdto.BrowseLibraryResponse, error) {
	library, err := s.loadLibrary(ctx, libraryID)
	if err != nil {
		return assetdto.BrowseLibraryResponse{}, err
	}
	currentDir, currentPath, err := s.resolveCurrentDirectory(ctx, libraryID, query.ParentID)
	if err != nil {
		return assetdto.BrowseLibraryResponse{}, err
	}

	breadcrumbs, err := s.buildBreadcrumbs(ctx, libraryID, library.Name, currentPath)
	if err != nil {
		return assetdto.BrowseLibraryResponse{}, err
	}
	endpointNames, _, err := s.loadLibraryMountSummary(ctx, libraryID)
	if err != nil {
		return assetdto.BrowseLibraryResponse{}, err
	}

	directories, err := s.loadDirectDirectories(ctx, libraryID, currentPath)
	if err != nil {
		return assetdto.BrowseLibraryResponse{}, err
	}
	assets, err := s.loadDirectAssets(ctx, libraryID, currentDir.ID)
	if err != nil {
		return assetdto.BrowseLibraryResponse{}, err
	}

	entries := make([]assetdto.EntryRecord, 0, len(directories)+len(assets))
	for _, directory := range directories {
		entry, err := s.buildDirectoryEntry(ctx, library, directory, false)
		if err != nil {
			return assetdto.BrowseLibraryResponse{}, err
		}
		entries = append(entries, entry)
	}
	for _, asset := range assets {
		entry, err := s.buildAssetEntry(ctx, library, asset, false)
		if err != nil {
			return assetdto.BrowseLibraryResponse{}, err
		}
		entries = append(entries, entry)
	}

	currentPathChildren := len(entries)
	filtered := filterEntries(entries, query)
	sortEntriesForBrowse(filtered, query.SortValue, query.SortDirection)

	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	if offset > len(filtered) {
		offset = len(filtered)
	}
	limit := offset + pageSize
	if limit > len(filtered) {
		limit = len(filtered)
	}

	return assetdto.BrowseLibraryResponse{
		Breadcrumbs:         breadcrumbs,
		Items:               filtered[offset:limit],
		Total:               len(filtered),
		CurrentPathChildren: currentPathChildren,
		EndpointNames:       endpointNames,
	}, nil
}

func (s *Service) LoadEntry(ctx context.Context, id string) (*assetdto.EntryRecord, error) {
	directory, err := s.loadDirectoryByID(ctx, id)
	if err == nil {
		library, loadErr := s.loadLibrary(ctx, directory.LibraryID)
		if loadErr != nil {
			return nil, loadErr
		}
		entry, buildErr := s.buildDirectoryEntry(ctx, library, directory, true)
		if buildErr != nil {
			return nil, buildErr
		}
		return &entry, nil
	}
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != "not_found" {
		return nil, err
	}

	asset, err := s.loadAssetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	library, err := s.loadLibrary(ctx, asset.LibraryID)
	if err != nil {
		return nil, err
	}
	entry, err := s.buildAssetEntry(ctx, library, asset, true)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *Service) loadLibrary(ctx context.Context, libraryID string) (libraryModel, error) {
	var row libraryModel
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, root_label
		FROM libraries
		WHERE id = $1
		  AND status <> 'ARCHIVED'
	`, libraryID).Scan(&row.ID, &row.Name, &row.RootLabel)
	if err != nil {
		if err == pgx.ErrNoRows {
			return libraryModel{}, apperrors.NotFound("未找到指定资产库")
		}
		return libraryModel{}, err
	}
	return row, nil
}

func (s *Service) resolveCurrentDirectory(ctx context.Context, libraryID string, parentID *string) (directoryModel, string, error) {
	if parentID == nil || strings.TrimSpace(*parentID) == "" {
		root, err := s.loadDirectoryByPath(ctx, libraryID, "/")
		if err != nil {
			return directoryModel{}, "", err
		}
		return root, "/", nil
	}
	row, err := s.loadDirectoryByID(ctx, *parentID)
	if err != nil {
		return directoryModel{}, "", err
	}
	if row.LibraryID != libraryID {
		return directoryModel{}, "", apperrors.BadRequest("目录不属于当前资产库")
	}
	return row, row.RelativePath, nil
}

func (s *Service) buildBreadcrumbs(ctx context.Context, libraryID string, libraryName string, relativePath string) ([]assetdto.Breadcrumb, error) {
	breadcrumbs := []assetdto.Breadcrumb{{ID: nil, Label: libraryName}}
	if relativePath == "/" {
		return breadcrumbs, nil
	}

	current := "/"
	for _, segment := range strings.Split(strings.Trim(relativePath, "/"), "/") {
		current = joinLogicalPath(current, segment)
		row, err := s.loadDirectoryByPath(ctx, libraryID, current)
		if err != nil {
			return nil, err
		}
		rowID := row.ID
		breadcrumbs = append(breadcrumbs, assetdto.Breadcrumb{ID: &rowID, Label: row.Name})
	}
	return breadcrumbs, nil
}

func (s *Service) loadDirectoryByPath(ctx context.Context, libraryID string, relativePath string) (directoryModel, error) {
	var row directoryModel
	err := s.pool.QueryRow(ctx, `
		SELECT id, library_id, relative_path, name, parent_path, depth, status, created_at, updated_at
		FROM library_directories
		WHERE library_id = $1
		  AND relative_path = $2
		  AND status <> 'DELETED'
	`, libraryID, relativePath).Scan(&row.ID, &row.LibraryID, &row.RelativePath, &row.Name, &row.ParentPath, &row.Depth, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return directoryModel{}, apperrors.NotFound("目录不存在")
		}
		return directoryModel{}, err
	}
	return row, nil
}

func (s *Service) loadDirectoryByID(ctx context.Context, id string) (directoryModel, error) {
	var row directoryModel
	err := s.pool.QueryRow(ctx, `
		SELECT id, library_id, relative_path, name, parent_path, depth, status, created_at, updated_at
		FROM library_directories
		WHERE id = $1
		  AND status <> 'DELETED'
	`, id).Scan(&row.ID, &row.LibraryID, &row.RelativePath, &row.Name, &row.ParentPath, &row.Depth, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return directoryModel{}, apperrors.NotFound("目录不存在")
		}
		return directoryModel{}, err
	}
	return row, nil
}

func (s *Service) loadAssetByID(ctx context.Context, id string) (assetModel, error) {
	var row assetModel
	err := s.pool.QueryRow(ctx, `
		SELECT
			id, library_id, directory_id, relative_path, name, extension, size_bytes,
			file_kind, lifecycle_state, rating, color_label, canonical_modified_at,
			created_at, updated_at
		FROM assets
		WHERE id = $1
		  AND lifecycle_state <> 'DELETED'
	`, id).Scan(
		&row.ID,
		&row.LibraryID,
		&row.DirectoryID,
		&row.RelativePath,
		&row.Name,
		&row.Extension,
		&row.SizeBytes,
		&row.FileKind,
		&row.LifecycleState,
		&row.Rating,
		&row.ColorLabel,
		&row.CanonicalModifiedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return assetModel{}, apperrors.NotFound("文件不存在")
		}
		return assetModel{}, err
	}
	return row, nil
}

func (s *Service) loadDirectDirectories(ctx context.Context, libraryID string, parentPath string) ([]directoryModel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, library_id, relative_path, name, parent_path, depth, status, created_at, updated_at
		FROM library_directories
		WHERE library_id = $1
		  AND parent_path IS NOT DISTINCT FROM $2
		  AND status = 'ACTIVE'
	`, libraryID, parentPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]directoryModel, 0)
	for rows.Next() {
		var row directoryModel
		if err := rows.Scan(&row.ID, &row.LibraryID, &row.RelativePath, &row.Name, &row.ParentPath, &row.Depth, &row.Status, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) loadDirectAssets(ctx context.Context, libraryID string, directoryID string) ([]assetModel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, library_id, directory_id, relative_path, name, extension, size_bytes,
			file_kind, lifecycle_state, rating, color_label, canonical_modified_at,
			created_at, updated_at
		FROM assets
		WHERE library_id = $1
		  AND directory_id = $2
		  AND lifecycle_state <> 'DELETED'
	`, libraryID, directoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]assetModel, 0)
	for rows.Next() {
		var row assetModel
		if err := rows.Scan(&row.ID, &row.LibraryID, &row.DirectoryID, &row.RelativePath, &row.Name, &row.Extension, &row.SizeBytes, &row.FileKind, &row.LifecycleState, &row.Rating, &row.ColorLabel, &row.CanonicalModifiedAt, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) buildDirectoryEntry(ctx context.Context, library libraryModel, directory directoryModel, withMetadata bool) (assetdto.EntryRecord, error) {
	endpoints, err := s.summarizeDirectoryEndpoints(ctx, library.ID, directory.RelativePath)
	if err != nil {
		return assetdto.EntryRecord{}, err
	}
	descendants, err := s.countDirectoryDescendants(ctx, library.ID, directory.RelativePath)
	if err != nil {
		return assetdto.EntryRecord{}, err
	}

	tags, err := s.loadDirectoryTags(ctx, directory.ID)
	if err != nil {
		return assetdto.EntryRecord{}, err
	}

	return assetdto.EntryRecord{
		ID:             directory.ID,
		LibraryID:      library.ID,
		ParentID:       directory.ParentPath,
		Type:           "folder",
		LifecycleState: "ACTIVE",
		Name:           directory.Name,
		FileKind:       "文件夹",
		DisplayType:    "文件夹",
		ModifiedAt:     formatTime(directory.UpdatedAt),
		CreatedAt:      formatTime(directory.CreatedAt),
		Size:           fmt.Sprintf("%d 项", descendants),
		Path:           buildDisplayPath(library.Name, directory.RelativePath),
		SourceLabel:    "统一目录树",
		LastTaskText:   "暂无任务",
		LastTaskTone:   "info",
		Rating:         0,
		ColorLabel:     "无",
		Badges:         []string{},
		RiskTags:       []string{},
		Tags:           tags,
		Endpoints:      endpoints,
	}, nil
}

func (s *Service) buildAssetEntry(ctx context.Context, library libraryModel, asset assetModel, withMetadata bool) (assetdto.EntryRecord, error) {
	endpoints, err := s.loadAssetEndpoints(ctx, asset.ID, asset.LibraryID)
	if err != nil {
		return assetdto.EntryRecord{}, err
	}

	parentID := asset.DirectoryID
	tags, err := s.loadAssetTags(ctx, asset.ID)
	if err != nil {
		return assetdto.EntryRecord{}, err
	}
	lastTaskText, lastTaskTone, err := s.loadLatestTaskStatusForAsset(ctx, asset.ID)
	if err != nil {
		return assetdto.EntryRecord{}, err
	}
	return assetdto.EntryRecord{
		ID:             asset.ID,
		LibraryID:      asset.LibraryID,
		ParentID:       &parentID,
		Type:           "file",
		LifecycleState: mapAssetLifecycle(asset.LifecycleState),
		Name:           asset.Name,
		FileKind:       mapFileKind(asset.FileKind),
		DisplayType:    mapDisplayType(asset.FileKind, asset.Extension),
		ModifiedAt:     formatOptionalTime(asset.CanonicalModifiedAt, asset.UpdatedAt),
		CreatedAt:      formatTime(asset.CreatedAt),
		Size:           formatBytes(asset.SizeBytes),
		Path:           buildDisplayPath(library.Name, asset.RelativePath),
		SourceLabel:    "统一资产",
		LastTaskText:   lastTaskText,
		LastTaskTone:   lastTaskTone,
		Rating:         int(asset.Rating),
		ColorLabel:     mapColorLabel(asset.ColorLabel),
		Badges:         []string{},
		RiskTags:       []string{},
		Tags:           tags,
		Endpoints:      endpoints,
	}, nil
}

func (s *Service) countActiveAssets(ctx context.Context, libraryID string) (int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM assets
		WHERE library_id = $1
		  AND lifecycle_state <> 'DELETED'
	`, libraryID).Scan(&total)
	return total, err
}

func (s *Service) computeLibraryHealth(ctx context.Context, libraryID string) (string, error) {
	totalAssets, err := s.countActiveAssets(ctx, libraryID)
	if err != nil {
		return "", err
	}
	if totalAssets == 0 {
		return "100%", nil
	}

	var availableAssets int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT a.id)
		FROM assets a
		INNER JOIN asset_replicas ar ON ar.asset_id = a.id
		WHERE a.library_id = $1
		  AND a.lifecycle_state <> 'DELETED'
		  AND ar.replica_state = 'AVAILABLE'
	`, libraryID).Scan(&availableAssets)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d%%", int(float64(availableAssets)/float64(totalAssets)*100)), nil
}

func (s *Service) loadLibraryMountSummary(ctx context.Context, libraryID string) ([]string, string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT m.name, sn.node_type
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		WHERE m.library_id = $1
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
		  AND m.enabled = TRUE
		ORDER BY m.name ASC
	`, libraryID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	endpoints := make([]string, 0)
	typeSet := map[string]struct{}{}
	for rows.Next() {
		var name string
		var nodeType string
		if err := rows.Scan(&name, &nodeType); err != nil {
			return nil, "", err
		}
		endpoints = append(endpoints, name)
		typeSet[nodeType] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	policies := make([]string, 0, len(typeSet))
	if _, ok := typeSet["LOCAL"]; ok {
		policies = append(policies, "本地")
	}
	if _, ok := typeSet["NAS"]; ok {
		policies = append(policies, "NAS")
	}
	if _, ok := typeSet["CLOUD"]; ok {
		policies = append(policies, "网盘")
	}
	if len(policies) == 0 {
		policies = append(policies, "未绑定端点")
	}

	return endpoints, strings.Join(policies, " + "), nil
}

func (s *Service) loadLibraryMounts(ctx context.Context, libraryID string) ([]mountModel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.name, sn.node_type
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		WHERE m.library_id = $1
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
		  AND m.enabled = TRUE
		ORDER BY m.name ASC
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]mountModel, 0)
	for rows.Next() {
		var row mountModel
		if err := rows.Scan(&row.ID, &row.Name, &row.NodeType); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) summarizeDirectoryEndpoints(ctx context.Context, libraryID string, relativePath string) ([]assetdto.EntryEndpoint, error) {
	mounts, err := s.loadLibraryMounts(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	prefix := strings.TrimSuffix(relativePath, "/")
	if prefix == "" {
		prefix = "/"
	}
	likePrefix := prefix + "/%"
	if prefix == "/" {
		likePrefix = "/%"
	}

	type aggregate struct {
		total     int
		available int
		lastSeen  *time.Time
	}
	aggregates := map[string]aggregate{}
	rows, err := s.pool.Query(ctx, `
		SELECT
			m.id,
			COUNT(a.id) AS total_assets,
			COUNT(ar.id) FILTER (WHERE ar.replica_state = 'AVAILABLE') AS available_assets,
			MAX(ar.last_seen_at) AS last_seen_at
		FROM mounts m
		LEFT JOIN assets a
			ON a.library_id = m.library_id
		   AND a.lifecycle_state <> 'DELETED'
		   AND a.relative_path LIKE $2
		LEFT JOIN asset_replicas ar
			ON ar.asset_id = a.id
		   AND ar.mount_id = m.id
		WHERE m.library_id = $1
		  AND m.deleted_at IS NULL
		  AND m.enabled = TRUE
		GROUP BY m.id
	`, libraryID, likePrefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mountID string
		var item aggregate
		if err := rows.Scan(&mountID, &item.total, &item.available, &item.lastSeen); err != nil {
			return nil, err
		}
		aggregates[mountID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	results := make([]assetdto.EntryEndpoint, 0, len(mounts))
	for _, mount := range mounts {
		aggregate := aggregates[mount.ID]
		state := "未同步"
		switch {
		case aggregate.total == 0:
			state = "未同步"
		case aggregate.available == 0:
			state = "未同步"
		case aggregate.available == aggregate.total:
			state = "已同步"
		default:
			state = "部分同步"
		}
		results = append(results, assetdto.EntryEndpoint{
			MountID:      mount.ID,
			Name:         mount.Name,
			State:        state,
			Tone:         mapEndpointTone(state),
			LastSyncAt:   formatOptionalTimestamp(aggregate.lastSeen),
			EndpointType: mapEndpointType(mount.NodeType),
		})
	}
	return results, nil
}

func (s *Service) loadAssetEndpoints(ctx context.Context, assetID string, libraryID string) ([]assetdto.EntryEndpoint, error) {
	mounts, err := s.loadLibraryMounts(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	pendingMounts, err := s.loadPendingEndpointOperations(ctx, assetID)
	if err != nil {
		return nil, err
	}

	type replica struct {
		state    string
		lastSeen *time.Time
	}
	replicas := map[string]replica{}
	rows, err := s.pool.Query(ctx, `
		SELECT mount_id, replica_state, last_seen_at
		FROM asset_replicas
		WHERE asset_id = $1
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mountID string
		var state string
		var lastSeen *time.Time
		if err := rows.Scan(&mountID, &state, &lastSeen); err != nil {
			return nil, err
		}
		replicas[mountID] = replica{state: state, lastSeen: lastSeen}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	results := make([]assetdto.EntryEndpoint, 0, len(mounts))
	for _, mount := range mounts {
		replica := replicas[mount.ID]
		state := "未同步"
		if replica.state == "AVAILABLE" {
			state = "已同步"
		}
		if _, ok := pendingMounts[mount.ID]; ok {
			state = "同步中"
		}
		results = append(results, assetdto.EntryEndpoint{
			MountID:      mount.ID,
			Name:         mount.Name,
			State:        state,
			Tone:         mapEndpointTone(state),
			LastSyncAt:   formatOptionalTimestamp(replica.lastSeen),
			EndpointType: mapEndpointType(mount.NodeType),
		})
	}
	return results, nil
}

func (s *Service) countDirectoryDescendants(ctx context.Context, libraryID string, relativePath string) (int, error) {
	prefix := strings.TrimSuffix(relativePath, "/")
	if prefix == "" {
		prefix = "/"
	}
	likePrefix := prefix + "/%"
	if prefix == "/" {
		likePrefix = "/%"
	}
	var total int
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT COUNT(1)
				FROM library_directories
				WHERE library_id = $1
				  AND status = 'ACTIVE'
				  AND relative_path LIKE $2
			), 0) +
			COALESCE((
				SELECT COUNT(1)
				FROM assets
				WHERE library_id = $1
				  AND lifecycle_state <> 'DELETED'
				  AND relative_path LIKE $2
			), 0)
	`, libraryID, likePrefix).Scan(&total)
	return total, err
}

func (s *Service) countDirectoryDirectChildren(ctx context.Context, libraryID string, directoryID string, relativePath string) (int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT COUNT(1)
				FROM library_directories
				WHERE library_id = $1
				  AND parent_path IS NOT DISTINCT FROM $2
				  AND status = 'ACTIVE'
			), 0) +
			COALESCE((
				SELECT COUNT(1)
				FROM assets
				WHERE library_id = $1
				  AND directory_id = $3
				  AND lifecycle_state <> 'DELETED'
			), 0)
	`, libraryID, relativePath, directoryID).Scan(&total)
	return total, err
}

func filterEntries(items []assetdto.EntryRecord, query assetdto.BrowseQuery) []assetdto.EntryRecord {
	filtered := make([]assetdto.EntryRecord, 0, len(items))
	for _, item := range items {
		if query.FileType != "" && query.FileType != "全部" && item.FileKind != query.FileType {
			continue
		}
		if keyword := strings.ToLower(strings.TrimSpace(query.SearchText)); keyword != "" {
			if !strings.Contains(strings.ToLower(item.Name), keyword) &&
				!strings.Contains(strings.ToLower(item.Path), keyword) &&
				!strings.Contains(strings.ToLower(item.SourceLabel), keyword) &&
				!entryContainsTag(item.Tags, keyword) {
				continue
			}
		}
		if !matchesStatus(item, query.StatusFilter, query.PartialSyncEndpointNames) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func entryContainsTag(tags []string, keyword string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), keyword) {
			return true
		}
	}
	return false
}

func matchesStatus(item assetdto.EntryRecord, statusFilter string, partialSyncEndpointNames []string) bool {
	switch statusFilter {
	case "", "全部":
		return true
	case "完全同步":
		return len(item.Endpoints) > 0 && countEndpointsByState(item.Endpoints, "已同步") == len(item.Endpoints)
	case "未同步":
		return len(item.Endpoints) > 0 && countEndpointsByState(item.Endpoints, "未同步") == len(item.Endpoints)
	case "部分同步":
		if len(partialSyncEndpointNames) == 0 {
			return countEndpointsByState(item.Endpoints, "部分同步") > 0
		}
		for _, endpointName := range partialSyncEndpointNames {
			found := false
			for _, endpoint := range item.Endpoints {
				if endpoint.Name == endpointName && endpoint.State == "部分同步" {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func countEndpointsByState(endpoints []assetdto.EntryEndpoint, state string) int {
	count := 0
	for _, endpoint := range endpoints {
		if endpoint.State == state {
			count++
		}
	}
	return count
}

func sortEntries(items []assetdto.EntryRecord, sortValue string, sortDirection string) {
	desc := sortDirection != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch sortValue {
		case "名称":
			if desc {
				return left.Name > right.Name
			}
			return left.Name < right.Name
		case "星级":
			if left.Rating != right.Rating {
				if desc {
					return left.Rating > right.Rating
				}
				return left.Rating < right.Rating
			}
		case "大小":
			if left.Size != right.Size {
				if desc {
					return left.Size > right.Size
				}
				return left.Size < right.Size
			}
		}
		if desc {
			return left.ModifiedAt > right.ModifiedAt
		}
		return left.ModifiedAt < right.ModifiedAt
	})
}

func sortEntriesForBrowse(items []assetdto.EntryRecord, sortValue string, sortDirection string) {
	if sortValue != "星级" {
		sortEntries(items, sortValue, sortDirection)
		return
	}

	desc := sortDirection != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]

		if left.Type != right.Type {
			return left.Type == "folder"
		}
		if left.Type == "folder" {
			return false
		}

		if left.Rating != right.Rating {
			if desc {
				return left.Rating > right.Rating
			}
			return left.Rating < right.Rating
		}
		if desc {
			return left.ModifiedAt > right.ModifiedAt
		}
		return left.ModifiedAt < right.ModifiedAt
	})
}
