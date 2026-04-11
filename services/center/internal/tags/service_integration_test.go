package tags_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/assets"
	"mare/services/center/internal/db"
	"mare/services/center/internal/storage"
	"mare/services/center/internal/tags"
	assetdto "mare/shared/contracts/dto/asset"
	storagedto "mare/shared/contracts/dto/storage"
	tagdto "mare/shared/contracts/dto/tag"
)

func TestManagementSnapshotReflectsCreatedTagsAndUsage(t *testing.T) {
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

	assetService, tagService, fileID := prepareTaggedAsset(t, ctx, pool)

	groupResult, err := tagService.CreateGroup(ctx, tagdto.CreateGroupRequest{Name: "项目语义"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	tagResult, err := tagService.CreateTag(ctx, tagdto.CreateTagRequest{
		Name:       "直播切片",
		GroupID:    groupResult.GroupID,
		LibraryIDs: []string{"photo"},
		IsPinned:   true,
	})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tagResult.TagID == "" {
		t.Fatal("expected created tag id")
	}

	if _, err := assetService.UpdateAnnotations(ctx, fileID, assetdto.UpdateAnnotationsRequest{
		Rating:     3,
		ColorLabel: "黄标",
		Tags:       []string{"直播切片"},
	}); err != nil {
		t.Fatalf("update annotations: %v", err)
	}

	snapshot, err := tagService.LoadManagementSnapshot(ctx, "")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}

	if snapshot.Overview.TotalTags != 1 {
		t.Fatalf("expected totalTags=1, got %#v", snapshot.Overview)
	}
	if snapshot.Overview.UsedTagCount != 1 {
		t.Fatalf("expected usedTagCount=1, got %#v", snapshot.Overview)
	}
	if len(snapshot.Groups) < 2 {
		t.Fatalf("expected default group plus created group, got %#v", snapshot.Groups)
	}
	if len(snapshot.Tags) != 1 {
		t.Fatalf("expected one tag, got %#v", snapshot.Tags)
	}
	if snapshot.Tags[0].Name != "直播切片" || snapshot.Tags[0].UsageCount != 1 {
		t.Fatalf("unexpected tag snapshot: %#v", snapshot.Tags[0])
	}
	if len(snapshot.Tags[0].LinkedLibraryIDs) != 1 || snapshot.Tags[0].LinkedLibraryIDs[0] != "photo" {
		t.Fatalf("unexpected linked libraries: %#v", snapshot.Tags[0].LinkedLibraryIDs)
	}

	suggestions, err := tagService.ListSuggestions(ctx, "直播", ptr("photo"))
	if err != nil {
		t.Fatalf("list suggestions: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Name != "直播切片" {
		t.Fatalf("unexpected suggestions: %#v", suggestions)
	}
}

func TestNewTagWithoutScopesReturnsEmptySlicesInsteadOfNull(t *testing.T) {
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

	_, tagService, _ := prepareTaggedAsset(t, ctx, pool)
	groupResult, err := tagService.CreateGroup(ctx, tagdto.CreateGroupRequest{Name: "项目语义"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := tagService.CreateTag(ctx, tagdto.CreateTagRequest{
		Name:       "无作用域标签",
		GroupID:    groupResult.GroupID,
		LibraryIDs: nil,
	}); err != nil {
		t.Fatalf("create tag: %v", err)
	}

	snapshot, err := tagService.LoadManagementSnapshot(ctx, "无作用域")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(snapshot.Tags) != 1 {
		t.Fatalf("expected one tag, got %#v", snapshot.Tags)
	}
	if snapshot.Tags[0].LibraryIDs == nil {
		t.Fatalf("expected empty libraryIds slice, got nil")
	}
	if snapshot.Tags[0].LinkedLibraryIDs == nil {
		t.Fatalf("expected empty linkedLibraryIds slice, got nil")
	}
}

func TestMergeAndDeleteTagUpdatesAssetLinks(t *testing.T) {
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

	assetService, tagService, fileID := prepareTaggedAsset(t, ctx, pool)
	groupResult, err := tagService.CreateGroup(ctx, tagdto.CreateGroupRequest{Name: "项目语义"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	sourceTag, err := tagService.CreateTag(ctx, tagdto.CreateTagRequest{
		Name:       "直播切片",
		GroupID:    groupResult.GroupID,
		LibraryIDs: []string{"photo"},
	})
	if err != nil {
		t.Fatalf("create source tag: %v", err)
	}
	targetTag, err := tagService.CreateTag(ctx, tagdto.CreateTagRequest{
		Name:       "客户精选",
		GroupID:    groupResult.GroupID,
		LibraryIDs: []string{"photo"},
	})
	if err != nil {
		t.Fatalf("create target tag: %v", err)
	}

	if _, err := assetService.UpdateAnnotations(ctx, fileID, assetdto.UpdateAnnotationsRequest{
		Rating:     0,
		ColorLabel: "无",
		Tags:       []string{"直播切片"},
	}); err != nil {
		t.Fatalf("update annotations: %v", err)
	}

	if _, err := tagService.MergeTag(ctx, sourceTag.TagID, tagdto.MergeTagRequest{TargetID: targetTag.TagID}); err != nil {
		t.Fatalf("merge tag: %v", err)
	}

	detail, err := assetService.LoadEntry(ctx, fileID)
	if err != nil {
		t.Fatalf("load detail after merge: %v", err)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "客户精选" {
		t.Fatalf("expected merged tag on asset, got %#v", detail.Tags)
	}

	snapshot, err := tagService.LoadManagementSnapshot(ctx, "")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(snapshot.Tags) != 1 || snapshot.Tags[0].Name != "客户精选" {
		t.Fatalf("expected only target tag after merge, got %#v", snapshot.Tags)
	}

	if _, err := tagService.DeleteTag(ctx, targetTag.TagID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}

	detail, err = assetService.LoadEntry(ctx, fileID)
	if err != nil {
		t.Fatalf("load detail after delete: %v", err)
	}
	if len(detail.Tags) != 0 {
		t.Fatalf("expected asset tags cleared after delete, got %#v", detail.Tags)
	}
}

func prepareTaggedAsset(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (*assets.Service, *tags.Service, string) {
	t.Helper()

	rootDir := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "标签测试节点",
		RootPath: rootDir,
		Notes:    "tag test",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}

	mount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "标签测试挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "source",
		Notes:           "tag test",
	})
	if err != nil {
		t.Fatalf("save mount: %v", err)
	}

	sourceDir := filepath.Join(rootDir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "cover.jpg"), []byte("cover-image"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := localFolders.RunLocalFolderScan(ctx, []string{mount.Record.ID}); err != nil {
		t.Fatalf("run scan: %v", err)
	}

	assetService := assets.NewService(pool)
	tagService := tags.NewService(pool)
	root, err := assetService.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
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
	if len(root.Items) != 1 {
		t.Fatalf("expected one asset, got %#v", root.Items)
	}
	return assetService, tagService, root.Items[0].ID
}

func ptr(value string) *string {
	return &value
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
	schema := fmt.Sprintf("test_tags_%d", time.Now().UnixNano())

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
