package assets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	assetdto "mare/shared/contracts/dto/asset"
)

type uploadMount struct {
	writableMount
	executor mountPathExecutor
}

type uploadFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (s *Service) UploadSelection(
	ctx context.Context,
	libraryID string,
	request assetdto.UploadSelectionRequest,
) (assetdto.UploadSelectionResponse, error) {
	if len(request.Files) == 0 {
		return assetdto.UploadSelectionResponse{}, apperrors.BadRequest("未选择任何上传内容")
	}

	mode := strings.TrimSpace(request.Mode)
	if mode != "files" && mode != "folder" {
		return assetdto.UploadSelectionResponse{}, apperrors.BadRequest("上传模式无效")
	}

	library, err := s.loadLibrary(ctx, libraryID)
	if err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}

	parent, _, err := s.resolveCurrentDirectory(ctx, libraryID, request.ParentID)
	if err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}

	mounts, err := s.loadUploadMounts(ctx, libraryID)
	if err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}
	if len(mounts) == 0 {
		return assetdto.UploadSelectionResponse{}, apperrors.BadRequest("当前资产库没有可用的本地或 NAS 可写挂载")
	}

	now := s.now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.upsertLibrary(ctx, tx, library.ID, library.Name, now); err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}
	if _, err := s.ensureDirectoryChain(ctx, tx, library.ID, parent.RelativePath, now); err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}

	directoryCache := map[string]string{
		parent.RelativePath: parent.ID,
	}
	for _, mount := range mounts {
		if err := s.ensureDirectoryPresenceOnMount(ctx, tx, mount, libraryID, parent.RelativePath, now); err != nil {
			return assetdto.UploadSelectionResponse{}, err
		}
	}

	createdCount := 0
	for _, file := range request.Files {
		relativePath, err := normalizeUploadRelativePath(mode, file)
		if err != nil {
			return assetdto.UploadSelectionResponse{}, err
		}

		logicalPath := joinLogicalPath(parent.RelativePath, relativePath)
		targetDirectoryPath := parentLogicalPath(logicalPath)
		directoryID, ok := directoryCache[targetDirectoryPath]
		if !ok {
			directoryID, err = s.ensureDirectoryChain(ctx, tx, libraryID, targetDirectoryPath, now)
			if err != nil {
				return assetdto.UploadSelectionResponse{}, err
			}
			directoryCache[targetDirectoryPath] = directoryID
		}

		for _, mount := range mounts {
			if err := s.ensureDirectoryPresenceOnMount(ctx, tx, mount, libraryID, targetDirectoryPath, now); err != nil {
				return assetdto.UploadSelectionResponse{}, err
			}
			physicalPath := resolveUploadPhysicalFilePath(mount.SourcePath, logicalPath)
			if err := mount.executor.WriteFile(ctx, pathExecutionContext{
				PhysicalPath:     physicalPath,
				Username:         mount.Username,
				SecretCiphertext: mount.SecretCiphertext,
				FileContent:      file.Content,
			}); err != nil {
				return assetdto.UploadSelectionResponse{}, apperrors.BadRequest("上传失败，请检查挂载目录是否可写")
			}
		}

		assetID, err := s.upsertAsset(ctx, tx, libraryID, directoryID, logicalPath, uploadFileInfo{
			name:    filepath.Base(logicalPath),
			size:    int64(len(file.Content)),
			modTime: now,
		}, now)
		if err != nil {
			return assetdto.UploadSelectionResponse{}, err
		}
		for _, mount := range mounts {
			physicalPath := resolveUploadPhysicalFilePath(mount.SourcePath, logicalPath)
			if err := s.upsertReplica(ctx, tx, assetID, mount.ID, physicalPath, uploadFileInfo{
				name:    filepath.Base(logicalPath),
				size:    int64(len(file.Content)),
				modTime: now,
			}, now); err != nil {
				return assetdto.UploadSelectionResponse{}, err
			}
		}
		createdCount++
	}

	if err := tx.Commit(ctx); err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}

	if mode == "folder" {
		return assetdto.UploadSelectionResponse{
			Message:      fmt.Sprintf("文件夹内容已上传，共写入 %d 个文件", createdCount),
			CreatedCount: createdCount,
		}, nil
	}

	return assetdto.UploadSelectionResponse{
		Message:      fmt.Sprintf("已上传 %d 个文件", createdCount),
		CreatedCount: createdCount,
	}, nil
}

func (s *Service) loadUploadMounts(ctx context.Context, libraryID string) ([]uploadMount, error) {
	mounts, err := s.loadWritableMounts(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	items := make([]uploadMount, 0, len(mounts))
	for _, mount := range mounts {
		if mount.NodeType == "CLOUD" {
			// TODO: 网盘上传后续接入 cd2 后通过 cd2 实现，这里暂不执行云盘写入。
			continue
		}
		executor, err := s.resolveExecutor(mount.NodeType)
		if err != nil {
			return nil, err
		}
		items = append(items, uploadMount{
			writableMount: mount,
			executor:      executor,
		})
	}
	return items, nil
}

func (s *Service) ensureDirectoryPresenceOnMount(
	ctx context.Context,
	tx pgx.Tx,
	mount uploadMount,
	libraryID string,
	directoryRelativePath string,
	now time.Time,
) error {
	currentPath := "/"
	rootID := "dir-root-" + libraryID
	if err := s.upsertDirectoryPresence(ctx, tx, rootID, mount.ID, mount.SourcePath, now); err != nil {
		return err
	}

	if normalizeLogicalPath(directoryRelativePath) == "/" {
		return nil
	}

	for _, segment := range strings.Split(strings.Trim(normalizeLogicalPath(directoryRelativePath), "/"), "/") {
		currentPath = joinLogicalPath(currentPath, segment)
		directoryID, err := s.ensureDirectoryChain(ctx, tx, libraryID, currentPath, now)
		if err != nil {
			return err
		}
		physicalPath := resolveMountPhysicalDirectoryPath(mount.SourcePath, mount.RelativeRootPath, currentPath)
		if err := mount.executor.EnsureDirectory(ctx, pathExecutionContext{
			PhysicalPath:     physicalPath,
			Username:         mount.Username,
			SecretCiphertext: mount.SecretCiphertext,
		}); err != nil {
			return err
		}
		if err := s.upsertDirectoryPresence(ctx, tx, directoryID, mount.ID, physicalPath, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) resolveExecutor(nodeType string) (mountPathExecutor, error) {
	if s.executorResolver == nil {
		return executorForNodeType(nodeType)
	}
	return s.executorResolver(nodeType)
}

func normalizeUploadRelativePath(mode string, file assetdto.UploadSelectionFile) (string, error) {
	raw := strings.TrimSpace(file.RelativePath)
	if raw == "" {
		raw = strings.TrimSpace(file.Name)
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = strings.TrimPrefix(raw, "/")
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" {
		return "", apperrors.BadRequest("上传文件路径无效")
	}

	segments := strings.Split(raw, "/")
	if mode == "files" && len(segments) != 1 {
		return "", apperrors.BadRequest("上传文件模式下不允许包含嵌套路径")
	}
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" || trimmed == "." || trimmed == ".." {
			return "", apperrors.BadRequest("上传文件路径无效")
		}
	}
	return strings.Join(segments, "/"), nil
}

func resolveUploadPhysicalFilePath(sourcePath string, logicalPath string) string {
	normalized := normalizeLogicalPath(logicalPath)
	suffix := strings.Trim(strings.ReplaceAll(normalized, "/", string(filepath.Separator)), string(filepath.Separator))
	if suffix == "" {
		return sourcePath
	}
	return filepath.Join(sourcePath, suffix)
}

func (u uploadFileInfo) Name() string       { return u.name }
func (u uploadFileInfo) Size() int64        { return u.size }
func (u uploadFileInfo) Mode() os.FileMode  { return 0o644 }
func (u uploadFileInfo) ModTime() time.Time { return u.modTime }
func (u uploadFileInfo) IsDir() bool        { return false }
func (u uploadFileInfo) Sys() any           { return nil }
