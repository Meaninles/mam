package assets_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/assets"
	"mare/services/center/internal/db"
	"mare/services/center/internal/storage"
	assetdto "mare/shared/contracts/dto/asset"
	storagedto "mare/shared/contracts/dto/storage"
)

func TestMountScanIndexesFilesForFileCenterQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rootDir := t.TempDir()
	localNodes := storage.NewLocalFolderService(pool)
	node, err := localNodes.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "本地素材根目录",
		RootPath: rootDir,
		Notes:    "集成测试",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}

	mount, err := localNodes.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "商业摄影原片库",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "原片",
		Notes:           "测试挂载",
	})
	if err != nil {
		t.Fatalf("save mount: %v", err)
	}

	sourceDir := filepath.Join(rootDir, "原片")
	if err := os.MkdirAll(filepath.Join(sourceDir, "已修图"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "cover.jpg"), []byte("cover-image"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "已修图", "final.txt"), []byte("final-doc"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	if _, err := localNodes.RunLocalFolderScan(ctx, []string{mount.Record.ID}); err != nil {
		t.Fatalf("run scan: %v", err)
	}

	service := assets.NewService(pool)

	libraries, err := service.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("list libraries: %v", err)
	}
	if len(libraries) != 1 || libraries[0].ID != "photo" {
		t.Fatalf("expected photo library, got %#v", libraries)
	}

	root, err := service.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
		Page:         1,
		PageSize:     20,
		FileType:     "全部",
		StatusFilter: "全部",
		SortValue:    "修改时间",
		SortDirection: "desc",
	})
	if err != nil {
		t.Fatalf("browse root: %v", err)
	}

	if root.Total != 2 {
		t.Fatalf("expected two root items, got %d", root.Total)
	}
	var topFolder *assetdto.EntryRecord
	for index := range root.Items {
		if root.Items[index].Name == "已修图" && root.Items[index].Type == "folder" {
			topFolder = &root.Items[index]
			break
		}
	}
	if topFolder == nil {
		t.Fatalf("expected 已修图 folder at root, got %#v", root.Items)
	}
	if len(topFolder.Endpoints) != 1 || topFolder.Endpoints[0].Name != "商业摄影原片库" {
		t.Fatalf("expected mount endpoint, got %#v", topFolder.Endpoints)
	}
	if topFolder.Endpoints[0].State != "已同步" {
		t.Fatalf("expected folder synced state, got %#v", topFolder.Endpoints[0].State)
	}

	var coverID string
	for _, item := range root.Items {
		if item.Name == "cover.jpg" {
			coverID = item.ID
		}
	}
	if coverID == "" {
		t.Fatalf("expected cover.jpg at root, got %#v", root.Items)
	}

	folderID := topFolder.ID
	child, err := service.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
		ParentID:      &folderID,
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "名称",
		SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse folder: %v", err)
	}

	if child.Total != 1 {
		t.Fatalf("expected one child entry, got %d", child.Total)
	}

	var finalID string
	for _, item := range child.Items {
		if item.Name == "final.txt" {
			finalID = item.ID
		}
	}
	if finalID == "" {
		t.Fatalf("expected final.txt in child items, got %#v", child.Items)
	}

	detail, err := service.LoadEntry(ctx, coverID)
	if err != nil {
		t.Fatalf("load detail: %v", err)
	}
	if detail == nil || detail.Path != "商业摄影资产库 / cover.jpg" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if len(detail.Endpoints) != 1 || detail.Endpoints[0].Name != "商业摄影原片库" {
		t.Fatalf("expected mount-based endpoint detail, got %#v", detail.Endpoints)
	}
}

func TestCreateLibraryCreatesRootDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := assets.NewService(pool)
	result, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "手动测试资产库"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	if result.Library.Name != "手动测试资产库" {
		t.Fatalf("unexpected library name: %#v", result.Library)
	}

	root, err := service.BrowseLibrary(ctx, result.Library.ID, assetdto.BrowseQuery{
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "修改时间",
		SortDirection: "desc",
	})
	if err != nil {
		t.Fatalf("browse library: %v", err)
	}

	if len(root.Breadcrumbs) != 1 || root.Breadcrumbs[0].Label != "手动测试资产库" {
		t.Fatalf("unexpected breadcrumbs: %#v", root.Breadcrumbs)
	}
	if root.Total != 0 {
		t.Fatalf("expected empty new library, got total=%d", root.Total)
	}
}

func TestCreateDirectoryPersistsAndAppearsInBrowseResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := assets.NewService(pool)
	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "目录测试资产库"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	rootDir := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "本地素材根目录",
		RootPath: rootDir,
		Notes:    "目录测试",
	})
	if err != nil {
		t.Fatalf("save node: %v", err)
	}

	mount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "目录测试挂载",
		LibraryID:       created.Library.ID,
		LibraryName:     created.Library.Name,
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "素材",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save mount: %v", err)
	}

	parentID := "dir-root-" + created.Library.ID
	result, err := service.CreateDirectory(ctx, created.Library.ID, assetdto.CreateDirectoryRequest{
		ParentID: &parentID,
		Name:     "新建目录",
	})
	if err != nil {
		t.Fatalf("create directory: %v", err)
	}

	if result.Entry.Name != "新建目录" {
		t.Fatalf("unexpected entry: %#v", result.Entry)
	}

	expectedPath := filepath.Join(rootDir, "素材", "新建目录")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected physical directory to exist: %v", err)
	}

	browse, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "名称",
		SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse: %v", err)
	}

	if len(browse.Items) != 1 {
		t.Fatalf("unexpected browse items: %#v", browse.Items)
	}
	_ = mount
}

func TestDeleteEmptyDirectoryRemovesPhysicalAndLogicalDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := assets.NewService(pool)
	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "删除目录测试"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	rootDir := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "删除目录测试节点",
		RootPath: rootDir,
		Notes:    "",
	})
	if err != nil {
		t.Fatalf("save node: %v", err)
	}

	_, err = localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "删除目录测试挂载",
		LibraryID:       created.Library.ID,
		LibraryName:     created.Library.Name,
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "素材",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save mount: %v", err)
	}

	parentID := "dir-root-" + created.Library.ID
	directory, err := service.CreateDirectory(ctx, created.Library.ID, assetdto.CreateDirectoryRequest{
		ParentID: &parentID,
		Name:     "待删除目录",
	})
	if err != nil {
		t.Fatalf("create directory: %v", err)
	}

	targetPath := filepath.Join(rootDir, "素材", "待删除目录")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected created physical directory: %v", err)
	}

	if _, err := service.DeleteEntry(ctx, directory.Entry.ID); err != nil {
		t.Fatalf("delete directory: %v", err)
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("expected physical directory removed, got err=%v", err)
	}

	browse, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "名称",
		SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse library: %v", err)
	}

	if len(browse.Items) != 0 {
		t.Fatalf("expected empty library root after delete, got %#v", browse.Items)
	}
}

func TestDeleteAssetRemovesPhysicalFileAndLogicalRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rootDir := t.TempDir()
	localNodes := storage.NewLocalFolderService(pool)
	node, err := localNodes.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "本地素材根目录",
		RootPath: rootDir,
		Notes:    "删除文件测试",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}

	mount, err := localNodes.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "商业摄影原片库",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "原片",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save mount: %v", err)
	}

	sourceDir := filepath.Join(rootDir, "原片")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	targetFile := filepath.Join(sourceDir, "cover.jpg")
	if err := os.WriteFile(targetFile, []byte("cover-image"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := localNodes.RunLocalFolderScan(ctx, []string{mount.Record.ID}); err != nil {
		t.Fatalf("run scan: %v", err)
	}

	service := assets.NewService(pool)
	root, err := service.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
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

	var coverID string
	for _, item := range root.Items {
		if item.Name == "cover.jpg" {
			coverID = item.ID
		}
	}
	if coverID == "" {
		t.Fatalf("expected cover.jpg at root, got %#v", root.Items)
	}

	if _, err := service.DeleteEntry(ctx, coverID); err != nil {
		t.Fatalf("delete asset: %v", err)
	}

	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Fatalf("expected physical file removed, got err=%v", err)
	}

	next, err := service.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "名称",
		SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse after delete: %v", err)
	}
	if len(next.Items) != 0 {
		t.Fatalf("expected file removed from root, got %#v", next.Items)
	}
}

func openTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pool, err := db.Open(ctx, isolatedSchemaDatabaseURL(t, ctx))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func isolatedSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	baseURL := "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
	schema := fmt.Sprintf("test_assets_%d", time.Now().UnixNano())

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

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
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
