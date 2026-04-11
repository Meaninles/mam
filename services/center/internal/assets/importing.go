package assets

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
)

type ImportTarget struct {
	MountID          string
	LibraryID        string
	LibraryName      string
	StorageNodeID    string
	Label            string
	NodeType         string
	MountMode        string
	SourcePath       string
	RelativeRootPath string
	Username         string
	SecretCiphertext string
	AvailableBytes   *int64
}

type ImportedReplicaInput struct {
	LibraryID     string
	MountID       string
	LogicalPath   string
	PhysicalPath  string
	SizeBytes     int64
	ModifiedAt    time.Time
	FileKind      string
}

func (s *Service) ListImportTargets(ctx context.Context, libraryID string) ([]ImportTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			m.id,
			m.library_id,
			m.library_name,
			m.storage_node_id,
			m.name,
			sn.node_type,
			m.mount_mode,
			m.source_path,
			m.relative_root_path,
			COALESCE(sn.username, ''),
			COALESCE(sn.secret_ciphertext, ''),
			mr.available_bytes
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
		  AND m.enabled = true
		  AND m.library_id = $1
		  AND m.mount_mode = 'READ_WRITE'
		  AND sn.node_type IN ('LOCAL', 'NAS')
		ORDER BY m.created_at ASC
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ImportTarget, 0)
	for rows.Next() {
		var row ImportTarget
		if err := rows.Scan(
			&row.MountID,
			&row.LibraryID,
			&row.LibraryName,
			&row.StorageNodeID,
			&row.Label,
			&row.NodeType,
			&row.MountMode,
			&row.SourcePath,
			&row.RelativeRootPath,
			&row.Username,
			&row.SecretCiphertext,
			&row.AvailableBytes,
		); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Service) IngestImportedReplica(ctx context.Context, input ImportedReplicaInput) error {
	if strings.TrimSpace(input.LibraryID) == "" {
		return apperrors.BadRequest("libraryId 不能为空")
	}
	if strings.TrimSpace(input.MountID) == "" {
		return apperrors.BadRequest("mountId 不能为空")
	}
	logicalPath := normalizeLogicalPath(input.LogicalPath)
	if logicalPath == "/" {
		return apperrors.BadRequest("logicalPath 无效")
	}

	targetMount, err := s.loadOperationMountByID(ctx, input.MountID)
	if err != nil {
		return err
	}
	if targetMount.LibraryID != input.LibraryID {
		return apperrors.BadRequest("目标挂载与资产库不匹配")
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.upsertLibrary(ctx, tx, targetMount.LibraryID, targetMount.LibraryName, now); err != nil {
		return err
	}

	directoryPath := parentLogicalPath(logicalPath)
	directoryID, err := s.ensureDirectoryChain(ctx, tx, targetMount.LibraryID, directoryPath, now)
	if err != nil {
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
		executor: noopMountPathExecutor{},
	}, targetMount.LibraryID, directoryPath, now); err != nil {
		return err
	}

	info := uploadFileInfo{
		name:    filepath.Base(logicalPath),
		size:    input.SizeBytes,
		modTime: input.ModifiedAt.UTC(),
	}
	assetID, err := s.upsertAsset(ctx, tx, targetMount.LibraryID, directoryID, logicalPath, info, now)
	if err != nil {
		return err
	}
	if err := s.upsertReplica(ctx, tx, assetID, targetMount.ID, input.PhysicalPath, info, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type noopMountPathExecutor struct{}

func (noopMountPathExecutor) EnsureDirectory(context.Context, pathExecutionContext) error { return nil }
func (noopMountPathExecutor) WriteFile(context.Context, pathExecutionContext) error      { return nil }
func (noopMountPathExecutor) WriteStream(context.Context, pathExecutionContext, io.Reader) error {
	return nil
}
func (noopMountPathExecutor) DeleteFile(context.Context, pathExecutionContext) error { return nil }
func (noopMountPathExecutor) DeleteDirectory(context.Context, pathExecutionContext) error {
	return nil
}
func (noopMountPathExecutor) StreamFile(context.Context, pathExecutionContext, func(io.Reader) error) error { return nil }
func (noopMountPathExecutor) StatFile(context.Context, pathExecutionContext) (fileMetadata, error) {
	return fileMetadata{}, fmt.Errorf("not implemented")
}
func (noopMountPathExecutor) SetFileModifiedTime(context.Context, pathExecutionContext, time.Time) error {
	return nil
}
