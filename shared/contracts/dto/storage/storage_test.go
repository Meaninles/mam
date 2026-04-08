package storage

import (
	"encoding/json"
	"testing"
)

func TestLocalFolderRecordJSONShape(t *testing.T) {
	t.Parallel()

	payload := LocalFolderRecord{
		ID:               "local-folder-1",
		Name:             "商业摄影原片库",
		LibraryID:        "photo",
		LibraryName:      "商业摄影资产库",
		FolderType:       "本地",
		Address:          "D:\\Mare\\Assets\\PhotoRaw",
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

	expected := `{"id":"local-folder-1","name":"商业摄影原片库","libraryId":"photo","libraryName":"商业摄影资产库","folderType":"本地","address":"D:\\Mare\\Assets\\PhotoRaw","mountMode":"可写","enabled":true,"scanStatus":"最近扫描成功","scanTone":"success","lastScanAt":"今天 09:12","heartbeatPolicy":"从不","nextHeartbeatAt":"—","capacitySummary":"待检测","freeSpaceSummary":"待检测","capacityPercent":0,"riskTags":[],"badges":["本地","可写"],"authStatus":"无需鉴权","authTone":"info","notes":"本机目录"}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
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

	expected := `{"message":"连接测试已完成","results":[{"id":"local-folder-1","name":"商业摄影原片库","overallTone":"success","summary":"目录可访问。","checks":[{"label":"可达性","status":"success","detail":"目录存在且可读取。"}],"testedAt":"刚刚"}]}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}
