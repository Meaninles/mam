package assets

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	assetdto "mare/shared/contracts/dto/asset"
)

func (s *Service) CreateLibrary(ctx context.Context, request assetdto.CreateLibraryRequest) (assetdto.CreateLibraryResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return assetdto.CreateLibraryResponse{}, apperrors.BadRequest("资产库名称不能为空")
	}

	var existingID string
	err := s.pool.QueryRow(ctx, `
		SELECT id
		FROM libraries
		WHERE name = $1
		  AND status <> 'ARCHIVED'
	`, name).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return assetdto.CreateLibraryResponse{}, err
	}
	if err == nil {
		return assetdto.CreateLibraryResponse{}, apperrors.BadRequest("同名资产库已存在")
	}

	now := s.now().UTC()
	libraryID := newID("library")

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return assetdto.CreateLibraryResponse{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO libraries (
			id, code, name, root_label, status, created_at, updated_at
		) VALUES ($1, $2, $3, '/', 'ACTIVE', $4, $4)
	`, libraryID, "library-"+libraryID, name, now)
	if err != nil {
		return assetdto.CreateLibraryResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO library_directories (
			id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
		) VALUES ($1, $2, '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $3, $3)
	`, "dir-root-"+libraryID, libraryID, now)
	if err != nil {
		return assetdto.CreateLibraryResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return assetdto.CreateLibraryResponse{}, err
	}

	return assetdto.CreateLibraryResponse{
		Message: "资产库已创建",
		Library: assetdto.LibraryRecord{
			ID:            libraryID,
			Name:          name,
			RootLabel:     "/",
			ItemCount:     "0",
			Health:        "100%",
			StoragePolicy: "未绑定端点",
			EndpointNames: []string{},
		},
	}, nil
}

func (s *Service) CreateDirectory(ctx context.Context, libraryID string, request assetdto.CreateDirectoryRequest) (assetdto.CreateDirectoryResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return assetdto.CreateDirectoryResponse{}, apperrors.BadRequest("目录名称不能为空")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return assetdto.CreateDirectoryResponse{}, apperrors.BadRequest("目录名称不能包含路径分隔符")
	}

	library, err := s.loadLibrary(ctx, libraryID)
	if err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}

	parentRelativePath := "/"
	if request.ParentID != nil && strings.TrimSpace(*request.ParentID) != "" {
		parent, err := s.loadDirectoryByID(ctx, *request.ParentID)
		if err != nil {
			return assetdto.CreateDirectoryResponse{}, err
		}
		if parent.LibraryID != libraryID {
			return assetdto.CreateDirectoryResponse{}, apperrors.BadRequest("父目录不属于当前资产库")
		}
		parentRelativePath = parent.RelativePath
	}

	relativePath := joinLogicalPath(parentRelativePath, name)
	if _, err := s.loadDirectoryByPath(ctx, libraryID, relativePath); err == nil {
		return assetdto.CreateDirectoryResponse{}, apperrors.BadRequest("同名目录已存在")
	} else if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != "not_found" {
		return assetdto.CreateDirectoryResponse{}, err
	}

	now := s.now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}
	defer tx.Rollback(ctx)

	directoryID, err := s.ensureDirectoryChain(ctx, tx, libraryID, relativePath, now)
	if err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}

	mounts, err := s.loadWritableMounts(ctx, libraryID)
	if err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}
	for _, mount := range mounts {
		physicalPath := resolveMountPhysicalDirectoryPath(mount.SourcePath, mount.RelativeRootPath, relativePath)
		executor, err := s.resolveExecutor(mount.NodeType)
		if err != nil {
			return assetdto.CreateDirectoryResponse{}, apperrors.BadRequest("当前挂载类型暂不支持创建目录")
		}
		if err := executor.EnsureDirectory(ctx, pathExecutionContext{
			PhysicalPath:     physicalPath,
			Username:         mount.Username,
			SecretCiphertext: mount.SecretCiphertext,
		}); err != nil {
			return assetdto.CreateDirectoryResponse{}, apperrors.BadRequest("目录创建失败，请检查挂载目录权限")
		}
		if err := s.upsertDirectoryPresence(ctx, tx, directoryID, mount.ID, physicalPath, now); err != nil {
			return assetdto.CreateDirectoryResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}

	directory, err := s.loadDirectoryByID(ctx, directoryID)
	if err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}
	entry, err := s.buildDirectoryEntry(ctx, library, directory, true)
	if err != nil {
		return assetdto.CreateDirectoryResponse{}, err
	}

	return assetdto.CreateDirectoryResponse{
		Message: "目录已创建",
		Entry:   entry,
	}, nil
}

func (s *Service) DeleteEntry(ctx context.Context, id string) (assetdto.DeleteEntryResponse, error) {
	if directory, err := s.loadDirectoryByID(ctx, id); err == nil {
		return s.deleteDirectory(ctx, directory)
	} else if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != "not_found" {
		return assetdto.DeleteEntryResponse{}, err
	}

	if _, err := s.loadAssetByID(ctx, id); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}

	replicas, err := loadAssetReplicaDeletions(ctx, s.pool, id)
	if err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	for _, replica := range replicas {
		if replica.NodeType == "CLOUD" {
			return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("当前暂不支持删除云端副本文件")
		}
		executor, err := s.resolveExecutor(replica.NodeType)
		if err != nil {
			return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("当前副本类型暂不支持删除")
		}
		if err := executor.DeleteFile(ctx, pathExecutionContext{
			PhysicalPath:     replica.PhysicalPath,
			Username:         replica.Username,
			SecretCiphertext: replica.SecretCiphertext,
		}); err != nil {
			return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("文件删除失败，请确认文件未被占用且具备写权限")
		}
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM asset_metadata WHERE asset_id = $1`, id); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM asset_replicas WHERE asset_id = $1`, id); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, id); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}

	return assetdto.DeleteEntryResponse{Message: "条目已删除"}, nil
}

type assetReplicaDeletion struct {
	PhysicalPath     string
	NodeType         string
	Username         string
	SecretCiphertext string
}

func loadAssetReplicaDeletions(ctx context.Context, pool *pgxpool.Pool, assetID string) ([]assetReplicaDeletion, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			ar.physical_path,
			sn.node_type,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM asset_replicas ar
		INNER JOIN mounts m ON m.id = ar.mount_id
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE ar.asset_id = $1
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]assetReplicaDeletion, 0)
	for rows.Next() {
		var row assetReplicaDeletion
		if err := rows.Scan(&row.PhysicalPath, &row.NodeType, &row.Username, &row.SecretCiphertext); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

type writableMount struct {
	ID               string
	SourcePath       string
	RelativeRootPath string
	NodeType         string
	Username         string
	SecretCiphertext string
}

func (s *Service) loadWritableMounts(ctx context.Context, libraryID string) ([]writableMount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			m.id,
			m.source_path,
			m.relative_root_path,
			sn.node_type,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE m.library_id = $1
		  AND m.deleted_at IS NULL
		  AND m.enabled = TRUE
		  AND m.mount_mode = 'READ_WRITE'
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]writableMount, 0)
	for rows.Next() {
		var row writableMount
		if err := rows.Scan(&row.ID, &row.SourcePath, &row.RelativeRootPath, &row.NodeType, &row.Username, &row.SecretCiphertext); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func resolveMountPhysicalDirectoryPath(sourcePath string, mountRoot string, directoryRelativePath string) string {
	dir := normalizeLogicalPath(directoryRelativePath)
	suffix := strings.Trim(strings.ReplaceAll(dir, "/", string(filepath.Separator)), string(filepath.Separator))
	if suffix == "" {
		return sourcePath
	}
	return filepath.Join(sourcePath, suffix)
}

func (s *Service) deleteDirectory(ctx context.Context, directory directoryModel) (assetdto.DeleteEntryResponse, error) {
	if directory.RelativePath == "/" {
		return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("不能删除资产库根目录")
	}

	var childDirectories int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM library_directories
		WHERE library_id = $1
		  AND parent_path = $2
		  AND status = 'ACTIVE'
	`, directory.LibraryID, directory.RelativePath).Scan(&childDirectories); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	if childDirectories > 0 {
		return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("目录非空，无法删除")
	}

	var childAssets int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM assets
		WHERE library_id = $1
		  AND directory_id = $2
		  AND lifecycle_state <> 'DELETED'
	`, directory.LibraryID, directory.ID).Scan(&childAssets); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	if childAssets > 0 {
		return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("目录非空，无法删除")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			dp.id,
			dp.physical_path,
			sn.node_type,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM directory_presences dp
		INNER JOIN mounts m ON m.id = dp.mount_id
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE dp.directory_id = $1
	`, directory.ID)
	if err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	defer rows.Close()

	var localPhysicalPaths []assetReplicaDeletion
	for rows.Next() {
		var presenceID string
		var row assetReplicaDeletion
		if err := rows.Scan(&presenceID, &row.PhysicalPath, &row.NodeType, &row.Username, &row.SecretCiphertext); err != nil {
			return assetdto.DeleteEntryResponse{}, err
		}
		localPhysicalPaths = append(localPhysicalPaths, row)
	}
	if err := rows.Err(); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}

	for _, physicalPath := range localPhysicalPaths {
		if strings.TrimSpace(physicalPath.PhysicalPath) == "" {
			continue
		}
		executor, err := s.resolveExecutor(physicalPath.NodeType)
		if err != nil {
			return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("当前目录所在挂载类型暂不支持删除")
		}
		if err := executor.DeleteDirectory(ctx, pathExecutionContext{
			PhysicalPath:     physicalPath.PhysicalPath,
			Username:         physicalPath.Username,
			SecretCiphertext: physicalPath.SecretCiphertext,
		}); err != nil {
			return assetdto.DeleteEntryResponse{}, apperrors.BadRequest("目录删除失败，请确认目录为空且具备写权限")
		}
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM directory_presences WHERE directory_id = $1`, directory.ID); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM library_directories WHERE id = $1`, directory.ID); err != nil {
		return assetdto.DeleteEntryResponse{}, err
	}

	return assetdto.DeleteEntryResponse{Message: "条目已删除"}, nil
}
