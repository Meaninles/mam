package assets

import (
	"context"
	"io"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/integration"
	assetdto "mare/shared/contracts/dto/asset"
)

type ReplicatePlan struct {
	LibraryID      string
	LibraryName    string
	EndpointName   string
	TargetMountID  string
	RequestedCount int
	SkippedCount   int
	Items          []ReplicatePlanItem
}

type ReplicatePlanItem struct {
	AssetID              string
	AssetName            string
	RouteType            string
	RelativePath         string
	SizeBytes            int64
	SourceReplicaID      string
	SourceMountID        string
	SourceMountName      string
	SourceStorageNodeID  string
	SourcePhysicalPath   string
	TargetMountID        string
	TargetMountName      string
	TargetStorageNodeID  string
	TargetPhysicalPath   string
}

type DeleteReplicaPlan struct {
	LibraryID      string
	LibraryName    string
	EndpointName   string
	TargetMountID  string
	RequestedCount int
	SkippedCount   int
	Items          []DeleteReplicaPlanItem
}

type DeleteReplicaPlanItem struct {
	AssetID             string
	AssetName           string
	RelativePath        string
	ReplicaID           string
	TargetMountID       string
	TargetMountName     string
	TargetStorageNodeID string
	TargetPhysicalPath  string
}

type DeleteAssetPlan struct {
	LibraryID      string
	LibraryName    string
	RequestedCount int
	SkippedCount   int
	Items          []DeleteAssetPlanItem
}

type DeleteAssetPlanItem struct {
	AssetID      *string
	DirectoryID  *string
	EntryName    string
	RelativePath string
	ReplicaCount int
	Depth        int
}

type operationMount struct {
	ID               string
	LibraryID        string
	LibraryName      string
	StorageNodeID    string
	Name             string
	SourcePath       string
	RelativeRootPath string
	MountMode        string
	NodeType         string
	ProviderVendor   string
	ProviderPayload  integration.CloudProviderPayload
	Username         string
	SecretCiphertext string
}

type operationReplica struct {
	ID               string
	AssetID          string
	MountID          string
	MountName        string
	StorageNodeID    string
	NodeType         string
	ProviderVendor   string
	ProviderPayload  integration.CloudProviderPayload
	SourcePath       string
	RelativeRootPath string
	MountMode        string
	Username         string
	SecretCiphertext string
	PhysicalPath     string
	ReplicaState     string
	SizeBytes        int64
	ModifiedAt       *time.Time
}

type operationAsset struct {
	assetModel
	LibraryName string
}

func (s *Service) PrepareReplicatePlan(ctx context.Context, request assetdto.CreateReplicateJobRequest) (ReplicatePlan, error) {
	entryIDs := normalizeOperationEntryIDs(request.EntryIDs)
	if len(entryIDs) == 0 {
		return ReplicatePlan{}, apperrors.BadRequest("至少需要选择一项条目")
	}
	endpointName := strings.TrimSpace(request.EndpointName)
	if endpointName == "" {
		return ReplicatePlan{}, apperrors.BadRequest("目标端点不能为空")
	}

	selectedAssets, err := s.expandSelectedAssets(ctx, entryIDs)
	if err != nil {
		return ReplicatePlan{}, err
	}
	targetMount, err := s.loadOperationMountByName(ctx, selectedAssets[0].LibraryID, endpointName)
	if err != nil {
		return ReplicatePlan{}, err
	}
	if !isSupportedWritableNodeType(targetMount.NodeType) || targetMount.MountMode != "READ_WRITE" {
		return ReplicatePlan{}, apperrors.BadRequest("当前端点暂不支持同步写入")
	}

	items := make([]ReplicatePlanItem, 0, len(selectedAssets))
	skippedCount := 0
	for _, asset := range selectedAssets {
		replicas, err := s.loadOperationReplicasByAsset(ctx, asset.ID)
		if err != nil {
			return ReplicatePlan{}, err
		}
		if hasAvailableReplicaOnMount(replicas, targetMount.ID) {
			skippedCount++
			continue
		}

		source := chooseSourceReplicaForReplication(replicas, targetMount.ID)
		if source == nil {
			skippedCount++
			continue
		}

		items = append(items, ReplicatePlanItem{
			AssetID:             asset.ID,
			AssetName:           asset.Name,
			RouteType:           determineReplicateRouteType(source.NodeType, targetMount.NodeType),
			RelativePath:        asset.RelativePath,
			SizeBytes:           asset.SizeBytes,
			SourceReplicaID:     source.ID,
			SourceMountID:       source.MountID,
			SourceMountName:     source.MountName,
			SourceStorageNodeID: source.StorageNodeID,
			SourcePhysicalPath:  source.PhysicalPath,
			TargetMountID:       targetMount.ID,
			TargetMountName:     targetMount.Name,
			TargetStorageNodeID: targetMount.StorageNodeID,
			TargetPhysicalPath:  resolveUploadPhysicalFilePath(targetMount.SourcePath, asset.RelativePath),
		})
	}

	if len(items) == 0 {
		return ReplicatePlan{}, apperrors.BadRequest("所选条目没有可同步的内容")
	}

	return ReplicatePlan{
		LibraryID:      selectedAssets[0].LibraryID,
		LibraryName:    selectedAssets[0].LibraryName,
		EndpointName:   endpointName,
		TargetMountID:  targetMount.ID,
		RequestedCount: len(selectedAssets),
		SkippedCount:   skippedCount,
		Items:          items,
	}, nil
}

func (s *Service) PrepareDeleteReplicaPlan(ctx context.Context, request assetdto.CreateDeleteReplicaJobRequest) (DeleteReplicaPlan, error) {
	entryIDs := normalizeOperationEntryIDs(request.EntryIDs)
	if len(entryIDs) == 0 {
		return DeleteReplicaPlan{}, apperrors.BadRequest("至少需要选择一项条目")
	}
	endpointName := strings.TrimSpace(request.EndpointName)
	if endpointName == "" {
		return DeleteReplicaPlan{}, apperrors.BadRequest("目标端点不能为空")
	}

	selectedAssets, err := s.expandSelectedAssets(ctx, entryIDs)
	if err != nil {
		return DeleteReplicaPlan{}, err
	}
	targetMount, err := s.loadOperationMountByName(ctx, selectedAssets[0].LibraryID, endpointName)
	if err != nil {
		return DeleteReplicaPlan{}, err
	}
	if !isSupportedWritableNodeType(targetMount.NodeType) || targetMount.MountMode != "READ_WRITE" {
		return DeleteReplicaPlan{}, apperrors.BadRequest("当前端点暂不支持删除副本")
	}

	items := make([]DeleteReplicaPlanItem, 0, len(selectedAssets))
	skippedCount := 0
	for _, asset := range selectedAssets {
		replicas, err := s.loadOperationReplicasByAsset(ctx, asset.ID)
		if err != nil {
			return DeleteReplicaPlan{}, err
		}
		replica := findReplicaOnMount(replicas, targetMount.ID)
		if replica == nil {
			skippedCount++
			continue
		}
		items = append(items, DeleteReplicaPlanItem{
			AssetID:             asset.ID,
			AssetName:           asset.Name,
			RelativePath:        asset.RelativePath,
			ReplicaID:           replica.ID,
			TargetMountID:       targetMount.ID,
			TargetMountName:     targetMount.Name,
			TargetStorageNodeID: targetMount.StorageNodeID,
			TargetPhysicalPath:  replica.PhysicalPath,
		})
	}

	if len(items) == 0 {
		return DeleteReplicaPlan{}, apperrors.BadRequest("所选条目在目标端点上没有可删除的副本")
	}

	return DeleteReplicaPlan{
		LibraryID:      selectedAssets[0].LibraryID,
		LibraryName:    selectedAssets[0].LibraryName,
		EndpointName:   endpointName,
		TargetMountID:  targetMount.ID,
		RequestedCount: len(selectedAssets),
		SkippedCount:   skippedCount,
		Items:          items,
	}, nil
}

func (s *Service) PrepareDeleteAssetPlan(ctx context.Context, request assetdto.CreateDeleteAssetJobRequest) (DeleteAssetPlan, error) {
	entryIDs := normalizeOperationEntryIDs(request.EntryIDs)
	if len(entryIDs) == 0 {
		return DeleteAssetPlan{}, apperrors.BadRequest("至少需要选择一项条目")
	}

	selectedAssets, err := s.expandSelectedAssets(ctx, entryIDs)
	if err != nil {
		return DeleteAssetPlan{}, err
	}
	selectedDirectories, err := s.expandSelectedDirectories(ctx, entryIDs)
	if err != nil {
		return DeleteAssetPlan{}, err
	}

	items := make([]DeleteAssetPlanItem, 0, len(selectedAssets)+len(selectedDirectories))
	skippedCount := 0
	for _, asset := range selectedAssets {
		replicas, err := s.loadOperationReplicasByAsset(ctx, asset.ID)
		if err != nil {
			return DeleteAssetPlan{}, err
		}
		if containsUnsupportedReplica(replicas) {
			skippedCount++
			continue
		}
		assetID := asset.ID
		items = append(items, DeleteAssetPlanItem{
			AssetID:      &assetID,
			EntryName:    asset.Name,
			RelativePath: asset.RelativePath,
			ReplicaCount: len(replicas),
			Depth:        0,
		})
	}

	for _, directory := range selectedDirectories {
		directoryID := directory.ID
		items = append(items, DeleteAssetPlanItem{
			DirectoryID:  &directoryID,
			EntryName:    directory.Name,
			RelativePath: directory.RelativePath,
			ReplicaCount: 0,
			Depth:        directory.Depth,
		})
	}

	if len(items) == 0 {
		return DeleteAssetPlan{}, apperrors.BadRequest("所选条目暂不支持删除")
	}

	return DeleteAssetPlan{
		LibraryID:      selectedAssets[0].LibraryID,
		LibraryName:    selectedAssets[0].LibraryName,
		RequestedCount: len(selectedAssets),
		SkippedCount:   skippedCount,
		Items:          items,
	}, nil
}

func (s *Service) ExecuteReplicaSync(ctx context.Context, sourceReplicaID string, targetMountID string) error {
	sourceReplica, err := s.loadOperationReplicaByID(ctx, sourceReplicaID)
	if err != nil {
		return err
	}
	targetMount, err := s.loadOperationMountByID(ctx, targetMountID)
	if err != nil {
		return err
	}
	if !isSupportedWritableNodeType(sourceReplica.NodeType) || !isSupportedWritableNodeType(targetMount.NodeType) {
		return apperrors.BadRequest("当前副本链路暂不支持同步")
	}

	asset, err := s.loadAssetByID(ctx, sourceReplica.AssetID)
	if err != nil {
		return err
	}
	library, err := s.loadLibrary(ctx, asset.LibraryID)
	if err != nil {
		return err
	}

	sourceExecutor, err := s.resolveExecutor(sourceReplica.NodeType)
	if err != nil {
		return err
	}
	targetExecutor, err := s.resolveExecutor(targetMount.NodeType)
	if err != nil {
		return err
	}
	sourceMetadata, err := sourceExecutor.StatFile(ctx, pathExecutionContext{
		PhysicalPath:     sourceReplica.PhysicalPath,
		Username:         sourceReplica.Username,
		SecretCiphertext: sourceReplica.SecretCiphertext,
	})
	if err != nil {
		return err
	}

	targetPath := resolveUploadPhysicalFilePath(targetMount.SourcePath, asset.RelativePath)
	if err := sourceExecutor.StreamFile(ctx, pathExecutionContext{
		PhysicalPath:     sourceReplica.PhysicalPath,
		Username:         sourceReplica.Username,
		SecretCiphertext: sourceReplica.SecretCiphertext,
	}, func(reader io.Reader) error {
		return targetExecutor.WriteStream(ctx, pathExecutionContext{
			PhysicalPath:     targetPath,
			Username:         targetMount.Username,
			SecretCiphertext: targetMount.SecretCiphertext,
		}, reader)
	}); err != nil {
		return err
	}
	if err := targetExecutor.SetFileModifiedTime(ctx, pathExecutionContext{
		PhysicalPath:     targetPath,
		Username:         targetMount.Username,
		SecretCiphertext: targetMount.SecretCiphertext,
	}, sourceMetadata.ModifiedAt); err != nil {
		return err
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.upsertLibrary(ctx, tx, library.ID, library.Name, now); err != nil {
		return err
	}
	if _, err := s.ensureDirectoryChain(ctx, tx, asset.LibraryID, parentLogicalPath(asset.RelativePath), now); err != nil {
		return err
	}
	if err := s.ensureDirectoryPresenceOnMount(ctx, tx, uploadMount{
		writableMount: writableMount{
			ID:               targetMount.ID,
			SourcePath:       targetMount.SourcePath,
			RelativeRootPath: targetMount.RelativeRootPath,
			NodeType:         targetMount.NodeType,
			Username:         targetMount.Username,
			SecretCiphertext: targetMount.SecretCiphertext,
		},
		executor: targetExecutor,
	}, asset.LibraryID, parentLogicalPath(asset.RelativePath), now); err != nil {
		return err
	}

	if err := s.upsertReplica(ctx, tx, asset.ID, targetMount.ID, targetPath, uploadFileInfo{
		name:    filepath.Base(asset.RelativePath),
		size:    sourceMetadata.SizeBytes,
		modTime: sourceMetadata.ModifiedAt,
	}, now); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) ExecuteReplicaDeletion(ctx context.Context, replicaID string) error {
	replica, err := s.loadOperationReplicaByID(ctx, replicaID)
	if err != nil {
		return err
	}
	if !isSupportedWritableNodeType(replica.NodeType) {
		return apperrors.BadRequest("当前副本类型暂不支持删除")
	}

	executor, err := s.resolveExecutor(replica.NodeType)
	if err != nil {
		return err
	}
	if err := executor.DeleteFile(ctx, pathExecutionContext{
		PhysicalPath:     replica.PhysicalPath,
		Username:         replica.Username,
		SecretCiphertext: replica.SecretCiphertext,
	}); err != nil {
		return apperrors.BadRequest("副本删除失败，请确认文件未被占用且具备写权限")
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM asset_replicas WHERE id = $1`, replica.ID); err != nil {
		return err
	}

	var remainingReplicas int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM asset_replicas
		WHERE asset_id = $1
	`, replica.AssetID).Scan(&remainingReplicas); err != nil {
		return err
	}
	if remainingReplicas == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM asset_metadata WHERE asset_id = $1`, replica.AssetID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM assets WHERE id = $1`, replica.AssetID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE assets SET updated_at = $2 WHERE id = $1`, replica.AssetID, now); err != nil && err != pgx.ErrNoRows {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) ExecuteAssetDeletion(ctx context.Context, assetID string) error {
	replicas, err := s.loadOperationReplicasByAsset(ctx, assetID)
	if err != nil {
		return err
	}
	for _, replica := range replicas {
		if !isSupportedWritableNodeType(replica.NodeType) {
			return apperrors.BadRequest("当前资产包含暂不支持删除的云端副本")
		}
		executor, err := s.resolveExecutor(replica.NodeType)
		if err != nil {
			return err
		}
		if err := executor.DeleteFile(ctx, pathExecutionContext{
			PhysicalPath:     replica.PhysicalPath,
			Username:         replica.Username,
			SecretCiphertext: replica.SecretCiphertext,
		}); err != nil {
			return apperrors.BadRequest("文件删除失败，请确认文件未被占用且具备写权限")
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM asset_metadata WHERE asset_id = $1`, assetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM asset_replicas WHERE asset_id = $1`, assetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM assets WHERE id = $1`, assetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ExecuteDirectoryDeletion(ctx context.Context, directoryID string) error {
	directory, err := s.loadDirectoryByID(ctx, directoryID)
	if err != nil {
		return err
	}
	_, err = s.deleteDirectory(ctx, directory)
	return err
}

func (s *Service) expandSelectedAssets(ctx context.Context, entryIDs []string) ([]operationAsset, error) {
	assetsByID := make(map[string]operationAsset)
	var libraryID string
	for _, id := range entryIDs {
		asset, err := s.loadAssetByID(ctx, id)
		if err == nil {
			libraryName, err := s.loadLibraryName(ctx, asset.LibraryID)
			if err != nil {
				return nil, err
			}
			if libraryID == "" {
				libraryID = asset.LibraryID
			} else if libraryID != asset.LibraryID {
				return nil, apperrors.BadRequest("不能跨资产库执行批量操作")
			}
			assetsByID[asset.ID] = operationAsset{assetModel: asset, LibraryName: libraryName}
			continue
		}
		appErr, ok := err.(*apperrors.AppError)
		if !ok || appErr.Code != "not_found" {
			return nil, err
		}

		directory, err := s.loadDirectoryByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if libraryID == "" {
			libraryID = directory.LibraryID
		} else if libraryID != directory.LibraryID {
			return nil, apperrors.BadRequest("不能跨资产库执行批量操作")
		}

		libraryName, err := s.loadLibraryName(ctx, directory.LibraryID)
		if err != nil {
			return nil, err
		}
		descendants, err := s.loadAssetsUnderDirectory(ctx, directory)
		if err != nil {
			return nil, err
		}
		for _, asset := range descendants {
			assetsByID[asset.ID] = operationAsset{assetModel: asset, LibraryName: libraryName}
		}
	}

	items := make([]operationAsset, 0, len(assetsByID))
	for _, item := range assetsByID {
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, apperrors.BadRequest("所选条目没有可执行的文件资产")
	}
	return items, nil
}

func (s *Service) expandSelectedDirectories(ctx context.Context, entryIDs []string) ([]directoryModel, error) {
	directoriesByID := make(map[string]directoryModel)
	var libraryID string
	for _, id := range entryIDs {
		directory, err := s.loadDirectoryByID(ctx, id)
		if err != nil {
			appErr, ok := err.(*apperrors.AppError)
			if ok && appErr.Code == "not_found" {
				continue
			}
			return nil, err
		}
		if directory.RelativePath == "/" {
			continue
		}
		if libraryID == "" {
			libraryID = directory.LibraryID
		} else if libraryID != directory.LibraryID {
			return nil, apperrors.BadRequest("不能跨资产库执行批量操作")
		}
		directoriesByID[directory.ID] = directory

		descendants, err := s.loadDescendantDirectories(ctx, directory)
		if err != nil {
			return nil, err
		}
		for _, descendant := range descendants {
			if descendant.RelativePath == "/" {
				continue
			}
			directoriesByID[descendant.ID] = descendant
		}
	}

	items := make([]directoryModel, 0, len(directoriesByID))
	for _, item := range directoriesByID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Depth != items[j].Depth {
			return items[i].Depth > items[j].Depth
		}
		return items[i].RelativePath > items[j].RelativePath
	})
	return items, nil
}

func (s *Service) loadDescendantDirectories(ctx context.Context, directory directoryModel) ([]directoryModel, error) {
	prefix := strings.TrimSuffix(directory.RelativePath, "/")
	if prefix == "" {
		prefix = "/"
	}
	likePrefix := prefix + "/%"
	if prefix == "/" {
		likePrefix = "/%"
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, library_id, relative_path, name, parent_path, depth, status, created_at, updated_at
		FROM library_directories
		WHERE library_id = $1
		  AND status = 'ACTIVE'
		  AND relative_path LIKE $2
		ORDER BY depth DESC, relative_path DESC
	`, directory.LibraryID, likePrefix)
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

func (s *Service) loadAssetsUnderDirectory(ctx context.Context, directory directoryModel) ([]assetModel, error) {
	prefix := strings.TrimSuffix(directory.RelativePath, "/")
	if prefix == "" {
		prefix = "/"
	}
	likePrefix := prefix + "/%"
	if prefix == "/" {
		likePrefix = "/%"
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			id, library_id, directory_id, relative_path, name, extension, size_bytes,
			file_kind, lifecycle_state, rating, color_label, canonical_modified_at,
			created_at, updated_at
		FROM assets
		WHERE library_id = $1
		  AND lifecycle_state <> 'DELETED'
		  AND relative_path LIKE $2
	`, directory.LibraryID, likePrefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]assetModel, 0)
	for rows.Next() {
		var row assetModel
		if err := rows.Scan(
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
		); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) loadLibraryName(ctx context.Context, libraryID string) (string, error) {
	var name string
	if err := s.pool.QueryRow(ctx, `
		SELECT name
		FROM libraries
		WHERE id = $1
		  AND status <> 'ARCHIVED'
	`, libraryID).Scan(&name); err != nil {
		if err == pgx.ErrNoRows {
			return "", apperrors.NotFound("未找到指定资产库")
		}
		return "", err
	}
	return name, nil
}

func (s *Service) loadOperationMountByName(ctx context.Context, libraryID string, endpointName string) (operationMount, error) {
	var row operationMount
	var providerPayload []byte
	err := s.pool.QueryRow(ctx, `
		SELECT
			m.id,
			m.library_id,
			m.library_name,
			m.storage_node_id,
			m.name,
			m.source_path,
			m.relative_root_path,
			m.mount_mode,
			sn.node_type,
			COALESCE(cp.provider_vendor, COALESCE(sn.vendor, '')),
			COALESCE(cp.provider_payload, '{}'::jsonb),
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN cloud_node_profiles cp ON cp.storage_node_id = sn.id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE m.library_id = $1
		  AND m.name = $2
		  AND m.deleted_at IS NULL
		  AND m.enabled = TRUE
		  AND sn.deleted_at IS NULL
	`, libraryID, endpointName).Scan(
		&row.ID,
		&row.LibraryID,
		&row.LibraryName,
		&row.StorageNodeID,
		&row.Name,
		&row.SourcePath,
		&row.RelativeRootPath,
		&row.MountMode,
		&row.NodeType,
		&row.ProviderVendor,
		&providerPayload,
		&row.Username,
		&row.SecretCiphertext,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return operationMount{}, apperrors.NotFound("未找到指定端点")
		}
		return operationMount{}, err
	}
	if err := json.Unmarshal(providerPayload, &row.ProviderPayload); err != nil {
		return operationMount{}, err
	}
	return row, nil
}

func (s *Service) loadOperationMountByID(ctx context.Context, mountID string) (operationMount, error) {
	var row operationMount
	var providerPayload []byte
	err := s.pool.QueryRow(ctx, `
		SELECT
			m.id,
			m.library_id,
			m.library_name,
			m.storage_node_id,
			m.name,
			m.source_path,
			m.relative_root_path,
			m.mount_mode,
			sn.node_type,
			COALESCE(cp.provider_vendor, COALESCE(sn.vendor, '')),
			COALESCE(cp.provider_payload, '{}'::jsonb),
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN cloud_node_profiles cp ON cp.storage_node_id = sn.id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE m.id = $1
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
	`, mountID).Scan(
		&row.ID,
		&row.LibraryID,
		&row.LibraryName,
		&row.StorageNodeID,
		&row.Name,
		&row.SourcePath,
		&row.RelativeRootPath,
		&row.MountMode,
		&row.NodeType,
		&row.ProviderVendor,
		&providerPayload,
		&row.Username,
		&row.SecretCiphertext,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return operationMount{}, apperrors.NotFound("未找到指定挂载")
		}
		return operationMount{}, err
	}
	if err := json.Unmarshal(providerPayload, &row.ProviderPayload); err != nil {
		return operationMount{}, err
	}
	return row, nil
}

func (s *Service) loadOperationReplicasByAsset(ctx context.Context, assetID string) ([]operationReplica, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			ar.id,
			ar.asset_id,
			ar.mount_id,
			m.name,
			m.storage_node_id,
			sn.node_type,
			COALESCE(cp.provider_vendor, COALESCE(sn.vendor, '')),
			COALESCE(cp.provider_payload, '{}'::jsonb),
			m.source_path,
			m.relative_root_path,
			m.mount_mode,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, ''),
			ar.physical_path,
			ar.replica_state,
			ar.size_bytes,
			ar.modified_at
		FROM asset_replicas ar
		INNER JOIN mounts m ON m.id = ar.mount_id
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN cloud_node_profiles cp ON cp.storage_node_id = sn.id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE ar.asset_id = $1
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]operationReplica, 0)
	for rows.Next() {
		var row operationReplica
		var providerPayload []byte
		if err := rows.Scan(
			&row.ID,
			&row.AssetID,
			&row.MountID,
			&row.MountName,
			&row.StorageNodeID,
			&row.NodeType,
			&row.ProviderVendor,
			&providerPayload,
			&row.SourcePath,
			&row.RelativeRootPath,
			&row.MountMode,
			&row.Username,
			&row.SecretCiphertext,
			&row.PhysicalPath,
			&row.ReplicaState,
			&row.SizeBytes,
			&row.ModifiedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(providerPayload, &row.ProviderPayload); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) loadOperationReplicaByID(ctx context.Context, replicaID string) (operationReplica, error) {
	var row operationReplica
	var providerPayload []byte
	err := s.pool.QueryRow(ctx, `
		SELECT
			ar.id,
			ar.asset_id,
			ar.mount_id,
			m.name,
			m.storage_node_id,
			sn.node_type,
			COALESCE(cp.provider_vendor, COALESCE(sn.vendor, '')),
			COALESCE(cp.provider_payload, '{}'::jsonb),
			m.source_path,
			m.relative_root_path,
			m.mount_mode,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, ''),
			ar.physical_path,
			ar.replica_state,
			ar.size_bytes,
			ar.modified_at
		FROM asset_replicas ar
		INNER JOIN mounts m ON m.id = ar.mount_id
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN cloud_node_profiles cp ON cp.storage_node_id = sn.id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE ar.id = $1
	`, replicaID).Scan(
		&row.ID,
		&row.AssetID,
		&row.MountID,
		&row.MountName,
		&row.StorageNodeID,
		&row.NodeType,
		&row.ProviderVendor,
		&providerPayload,
		&row.SourcePath,
		&row.RelativeRootPath,
		&row.MountMode,
		&row.Username,
		&row.SecretCiphertext,
		&row.PhysicalPath,
		&row.ReplicaState,
		&row.SizeBytes,
		&row.ModifiedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return operationReplica{}, apperrors.NotFound("未找到指定副本")
		}
		return operationReplica{}, err
	}
	if err := json.Unmarshal(providerPayload, &row.ProviderPayload); err != nil {
		return operationReplica{}, err
	}
	return row, nil
}

func normalizeOperationEntryIDs(ids []string) []string {
	results := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		results = append(results, trimmed)
	}
	return results
}

func determineReplicateRouteType(sourceNodeType string, targetNodeType string) string {
	switch {
	case targetNodeType == "CLOUD":
		return "UPLOAD"
	case sourceNodeType == "CLOUD":
		return "DOWNLOAD"
	default:
		return "COPY"
	}
}

func isSupportedWritableNodeType(nodeType string) bool {
	return nodeType == "LOCAL" || nodeType == "NAS" || nodeType == "CLOUD"
}

func hasAvailableReplicaOnMount(replicas []operationReplica, mountID string) bool {
	for _, replica := range replicas {
		if replica.MountID == mountID && replica.ReplicaState == "AVAILABLE" {
			return true
		}
	}
	return false
}

func chooseSourceReplicaForReplication(replicas []operationReplica, targetMountID string) *operationReplica {
	var best *operationReplica
	bestWeight := 100
	for index := range replicas {
		replica := &replicas[index]
		if replica.MountID == targetMountID || replica.ReplicaState != "AVAILABLE" || !isSupportedWritableNodeType(replica.NodeType) {
			continue
		}
		weight := 2
		switch replica.NodeType {
		case "LOCAL":
			weight = 0
		case "NAS":
			weight = 1
		case "CLOUD":
			weight = 2
		}
		if best == nil || weight < bestWeight {
			best = replica
			bestWeight = weight
		}
	}
	return best
}

func findReplicaOnMount(replicas []operationReplica, mountID string) *operationReplica {
	for index := range replicas {
		replica := &replicas[index]
		if replica.MountID == mountID && replica.ReplicaState != "DELETED" {
			return replica
		}
	}
	return nil
}

func containsUnsupportedReplica(replicas []operationReplica) bool {
	for _, replica := range replicas {
		if !isSupportedWritableNodeType(replica.NodeType) {
			return true
		}
	}
	return false
}
