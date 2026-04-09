package assets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

func newID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + "-" + hex.EncodeToString(buf)
}

func normalizeLogicalPath(value string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return "/"
	}
	return "/" + trimmed
}

func normalizeMountedLogicalRoot(relativeRootPath string) string {
	return normalizeLogicalPath(relativeRootPath)
}

func joinLogicalPath(base string, segment string) string {
	base = normalizeLogicalPath(base)
	segment = strings.Trim(strings.ReplaceAll(segment, "\\", "/"), "/")
	if segment == "" {
		return base
	}
	if base == "/" {
		return "/" + segment
	}
	return base + "/" + segment
}

func parentLogicalPath(value string) string {
	normalized := normalizeLogicalPath(value)
	if normalized == "/" {
		return "/"
	}
	index := strings.LastIndex(normalized, "/")
	if index <= 0 {
		return "/"
	}
	return normalized[:index]
}

func buildLogicalPath(logicalRoot string, relativePhysicalPath string) string {
	if relativePhysicalPath == "." {
		return normalizeLogicalPath(logicalRoot)
	}
	current := normalizeLogicalPath(logicalRoot)
	for _, part := range strings.Split(filepath.ToSlash(relativePhysicalPath), "/") {
		current = joinLogicalPath(current, part)
	}
	return current
}

func formatTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04")
}

func formatOptionalTime(value *time.Time, fallback time.Time) string {
	if value == nil {
		return formatTime(fallback)
	}
	return formatTime(*value)
}

func formatOptionalTimestamp(value *time.Time) string {
	if value == nil {
		return "未同步"
	}
	return formatTime(*value)
}

func formatCount(count int) string {
	return fmt.Sprintf("%d", count)
}

func formatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func normalizeExtension(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

func mapDetectedFileKind(extension string) string {
	switch extension {
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "heic":
		return "IMAGE"
	case "mp4", "mov", "mkv", "avi":
		return "VIDEO"
	case "mp3", "wav", "aac", "flac":
		return "AUDIO"
	case "zip", "rar", "7z", "tar", "gz":
		return "ARCHIVE"
	default:
		return "DOCUMENT"
	}
}

func mapFileKind(fileKind string) string {
	switch fileKind {
	case "IMAGE":
		return "图片"
	case "VIDEO":
		return "视频"
	case "AUDIO":
		return "音频"
	default:
		return "文档"
	}
}

func mapDisplayType(fileKind string, extension *string) string {
	if extension != nil && *extension != "" {
		label := strings.ToUpper(*extension)
		switch fileKind {
		case "IMAGE":
			return label + " 图片"
		case "VIDEO":
			return label + " 视频"
		case "AUDIO":
			return label + " 音频"
		default:
			return label + " 文档"
		}
	}
	switch fileKind {
	case "IMAGE":
		return "图片"
	case "VIDEO":
		return "视频"
	case "AUDIO":
		return "音频"
	default:
		return "文档"
	}
}

func mapEndpointTone(state string) string {
	switch state {
	case "已同步":
		return "success"
	case "部分同步", "同步中":
		return "warning"
	default:
		return "critical"
	}
}

func mapEndpointType(nodeType string) string {
	switch nodeType {
	case "NAS":
		return "nas"
	case "CLOUD":
		return "cloud"
	default:
		return "local"
	}
}

func mapColorLabel(value string) string {
	switch value {
	case "RED":
		return "红标"
	case "YELLOW":
		return "黄标"
	case "GREEN":
		return "绿标"
	case "BLUE":
		return "蓝标"
	case "PURPLE":
		return "紫标"
	default:
		return "无"
	}
}

func mapAssetLifecycle(value string) string {
	if value == "DELETE_PENDING" {
		return "PENDING_DELETE"
	}
	return "ACTIVE"
}

func mapLifecycleState(value string) string {
	if value == "DELETE_PENDING" {
		return "待删除"
	}
	return "正常"
}

func mapDirectoryStatus(value string) string {
	if value == "HIDDEN" {
		return "隐藏"
	}
	return "正常"
}

func buildDisplayPath(libraryName string, relativePath string) string {
	if relativePath == "/" {
		return libraryName
	}
	return libraryName + " / " + strings.Join(strings.Split(strings.Trim(relativePath, "/"), "/"), " / ")
}

func stringifyJSON(raw []byte) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	switch value := decoded.(type) {
	case string:
		return value
	default:
		normalized, err := json.Marshal(value)
		if err != nil {
			return string(raw)
		}
		return string(normalized)
	}
}

func detectMimeType(extension string) string {
	if extension == "" {
		return "application/octet-stream"
	}
	if value := mime.TypeByExtension("." + extension); value != "" {
		return value
	}
	return "application/octet-stream"
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
