package assets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	assetdto "mare/shared/contracts/dto/asset"
)

type fakeUploadExecutor struct {
	ensured []string
	writes  map[string][]byte
}

func (f *fakeUploadExecutor) EnsureDirectory(_ context.Context, input pathExecutionContext) error {
	f.ensured = append(f.ensured, input.PhysicalPath)
	return nil
}

func (f *fakeUploadExecutor) WriteFile(_ context.Context, input pathExecutionContext) error {
	if f.writes == nil {
		f.writes = map[string][]byte{}
	}
	f.writes[input.PhysicalPath] = append([]byte(nil), input.FileContent...)
	return nil
}

func (f *fakeUploadExecutor) WriteStream(_ context.Context, input pathExecutionContext, reader io.Reader) error {
	if f.writes == nil {
		f.writes = map[string][]byte{}
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.writes[input.PhysicalPath] = content
	return nil
}

func (f *fakeUploadExecutor) DeleteFile(context.Context, pathExecutionContext) error {
	return nil
}

func (f *fakeUploadExecutor) DeleteDirectory(context.Context, pathExecutionContext) error {
	return nil
}

func (f *fakeUploadExecutor) StreamFile(_ context.Context, input pathExecutionContext, consume func(reader io.Reader) error) error {
	content := []byte(nil)
	if f.writes != nil {
		content = append(content, f.writes[input.PhysicalPath]...)
	}
	return consume(bytes.NewReader(content))
}

func (f *fakeUploadExecutor) StatFile(_ context.Context, input pathExecutionContext) (fileMetadata, error) {
	content := []byte(nil)
	if f.writes != nil {
		content = append(content, f.writes[input.PhysicalPath]...)
	}
	return fileMetadata{
		SizeBytes:  int64(len(content)),
		ModifiedAt: time.Now().UTC(),
	}, nil
}

func (f *fakeUploadExecutor) SetFileModifiedTime(context.Context, pathExecutionContext, time.Time) error {
	return nil
}

func TestUploadSelectionWritesFilesToNASMounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openUploadTestPool(t, ctx)
	defer pool.Close()
	resetUploadSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	fakeExecutor := &fakeUploadExecutor{}
	service := NewService(pool)
	service.executorResolver = func(nodeType string) (mountPathExecutor, error) {
		if nodeType == "NAS" {
			return fakeExecutor, nil
		}
		return nil, fmt.Errorf("unexpected node type %s", nodeType)
	}

	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "NAS 上传测试库"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (
			id, code, name, node_type, address, access_mode, enabled, created_at, updated_at
		) VALUES (
			'nas-node-1', 'nas-node-1', 'NAS 节点', 'NAS', '\\nas\share', 'SMB', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert nas node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (
			id, storage_node_id, credential_kind, username, secret_ciphertext, updated_at, created_at
		) VALUES (
			'nas-cred-1', 'nas-node-1', 'PASSWORD', 'mare', 'cipher', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert nas credential: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-nas-1', 'mount-nas-1', $1, $2, 'nas-node-1', 'NAS 可写挂载', 'NAS_SHARE', 'READ_WRITE',
			'\\nas\share\uploads', 'uploads', 'NEVER', 'MANUAL', true, 0, $3, $3
		)
	`, created.Library.ID, created.Library.Name, now); err != nil {
		t.Fatalf("insert nas mount: %v", err)
	}

	result, err := service.UploadSelection(ctx, created.Library.ID, assetdto.UploadSelectionRequest{
		Mode: "files",
		Files: []assetdto.UploadSelectionFile{
			{
				Name:         "brief.pdf",
				RelativePath: "brief.pdf",
				Size:         int64(len([]byte("brief"))),
				Content:      []byte("brief"),
			},
		},
	})
	if err != nil {
		t.Fatalf("upload selection: %v", err)
	}
	if result.CreatedCount != 1 {
		t.Fatalf("expected one uploaded file, got %#v", result)
	}

	if string(fakeExecutor.writes[`\\nas\share\uploads\brief.pdf`]) != "brief" {
		t.Fatalf("expected NAS upload write, got %#v", fakeExecutor.writes)
	}

	root, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "名称",
		SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse root: %v", err)
	}
	if len(root.Items) != 1 || root.Items[0].Name != "brief.pdf" {
		t.Fatalf("expected uploaded NAS file in browse result, got %#v", root.Items)
	}
}

func openUploadTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pool, err := db.Open(ctx, isolatedUploadSchemaDatabaseURL(t, ctx))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func isolatedUploadSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	baseURL := "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
	schema := fmt.Sprintf("test_assets_upload_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
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

func resetUploadSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS directory_tag_links;
		DROP TABLE IF EXISTS asset_tag_links;
		DROP TABLE IF EXISTS tag_library_scopes;
		DROP TABLE IF EXISTS tags;
		DROP TABLE IF EXISTS tag_groups;
		DROP TABLE IF EXISTS asset_metadata;
		DROP TABLE IF EXISTS directory_presences;
		DROP TABLE IF EXISTS asset_replicas;
		DROP TABLE IF EXISTS assets;
		DROP TABLE IF EXISTS library_directories;
		DROP TABLE IF EXISTS libraries;
		DROP TABLE IF EXISTS mount_scan_histories;
		DROP TABLE IF EXISTS mount_runtime;
		DROP TABLE IF EXISTS mounts;
		DROP TABLE IF EXISTS storage_node_runtime;
		DROP TABLE IF EXISTS storage_node_credentials;
		DROP TABLE IF EXISTS storage_nodes;
		DROP TABLE IF EXISTS agents;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
}
