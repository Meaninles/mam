package job

import (
	"encoding/json"
	"testing"
)

func TestCreateResponseJSONShape(t *testing.T) {
	t.Parallel()

	libraryID := "photo"
	payload := CreateResponse{
		Message: "扫描任务已创建",
		JobID:   "job-1",
		Job: Record{
			ID:              "job-1",
			Code:            "job-20260411-1",
			LibraryID:       &libraryID,
			JobFamily:       "MAINTENANCE",
			JobIntent:       "SCAN_DIRECTORY",
			Status:          "PENDING",
			Priority:        "NORMAL",
			Title:           "扫描目录：/",
			Summary:         "已创建 2 个扫描子项",
			SourceDomain:    "FILE_CENTER",
			ProgressPercent: 0,
			TotalItems:      2,
			SuccessItems:    0,
			FailedItems:     0,
			SkippedItems:    0,
			IssueCount:      0,
			CreatedByType:   "USER",
			CreatedAt:       "2026-04-11T12:00:00Z",
			UpdatedAt:       "2026-04-11T12:00:00Z",
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal create response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	if decoded["jobId"] != "job-1" {
		t.Fatalf("expected jobId job-1, got %#v", decoded["jobId"])
	}
	jobValue, ok := decoded["job"].(map[string]any)
	if !ok {
		t.Fatalf("expected job object, got %#v", decoded["job"])
	}
	if jobValue["jobIntent"] != "SCAN_DIRECTORY" {
		t.Fatalf("expected jobIntent SCAN_DIRECTORY, got %#v", jobValue["jobIntent"])
	}
}

func TestDetailJSONShape(t *testing.T) {
	t.Parallel()

	mountID := "mount-1"
	payload := Detail{
		Job: Record{
			ID:              "job-1",
			Code:            "job-20260411-1",
			JobFamily:       "MAINTENANCE",
			JobIntent:       "SCAN_DIRECTORY",
			Status:          "RUNNING",
			Priority:        "HIGH",
			Title:           "扫描挂载：商业摄影原片库",
			Summary:         "正在执行第 1/1 个子项",
			SourceDomain:    "STORAGE_NODES",
			ProgressPercent: 50,
			TotalItems:      1,
			SuccessItems:    0,
			FailedItems:     0,
			SkippedItems:    0,
			IssueCount:      0,
			CreatedByType:   "USER",
			CreatedAt:       "2026-04-11T12:00:00Z",
			UpdatedAt:       "2026-04-11T12:00:01Z",
		},
		Items: []ItemRecord{
			{
				ID:              "item-1",
				JobID:           "job-1",
				ItemKey:         "mount:mount-1",
				ItemType:        "DIRECTORY_SCAN",
				Status:          "RUNNING",
				Title:           "扫描挂载：商业摄影原片库",
				Summary:         "正在读取目录",
				ProgressPercent: 50,
				AttemptCount:    1,
				IssueCount:      0,
				UpdatedAt:       "2026-04-11T12:00:01Z",
				CreatedAt:       "2026-04-11T12:00:00Z",
			},
		},
		Links: []ObjectLinkRecord{
			{
				ID:         "link-1",
				JobID:      "job-1",
				LinkRole:   "TARGET_MOUNT",
				ObjectType: "MOUNT",
				MountID:    &mountID,
				CreatedAt:  "2026-04-11T12:00:00Z",
			},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}

	items, ok := decoded["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one item, got %#v", decoded["items"])
	}
	links, ok := decoded["links"].([]any)
	if !ok || len(links) != 1 {
		t.Fatalf("expected one link, got %#v", decoded["links"])
	}
}

func TestStreamEventJSONShape(t *testing.T) {
	t.Parallel()

	itemID := "item-1"
	jobStatus := "RUNNING"
	itemStatus := "RUNNING"
	payload := StreamEvent{
		EventID:    "evt-1",
		Topic:      "JOB",
		EventType:  "JOB_ITEM_STARTED",
		JobID:      "job-1",
		JobItemID:  &itemID,
		JobStatus:  &jobStatus,
		ItemStatus: &itemStatus,
		Message:    "开始执行扫描子项",
		CreatedAt:  "2026-04-11T12:00:01Z",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal stream event: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal stream event: %v", err)
	}

	if decoded["topic"] != "JOB" {
		t.Fatalf("expected topic JOB, got %#v", decoded["topic"])
	}
	if decoded["eventType"] != "JOB_ITEM_STARTED" {
		t.Fatalf("expected eventType JOB_ITEM_STARTED, got %#v", decoded["eventType"])
	}
}
