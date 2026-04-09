package storage

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
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

	mountService := NewLocalFolderService(pool)
	mountService.cipher = fakeCredentialCipher{}
	mountService.cloud = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/files") {
				query := request.URL.Query()
				switch query.Get("cid") {
				case "0":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				case "100":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				case "101":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				default:
					t.Fatalf("unexpected list cid: %s", query.Get("cid"))
					return nil, nil
				}
			}
			if request.Method == http.MethodPost && strings.Contains(request.URL.Path, "/files/add") {
				body, _ := io.ReadAll(request.Body)
				values, err := url.ParseQuery(string(body))
				if err != nil {
					t.Fatalf("parse create folder body: %v", err)
				}
				switch values.Get("cname") {
				case "MareArchive":
					return jsonHTTPResponse(`{"state":true,"cid":"100"}`), nil
				case "Projects":
					return jsonHTTPResponse(`{"state":true,"cid":"101"}`), nil
				case "Shanghai":
					return jsonHTTPResponse(`{"state":true,"cid":"102"}`), nil
				default:
					t.Fatalf("unexpected create folder name: %s", values.Get("cname"))
					return nil, nil
				}
			}
			t.Fatalf("unexpected cloud request: %s %s", request.Method, request.URL.String())
			return nil, nil
		}),
	}

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
	mountService.cloud = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/login_devices") {
				return jsonHTTPResponse(`{"state":true,"code":0,"data":{"devices":[]}}`), nil
			}
			if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/files") {
				query := request.URL.Query()
				switch query.Get("cid") {
				case "0":
					return jsonHTTPResponse(`{"state":true,"data":[{"cid":"100","n":"MareArchive"}]}`), nil
				case "100":
					return jsonHTTPResponse(`{"state":true,"data":[{"cid":"101","n":"Projects"}]}`), nil
				case "101":
					return jsonHTTPResponse(`{"state":true,"data":[{"cid":"102","n":"Shanghai"}]}`), nil
				case "102":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				default:
					t.Fatalf("unexpected cid: %s", query.Get("cid"))
					return nil, nil
				}
			}
			t.Fatalf("unexpected cloud request: %s %s", request.Method, request.URL.String())
			return nil, nil
		}),
	}

	response, err := mountService.RunLocalFolderConnectionTest(ctx, []string{"mount-cloud-1"})
	if err != nil {
		t.Fatalf("run cloud mount connection test: %v", err)
	}

	if len(response.Results) != 1 || response.Results[0].OverallTone != "success" {
		t.Fatalf("unexpected response: %+v", response.Results)
	}
}
