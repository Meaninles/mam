package assets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

type UploadPlan struct {
	LibraryID      string
	LibraryName    string
	EndpointName   string
	TargetMountID  string
	TargetNodeType string
	RouteType      string
	RequestedCount int
	StagingRoot    string
	Items          []UploadPlanItem
}

type UploadPlanItem struct {
	ItemKey             string
	Title               string
	LogicalPath         string
	SourcePath          string
	TargetPath          string
	TargetMountID       string
	TargetStorageNodeID string
	SizeBytes           int64
}

type uploadSourceDescriptor struct {
	ReferenceID       string
	PhysicalPath      string
	NodeType          string
	Username          string
	SecretCiphertext  string
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
	mode, library, parent, targetMount, err := s.prepareUploadTarget(ctx, libraryID, request)
	if err != nil {
		return assetdto.UploadSelectionResponse{}, err
	}

	createdCount := 0
	for _, file := range request.Files {
		relativePath, err := normalizeUploadRelativePath(mode, file)
		if err != nil {
			return assetdto.UploadSelectionResponse{}, err
		}
		logicalPath := joinLogicalPath(parent.RelativePath, relativePath)
		if err := s.ingestUploadedContent(ctx, library.ID, targetMount, logicalPath, file.Content, s.now().UTC()); err != nil {
			return assetdto.UploadSelectionResponse{}, err
		}
		createdCount++
	}

	if mode == "folder" {
		return assetdto.UploadSelectionResponse{
			Message:      fmt.Sprintf("文件夹内容已上传到 %s，共写入 %d 个文件", targetMount.SourcePathLabel(), createdCount),
			CreatedCount: createdCount,
		}, nil
	}

	return assetdto.UploadSelectionResponse{
		Message:      fmt.Sprintf("已上传 %d 个文件到 %s", createdCount, targetMount.SourcePathLabel()),
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
		executor := mountPathExecutor(noopMountPathExecutor{})
		if mount.NodeType != "CLOUD" {
			executor, err = s.resolveExecutor(mount.NodeType)
			if err != nil {
				return nil, err
			}
		}
		items = append(items, uploadMount{
			writableMount: mount,
			executor:      executor,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := uploadMountPriority(items[i].NodeType)
		right := uploadMountPriority(items[j].NodeType)
		if left != right {
			return left < right
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	return items, nil
}

func (s *Service) PrepareUploadPlan(
	ctx context.Context,
	libraryID string,
	request assetdto.UploadSelectionRequest,
) (UploadPlan, error) {
	mode, library, parent, targetMount, err := s.prepareUploadTarget(ctx, libraryID, request)
	if err != nil {
		return UploadPlan{}, err
	}

	stagingRoot, err := os.MkdirTemp("", "mare-file-center-upload-*")
	if err != nil {
		return UploadPlan{}, err
	}

	items := make([]UploadPlanItem, 0, len(request.Files))
	for _, file := range request.Files {
		relativePath, err := normalizeUploadRelativePath(mode, file)
		if err != nil {
			_ = os.RemoveAll(stagingRoot)
			return UploadPlan{}, err
		}

		stagedPath := filepath.Join(stagingRoot, filepath.FromSlash(relativePath))
		if err := writeLocalFile(stagedPath, file.Content); err != nil {
			_ = os.RemoveAll(stagingRoot)
			return UploadPlan{}, apperrors.BadRequest("上传暂存失败，请稍后重试")
		}

		logicalPath := joinLogicalPath(parent.RelativePath, relativePath)
		items = append(items, UploadPlanItem{
			ItemKey:             logicalPath,
			Title:               filepath.Base(logicalPath),
			LogicalPath:         logicalPath,
			SourcePath:          stagedPath,
			TargetPath:          resolveUploadPhysicalFilePath(targetMount.SourcePath, logicalPath),
			TargetMountID:       targetMount.ID,
			TargetStorageNodeID: targetMount.StorageNodeID,
			SizeBytes:           int64(len(file.Content)),
		})
	}

	routeType := "COPY"
	if targetMount.NodeType == "CLOUD" {
		routeType = "UPLOAD"
	}

	return UploadPlan{
		LibraryID:      library.ID,
		LibraryName:    library.Name,
		EndpointName:   targetMount.Name,
		TargetMountID:  targetMount.ID,
		TargetNodeType: targetMount.NodeType,
		RouteType:      routeType,
		RequestedCount: len(items),
		StagingRoot:    stagingRoot,
		Items:          items,
	}, nil
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

func (s *Service) prepareUploadTarget(
	ctx context.Context,
	libraryID string,
	request assetdto.UploadSelectionRequest,
) (string, libraryModel, directoryModel, uploadMount, error) {
	if len(request.Files) == 0 {
		return "", libraryModel{}, directoryModel{}, uploadMount{}, apperrors.BadRequest("未选择任何上传内容")
	}

	mode := strings.TrimSpace(request.Mode)
	if mode != "files" && mode != "folder" {
		return "", libraryModel{}, directoryModel{}, uploadMount{}, apperrors.BadRequest("上传模式无效")
	}

	library, err := s.loadLibrary(ctx, libraryID)
	if err != nil {
		return "", libraryModel{}, directoryModel{}, uploadMount{}, err
	}

	parent, _, err := s.resolveCurrentDirectory(ctx, libraryID, request.ParentID)
	if err != nil {
		return "", libraryModel{}, directoryModel{}, uploadMount{}, err
	}

	mounts, err := s.loadUploadMounts(ctx, libraryID)
	if err != nil {
		return "", libraryModel{}, directoryModel{}, uploadMount{}, err
	}
	if len(mounts) == 0 {
		return "", libraryModel{}, directoryModel{}, uploadMount{}, apperrors.BadRequest("当前资产库没有可用的可写挂载")
	}
	return mode, library, parent, mounts[0], nil
}

func (s *Service) ingestUploadedContent(
	ctx context.Context,
	libraryID string,
	targetMount uploadMount,
	logicalPath string,
	content []byte,
	modifiedAt time.Time,
) error {
	physicalPath, sizeBytes, err := s.writeUploadedContentToMount(ctx, targetMount, logicalPath, content, modifiedAt)
	if err != nil {
		return err
	}
	return s.IngestImportedReplica(ctx, ImportedReplicaInput{
		LibraryID:    libraryID,
		MountID:      targetMount.ID,
		LogicalPath:  logicalPath,
		PhysicalPath: physicalPath,
		SizeBytes:    sizeBytes,
		ModifiedAt:   modifiedAt,
		FileKind:     mapDetectedFileKind(normalizeExtension(filepath.Base(logicalPath))),
	})
}

func (s *Service) writeUploadedContentToMount(
	ctx context.Context,
	targetMount uploadMount,
	logicalPath string,
	content []byte,
	modifiedAt time.Time,
) (string, int64, error) {
	if targetMount.NodeType == "CLOUD" {
		source := &memoryUploadSource{content: append([]byte(nil), content...)}
		physicalPath, err := s.uploadSourceToCloudTarget(ctx, "", "", uploadSourceDescriptor{
			ReferenceID:  "memory-upload",
			PhysicalPath: logicalPath,
			NodeType:     "LOCAL",
		}, source, s.toOperationMount(targetMount), logicalPath, nil)
		if err != nil {
			return "", 0, err
		}
		return physicalPath, int64(len(content)), nil
	}

	physicalPath := resolveUploadPhysicalFilePath(targetMount.SourcePath, logicalPath)
	if err := targetMount.executor.WriteFile(ctx, pathExecutionContext{
		PhysicalPath:     physicalPath,
		Username:         targetMount.Username,
		SecretCiphertext: targetMount.SecretCiphertext,
		FileContent:      content,
	}); err != nil {
		return "", 0, apperrors.BadRequest("上传失败，请检查挂载目录是否可写")
	}
	if err := targetMount.executor.SetFileModifiedTime(ctx, pathExecutionContext{
		PhysicalPath:     physicalPath,
		Username:         targetMount.Username,
		SecretCiphertext: targetMount.SecretCiphertext,
	}, modifiedAt); err != nil {
		return "", 0, err
	}
	return physicalPath, int64(len(content)), nil
}

func (s *Service) toOperationMount(mount uploadMount) operationMount {
	return operationMount{
		ID:               mount.ID,
		StorageNodeID:    mount.StorageNodeID,
		Name:             mount.Name,
		SourcePath:       mount.SourcePath,
		RelativeRootPath: mount.RelativeRootPath,
		MountMode:        mount.MountMode,
		NodeType:         mount.NodeType,
		ProviderVendor:   mount.ProviderVendor,
		ProviderPayload:  mount.ProviderPayload,
		Username:         mount.Username,
		SecretCiphertext: mount.SecretCiphertext,
	}
}

func uploadMountPriority(nodeType string) int {
	switch nodeType {
	case "LOCAL":
		return 0
	case "NAS":
		return 1
	case "CLOUD":
		return 2
	default:
		return 3
	}
}

func resolveUploadPhysicalFilePath(sourcePath string, logicalPath string) string {
	normalized := normalizeLogicalPath(logicalPath)
	suffix := strings.Trim(strings.ReplaceAll(normalized, "/", string(filepath.Separator)), string(filepath.Separator))
	if suffix == "" {
		return sourcePath
	}
	return filepath.Join(sourcePath, suffix)
}

type memoryUploadSource struct {
	content []byte
}

func (m *memoryUploadSource) Size() int64 {
	return int64(len(m.content))
}

func (m *memoryUploadSource) ReadChunk(_ context.Context, offset int64, length int64) ([]byte, bool, error) {
	if offset >= int64(len(m.content)) {
		return []byte{}, true, nil
	}
	end := offset + length
	if end > int64(len(m.content)) {
		end = int64(len(m.content))
	}
	return append([]byte(nil), m.content[offset:end]...), end >= int64(len(m.content)), nil
}

func (m *memoryUploadSource) Close() error {
	return nil
}

func (u uploadFileInfo) Name() string       { return u.name }
func (u uploadFileInfo) Size() int64        { return u.size }
func (u uploadFileInfo) Mode() os.FileMode  { return 0o644 }
func (u uploadFileInfo) ModTime() time.Time { return u.modTime }
func (u uploadFileInfo) IsDir() bool        { return false }
func (u uploadFileInfo) Sys() any           { return nil }

func (w writableMount) SourcePathLabel() string {
	if strings.TrimSpace(w.Name) != "" {
		return w.Name
	}
	return w.SourcePath
}
