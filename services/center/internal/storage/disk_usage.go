package storage

import (
	"path/filepath"
	"strings"
)

func resolveDiskUsageTarget(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "." || cleaned == "" {
		return path
	}

	volume := filepath.VolumeName(cleaned)
	if volume != "" {
		return volume + string(filepath.Separator)
	}

	return cleaned
}
