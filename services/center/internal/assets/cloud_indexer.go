package assets

import (
	"context"
	"io/fs"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
)

type CloudMountEntry struct {
	Name        string
	IsDirectory bool
	SizeBytes   int64
	ModifiedAt  *time.Time
}

func (s *Service) SyncCloudMountFirstLevel(ctx context.Context, mountID string, entries []CloudMountEntry) error {
	mount, err := s.loadMountConfig(ctx, mountID)
	if err != nil {
		return err
	}
	if mount.NodeType != "CLOUD" {
		return apperrors.BadRequest("当前挂载类型暂不支持云盘目录索引")
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
	rootDirectoryID, err := s.ensureDirectoryChain(ctx, tx, mount.LibraryID, "/", now)
	if err != nil {
		return err
	}
	if err := s.upsertDirectoryPresence(ctx, tx, rootDirectoryID, mount.ID, mount.SourcePath, now); err != nil {
		return err
	}

	seenReplicas := map[string]struct{}{}
	seenDirectories := map[string]struct{}{}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		logicalPath := joinLogicalPath("/", name)
		physicalPath := joinCloudReplicaPath(mount.SourcePath, name)

		if entry.IsDirectory {
			directoryID, err := s.ensureDirectoryChain(ctx, tx, mount.LibraryID, logicalPath, now)
			if err != nil {
				return err
			}
			if err := s.upsertDirectoryPresence(ctx, tx, directoryID, mount.ID, physicalPath, now); err != nil {
				return err
			}
			seenDirectories[physicalPath] = struct{}{}
			continue
		}

		info := cloudEntryFileInfo{
			name:    name,
			size:    max64(0, entry.SizeBytes),
			modTime: resolveCloudEntryTime(entry.ModifiedAt, now),
		}
		assetID, err := s.upsertAsset(ctx, tx, mount.LibraryID, rootDirectoryID, logicalPath, info, now)
		if err != nil {
			return err
		}
		if err := s.upsertReplica(ctx, tx, assetID, mount.ID, physicalPath, info, now); err != nil {
			return err
		}
		seenReplicas[physicalPath] = struct{}{}
	}

	if err := s.markMissingDirectReplicas(ctx, tx, mount.ID, rootDirectoryID, seenReplicas, now); err != nil {
		return err
	}
	if err := s.markMissingDirectDirectoryPresences(ctx, tx, mount.ID, mount.LibraryID, "/", seenDirectories, now); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func resolveCloudEntryTime(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback
	}
	return value.UTC()
}

func max64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

type cloudEntryFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (c cloudEntryFileInfo) Name() string       { return c.name }
func (c cloudEntryFileInfo) Size() int64        { return c.size }
func (c cloudEntryFileInfo) Mode() fs.FileMode  { return 0 }
func (c cloudEntryFileInfo) ModTime() time.Time { return c.modTime }
func (c cloudEntryFileInfo) IsDir() bool        { return false }
func (c cloudEntryFileInfo) Sys() any           { return nil }
