package importing_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/assets"
	"mare/services/center/internal/db"
	"mare/services/center/internal/importing"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/storage"
	assetdto "mare/shared/contracts/dto/asset"
	importdto "mare/shared/contracts/dto/importing"
	storagedto "mare/shared/contracts/dto/storage"
)

const developmentDatabaseURL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"

func TestServiceRefreshesSessionsAndCompletesImportJob(t *testing.T) {
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
	secondTargetRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceRoot, "A001"), 0o755); err != nil {
		t.Fatalf("mkdir source nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "A001", "clip.mov"), []byte("clip-payload"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "导入目标本地节点",
		RootPath: targetRoot,
		Notes:    "integration",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}
	mount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "商业摄影原片库",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "原片",
		Notes:           "integration",
	})
	if err != nil {
		t.Fatalf("save local folder: %v", err)
	}
	secondNode, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "导入第二目标本地节点",
		RootPath: secondTargetRoot,
		Notes:    "integration",
	})
	if err != nil {
		t.Fatalf("save second local node: %v", err)
	}
	secondMount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "商业摄影第二备份",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          secondNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "原片2",
		Notes:           "integration",
	})
	if err != nil {
		t.Fatalf("save second local folder: %v", err)
	}

	jobService := jobs.NewService(pool)
	assetService := assets.NewService(pool)
	bridge := &fakeAgentBridge{
		sources: []importdto.SourceDescriptor{
			{
				DeviceKey:   "device-local-1",
				SourceType:  "LOCAL_DIRECTORY",
				DeviceLabel: "测试素材目录",
				DeviceType:  "本地目录",
				SourcePath:  sourceRoot,
				MountPath:   sourceRoot,
				ConnectedAt: time.Now().UTC().Format(time.RFC3339),
				LastSeenAt:  time.Now().UTC().Format(time.RFC3339),
			},
		},
		browse: importdto.BrowseResponse{
			Entries: []importdto.BrowseEntry{
				{
					EntryType:    "DIRECTORY",
					RelativePath: "A001",
					Name:         "A001",
					FileKind:     "文件夹",
					ModifiedAt:   time.Now().UTC().Format(time.RFC3339),
					HasChildren:  true,
				},
				{
					EntryType:    "FILE",
					RelativePath: "A001/clip.mov",
					Name:         "clip.mov",
					FileKind:     "视频",
					SizeBytes:    ptr(int64(12)),
					ModifiedAt:   time.Now().UTC().Format(time.RFC3339),
				},
			},
			Total:     2,
			Limit:     50,
			Offset:    0,
			HasMore:   false,
			ScannedAt: time.Now().UTC().Format(time.RFC3339),
		},
		sourceRoot: sourceRoot,
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO agents (
			agent_id, version, hostname, platform, mode, process_id, callback_base_url, registered_at, last_heartbeat_at
		) VALUES (
			'agent-dev-1', 'dev', 'workstation', 'windows/amd64', 'attached', 1024, 'http://127.0.0.1:61337', $1, $1
		)
	`, time.Now().UTC()); err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	service := importing.NewService(pool, bridge, jobService, assetService)
	jobService.RegisterExecutor(jobs.JobIntentImport, service.ExecuteImportJobItem)
	jobService.Start(ctx)

	dashboard, err := service.RefreshDashboard(ctx)
	if err != nil {
		t.Fatalf("refresh dashboard: %v", err)
	}
	if len(dashboard.Devices) != 1 {
		t.Fatalf("expected 1 device, got %+v", dashboard.Devices)
	}
	if len(dashboard.Drafts) != 1 {
		t.Fatalf("expected 1 draft, got %+v", dashboard.Drafts)
	}
	draftID := dashboard.Drafts[0].ID
	if _, err := service.SetDraftLibrary(ctx, draftID, "photo"); err != nil {
		t.Fatalf("set draft library: %v", err)
	}
	browse, err := service.BrowseSession(ctx, dashboard.Devices[0].ID, "", 50, 0)
	if err != nil {
		t.Fatalf("browse session root: %v", err)
	}
	if len(browse.Items) != 2 {
		t.Fatalf("expected root browse items, got %+v", browse.Items)
	}
	if _, err := service.SaveSelectionTargets(ctx, dashboard.Devices[0].ID, "DIRECTORY", "A001", "A001", []string{mount.Record.ID, secondMount.Record.ID}); err != nil {
		t.Fatalf("save selection targets: %v", err)
	}
	if _, err := service.RefreshPrecheck(ctx, draftID); err != nil {
		t.Fatalf("refresh precheck: %v", err)
	}

	submit, err := service.Submit(ctx, dashboard.Devices[0].ID)
	if err != nil {
		t.Fatalf("submit import: %v", err)
	}
	waitForJobStatus(t, ctx, jobService, submit.Report.TaskID, jobs.StatusCompleted)
	report, err := service.LoadDashboard(ctx)
	if err != nil {
		t.Fatalf("reload dashboard: %v", err)
	}
	if len(report.Reports) == 0 {
		t.Fatalf("expected import reports, got %+v", report)
	}
	latestReport := report.Reports[0]
	if latestReport.VerifyMode != "轻校验" {
		t.Fatalf("expected light verify mode, got %+v", latestReport)
	}
	if latestReport.VerifiedCount != 2 {
		t.Fatalf("expected verified count 2, got %+v", latestReport)
	}
	if latestReport.VerifyFailedCount != 0 {
		t.Fatalf("expected no verify failures, got %+v", latestReport)
	}
	if len(latestReport.TargetSummaries) != 2 {
		t.Fatalf("expected two target summaries, got %+v", latestReport.TargetSummaries)
	}
	for _, targetSummary := range latestReport.TargetSummaries {
		if targetSummary.SuccessCount != 1 || targetSummary.VerifiedCount != 1 {
			t.Fatalf("expected target summary success/verify counts, got %+v", targetSummary)
		}
	}

	root, err := assetService.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
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
	if root.Total == 0 {
		t.Fatalf("expected imported assets in library root, got %+v", root)
	}
}

func TestServiceLoadDashboardIncludesNASTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:5432", time.Second)
	if err != nil {
		t.Skip("postgres is not available in current environment")
	}
	_ = conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	targetRoot := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "导入目标本地节点",
		RootPath: targetRoot,
		Notes:    "integration",
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}
	if _, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "本地入库",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "originals",
		Notes:           "integration",
	}); err != nil {
		t.Fatalf("save local folder: %v", err)
	}

	nasRoot := t.TempDir()
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (
			id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
		) VALUES (
			'nas-node-1', 'nas-node-1', '影像 NAS 01', 'NAS', 'SMB', '\\\\192.168.10.20\\media', 'DIRECT', 'mare-sync', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert NAS node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, created_at, updated_at
		) VALUES (
			'mount-nas-1', 'mount-nas-1', 'photo', '商业摄影资产库', 'nas-node-1', '影像 NAS 01', 'NAS_SHARE',
			'READ_WRITE', $2, '/', 'NEVER', 'MANUAL', true, $1, $1
		)
	`, now, nasRoot); err != nil {
		t.Fatalf("insert NAS mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-nas-1', 'mount-nas-1', 'IDLE', 'AUTHORIZED', 'ONLINE', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert NAS mount runtime: %v", err)
	}

	bridge := &fakeAgentBridge{
		sources: []importdto.SourceDescriptor{
			{
				DeviceKey:   "device-local-1",
				SourceType:  "LOCAL_DIRECTORY",
				DeviceLabel: "测试素材目录",
				DeviceType:  "本地目录",
				SourcePath:  targetRoot,
				MountPath:   targetRoot,
				ConnectedAt: now.Format(time.RFC3339),
				LastSeenAt:  now.Format(time.RFC3339),
			},
		},
		browse: importdto.BrowseResponse{
			Entries:   []importdto.BrowseEntry{},
			Total:     0,
			Limit:     50,
			Offset:    0,
			HasMore:   false,
			ScannedAt: now.Format(time.RFC3339),
		},
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO agents (
			agent_id, version, hostname, platform, mode, process_id, callback_base_url, registered_at, last_heartbeat_at
		) VALUES (
			'agent-dev-1', 'dev', 'workstation', 'windows/amd64', 'attached', 1024, 'http://127.0.0.1:61337', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	service := importing.NewService(pool, bridge, jobs.NewService(pool), assets.NewService(pool))
	dashboard, err := service.RefreshDashboard(ctx)
	if err != nil {
		t.Fatalf("refresh dashboard: %v", err)
	}

	if len(dashboard.TargetEndpoints) < 2 {
		t.Fatalf("expected local and NAS targets, got %+v", dashboard.TargetEndpoints)
	}

	foundNASTarget := false
	for _, target := range dashboard.TargetEndpoints {
		if target.ID == "mount-nas-1" {
			foundNASTarget = true
			if target.Type != "NAS/SMB" {
				t.Fatalf("expected NAS target type NAS/SMB, got %+v", target)
			}
			if target.StatusLabel != "可用" {
				t.Fatalf("expected NAS target status 可用, got %+v", target)
			}
		}
	}
	if !foundNASTarget {
		t.Fatalf("expected NAS target to be listed, got %+v", dashboard.TargetEndpoints)
	}

	if len(dashboard.Drafts) != 1 {
		t.Fatalf("expected one draft, got %+v", dashboard.Drafts)
	}
	if _, err := service.SetDraftLibrary(ctx, dashboard.Drafts[0].ID, "photo"); err != nil {
		t.Fatalf("set draft library: %v", err)
	}
	dashboard, err = service.LoadDashboard(ctx)
	if err != nil {
		t.Fatalf("reload dashboard: %v", err)
	}
	if len(dashboard.Devices) != 1 {
		t.Fatalf("expected one device session, got %+v", dashboard.Devices)
	}
	if !containsString(dashboard.Devices[0].AvailableTargetEndpointIDs, "mount-nas-1") {
		t.Fatalf("expected NAS target to be available for device session, got %+v", dashboard.Devices[0].AvailableTargetEndpointIDs)
	}
}

func ptr[T any](value T) *T {
	return &value
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type fakeAgentBridge struct {
	sources    []importdto.SourceDescriptor
	browse     importdto.BrowseResponse
	sourceRoot string
}

func (f *fakeAgentBridge) DiscoverSources(context.Context, string) ([]importdto.SourceDescriptor, error) {
	return f.sources, nil
}

func (f *fakeAgentBridge) BrowseSource(_ context.Context, _ string, request importdto.BrowseRequest) (importdto.BrowseResponse, error) {
	if f.browse.Entries == nil {
		return importdto.BrowseResponse{}, nil
	}
	if f.browse.Total == 0 {
		return f.browse, nil
	}
	if request.RelativePath != nil && *request.RelativePath == "A001" {
		fileOnly := f.browse.Entries[1:]
		return importdto.BrowseResponse{
			Entries:   fileOnly,
			Total:     len(fileOnly),
			Limit:     50,
			Offset:    0,
			HasMore:   false,
			ScannedAt: f.browse.ScannedAt,
		}, nil
	}
	return f.browse, nil
}

func (f *fakeAgentBridge) ExecuteImport(_ context.Context, _ string, request importdto.ExecuteImportRequest) (importdto.ExecuteImportResponse, error) {
	results := make([]importdto.ExecuteImportTargetResult, 0, len(request.Targets))
	for _, target := range request.Targets {
		data, err := os.ReadFile(request.SourcePath)
		if err != nil {
			return importdto.ExecuteImportResponse{}, err
		}
		if err := os.MkdirAll(filepath.Dir(target.PhysicalPath), 0o755); err != nil {
			return importdto.ExecuteImportResponse{}, err
		}
		if err := os.WriteFile(target.PhysicalPath, data, 0o644); err != nil {
			return importdto.ExecuteImportResponse{}, err
		}
		info, err := os.Stat(request.SourcePath)
		if err != nil {
			return importdto.ExecuteImportResponse{}, err
		}
		if err := os.Chtimes(target.PhysicalPath, info.ModTime(), info.ModTime()); err != nil {
			return importdto.ExecuteImportResponse{}, err
		}
		results = append(results, importdto.ExecuteImportTargetResult{
			TargetID:      target.TargetID,
			PhysicalPath:  target.PhysicalPath,
			BytesWritten:  int64(len(data)),
			ModifiedAt:    info.ModTime().UTC().Format(time.RFC3339),
			Status:        "SUCCEEDED",
			VerifyMode:    "LIGHT",
			VerifyStatus:  "PASSED",
			VerifySummary: "轻校验通过",
		})
	}
	return importdto.ExecuteImportResponse{Targets: results}, nil
}

func openTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	parsed, err := url.Parse(developmentDatabaseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	schemaName := "test_importing_" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	query := parsed.Query()
	query.Set("search_path", schemaName)
	parsed.RawQuery = query.Encode()

	config, err := pgxpool.ParseConfig(parsed.String())
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}

	admin, err := pgxpool.New(ctx, developmentDatabaseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer admin.Close()

	if _, err := admin.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schemaName); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect test pool: %v", err)
	}
	return pool
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
DROP TABLE IF EXISTS import_reports;
DROP TABLE IF EXISTS import_session_entries;
DROP TABLE IF EXISTS import_plans;
DROP TABLE IF EXISTS import_sessions;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS issue_object_links;
DROP TABLE IF EXISTS issue_events;
DROP TABLE IF EXISTS issues;
DROP TABLE IF EXISTS job_object_links;
DROP TABLE IF EXISTS job_events;
DROP TABLE IF EXISTS job_attempts;
DROP TABLE IF EXISTS job_items;
DROP TABLE IF EXISTS jobs;
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
DROP TABLE IF EXISTS storage_nodes;
DROP TABLE IF EXISTS agents;
`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

func waitForJobStatus(t *testing.T, ctx context.Context, service *jobs.Service, jobID string, expected string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := service.LoadJobDetail(ctx, jobID)
		if err == nil && detail.Job.Status == expected {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	detail, err := service.LoadJobDetail(ctx, jobID)
	if err != nil {
		t.Fatalf("load final job detail: %v", err)
	}
	t.Fatalf("expected job status %s, got %+v", expected, detail.Job)
}
