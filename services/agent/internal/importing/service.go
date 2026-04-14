package importing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	importdto "mare/shared/contracts/dto/importing"
)

const (
	SourceTypeLocalDirectory = "LOCAL_DIRECTORY"
)

type Options struct {
	ImportSourcePaths []string
	DiscoverAttachedSources func(ctx context.Context) ([]SourceDescriptor, error)
	Now               func() time.Time
}

type Service struct {
	now               func() time.Time
	importSourcePaths []string
	discoverAttachedSources func(ctx context.Context) ([]SourceDescriptor, error)
}

type BrowseRequest = importdto.BrowseRequest
type BrowseResponse = importdto.BrowseResponse
type BrowseEntry = importdto.BrowseEntry
type ExecuteImportRequest = importdto.ExecuteImportRequest
type ExecuteImportTarget = importdto.ExecuteImportTarget
type ExecuteImportResponse = importdto.ExecuteImportResponse
type ExecuteImportTargetResult = importdto.ExecuteImportTargetResult
type SourceDescriptor = importdto.SourceDescriptor

func NewService(options Options) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	paths := make([]string, 0, len(options.ImportSourcePaths))
	for _, item := range options.ImportSourcePaths {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		paths = append(paths, trimmed)
	}
	return &Service{
		now:               now,
		importSourcePaths: paths,
		discoverAttachedSources: options.DiscoverAttachedSources,
	}
}

func (s *Service) DiscoverSources(ctx context.Context) ([]SourceDescriptor, error) {
	now := s.now().UTC().Format(time.RFC3339)
	items := make([]SourceDescriptor, 0, len(s.importSourcePaths))
	seenPaths := make(map[string]struct{})
	for _, sourcePath := range s.importSourcePaths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		info, err := os.Stat(sourcePath)
		if err != nil || !info.IsDir() {
			continue
		}
		deviceKey := normalizeDeviceKey(sourcePath)
		items = append(items, SourceDescriptor{
			DeviceKey:      deviceKey,
			SourceType:     SourceTypeLocalDirectory,
			DeviceLabel:    filepath.Base(sourcePath),
			DeviceType:     "本地目录",
			SourcePath:     sourcePath,
			MountPath:      sourcePath,
			ConnectedAt:    now,
			LastSeenAt:     now,
			SourceSnapshot: map[string]any{"sourceType": SourceTypeLocalDirectory},
		})
		seenPaths[strings.ToLower(filepath.Clean(sourcePath))] = struct{}{}
	}

	discoverFn := s.discoverAttachedSources
	if discoverFn == nil {
		discoverFn = discoverAttachedSources
	}
	discovered, err := discoverFn(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range discovered {
		normalized := strings.ToLower(filepath.Clean(item.SourcePath))
		if _, exists := seenPaths[normalized]; exists {
			continue
		}
		if strings.TrimSpace(item.ConnectedAt) == "" {
			item.ConnectedAt = now
		}
		if strings.TrimSpace(item.LastSeenAt) == "" {
			item.LastSeenAt = now
		}
		items = append(items, item)
		seenPaths[normalized] = struct{}{}
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.Compare(items[i].DeviceLabel, items[j].DeviceLabel) < 0
	})
	return items, nil
}

func (s *Service) BrowseSource(ctx context.Context, request BrowseRequest) (BrowseResponse, error) {
	root := strings.TrimSpace(request.SourcePath)
	if root == "" {
		return BrowseResponse{}, fmt.Errorf("sourcePath 不能为空")
	}
	root = filepath.Clean(root)

	info, err := os.Stat(root)
	if err != nil {
		return BrowseResponse{}, err
	}
	if !info.IsDir() {
		return BrowseResponse{}, fmt.Errorf("sourcePath 不是目录")
	}

	currentRoot := root
	currentPath := ""
	if request.RelativePath != nil {
		trimmed := strings.Trim(strings.ReplaceAll(strings.TrimSpace(*request.RelativePath), "\\", "/"), "/")
		if trimmed != "" {
			currentPath = trimmed
			currentRoot = filepath.Join(root, filepath.FromSlash(trimmed))
		}
	}
	currentInfo, err := os.Stat(currentRoot)
	if err != nil {
		return BrowseResponse{}, err
	}
	if !currentInfo.IsDir() {
		return BrowseResponse{}, fmt.Errorf("当前路径不是目录")
	}

	children, err := os.ReadDir(currentRoot)
	if err != nil {
		return BrowseResponse{}, err
	}

	entries := make([]BrowseEntry, 0, len(children))
	for _, child := range children {
		if ctx.Err() != nil {
			return BrowseResponse{}, ctx.Err()
		}
		relativePath := child.Name()
		if currentPath != "" {
			relativePath = currentPath + "/" + child.Name()
		}
		record := BrowseEntry{
			RelativePath: relativePath,
			Name:         child.Name(),
			FileKind:     "文件夹",
			ModifiedAt:   s.now().UTC().Format(time.RFC3339),
			IsHidden:     isHiddenName(child.Name()),
		}
		if child.IsDir() {
			record.EntryType = "DIRECTORY"
			record.HasChildren = directoryHasChildren(filepath.Join(currentRoot, child.Name()))
			info, infoErr := child.Info()
			if infoErr == nil {
				record.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
			}
			entries = append(entries, record)
			continue
		}

		stat, err := child.Info()
		if err != nil {
			return BrowseResponse{}, err
		}
		record.EntryType = "FILE"
		record.HasChildren = false
		record.FileKind = classifyFileKind(child.Name())
		record.ModifiedAt = stat.ModTime().UTC().Format(time.RFC3339)
		size := stat.Size()
		record.SizeBytes = &size
		extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(child.Name())), ".")
		if extension != "" {
			record.Extension = &extension
		}
		entries = append(entries, record)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RelativePath == entries[j].RelativePath {
			return entries[i].EntryType < entries[j].EntryType
		}
		return entries[i].RelativePath < entries[j].RelativePath
	})

	total := len(entries)
	limit := request.Limit
	if limit <= 0 {
		limit = 200
	}
	offset := request.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paged := entries[offset:end]

	return BrowseResponse{
		Entries:   paged,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		HasMore:   end < total,
		ScannedAt: s.now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) ExecuteImport(ctx context.Context, request ExecuteImportRequest) (ExecuteImportResponse, error) {
	sourcePath := strings.TrimSpace(request.SourcePath)
	if sourcePath == "" {
		return ExecuteImportResponse{}, fmt.Errorf("sourcePath 不能为空")
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return ExecuteImportResponse{}, err
	}
	if sourceInfo.IsDir() {
		return ExecuteImportResponse{}, fmt.Errorf("sourcePath 必须是文件")
	}
	if len(request.Targets) == 0 {
		return ExecuteImportResponse{}, fmt.Errorf("targets 不能为空")
	}

	results := make([]ExecuteImportTargetResult, 0, len(request.Targets))
	for _, target := range request.Targets {
		if ctx.Err() != nil {
			return ExecuteImportResponse{}, ctx.Err()
		}
		result := ExecuteImportTargetResult{
			TargetID:   target.TargetID,
			PhysicalPath: target.PhysicalPath,
			VerifyMode: "LIGHT",
		}
		if target.NodeType != "LOCAL" && target.NodeType != "NAS" {
			result.Status = "FAILED"
			result.VerifyStatus = "SKIPPED"
			result.VerifySummary = "未执行校验"
			result.ErrorMessage = fmt.Sprintf("暂不支持 %s 目标写入", target.NodeType)
			results = append(results, result)
			continue
		}
		written, modifiedAt, err := copyFileToLocalPath(sourcePath, target.PhysicalPath, target.PreserveMtime)
		if err != nil {
			result.Status = "FAILED"
			result.VerifyStatus = "SKIPPED"
			result.VerifySummary = "未执行校验"
			result.ErrorMessage = err.Error()
			results = append(results, result)
			continue
		}
		verifyStatus, verifySummary, verifyErr := verifyLightCopy(sourcePath, target.PhysicalPath, target.PreserveMtime)
		result.BytesWritten = written
		result.ModifiedAt = modifiedAt.UTC().Format(time.RFC3339)
		result.VerifyStatus = verifyStatus
		result.VerifySummary = verifySummary
		if verifyErr != nil {
			result.Status = "FAILED"
			result.ErrorMessage = verifyErr.Error()
		} else {
			result.Status = "SUCCEEDED"
		}
		results = append(results, result)
	}

	return ExecuteImportResponse{Targets: results}, nil
}

func copyFileToLocalPath(sourcePath string, targetPath string, preserveMtime bool) (int64, time.Time, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return 0, time.Time{}, err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer targetFile.Close()

	written, err := io.Copy(targetFile, sourceFile)
	if err != nil {
		return 0, time.Time{}, err
	}

	modifiedAt := time.Now().UTC()
	if preserveMtime {
		info, err := os.Stat(sourcePath)
		if err != nil {
			return 0, time.Time{}, err
		}
		modifiedAt = info.ModTime().UTC()
		if err := os.Chtimes(targetPath, modifiedAt, modifiedAt); err != nil {
			return 0, time.Time{}, err
		}
	}
	return written, modifiedAt, nil
}

func verifyLightCopy(sourcePath string, targetPath string, preserveMtime bool) (string, string, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "FAILED", "轻校验失败", err
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return "FAILED", "轻校验失败", err
	}
	if sourceInfo.Size() != targetInfo.Size() {
		return "FAILED", "轻校验失败：大小不一致", fmt.Errorf("light verify failed: size mismatch")
	}
	if preserveMtime && !sourceInfo.ModTime().UTC().Equal(targetInfo.ModTime().UTC()) {
		return "FAILED", "轻校验失败：修改时间不一致", fmt.Errorf("light verify failed: mtime mismatch")
	}
	return "PASSED", "轻校验通过", nil
}

func normalizeDeviceKey(sourcePath string) string {
	key := strings.ToLower(strings.TrimSpace(filepath.Clean(sourcePath)))
	key = strings.ReplaceAll(key, "\\", "/")
	key = strings.ReplaceAll(key, ":", "")
	key = strings.Trim(key, "/")
	key = strings.ReplaceAll(key, "/", "-")
	if key == "" {
		return "import-source"
	}
	return "source-" + key
}

func classifyFileKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".arw", ".cr3", ".nef":
		return "图片"
	case ".mp4", ".mov", ".mxf", ".avi":
		return "视频"
	case ".wav", ".mp3", ".flac", ".aac":
		return "音频"
	case ".txt", ".md", ".pdf", ".doc", ".docx":
		return "文档"
	default:
		return "文件"
	}
}

func directoryHasChildren(path string) bool {
	items, err := os.ReadDir(path)
	return err == nil && len(items) > 0
}

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}
