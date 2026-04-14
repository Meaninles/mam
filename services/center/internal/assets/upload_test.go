package assets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	"mare/services/center/internal/integration"
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

type fakeUploadCloudDriver struct {
	ensuredRoots []string
	uploads      map[string][]byte
	waitBlock    chan struct{}
	waitErr      error
}

func (f *fakeUploadCloudDriver) Vendor() string { return "115" }

func (f *fakeUploadCloudDriver) AuthenticateToken(context.Context, string) (integration.ProviderAuthResult, error) {
	return integration.ProviderAuthResult{}, nil
}

func (f *fakeUploadCloudDriver) CreateQRCodeSession(context.Context, string) (integration.QRCodeSession, error) {
	return integration.QRCodeSession{}, nil
}

func (f *fakeUploadCloudDriver) GetQRCodeSession(context.Context, string) (integration.QRCodeSession, error) {
	return integration.QRCodeSession{}, nil
}

func (f *fakeUploadCloudDriver) ConsumeQRCodeSession(context.Context, string) (integration.ProviderAuthResult, error) {
	return integration.ProviderAuthResult{}, nil
}

func (f *fakeUploadCloudDriver) EnsureRemoteRoot(_ context.Context, _ integration.CloudProviderPayload, remoteRootPath string) error {
	f.ensuredRoots = append(f.ensuredRoots, remoteRootPath)
	return nil
}

func (f *fakeUploadCloudDriver) ListRemoteEntries(context.Context, integration.CloudProviderPayload, string) ([]integration.CloudFileEntry, error) {
	items := make([]integration.CloudFileEntry, 0, len(f.uploads))
	for path, content := range f.uploads {
		items = append(items, integration.CloudFileEntry{
			Name:      filepath.Base(path),
			SizeBytes: int64(len(content)),
		})
	}
	return items, nil
}

func (f *fakeUploadCloudDriver) StartUpload(
	ctx context.Context,
	_ integration.CloudProviderPayload,
	remoteRootPath string,
	relativePath string,
	source integration.UploadSource,
) (string, string, error) {
	if f.uploads == nil {
		f.uploads = map[string][]byte{}
	}
	content, err := readUploadSourceContent(ctx, source)
	if err != nil {
		return "", "", err
	}
	fullPath := filepath.ToSlash(filepath.Join(remoteRootPath, relativePath))
	f.uploads[fullPath] = content
	return "upload-task-1", fullPath, nil
}

func (f *fakeUploadCloudDriver) AttachUpload(context.Context, string, string, integration.UploadSource) error {
	return nil
}

func (f *fakeUploadCloudDriver) WaitUpload(ctx context.Context, _ string, _ string, _ func(integration.TransferProgress)) error {
	if f.waitBlock != nil {
		select {
		case <-f.waitBlock:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.waitErr
}

func (f *fakeUploadCloudDriver) ResetUploadSession(context.Context) error { return nil }
func (f *fakeUploadCloudDriver) PauseUpload(context.Context, string) error { return nil }
func (f *fakeUploadCloudDriver) ResumeUpload(context.Context, string) error { return nil }
func (f *fakeUploadCloudDriver) CancelUpload(context.Context, string) error { return nil }

func (f *fakeUploadCloudDriver) ResolveDownloadSource(context.Context, integration.CloudProviderPayload, string, string) (integration.DownloadSource, error) {
	return integration.DownloadSource{}, nil
}

func (f *fakeUploadCloudDriver) DeleteFile(context.Context, integration.CloudProviderPayload, string, string) error {
	return nil
}

type fakeUploadCloudResolver struct {
	driver *fakeUploadCloudDriver
}

func (f fakeUploadCloudResolver) Provider(string) (integration.CloudProviderDriver, error) {
	return f.driver, nil
}

func (f fakeUploadCloudResolver) Downloader(string) (integration.DownloadEngine, error) {
	return nil, fmt.Errorf("downloader not implemented in upload test")
}

func (f fakeUploadCloudResolver) EnsureCD2ClientDeviceID(context.Context) (string, error) {
	return "cd2-device-1", nil
}

func readUploadSourceContent(ctx context.Context, source integration.UploadSource) ([]byte, error) {
	defer source.Close()
	total := source.Size()
	if total <= 0 {
		return []byte{}, nil
	}

	buffer := make([]byte, 0, total)
	var offset int64
	for {
		chunkSize := int64(64 * 1024)
		if remaining := total - offset; remaining < chunkSize {
			chunkSize = remaining
		}
		chunk, done, err := source.ReadChunk(ctx, offset, chunkSize)
		if err != nil {
			return nil, err
		}
		buffer = append(buffer, chunk...)
		offset += int64(len(chunk))
		if done {
			return buffer, nil
		}
	}
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

func TestUploadSelectionPrefersLocalMountOverNASAndCloud(t *testing.T) {
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

	localExecutor := &fakeUploadExecutor{}
	nasExecutor := &fakeUploadExecutor{}
	cloudDriver := &fakeUploadCloudDriver{}
	service := NewService(pool)
	service.executorResolver = func(nodeType string) (mountPathExecutor, error) {
		switch nodeType {
		case "LOCAL":
			return localExecutor, nil
		case "NAS":
			return nasExecutor, nil
		default:
			return nil, fmt.Errorf("unexpected node type %s", nodeType)
		}
	}
	service.SetCloudResolver(fakeUploadCloudResolver{driver: cloudDriver})

	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "优先本地上传库"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (id, code, name, node_type, address, access_mode, enabled, created_at, updated_at) VALUES
		('local-node-1', 'local-node-1', '本地节点', 'LOCAL', 'D:\\Assets', 'DIRECT', true, $1, $1),
		('nas-node-1', 'nas-node-1', 'NAS 节点', 'NAS', '\\nas\share', 'SMB', true, $1, $1),
		('cloud-node-1', 'cloud-node-1', '云节点', 'CLOUD', '/cloud', 'CD2', true, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert storage nodes: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (id, storage_node_id, credential_kind, username, secret_ciphertext, updated_at, created_at) VALUES
		('nas-cred-1', 'nas-node-1', 'PASSWORD', 'mare', 'cipher', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert nas credentials: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'TOKEN', '/remote-root', '{"cloudPath":"/remote-root"}', $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES
		('mount-local-1', 'mount-local-1', $1, $2, 'local-node-1', '本地挂载', 'LOCAL_PATH', 'READ_WRITE', 'D:\\Assets\\local', 'local', 'NEVER', 'MANUAL', true, 0, $3, $3),
		('mount-nas-1', 'mount-nas-1', $1, $2, 'nas-node-1', 'NAS 挂载', 'NAS_SHARE', 'READ_WRITE', '\\nas\share\uploads', 'uploads', 'NEVER', 'MANUAL', true, 1, $3, $3),
		('mount-cloud-1', 'mount-cloud-1', $1, $2, 'cloud-node-1', '云挂载', 'CLOUD_FOLDER', 'READ_WRITE', '/remote-root', '/', 'NEVER', 'MANUAL', true, 2, $3, $3)
	`, created.Library.ID, created.Library.Name, now); err != nil {
		t.Fatalf("insert mounts: %v", err)
	}

	result, err := service.UploadSelection(ctx, created.Library.ID, assetdto.UploadSelectionRequest{
		Mode: "files",
		Files: []assetdto.UploadSelectionFile{{
			Name:         "cover.jpg",
			RelativePath: "cover.jpg",
			Size:         5,
			Content:      []byte("cover"),
		}},
	})
	if err != nil {
		t.Fatalf("upload selection: %v", err)
	}
	if result.CreatedCount != 1 {
		t.Fatalf("expected one upload result, got %#v", result)
	}
	if string(localExecutor.writes[`D:\Assets\local\cover.jpg`]) != "cover" {
		t.Fatalf("expected upload to local mount, got local=%#v", localExecutor.writes)
	}
	if len(nasExecutor.writes) != 0 {
		t.Fatalf("expected NAS mount to stay untouched, got %#v", nasExecutor.writes)
	}
	if len(cloudDriver.uploads) != 0 {
		t.Fatalf("expected cloud mount to stay untouched, got %#v", cloudDriver.uploads)
	}

	root, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		Page: 1, PageSize: 20, FileType: "全部", StatusFilter: "全部", SortValue: "名称", SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse root: %v", err)
	}
	if len(root.Items) != 1 {
		t.Fatalf("expected one uploaded file, got %#v", root.Items)
	}
	endpoints := root.Items[0].Endpoints
	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %#v", endpoints)
	}
	states := map[string]string{}
	for _, endpoint := range endpoints {
		states[endpoint.Name] = endpoint.State
	}
	if states["本地挂载"] != "已同步" || states["NAS 挂载"] != "未同步" || states["云挂载"] != "未同步" {
		t.Fatalf("unexpected endpoint states: %#v", states)
	}
}

func TestUploadSelectionPrefersNASWhenLocalMountMissing(t *testing.T) {
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

	nasExecutor := &fakeUploadExecutor{}
	cloudDriver := &fakeUploadCloudDriver{}
	service := NewService(pool)
	service.executorResolver = func(nodeType string) (mountPathExecutor, error) {
		if nodeType == "NAS" {
			return nasExecutor, nil
		}
		return nil, fmt.Errorf("unexpected node type %s", nodeType)
	}
	service.SetCloudResolver(fakeUploadCloudResolver{driver: cloudDriver})

	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "优先 NAS 上传库"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (id, code, name, node_type, address, access_mode, enabled, created_at, updated_at) VALUES
		('nas-node-1', 'nas-node-1', 'NAS 节点', 'NAS', '\\nas\share', 'SMB', true, $1, $1),
		('cloud-node-1', 'cloud-node-1', '云节点', 'CLOUD', '/cloud', 'CD2', true, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert storage nodes: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (id, storage_node_id, credential_kind, username, secret_ciphertext, updated_at, created_at) VALUES
		('nas-cred-1', 'nas-node-1', 'PASSWORD', 'mare', 'cipher', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert nas credentials: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'TOKEN', '/remote-root', '{"cloudPath":"/remote-root"}', $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES
		('mount-nas-1', 'mount-nas-1', $1, $2, 'nas-node-1', 'NAS 挂载', 'NAS_SHARE', 'READ_WRITE', '\\nas\share\uploads', 'uploads', 'NEVER', 'MANUAL', true, 0, $3, $3),
		('mount-cloud-1', 'mount-cloud-1', $1, $2, 'cloud-node-1', '云挂载', 'CLOUD_FOLDER', 'READ_WRITE', '/remote-root', '/', 'NEVER', 'MANUAL', true, 1, $3, $3)
	`, created.Library.ID, created.Library.Name, now); err != nil {
		t.Fatalf("insert mounts: %v", err)
	}

	if _, err := service.UploadSelection(ctx, created.Library.ID, assetdto.UploadSelectionRequest{
		Mode: "files",
		Files: []assetdto.UploadSelectionFile{{
			Name:         "brief.pdf",
			RelativePath: "brief.pdf",
			Size:         5,
			Content:      []byte("brief"),
		}},
	}); err != nil {
		t.Fatalf("upload selection: %v", err)
	}

	if string(nasExecutor.writes[`\\nas\share\uploads\brief.pdf`]) != "brief" {
		t.Fatalf("expected upload to NAS mount, got %#v", nasExecutor.writes)
	}
	if len(cloudDriver.uploads) != 0 {
		t.Fatalf("expected cloud mount untouched when NAS exists, got %#v", cloudDriver.uploads)
	}
}

func TestUploadSelectionUploadsToCloudWhenOnlyCloudMountExists(t *testing.T) {
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

	cloudDriver := &fakeUploadCloudDriver{}
	service := NewService(pool)
	service.SetCloudResolver(fakeUploadCloudResolver{driver: cloudDriver})

	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "云端上传库"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (id, code, name, node_type, address, access_mode, enabled, created_at, updated_at) VALUES
		('cloud-node-1', 'cloud-node-1', '云节点', 'CLOUD', '/cloud', 'CD2', true, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'TOKEN', '/remote-root', '{"cloudPath":"/remote-root"}', $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-cloud-1', 'mount-cloud-1', $1, $2, 'cloud-node-1', '云挂载', 'CLOUD_FOLDER', 'READ_WRITE',
			'/remote-root', '/', 'NEVER', 'MANUAL', true, 0, $3, $3
		)
	`, created.Library.ID, created.Library.Name, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}

	result, err := service.UploadSelection(ctx, created.Library.ID, assetdto.UploadSelectionRequest{
		Mode: "folder",
		Files: []assetdto.UploadSelectionFile{{
			Name:         "cover.jpg",
			RelativePath: "nested/cover.jpg",
			Size:         int64(len([]byte("cover-image"))),
			Content:      []byte("cover-image"),
		}},
	})
	if err != nil {
		t.Fatalf("upload selection: %v", err)
	}
	if result.CreatedCount != 1 {
		t.Fatalf("expected one uploaded file, got %#v", result)
	}
	if string(cloudDriver.uploads["/remote-root/nested/cover.jpg"]) != "cover-image" {
		t.Fatalf("expected cloud upload write, got %#v", cloudDriver.uploads)
	}
	if len(cloudDriver.ensuredRoots) != 1 || cloudDriver.ensuredRoots[0] != "/remote-root" {
		t.Fatalf("expected remote root ensured once, got %#v", cloudDriver.ensuredRoots)
	}

	root, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		Page: 1, PageSize: 20, FileType: "全部", StatusFilter: "全部", SortValue: "名称", SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse root: %v", err)
	}
	if len(root.Items) != 1 || root.Items[0].Name != "nested" || root.Items[0].Type != "folder" {
		t.Fatalf("expected nested folder at root, got %#v", root.Items)
	}

	folderID := root.Items[0].ID
	child, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		ParentID: &folderID, Page: 1, PageSize: 20, FileType: "全部", StatusFilter: "全部", SortValue: "名称", SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse nested folder: %v", err)
	}
	if len(child.Items) != 1 || child.Items[0].Name != "cover.jpg" {
		t.Fatalf("expected uploaded file in nested folder, got %#v", child.Items)
	}
	if len(child.Items[0].Endpoints) != 1 || child.Items[0].Endpoints[0].Name != "云挂载" || child.Items[0].Endpoints[0].State != "已同步" {
		t.Fatalf("expected synced cloud endpoint, got %#v", child.Items[0].Endpoints)
	}
}

func TestExecuteFileCenterUploadJobItemUploadsStagedFileToCloud(t *testing.T) {
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

	cloudDriver := &fakeUploadCloudDriver{}
	service := NewService(pool)
	service.SetCloudResolver(fakeUploadCloudResolver{driver: cloudDriver})

	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "文件中心上传执行器"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (id, code, name, node_type, address, access_mode, enabled, created_at, updated_at) VALUES
		('cloud-node-1', 'cloud-node-1', '云节点', 'CLOUD', '/cloud', 'CD2', true, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'TOKEN', '/remote-root', '{"cloudPath":"/remote-root"}', $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-cloud-1', 'mount-cloud-1', $1, $2, 'cloud-node-1', '云挂载', 'CLOUD_FOLDER', 'READ_WRITE',
			'/remote-root', '/', 'NEVER', 'MANUAL', true, 0, $3, $3
		)
	`, created.Library.ID, created.Library.Name, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}

	stagedFile := filepath.Join(t.TempDir(), "nested", "cover.jpg")
	if err := writeLocalFile(stagedFile, []byte("cover-image")); err != nil {
		t.Fatalf("write staged file: %v", err)
	}

	if err := service.ExecuteFileCenterUploadJobItem(
		ctx,
		"job-upload-1",
		"job-item-1",
		created.Library.ID,
		"/nested/cover.jpg",
		stagedFile,
		nil,
		"mount-cloud-1",
	); err != nil {
		t.Fatalf("execute upload job item: %v", err)
	}

	if string(cloudDriver.uploads["/remote-root/nested/cover.jpg"]) != "cover-image" {
		t.Fatalf("expected staged file uploaded to cloud, got %#v", cloudDriver.uploads)
	}
	if _, err := os.Stat(stagedFile); !os.IsNotExist(err) {
		t.Fatalf("expected staged file removed after execution, err=%v", err)
	}

	root, err := service.BrowseLibrary(ctx, created.Library.ID, assetdto.BrowseQuery{
		Page: 1, PageSize: 20, FileType: "全部", StatusFilter: "全部", SortValue: "名称", SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse root: %v", err)
	}
	if len(root.Items) != 1 || root.Items[0].Name != "nested" {
		t.Fatalf("expected nested folder after upload execution, got %#v", root.Items)
	}
}

func TestExecuteFileCenterUploadJobItemKeepsStagedFileWhenPausedThenAllowsResume(t *testing.T) {
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

	cloudDriver := &fakeUploadCloudDriver{waitBlock: make(chan struct{})}
	service := NewService(pool)
	service.SetCloudResolver(fakeUploadCloudResolver{driver: cloudDriver})

	created, err := service.CreateLibrary(ctx, assetdto.CreateLibraryRequest{Name: "暂停恢复上传测试"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (id, code, name, node_type, address, access_mode, enabled, created_at, updated_at) VALUES
		('cloud-node-1', 'cloud-node-1', '云节点', 'CLOUD', '/cloud', 'CD2', true, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert cloud node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES (
			'cloud-profile-1', 'cloud-node-1', '115', 'TOKEN', '/remote-root', '{"cloudPath":"/remote-root"}', $1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert cloud profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-cloud-1', 'mount-cloud-1', $1, $2, 'cloud-node-1', '云挂载', 'CLOUD_FOLDER', 'READ_WRITE',
			'/remote-root', '/', 'NEVER', 'MANUAL', true, 0, $3, $3
		)
	`, created.Library.ID, created.Library.Name, now); err != nil {
		t.Fatalf("insert cloud mount: %v", err)
	}

	stagedFile := filepath.Join(t.TempDir(), "nested", "cover.jpg")
	if err := writeLocalFile(stagedFile, []byte("cover-image")); err != nil {
		t.Fatalf("write staged file: %v", err)
	}

	pausedCtx, pauseCancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.ExecuteFileCenterUploadJobItem(
			pausedCtx,
			"job-upload-pause-1",
			"job-item-pause-1",
			created.Library.ID,
			"/nested/cover.jpg",
			stagedFile,
			nil,
			"mount-cloud-1",
		)
	}()

	time.Sleep(50 * time.Millisecond)
	pauseCancel()
	err = <-errCh
	if err == nil {
		t.Fatalf("expected paused upload to return interruption error")
	}
	if _, statErr := os.Stat(stagedFile); statErr != nil {
		t.Fatalf("expected staged file kept after pause, stat err=%v", statErr)
	}

	cloudDriver.waitBlock = nil
	if err := service.ExecuteFileCenterUploadJobItem(
		ctx,
		"job-upload-resume-1",
		"job-item-resume-1",
		created.Library.ID,
		"/nested/cover.jpg",
		stagedFile,
		nil,
		"mount-cloud-1",
	); err != nil {
		t.Fatalf("resume upload execution: %v", err)
	}

	if string(cloudDriver.uploads["/remote-root/nested/cover.jpg"]) != "cover-image" {
		t.Fatalf("expected resumed upload to reach cloud, got %#v", cloudDriver.uploads)
	}
	if _, statErr := os.Stat(stagedFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected staged file cleaned after successful resume, err=%v", statErr)
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
