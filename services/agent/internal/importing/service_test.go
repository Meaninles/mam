package importing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceDiscoversConfiguredLocalDirectories(t *testing.T) {
	root := t.TempDir()
	service := NewService(Options{
		ImportSourcePaths: []string{root},
		DiscoverAttachedSources: func(context.Context) ([]SourceDescriptor, error) {
			return []SourceDescriptor{}, nil
		},
	})

	sources, err := service.DiscoverSources(context.Background())
	if err != nil {
		t.Fatalf("discover sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].SourceType != SourceTypeLocalDirectory {
		t.Fatalf("expected local directory source, got %+v", sources[0])
	}
	if sources[0].SourcePath != root {
		t.Fatalf("expected source path %s, got %s", root, sources[0].SourcePath)
	}
}

func TestServiceBrowsesSourceEntries(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "A001")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cover.jpg"), []byte("cover"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "clip.mov"), []byte("clip"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	service := NewService(Options{
		DiscoverAttachedSources: func(context.Context) ([]SourceDescriptor, error) {
			return []SourceDescriptor{}, nil
		},
	})
	result, err := service.BrowseSource(context.Background(), BrowseRequest{SourcePath: root, Limit: 50})
	if err != nil {
		t.Fatalf("browse source: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected 2 root entries, got %d", result.Total)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].EntryType != "DIRECTORY" || !result.Entries[0].HasChildren {
		t.Fatalf("expected first entry to be directory with children, got %+v", result.Entries[0])
	}
	if result.Entries[1].EntryType != "FILE" || result.Entries[1].SizeBytes == nil {
		t.Fatalf("expected second entry to be file, got %+v", result.Entries[1])
	}
}

func TestServiceBrowsesNestedLevelOnly(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "A001")
	deeper := filepath.Join(nested, "B001")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatalf("mkdir deeper: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "clip.mov"), []byte("clip"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deeper, "deep.mov"), []byte("deep"), 0o644); err != nil {
		t.Fatalf("write deep file: %v", err)
	}

	service := NewService(Options{
		DiscoverAttachedSources: func(context.Context) ([]SourceDescriptor, error) {
			return []SourceDescriptor{}, nil
		},
	})
	relativePath := "A001"
	result, err := service.BrowseSource(context.Background(), BrowseRequest{SourcePath: root, RelativePath: &relativePath, Limit: 50})
	if err != nil {
		t.Fatalf("browse nested source: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 nested entries, got %d", result.Total)
	}
	for _, entry := range result.Entries {
		if entry.RelativePath == "A001/B001/deep.mov" {
			t.Fatalf("expected one-level browse, got deep descendant %+v", result.Entries)
		}
	}
}

func TestServiceExecutesLocalFileCopyAndPreservesMtime(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.mov")
	targetDir := filepath.Join(root, "target")
	targetPath := filepath.Join(targetDir, "imported.mov")
	modTime := time.Date(2026, 4, 12, 12, 30, 0, 0, time.UTC)

	if err := os.WriteFile(sourcePath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.Chtimes(sourcePath, modTime, modTime); err != nil {
		t.Fatalf("chtimes source: %v", err)
	}

	service := NewService(Options{
		DiscoverAttachedSources: func(context.Context) ([]SourceDescriptor, error) {
			return []SourceDescriptor{}, nil
		},
	})
	result, err := service.ExecuteImport(context.Background(), ExecuteImportRequest{
		SourcePath: sourcePath,
		Targets: []ExecuteImportTarget{
			{
				TargetID:      "target-local-1",
				NodeType:      "LOCAL",
				PhysicalPath:  targetPath,
				PreserveMtime: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("execute import: %v", err)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target result, got %d", len(result.Targets))
	}
	if result.Targets[0].Status != "SUCCEEDED" {
		t.Fatalf("expected target success status, got %+v", result.Targets[0])
	}
	if result.Targets[0].VerifyStatus != "PASSED" {
		t.Fatalf("expected verify passed status, got %+v", result.Targets[0])
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.ModTime().UTC() != modTime {
		t.Fatalf("expected target mtime %s, got %s", modTime, info.ModTime().UTC())
	}
}

func TestServiceContinuesOtherTargetsWhenOneTargetFails(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.mov")
	successTargetPath := filepath.Join(root, "success", "imported.mov")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	service := NewService(Options{
		DiscoverAttachedSources: func(context.Context) ([]SourceDescriptor, error) {
			return []SourceDescriptor{}, nil
		},
	})
	result, err := service.ExecuteImport(context.Background(), ExecuteImportRequest{
		SourcePath: sourcePath,
		Targets: []ExecuteImportTarget{
			{
				TargetID:      "target-success",
				NodeType:      "LOCAL",
				PhysicalPath:  successTargetPath,
				PreserveMtime: true,
			},
			{
				TargetID:      "target-failed",
				NodeType:      "NAS",
				PhysicalPath:  `\\nas\share\imported.mov`,
				PreserveMtime: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("execute import should not fail as a whole: %v", err)
	}
	if len(result.Targets) != 2 {
		t.Fatalf("expected 2 target results, got %d", len(result.Targets))
	}
	if result.Targets[0].Status != "SUCCEEDED" {
		t.Fatalf("expected first target success, got %+v", result.Targets[0])
	}
	if result.Targets[1].Status != "FAILED" {
		t.Fatalf("expected second target failure, got %+v", result.Targets[1])
	}
	if _, statErr := os.Stat(successTargetPath); statErr != nil {
		t.Fatalf("expected successful target file written: %v", statErr)
	}
}
