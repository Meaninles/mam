package storage

import (
	"context"
	"testing"
	"time"

	"mare/services/center/internal/db"
	storagedto "mare/shared/contracts/dto/storage"
)

func TestLocalFolderServiceSaveLocalFolderPersistsScheduledScanPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openStorageTestPool(t, ctx)
	defer pool.Close()
	resetStorageSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (
			id, code, name, node_type, address, access_mode, enabled, created_at, updated_at
		) VALUES (
			'local-node-1', 'local-node-1', '本地节点', 'LOCAL', 'C:/mare', 'DIRECT', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert local node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_runtime (
			id, storage_node_id, health_status, auth_status, created_at, updated_at
		) VALUES (
			'local-node-runtime-1', 'local-node-1', 'ONLINE', 'NOT_REQUIRED', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert local runtime: %v", err)
	}

	service := NewLocalFolderService(pool)
	service.now = func() time.Time { return now }

	result, err := service.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "计划扫描挂载",
		LibraryID:       "library-1",
		LibraryName:     "资料库",
		NodeID:          "local-node-1",
		MountMode:       "只读",
		HeartbeatPolicy: "每日（深夜）",
		ScanPolicy:      "定时扫描",
		RelativePath:    "projects",
	})
	if err != nil {
		t.Fatalf("save local folder: %v", err)
	}

	var scanPolicy string
	var nextScanAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT m.scan_policy, mr.next_scan_at
		FROM mounts m
		INNER JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.id = $1
	`, result.Record.ID).Scan(&scanPolicy, &nextScanAt); err != nil {
		t.Fatalf("query saved mount runtime: %v", err)
	}

	if scanPolicy != "SCHEDULED" {
		t.Fatalf("expected scan_policy SCHEDULED, got %s", scanPolicy)
	}
	if nextScanAt == nil || !nextScanAt.After(now) {
		t.Fatalf("expected next_scan_at after now, got %v", nextScanAt)
	}
}
