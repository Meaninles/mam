package storage

import (
	"encoding/json"
	"testing"
)

func TestNasRecordJSONShape(t *testing.T) {
	t.Parallel()

	payload := NasRecord{
		ID:           "nas-node-1",
		Name:         "影像 NAS 01",
		Address:      `\\192.168.10.20\media`,
		AccessMode:   "SMB",
		Username:     "mare-sync",
		PasswordHint: "已保存",
		LastTestAt:   "2026-04-09 10:20",
		Status:       "鉴权正常",
		Tone:         "success",
		MountCount:   2,
		Notes:        "主 NAS",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal nas record: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal nas record: %v", err)
	}

	if decoded["accessMode"] != "SMB" {
		t.Fatalf("expected accessMode SMB, got %#v", decoded["accessMode"])
	}
	if decoded["mountCount"] != float64(2) {
		t.Fatalf("expected mountCount 2, got %#v", decoded["mountCount"])
	}
	if _, exists := decoded["password"]; exists {
		t.Fatal("nas record json should not expose password")
	}
}

func TestLocalFolderRecordJSONShape(t *testing.T) {
	t.Parallel()

	payload := LocalFolderRecord{
		ID:               "local-folder-1",
		Name:             "商业摄影原片库",
		LibraryID:        "photo",
		LibraryName:      "商业摄影资产库",
		FolderType:       "本地",
		NodeID:           "local-node-1",
		NodeName:         "本地素材根目录",
		NodeRootPath:     `D:\Mare\Assets`,
		RelativePath:     `PhotoRaw`,
		Address:          `D:\Mare\Assets\PhotoRaw`,
		MountMode:        "可写",
		Enabled:          true,
		ScanStatus:       "最近扫描成功",
		ScanTone:         "success",
		LastScanAt:       "今天 09:12",
		HeartbeatPolicy:  "从不",
		NextHeartbeatAt:  "—",
		CapacitySummary:  "待检测",
		FreeSpaceSummary: "待检测",
		CapacityPercent:  0,
		RiskTags:         []string{},
		Badges:           []string{"本地", "可写"},
		AuthStatus:       "无需鉴权",
		AuthTone:         "info",
		Notes:            "本机目录",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal local folder record: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal local folder record: %v", err)
	}

	if decoded["libraryId"] != "photo" {
		t.Fatalf("expected libraryId photo, got %#v", decoded["libraryId"])
	}
	if decoded["nodeId"] != "local-node-1" {
		t.Fatalf("expected nodeId local-node-1, got %#v", decoded["nodeId"])
	}
	if decoded["relativePath"] != "PhotoRaw" {
		t.Fatalf("expected relativePath PhotoRaw, got %#v", decoded["relativePath"])
	}
}

func TestLocalFolderConnectionTestResponseJSONShape(t *testing.T) {
	t.Parallel()

	payload := RunLocalFolderConnectionTestResponse{
		Message: "连接测试已完成",
		Results: []ConnectionTestResult{
			{
				ID:          "local-folder-1",
				Name:        "商业摄影原片库",
				OverallTone: "success",
				Summary:     "目录可访问。",
				Checks: []ConnectionCheck{
					{
						Label:  "可达性",
						Status: "success",
						Detail: "目录存在且可读取。",
					},
				},
				TestedAt: "刚刚",
			},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal connection test response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal connection test response: %v", err)
	}

	results, ok := decoded["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected single result, got %#v", decoded["results"])
	}
}
