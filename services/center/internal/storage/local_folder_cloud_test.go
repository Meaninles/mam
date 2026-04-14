package storage

import (
	"context"
	"testing"
	"time"

	"mare/services/center/internal/db"
	storagedto "mare/shared/contracts/dto/storage"
)

func TestLocalFolderServiceSaveMountSupportsCloudNode(t *testing.T) {
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
			id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
		) VALUES (
			'cloud-node-1', 'cloud-node-1', '115 云归档', 'CLOUD', '115', '/MareArchive', 'QR', '115 云归档', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (
			id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
		) VALUES (
			'cloud-cred-1', 'cloud-node-1', 'TOKEN', $1, 'tv', 'CONFIGURED', $2, $2
		)
	`, "cipher::UID=uid-1; CID=cid-1", now); err != nil {
		t.Fatalf("insert cloud credential: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'QR', '/MareArchive', '{"cloudName":"115","cloudUserName":"mare-user","cloudPath":"/115open(123)/MareArchive"}'::jsonb, $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}

	mountService := NewLocalFolderService(pool)
	mountService.cipher = fakeCredentialCipher{}
	mountService.SetIntegrationService(fakeCloudIntegration{driver: &fakeCloudProviderDriver{}})

	result, err := mountService.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "云盘挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          "cloud-node-1",
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    `Projects/Shanghai`,
	})
	if err != nil {
		t.Fatalf("save cloud mount: %v", err)
	}

	if result.Record.FolderType != "网盘" {
		t.Fatalf("expected folder type 网盘, got %s", result.Record.FolderType)
	}
	if result.Record.Address != `/MareArchive/Projects/Shanghai` {
		t.Fatalf("unexpected cloud mount address: %s", result.Record.Address)
	}
}

func TestLocalFolderServiceListLocalFoldersIncludesCloudMount(t *testing.T) {
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
			id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
		) VALUES (
			'cloud-node-1', 'cloud-node-1', '115 云归档', 'CLOUD', '115', '/MareArchive', 'QR', '115 云归档', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-cloud-1', 'mount-cloud-1', 'photo', '商业摄影资产库', 'cloud-node-1', '云盘挂载', 'CLOUD_FOLDER', 'READ_WRITE',
			'/MareArchive/Projects/Shanghai', 'Projects/Shanghai', 'NEVER', 'MANUAL', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-1', 'mount-cloud-1', 'IDLE', 'AUTHORIZED', 'ONLINE', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount runtime: %v", err)
	}

	mountService := NewLocalFolderService(pool)
	items, err := mountService.ListLocalFolders(ctx)
	if err != nil {
		t.Fatalf("list local folders: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(items))
	}
	if items[0].FolderType != "网盘" {
		t.Fatalf("expected folder type 网盘, got %s", items[0].FolderType)
	}
	if items[0].Address != "/MareArchive/Projects/Shanghai" {
		t.Fatalf("unexpected cloud mount address: %s", items[0].Address)
	}
}

func TestLocalFolderServiceConnectionTestSupportsCloudMount(t *testing.T) {
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
			id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
		) VALUES (
			'cloud-node-1', 'cloud-node-1', '115 云归档', 'CLOUD', '115', '/MareArchive', 'QR', '115 云归档', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (
			id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
		) VALUES (
			'cred-1', 'cloud-node-1', 'TOKEN', $1, 'tv', 'CONFIGURED', $2, $2
		)
	`, "cipher::UID=uid-1; CID=cid-1", now); err != nil {
		t.Fatalf("insert cloud credential: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'QR', '/MareArchive', '{"cloudName":"115","cloudUserName":"mare-user","cloudPath":"/115open(123)/MareArchive"}'::jsonb, $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-cloud-1', 'mount-cloud-1', 'photo', '商业摄影资产库', 'cloud-node-1', '云盘挂载', 'CLOUD_FOLDER', 'READ_WRITE',
			'/MareArchive/Projects/Shanghai', 'Projects/Shanghai', 'NEVER', 'MANUAL', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-1', 'mount-cloud-1', 'IDLE', 'UNKNOWN', 'UNKNOWN', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount runtime: %v", err)
	}

	mountService := NewLocalFolderService(pool)
	mountService.cipher = fakeCredentialCipher{}
	mountService.SetIntegrationService(fakeCloudIntegration{driver: &fakeCloudProviderDriver{}})

	response, err := mountService.RunLocalFolderConnectionTest(ctx, []string{"mount-cloud-1"})
	if err != nil {
		t.Fatalf("run cloud mount connection test: %v", err)
	}

	if len(response.Results) != 1 || response.Results[0].OverallTone != "success" {
		t.Fatalf("unexpected response: %+v", response.Results)
	}
}

func TestLocalFolderServiceRunSingleMountScanSupportsCloudMount(t *testing.T) {
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
			id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
		) VALUES (
			'cloud-node-1', 'cloud-node-1', '115 云归档', 'CLOUD', '115', '/MareArchive', 'QR', '115 云归档', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (
			id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
		) VALUES (
			'cred-1', 'cloud-node-1', 'TOKEN', $1, 'tv', 'CONFIGURED', $2, $2
		)
	`, "cipher::UID=uid-1; CID=cid-1", now); err != nil {
		t.Fatalf("insert cloud credential: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'QR', '/MareArchive', '{"cloudName":"115","cloudUserName":"mare-user","cloudPath":"/115open(123)/MareArchive"}'::jsonb, $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-cloud-1', 'mount-cloud-1', 'photo', '商业摄影资产库', 'cloud-node-1', '云盘挂载', 'CLOUD_FOLDER', 'READ_WRITE',
			'/MareArchive/Projects/Shanghai', 'Projects/Shanghai', 'NEVER', 'MANUAL', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-1', 'mount-cloud-1', 'IDLE', 'UNKNOWN', 'UNKNOWN', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount runtime: %v", err)
	}

	mountService := NewLocalFolderService(pool)
	mountService.cipher = fakeCredentialCipher{}
	mountService.SetIntegrationService(fakeCloudIntegration{driver: &fakeCloudProviderDriver{}})

	if err := mountService.RunSingleMountScan(ctx, "mount-cloud-1"); err != nil {
		t.Fatalf("run cloud mount scan: %v", err)
	}

	var scanStatus string
	var authStatus string
	var healthStatus string
	var lastScanSummary string
	if err := pool.QueryRow(ctx, `
		SELECT scan_status, auth_status, health_status, COALESCE(last_scan_summary, '')
		FROM mount_runtime
		WHERE mount_id = 'mount-cloud-1'
	`).Scan(&scanStatus, &authStatus, &healthStatus, &lastScanSummary); err != nil {
		t.Fatalf("load mount runtime: %v", err)
	}
	if scanStatus != "SUCCESS" {
		t.Fatalf("expected scan status SUCCESS, got %s", scanStatus)
	}
	if authStatus != "AUTHORIZED" {
		t.Fatalf("expected auth status AUTHORIZED, got %s", authStatus)
	}
	if healthStatus != "ONLINE" {
		t.Fatalf("expected health status ONLINE, got %s", healthStatus)
	}
	if lastScanSummary == "" {
		t.Fatalf("expected scan summary to be persisted")
	}

	var historyCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mount_scan_histories
		WHERE mount_id = 'mount-cloud-1'
	`).Scan(&historyCount); err != nil {
		t.Fatalf("count scan histories: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("expected 1 scan history, got %d", historyCount)
	}

	var replicaCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM asset_replicas
		WHERE mount_id = 'mount-cloud-1'
		  AND replica_state = 'AVAILABLE'
	`).Scan(&replicaCount); err != nil {
		t.Fatalf("count cloud replicas: %v", err)
	}
	if replicaCount != 1 {
		t.Fatalf("expected 1 cloud replica after first-level scan, got %d", replicaCount)
	}

	var directoryPresenceCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM directory_presences
		WHERE mount_id = 'mount-cloud-1'
		  AND presence_state = 'PRESENT'
	`).Scan(&directoryPresenceCount); err != nil {
		t.Fatalf("count cloud directory presences: %v", err)
	}
	if directoryPresenceCount < 2 {
		t.Fatalf("expected at least 2 directory presences(root + first-level dirs), got %d", directoryPresenceCount)
	}
}
