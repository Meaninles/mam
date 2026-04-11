//go:build !windows

package importing

import "context"

func discoverAttachedSources(context.Context) ([]SourceDescriptor, error) {
	return []SourceDescriptor{}, nil
}
