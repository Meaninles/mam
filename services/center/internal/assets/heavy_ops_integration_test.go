package assets_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mare/services/center/internal/assets"
	"mare/services/center/internal/db"
	"mare/services/center/internal/storage"
	assetdto "mare/shared/contracts/dto/asset"
	storagedto "mare/shared/contracts/dto/storage"
)

func TestHeavyOperationsSyncDeleteReplicaAndDeleteAsset(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)

	sourceNode, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "源节点",
		RootPath: sourceRoot,
		Notes:    "source",
	})
	if err != nil {
		t.Fatalf("save source node: %v", err)
	}
	targetNode, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "目标节点",
		RootPath: targetRoot,
		Notes:    "target",
	})
	if err != nil {
		t.Fatalf("save target node: %v", err)
	}

	sourceMount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "源挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          sourceNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "source",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save source mount: %v", err)
	}
	targetMount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "目标挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          targetNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "target",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save target mount: %v", err)
	}

	sourceDir := filepath.Join(sourceRoot, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	sourceFile := filepath.Join(sourceDir, "cover.jpg")
	if err := os.WriteFile(sourceFile, []byte("cover-image"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	expectedModifiedAt := time.Date(2024, 3, 14, 9, 26, 53, 0, time.UTC)
	if err := os.Chtimes(sourceFile, expectedModifiedAt, expectedModifiedAt); err != nil {
		t.Fatalf("set source mtime: %v", err)
	}

	if _, err := localFolders.RunLocalFolderScan(ctx, []string{sourceMount.Record.ID}); err != nil {
		t.Fatalf("run source scan: %v", err)
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
	if len(root.Items) != 1 {
		t.Fatalf("expected one indexed asset, got %#v", root.Items)
	}
	assetID := root.Items[0].ID

	replicatePlan, err := service.PrepareReplicatePlan(ctx, assetdto.CreateReplicateJobRequest{
		EntryIDs:     []string{assetID},
		EndpointName: targetMount.Record.Name,
	})
	if err != nil {
		t.Fatalf("prepare replicate plan: %v", err)
	}
	if len(replicatePlan.Items) != 1 {
		t.Fatalf("expected one replicate item, got %#v", replicatePlan)
	}

	if err := service.ExecuteReplicaSync(ctx, replicatePlan.Items[0].SourceReplicaID, replicatePlan.Items[0].TargetMountID); err != nil {
		t.Fatalf("execute replica sync: %v", err)
	}

	targetFile := filepath.Join(targetRoot, "target", "cover.jpg")
	if content, err := os.ReadFile(targetFile); err != nil {
		t.Fatalf("read replicated file: %v", err)
	} else if string(content) != "cover-image" {
		t.Fatalf("unexpected target content: %q", string(content))
	}
	if info, err := os.Stat(targetFile); err != nil {
		t.Fatalf("stat replicated file: %v", err)
	} else if !info.ModTime().UTC().Equal(expectedModifiedAt) {
		t.Fatalf("expected target mtime %s, got %s", expectedModifiedAt.Format(time.RFC3339), info.ModTime().UTC().Format(time.RFC3339))
	}

	detailAfterSync, err := service.LoadEntry(ctx, assetID)
	if err != nil {
		t.Fatalf("load detail after sync: %v", err)
	}
	if countSyncedEndpoints(detailAfterSync.Endpoints) != 2 {
		t.Fatalf("expected two synced endpoints, got %#v", detailAfterSync.Endpoints)
	}

	deleteReplicaPlan, err := service.PrepareDeleteReplicaPlan(ctx, assetdto.CreateDeleteReplicaJobRequest{
		EntryIDs:     []string{assetID},
		EndpointName: targetMount.Record.Name,
	})
	if err != nil {
		t.Fatalf("prepare delete replica plan: %v", err)
	}
	if len(deleteReplicaPlan.Items) != 1 {
		t.Fatalf("expected one replica delete item, got %#v", deleteReplicaPlan)
	}
	if err := service.ExecuteReplicaDeletion(ctx, deleteReplicaPlan.Items[0].ReplicaID); err != nil {
		t.Fatalf("execute replica deletion: %v", err)
	}
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Fatalf("expected target file deleted, got err=%v", err)
	}

	detailAfterReplicaDelete, err := service.LoadEntry(ctx, assetID)
	if err != nil {
		t.Fatalf("load detail after replica delete: %v", err)
	}
	if countSyncedEndpoints(detailAfterReplicaDelete.Endpoints) != 1 {
		t.Fatalf("expected one remaining synced endpoint, got %#v", detailAfterReplicaDelete.Endpoints)
	}

	deleteAssetPlan, err := service.PrepareDeleteAssetPlan(ctx, assetdto.CreateDeleteAssetJobRequest{
		EntryIDs: []string{assetID},
	})
	if err != nil {
		t.Fatalf("prepare delete asset plan: %v", err)
	}
	if len(deleteAssetPlan.Items) != 1 {
		t.Fatalf("expected one asset delete item, got %#v", deleteAssetPlan)
	}
	if deleteAssetPlan.Items[0].AssetID == nil {
		t.Fatalf("expected asset delete item, got %#v", deleteAssetPlan.Items[0])
	}
	if err := service.ExecuteAssetDeletion(ctx, *deleteAssetPlan.Items[0].AssetID); err != nil {
		t.Fatalf("execute asset deletion: %v", err)
	}
	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Fatalf("expected source file deleted, got err=%v", err)
	}
	if _, err := service.LoadEntry(ctx, assetID); err == nil {
		t.Fatalf("expected asset deleted from read model")
	}
}

func TestPrepareDeleteAssetPlanIncludesSelectedDirectories(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rootDir := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "本地节点",
		RootPath: rootDir,
		Notes:    "",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}
	mount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "本地源挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "source",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save local mount: %v", err)
	}

	nestedDir := filepath.Join(rootDir, "source", "clip")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "shot-01.mov"), []byte("video"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	if _, err := localFolders.RunLocalFolderScan(ctx, []string{mount.Record.ID}); err != nil {
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
	if len(root.Items) != 1 || root.Items[0].Type != "folder" {
		t.Fatalf("expected one folder, got %#v", root.Items)
	}

	plan, err := service.PrepareDeleteAssetPlan(ctx, assetdto.CreateDeleteAssetJobRequest{
		EntryIDs: []string{root.Items[0].ID},
	})
	if err != nil {
		t.Fatalf("prepare delete asset plan: %v", err)
	}

	var assetItems int
	var directoryItems int
	for _, item := range plan.Items {
		if item.AssetID != nil {
			assetItems++
		}
		if item.DirectoryID != nil {
			directoryItems++
		}
	}
	if assetItems != 1 || directoryItems != 1 {
		t.Fatalf("expected one asset item and one directory item, got %#v", plan.Items)
	}
}

func countSyncedEndpoints(endpoints []assetdto.EntryEndpoint) int {
	count := 0
	for _, endpoint := range endpoints {
		if endpoint.State == "已同步" {
			count++
		}
	}
	return count
}

func TestPrepareDeleteAssetPlanRejectsCloudReplica(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rootDir := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)
	localNode, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "本地节点",
		RootPath: rootDir,
		Notes:    "",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}
	localMount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "本地源挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          localNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "source",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save local mount: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (
			id, code, name, node_type, access_mode, enabled, created_at, updated_at
		) VALUES (
			'cloud-node-1', 'cloud-node-1', '云盘节点', 'CLOUD', 'DIRECT', true, $1, $1
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
			'/cloud', '/', 'NEVER', 'MANUAL', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}

	sourceDir := filepath.Join(rootDir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "cover.jpg"), []byte("cover-image"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	if _, err := localFolders.RunLocalFolderScan(ctx, []string{localMount.Record.ID}); err != nil {
		t.Fatalf("run local scan: %v", err)
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
	if len(root.Items) != 1 {
		t.Fatalf("expected one asset, got %#v", root.Items)
	}
	assetID := root.Items[0].ID

	if _, err := pool.Exec(ctx, `
		INSERT INTO asset_replicas (
			id, asset_id, mount_id, physical_path, size_bytes, replica_state, sync_state, verification_state,
			last_seen_at, created_at, updated_at
		) VALUES (
			'replica-cloud-1', $1, 'mount-cloud-1', '/cloud/cover.jpg', 11, 'AVAILABLE', 'IN_SYNC', 'UNVERIFIED',
			$2, $2, $2
		)
	`, assetID, now); err != nil {
		t.Fatalf("insert cloud replica: %v", err)
	}

	plan, err := service.PrepareDeleteAssetPlan(ctx, assetdto.CreateDeleteAssetJobRequest{
		EntryIDs: []string{assetID},
	})
	if err != nil {
		t.Fatalf("prepare delete asset plan with cloud replica: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("expected one delete item, got %+v", plan.Items)
	}
}
