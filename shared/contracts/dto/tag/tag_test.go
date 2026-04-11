package tag

import (
	"encoding/json"
	"testing"
)

func TestManagementSnapshotJSONShape(t *testing.T) {
	t.Parallel()

	payload := ManagementSnapshot{
		Overview: ManagementOverview{
			TotalTags:            3,
			UsedTagCount:         2,
			UngroupedTagCount:    1,
			CrossLibraryTagCount: 1,
		},
		Groups: []GroupRecord{
			{
				ID:           "tag-group-ungrouped",
				Name:         "未分组",
				OrderIndex:   0,
				TagCount:     1,
				UsedTagCount: 1,
			},
		},
		Tags: []Record{
			{
				ID:                   "tag-1",
				Name:                 "直播切片",
				NormalizedName:       "直播切片",
				GroupID:              "tag-group-ungrouped",
				GroupName:            "未分组",
				OrderIndex:           0,
				IsPinned:             true,
				UsageCount:           2,
				LibraryIDs:           []string{"photo", "video"},
				LinkedLibraryIDs:     []string{"photo"},
				OutOfScopeUsageCount: 0,
				CreatedAt:            "2026-04-11 10:00",
				UpdatedAt:            "2026-04-11 10:00",
			},
		},
		Libraries: []LibraryRecord{
			{ID: "photo", Name: "商业摄影资产库"},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal management snapshot: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal management snapshot: %v", err)
	}

	if _, ok := decoded["overview"].(map[string]any); !ok {
		t.Fatalf("expected overview object, got %#v", decoded["overview"])
	}
	if groups, ok := decoded["groups"].([]any); !ok || len(groups) != 1 {
		t.Fatalf("expected one group, got %#v", decoded["groups"])
	}
	if tags, ok := decoded["tags"].([]any); !ok || len(tags) != 1 {
		t.Fatalf("expected one tag, got %#v", decoded["tags"])
	}
}

func TestSuggestionJSONShape(t *testing.T) {
	t.Parallel()

	payload := Suggestion{
		ID:         "tag-1",
		Name:       "直播切片",
		Count:      5,
		GroupName:  "未分组",
		IsPinned:   true,
		LibraryIDs: []string{"photo"},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal suggestion: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal suggestion: %v", err)
	}

	if decoded["groupName"] != "未分组" {
		t.Fatalf("expected groupName 未分组, got %#v", decoded["groupName"])
	}
	if decoded["count"] != float64(5) {
		t.Fatalf("expected count 5, got %#v", decoded["count"])
	}
}
