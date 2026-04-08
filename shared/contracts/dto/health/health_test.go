package health

import (
	"encoding/json"
	"testing"
)

func TestLivezResponseJSONShape(t *testing.T) {
	t.Parallel()

	payload := LivezResponse{
		Status:    "up",
		Service:   "mare-center",
		Version:   "dev",
		Timestamp: "2026-04-08T12:00:00Z",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal health response: %v", err)
	}

	expected := `{"status":"up","service":"mare-center","version":"dev","timestamp":"2026-04-08T12:00:00Z"}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

func TestReadyzResponseAllowsPendingMigration(t *testing.T) {
	t.Parallel()

	payload := ReadyzResponse{
		Status: "not_ready",
		Service: ComponentStatus{
			Status:  "up",
			Message: "中心服务已启动",
		},
		Database: ComponentStatus{
			Status:  "up",
			Message: "数据库连接正常",
		},
		Migration: MigrationStatus{
			Status:         "pending",
			CurrentVersion: 0,
			LatestVersion:  1,
		},
		Version:   "dev",
		Timestamp: "2026-04-08T12:00:00Z",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal ready response: %v", err)
	}

	expected := `{"status":"not_ready","service":{"status":"up","message":"中心服务已启动"},"database":{"status":"up","message":"数据库连接正常"},"migration":{"status":"pending","currentVersion":0,"latestVersion":1},"version":"dev","timestamp":"2026-04-08T12:00:00Z"}`
	if string(raw) != expected {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}
