package asset

import (
	"encoding/json"
	"testing"
)

func TestLibraryRecordJSONShape(t *testing.T) {
	t.Parallel()

	payload := LibraryRecord{
		ID:            "photo",
		Name:          "商业摄影资产库",
		RootLabel:     "/",
		ItemCount:     "128",
		Health:        "100%",
		StoragePolicy: "本地 + NAS",
		EndpointNames: []string{"商业摄影原片库", "影像 NAS"},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal library record: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal library record: %v", err)
	}

	if decoded["rootLabel"] != "/" {
		t.Fatalf("expected rootLabel /, got %#v", decoded["rootLabel"])
	}
	endpointNames, ok := decoded["endpointNames"].([]any)
	if !ok || len(endpointNames) != 2 {
		t.Fatalf("expected two endpoint names, got %#v", decoded["endpointNames"])
	}
}

func TestBrowseLibraryResponseJSONShape(t *testing.T) {
	t.Parallel()

	rootID := "dir-root-photo"
	payload := BrowseLibraryResponse{
		Breadcrumbs: []Breadcrumb{
			{ID: nil, Label: "商业摄影资产库"},
			{ID: &rootID, Label: "原片"},
		},
		Items: []EntryRecord{
			{
				ID:             "asset-1",
				LibraryID:      "photo",
				Type:           "file",
				LifecycleState: "ACTIVE",
				Name:           "cover.jpg",
				FileKind:       "图片",
				DisplayType:    "JPEG 图片",
				ModifiedAt:     "2026-04-10 12:20",
				CreatedAt:      "2026-04-10 12:20",
				Size:           "1.2 MB",
				Path:           "商业摄影资产库 / 原片 / cover.jpg",
				SourceLabel:    "统一资产",
				LastTaskText:   "暂无任务",
				LastTaskTone:   "info",
				Rating:         0,
				ColorLabel:     "无",
				Badges:         []string{},
				RiskTags:       []string{},
				Tags:           []string{},
				Endpoints: []EntryEndpoint{
					{Name: "商业摄影原片库", State: "已同步", Tone: "success", LastSyncAt: "2026-04-10 12:20", EndpointType: "local"},
				},
			},
		},
		Total:               1,
		CurrentPathChildren: 1,
		EndpointNames:       []string{"商业摄影原片库"},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal browse response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal browse response: %v", err)
	}

	items, ok := decoded["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one item, got %#v", decoded["items"])
	}
	breadcrumbs, ok := decoded["breadcrumbs"].([]any)
	if !ok || len(breadcrumbs) != 2 {
		t.Fatalf("expected two breadcrumbs, got %#v", decoded["breadcrumbs"])
	}
}

func TestUpdateAnnotationsRequestJSONShape(t *testing.T) {
	t.Parallel()

	payload := UpdateAnnotationsRequest{
		Rating:     4,
		ColorLabel: "蓝标",
		Tags:       []string{"直播切片", "客户精选"},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update annotations request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal update annotations request: %v", err)
	}

	tags, ok := decoded["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("expected two tags, got %#v", decoded["tags"])
	}
}
