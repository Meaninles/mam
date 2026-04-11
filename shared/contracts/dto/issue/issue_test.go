package issue

import (
	"encoding/json"
	"testing"
)

func TestListResponseJSONShape(t *testing.T) {
	t.Parallel()

	libraryID := "photo"
	taskID := "job-1"
	taskItemID := "item-1"
	entryID := "asset-1"
	endpointID := "mount-1"
	endpointLabel := "商业摄影原片库"
	path := "D:\\Assets\\Photo\\cover.jpg"
	action := "RETRY"
	actionLabel := "重试"
	suggestion := "建议先重试当前任务。"
	detail := "目标端点返回权限不足。"

	payload := ListResponse{
		Items: []Record{
			{
				ID:                   "issue-1",
				Code:                 "issue-20260412-1",
				LibraryID:            &libraryID,
				TaskID:               &taskID,
				TaskItemID:           &taskItemID,
				IssueCategory:        "TRANSFER",
				IssueType:            "TARGET_WRITE_FAILED",
				Nature:               "BLOCKING",
				SourceDomain:         "TRANSFER_JOB",
				Severity:             "CRITICAL",
				Status:               "OPEN",
				Title:                "封面图同步失败",
				Summary:              "目标端点写入失败，需要处理后才能继续。",
				ObjectLabel:          "cover.jpg / 商业摄影原片库",
				AssetLabel:           ptr("cover.jpg"),
				SuggestedAction:      &action,
				SuggestedActionLabel: &actionLabel,
				Suggestion:           &suggestion,
				Detail:               &detail,
				CreatedAt:            "2026-04-12T01:02:03Z",
				UpdatedAt:            "2026-04-12T01:02:03Z",
				Source: SourceContext{
					TaskID:        &taskID,
					TaskItemID:    &taskItemID,
					EntryID:       &entryID,
					EndpointID:    &endpointID,
					EndpointLabel: &endpointLabel,
					Path:          &path,
				},
				Impact: ImpactSummary{
					AssetCount:          1,
					ReplicaCount:        1,
					DirectoryCount:      1,
					EndpointCount:       1,
					BlocksStatusCommit:  true,
					BlocksTaskExecution: true,
				},
				Capabilities: Capabilities{
					CanRetry:          true,
					CanOpenTaskCenter: true,
					CanOpenFileCenter: true,
				},
				Histories: []HistoryRecord{
					{
						ID:            "evt-1",
						IssueID:       "issue-1",
						Action:        "自动发现",
						OperatorLabel: "系统",
						Result:        "同步失败，已进入异常中心。",
						CreatedAt:     "2026-04-12T01:02:03Z",
					},
				},
			},
		},
		Total:    1,
		Page:     1,
		PageSize: 20,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal list response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}

	items, ok := decoded["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one issue item, got %#v", decoded["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected issue object, got %#v", items[0])
	}
	if item["issueCategory"] != "TRANSFER" {
		t.Fatalf("expected issueCategory TRANSFER, got %#v", item["issueCategory"])
	}
	source, ok := item["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got %#v", item["source"])
	}
	if source["endpointId"] != "mount-1" {
		t.Fatalf("expected endpointId mount-1, got %#v", source["endpointId"])
	}
}

func TestActionRequestJSONShape(t *testing.T) {
	t.Parallel()

	payload := ActionRequest{
		IDs:    []string{"issue-1", "issue-2"},
		Action: "retry",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal action request: %v", err)
	}

	if decoded["action"] != "retry" {
		t.Fatalf("expected action retry, got %#v", decoded["action"])
	}
}

func ptr(value string) *string {
	return &value
}
