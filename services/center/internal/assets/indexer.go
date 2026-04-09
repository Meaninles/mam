package assets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
)

type mountConfig struct {
	ID               string
	Name             string
	LibraryID        string
	LibraryName      string
	SourcePath       string
	RelativeRootPath string
	NodeType         string
}

func (s *Service) SyncMount(ctx context.Context, mountID string) error {
	mount, err := s.loadMountConfig(ctx, mountID)
	if err != nil {
		return err
	}
	if mount.NodeType != "LOCAL" && mount.NodeType != "NAS" {
		return apperrors.BadRequest("当前挂载类型暂不支持写入资产索引")
	}
	if _, err := os.Stat(mount.SourcePath); err != nil {
		return apperrors.BadRequest("挂载目录不可读取，无法写入资产索引")
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.upsertLibrary(ctx, tx, mount.LibraryID, mount.LibraryName, now); err != nil {
		return err
	}
	if _, err := s.ensureDirectoryChain(ctx, tx, mount.LibraryID, "/", now); err != nil {
		return err
	}

	logicalRoot := "/"
	if _, err := s.ensureDirectoryChain(ctx, tx, mount.LibraryID, logicalRoot, now); err != nil {
		return err
	}

	seenReplicas := map[string]struct{}{}
	seenDirectories := map[string]struct{}{}

	if err := filepath.WalkDir(mount.SourcePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(mount.SourcePath, path)
		if err != nil {
			return err
		}
		logicalPath := buildLogicalPath(logicalRoot, relativePath)

		if entry.IsDir() {
			directoryID, err := s.ensureDirectoryChain(ctx, tx, mount.LibraryID, logicalPath, now)
			if err != nil {
				return err
			}
			if err := s.upsertDirectoryPresence(ctx, tx, directoryID, mount.ID, path, now); err != nil {
				return err
			}
			seenDirectories[path] = struct{}{}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		parentPath := parentLogicalPath(logicalPath)
		directoryID, err := s.ensureDirectoryChain(ctx, tx, mount.LibraryID, parentPath, now)
		if err != nil {
			return err
		}
		assetID, err := s.upsertAsset(ctx, tx, mount.LibraryID, directoryID, logicalPath, info, now)
		if err != nil {
			return err
		}
		if err := s.upsertReplica(ctx, tx, assetID, mount.ID, path, info, now); err != nil {
			return err
		}
		seenReplicas[path] = struct{}{}
		return nil
	}); err != nil {
		return err
	}

	if err := s.markMissingReplicas(ctx, tx, mount.ID, seenReplicas, now); err != nil {
		return err
	}
	if err := s.markMissingDirectoryPresences(ctx, tx, mount.ID, seenDirectories, now); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) loadMountConfig(ctx context.Context, mountID string) (mountConfig, error) {
	var row mountConfig
	err := s.pool.QueryRow(ctx, `
		SELECT
			m.id,
			m.name,
			m.library_id,
			m.library_name,
			m.source_path,
			m.relative_root_path,
			sn.node_type
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		WHERE m.id = $1
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
	`, mountID).Scan(&row.ID, &row.Name, &row.LibraryID, &row.LibraryName, &row.SourcePath, &row.RelativeRootPath, &row.NodeType)
	if err != nil {
		if err == pgx.ErrNoRows {
			return mountConfig{}, apperrors.NotFound("挂载不存在")
		}
		return mountConfig{}, err
	}
	return row, nil
}

func (s *Service) upsertLibrary(ctx context.Context, tx pgx.Tx, libraryID string, libraryName string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at)
		VALUES ($1, $2, $3, '/', 'ACTIVE', $4, $4)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
		    updated_at = EXCLUDED.updated_at
	`, libraryID, "library-"+libraryID, libraryName, now)
	return err
}

func (s *Service) ensureDirectoryChain(ctx context.Context, tx pgx.Tx, libraryID string, relativePath string, now time.Time) (string, error) {
	normalized := normalizeLogicalPath(relativePath)
	if normalized == "/" {
		rootID := "dir-root-" + libraryID
		_, err := tx.Exec(ctx, `
			INSERT INTO library_directories (
				id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
			) VALUES ($1, $2, '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $3, $3)
			ON CONFLICT (library_id, relative_path) DO UPDATE
			SET updated_at = EXCLUDED.updated_at
		`, rootID, libraryID, now)
		return rootID, err
	}

	if _, err := s.ensureDirectoryChain(ctx, tx, libraryID, "/", now); err != nil {
		return "", err
	}

	current := "/"
	currentID := "dir-root-" + libraryID
	for index, segment := range strings.Split(strings.Trim(normalized, "/"), "/") {
		current = joinLogicalPath(current, segment)
		var existingID string
		err := tx.QueryRow(ctx, `
			SELECT id
			FROM library_directories
			WHERE library_id = $1
			  AND relative_path = $2
		`, libraryID, current).Scan(&existingID)
		if err == nil {
			currentID = existingID
			continue
		}
		if err != pgx.ErrNoRows {
			return "", err
		}

		currentID = newID("dir")
		parentPath := parentLogicalPath(current)
		_, err = tx.Exec(ctx, `
			INSERT INTO library_directories (
				id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'SCANNED', 'ACTIVE', $7, $8, $8)
		`, currentID, libraryID, current, segment, parentPath, index+1, strings.ToLower(segment), now)
		if err != nil {
			return "", err
		}
	}
	return currentID, nil
}

func (s *Service) upsertDirectoryPresence(ctx context.Context, tx pgx.Tx, directoryID string, mountID string, physicalPath string, now time.Time) error {
	var existingID string
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM directory_presences
		WHERE directory_id = $1
		  AND mount_id = $2
	`, directoryID, mountID).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows {
		_, err = tx.Exec(ctx, `
			INSERT INTO directory_presences (
				id, directory_id, mount_id, physical_path, presence_state, last_seen_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, 'PRESENT', $5, $5, $5)
		`, newID("dir-presence"), directoryID, mountID, physicalPath, now)
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE directory_presences
		SET physical_path = $2,
		    presence_state = 'PRESENT',
		    last_seen_at = $3,
		    missing_detected_at = NULL,
		    updated_at = $3
		WHERE id = $1
	`, existingID, physicalPath, now)
	return err
}

func (s *Service) upsertAsset(ctx context.Context, tx pgx.Tx, libraryID string, directoryID string, relativePath string, info os.FileInfo, now time.Time) (string, error) {
	name := filepath.Base(info.Name())
	extension := normalizeExtension(name)
	var existingID string
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM assets
		WHERE library_id = $1
		  AND relative_path = $2
	`, libraryID, relativePath).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return "", err
	}

	if err == pgx.ErrNoRows {
		assetID := newID("asset")
		_, err = tx.Exec(ctx, `
			INSERT INTO assets (
				id, library_id, directory_id, relative_path, name, extension, size_bytes, mime_type,
				file_kind, lifecycle_state, rating, color_label, note, canonical_modified_at,
				content_changed_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9, 'ACTIVE', 0, 'NONE', NULL, $10, $10, $11, $11)
		`, assetID, libraryID, directoryID, relativePath, name, extension, info.Size(), detectMimeType(extension), mapDetectedFileKind(extension), info.ModTime().UTC(), now)
		return assetID, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE assets
		SET directory_id = $2,
		    name = $3,
		    extension = NULLIF($4, ''),
		    size_bytes = $5,
		    mime_type = $6,
		    file_kind = $7,
		    lifecycle_state = 'ACTIVE',
		    canonical_modified_at = $8,
		    content_changed_at = $8,
		    updated_at = $9
		WHERE id = $1
	`, existingID, directoryID, name, extension, info.Size(), detectMimeType(extension), mapDetectedFileKind(extension), info.ModTime().UTC(), now)
	return existingID, err
}

func (s *Service) upsertReplica(ctx context.Context, tx pgx.Tx, assetID string, mountID string, physicalPath string, info os.FileInfo, now time.Time) error {
	var existingID string
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM asset_replicas
		WHERE asset_id = $1
		  AND mount_id = $2
	`, assetID, mountID).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows {
		_, err = tx.Exec(ctx, `
			INSERT INTO asset_replicas (
				id, asset_id, mount_id, physical_path, size_bytes, modified_at, replica_state, sync_state,
				verification_state, last_seen_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'AVAILABLE', 'IN_SYNC', 'UNVERIFIED', $7, $7, $7)
		`, newID("replica"), assetID, mountID, physicalPath, info.Size(), info.ModTime().UTC(), now)
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE asset_replicas
		SET physical_path = $2,
		    size_bytes = $3,
		    modified_at = $4,
		    replica_state = 'AVAILABLE',
		    sync_state = 'IN_SYNC',
		    last_seen_at = $5,
		    missing_detected_at = NULL,
		    last_error_code = NULL,
		    last_error_message = NULL,
		    updated_at = $5
		WHERE id = $1
	`, existingID, physicalPath, info.Size(), info.ModTime().UTC(), now)
	return err
}

func (s *Service) markMissingReplicas(ctx context.Context, tx pgx.Tx, mountID string, seenPaths map[string]struct{}, now time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT id, physical_path
		FROM asset_replicas
		WHERE mount_id = $1
		  AND replica_state <> 'DELETED'
	`, mountID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var physicalPath string
		if err := rows.Scan(&id, &physicalPath); err != nil {
			return err
		}
		if _, ok := seenPaths[physicalPath]; ok {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE asset_replicas
			SET replica_state = 'MISSING',
			    sync_state = 'OUT_OF_SYNC',
			    missing_detected_at = $2,
			    updated_at = $2
			WHERE id = $1
		`, id, now); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) markMissingDirectoryPresences(ctx context.Context, tx pgx.Tx, mountID string, seenPaths map[string]struct{}, now time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT id, physical_path
		FROM directory_presences
		WHERE mount_id = $1
	`, mountID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var physicalPath string
		if err := rows.Scan(&id, &physicalPath); err != nil {
			return err
		}
		if _, ok := seenPaths[physicalPath]; ok {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE directory_presences
			SET presence_state = 'MISSING',
			    missing_detected_at = $2,
			    updated_at = $2
			WHERE id = $1
		`, id, now); err != nil {
			return err
		}
	}
	return rows.Err()
}
