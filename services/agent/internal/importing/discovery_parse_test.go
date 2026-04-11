package importing

import (
	"context"
	"testing"
)

func TestDiscoverSourcesMergesConfiguredAndAttachedSources(t *testing.T) {
	root := t.TempDir()
	service := NewService(Options{
		ImportSourcePaths: []string{root},
		DiscoverAttachedSources: func(context.Context) ([]SourceDescriptor, error) {
			return []SourceDescriptor{
				{
					DeviceKey:   "source-d-drive",
					SourceType:  "REMOVABLE_VOLUME",
					DeviceLabel: "T7",
					DeviceType:  "移动硬盘",
					SourcePath:  "D:\\",
					MountPath:   "D:\\",
				},
			}, nil
		},
	})

	sources, err := service.DiscoverSources(context.Background())
	if err != nil {
		t.Fatalf("discover sources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %+v", sources)
	}
}

func TestParseWindowsVolumesJSONHandlesSingleObject(t *testing.T) {
	items, err := parseWindowsVolumesJSON([]byte(`{"DriveLetter":"D","Path":"D:\\","FileSystemLabel":"T7","DriveType":"Fixed","BusType":"USB","FriendlyName":"Samsung PSSD T7","Size":1000,"SizeRemaining":800}`))
	if err != nil {
		t.Fatalf("parse windows volume json: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %+v", items)
	}
	if !isImportableWindowsVolume(items[0]) {
		t.Fatalf("expected USB fixed disk to be importable, got %+v", items[0])
	}
}

func TestParseWindowsVolumesJSONHandlesArray(t *testing.T) {
	items, err := parseWindowsVolumesJSON([]byte(`[{"DriveLetter":"C","Path":"C:\\","DriveType":"Fixed","BusType":"NVMe"},{"DriveLetter":"D","Path":"D:\\","FileSystemLabel":"T7","DriveType":"Fixed","BusType":"USB"}]`))
	if err != nil {
		t.Fatalf("parse array json: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %+v", items)
	}
	if isImportableWindowsVolume(items[0]) {
		t.Fatalf("expected internal NVMe disk to be ignored, got %+v", items[0])
	}
	if !isImportableWindowsVolume(items[1]) {
		t.Fatalf("expected USB disk to be importable, got %+v", items[1])
	}
}
