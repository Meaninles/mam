package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	storagedto "mare/shared/contracts/dto/storage"
)

type fakeCredentialCipher struct{}

func (fakeCredentialCipher) Encrypt(plaintext string) (string, error) {
	return "cipher::" + plaintext, nil
}

func (fakeCredentialCipher) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "cipher::"), nil
}

type failingCredentialCipher struct{}

func (failingCredentialCipher) Encrypt(plaintext string) (string, error) {
	return "broken::" + plaintext, nil
}

func (failingCredentialCipher) Decrypt(string) (string, error) {
	return "", fmt.Errorf("credential decode failed")
}

type fakeNASConnector struct {
	probe              nasConnectionProbe
	ensuredDirectories []string
}

func (f *fakeNASConnector) Test(context.Context, string, string, string) (nasConnectionProbe, error) {
	return f.probe, nil
}

func (f *fakeNASConnector) EnsureDirectory(_ context.Context, address string, _ string, _ string) error {
	f.ensuredDirectories = append(f.ensuredDirectories, address)
	return nil
}

func TestNASNodeServiceSaveAndListNodes(t *testing.T) {
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

	service := NewNASNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.connector = &fakeNASConnector{}
	service.now = func() time.Time {
		return time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC)
	}

	result, err := service.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
		Notes:    "主 NAS",
	})
	if err != nil {
		t.Fatalf("save nas node: %v", err)
	}

	if result.Record.AccessMode != "SMB" {
		t.Fatalf("expected access mode SMB, got %s", result.Record.AccessMode)
	}

	var secretCiphertext string
	if err := pool.QueryRow(ctx, `
		SELECT secret_ciphertext
		FROM storage_node_credentials
		WHERE storage_node_id = $1
	`, result.Record.ID).Scan(&secretCiphertext); err != nil {
		t.Fatalf("load nas credential: %v", err)
	}
	if secretCiphertext != "cipher::secret" {
		t.Fatalf("unexpected ciphertext: %s", secretCiphertext)
	}

	items, err := service.ListNasNodes(ctx)
	if err != nil {
		t.Fatalf("list nas nodes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 nas node, got %d", len(items))
	}
	if items[0].PasswordHint == "" {
		t.Fatal("expected password hint to be populated")
	}
}

func TestNASNodeServiceUpdateKeepsExistingPasswordWhenBlank(t *testing.T) {
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

	service := NewNASNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.connector = &fakeNASConnector{}

	created, err := service.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create nas node: %v", err)
	}

	if _, err := service.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		ID:       created.Record.ID,
		Name:     "影像 NAS 01（新）",
		Address:  `\\192.168.10.21\media`,
		Username: "mare-sync",
		Password: "",
	}); err != nil {
		t.Fatalf("update nas node: %v", err)
	}

	var secretCiphertext string
	if err := pool.QueryRow(ctx, `
		SELECT secret_ciphertext
		FROM storage_node_credentials
		WHERE storage_node_id = $1
	`, created.Record.ID).Scan(&secretCiphertext); err != nil {
		t.Fatalf("load nas credential: %v", err)
	}
	if secretCiphertext != "cipher::secret" {
		t.Fatalf("expected existing ciphertext to be retained, got %s", secretCiphertext)
	}
}

func TestNASNodeServiceDeleteRejectsMountedNode(t *testing.T) {
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

	service := NewNASNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.connector = &fakeNASConnector{}

	created, err := service.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create nas node: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-1', 'mount-1', 'photo', '商业摄影资产库', $1, '挂载 1', 'NAS_SHARE', 'READ_WRITE',
			'\\192.168.10.20\media\projects', '/', 'NEVER', 'MANUAL', true, 0, $2, $2
		)
	`, created.Record.ID, now); err != nil {
		t.Fatalf("insert mount: %v", err)
	}

	if _, err := service.DeleteNasNode(ctx, created.Record.ID); err == nil {
		t.Fatal("expected delete to reject mounted nas node")
	}
}

func TestNASNodeServiceConnectionTestUpdatesRuntime(t *testing.T) {
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

	service := NewNASNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.connector = &fakeNASConnector{
		probe: nasConnectionProbe{
			OverallTone:  "success",
			Summary:      "NAS 连接测试通过",
			Suggestion:   "可继续配置挂载关系",
			Checks:       []storagedto.ConnectionCheck{{Label: "SMB 鉴权", Status: "success", Detail: "账号认证通过"}},
			HealthStatus: "ONLINE",
			AuthStatus:   "AUTHORIZED",
		},
	}

	created, err := service.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create nas node: %v", err)
	}

	response, err := service.RunNasNodeConnectionTest(ctx, []string{created.Record.ID})
	if err != nil {
		t.Fatalf("run connection test: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].OverallTone != "success" {
		t.Fatalf("unexpected response: %+v", response.Results)
	}

	var authStatus string
	var healthStatus string
	if err := pool.QueryRow(ctx, `
		SELECT auth_status, health_status
		FROM storage_node_runtime
		WHERE storage_node_id = $1
	`, created.Record.ID).Scan(&authStatus, &healthStatus); err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if authStatus != "AUTHORIZED" || healthStatus != "ONLINE" {
		t.Fatalf("unexpected runtime status: auth=%s health=%s", authStatus, healthStatus)
	}
}

func TestLocalFolderServiceSaveMountSupportsNASNode(t *testing.T) {
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

	nasService := NewNASNodeService(pool)
	nasService.cipher = fakeCredentialCipher{}
	nasService.connector = &fakeNASConnector{}

	nasNode, err := nasService.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create nas node: %v", err)
	}

	mountService := NewLocalFolderService(pool)
	mountService.cipher = fakeCredentialCipher{}
	fakeConnector := &fakeNASConnector{}
	mountService.nas = fakeConnector

	result, err := mountService.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "NAS 挂载",
		LibraryID:       "video",
		LibraryName:     "视频工作流资产库",
		NodeID:          nasNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    `projects\raw`,
	})
	if err != nil {
		t.Fatalf("save nas mount: %v", err)
	}

	if result.Record.FolderType != "NAS" {
		t.Fatalf("expected folder type NAS, got %s", result.Record.FolderType)
	}
	if result.Record.Address != `\\192.168.10.20\media\projects\raw` {
		t.Fatalf("unexpected nas mount address: %s", result.Record.Address)
	}
	if len(fakeConnector.ensuredDirectories) != 1 || fakeConnector.ensuredDirectories[0] != `\\192.168.10.20\media\projects\raw` {
		t.Fatalf("expected nas directory creation for saved mount, got %+v", fakeConnector.ensuredDirectories)
	}
}

func TestNASNodeServiceConnectionTestReturnsBusinessFailureWhenCredentialUnreadable(t *testing.T) {
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

	service := NewNASNodeService(pool)
	service.cipher = fakeCredentialCipher{}

	created, err := service.SaveNasNode(ctx, storagedto.SaveNasNodeRequest{
		Name:     "影像 NAS 01",
		Address:  `\\192.168.10.20\media`,
		Username: "mare-sync",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create nas node: %v", err)
	}

	service.cipher = failingCredentialCipher{}
	response, err := service.RunNasNodeConnectionTest(ctx, []string{created.Record.ID})
	if err != nil {
		t.Fatalf("run connection test: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].OverallTone != "critical" {
		t.Fatalf("unexpected response: %+v", response.Results)
	}
}

func openStorageTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	databaseURL := isolatedSchemaDatabaseURL(t, ctx)
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func isolatedSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	baseURL := "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
	schema := fmt.Sprintf("test_storage_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
	})

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func resetStorageSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS mount_scan_histories;
		DROP TABLE IF EXISTS mount_runtime;
		DROP TABLE IF EXISTS mounts;
		DROP TABLE IF EXISTS storage_node_runtime;
		DROP TABLE IF EXISTS storage_node_credentials;
		DROP TABLE IF EXISTS storage_nodes;
		DROP TABLE IF EXISTS agents;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}
