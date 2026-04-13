package integration

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	cd2pb "mare/services/center/internal/integration/cd2/pb"
)

type CD2115Driver struct {
	service    *Service
	httpClient *http.Client
	deviceID   string

	mu           sync.Mutex
	qrSessions   map[string]*qrSessionState
	uploads      map[string]*cd2UploadSession
	channelCtx   context.Context
	cancelChan   context.CancelFunc
	uploadClient *cd2Client
	uploadStream cd2pb.CloudDriveFileSrv_RemoteUploadChannelClient
}

type qrSessionState struct {
	session QRCodeSession
}

type cd2UploadSession struct {
	id          string
	destPath    string
	source      UploadSource
	status      cd2pb.UploadFileInfo_Status
	err         error
	done        chan struct{}
	createdAt   time.Time
	firstReadAt *time.Time
	hashMu      sync.Mutex
	hashJobs    map[string]context.CancelFunc
}

func NewCD2115Driver(service *Service) *CD2115Driver {
	return &CD2115Driver{
		service:    service,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		deviceID:   "",
		qrSessions: make(map[string]*qrSessionState),
		uploads:    make(map[string]*cd2UploadSession),
	}
}

func (d *CD2115Driver) Vendor() string {
	return "115"
}

func (d *CD2115Driver) TestGateway(ctx context.Context, config CD2GatewayConfig) error {
	client, err := newCD2Client(ctx, config)
	if err != nil {
		return err
	}
	defer client.Close()

	if _, err := client.getRuntimeInfo(ctx); err != nil {
		return err
	}
	return nil
}

func (d *CD2115Driver) AuthenticateToken(ctx context.Context, token string) (ProviderAuthResult, error) {
	config, err := d.service.LoadCD2GatewayConfig(ctx)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	client, err := newCD2Client(ctx, config)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	defer client.Close()

	before, _ := client.getAllCloudAPIs(ctx)
	authCtx, err := client.authContext(ctx)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	result, err := client.client.APILogin115Editthiscookie(authCtx, &cd2pb.Login115EditthiscookieRequest{
		EditThiscookieString: strings.TrimSpace(token),
	})
	if err != nil {
		return ProviderAuthResult{}, fmt.Errorf("通过 CloudDrive2 登录 115 失败: %w", err)
	}
	if !result.GetSuccess() {
		return ProviderAuthResult{}, fmt.Errorf("%s", strings.TrimSpace(result.GetErrorMessage()))
	}
	after, err := client.getAllCloudAPIs(ctx)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	api, err := resolveCloudAPI(before, after, "115")
	if err != nil {
		return ProviderAuthResult{}, err
	}
	return buildProviderAuthResult("115", api), nil
}

func (d *CD2115Driver) AuthenticateOpenToken(ctx context.Context, token OpenOAuthToken) (ProviderAuthResult, error) {
	config, err := d.service.LoadCD2GatewayConfig(ctx)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	client, err := newCD2Client(ctx, config)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	defer client.Close()

	before, _ := client.getAllCloudAPIs(ctx)
	authCtx, err := client.authContext(ctx)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	result, err := client.client.APILogin115OpenOAuth(authCtx, &cd2pb.Login115OpenOAuthRequest{
		RefreshToken: token.RefreshToken,
		AccessToken:  token.AccessToken,
		ExpiresIn:    token.ExpiresIn,
	})
	if err != nil {
		return ProviderAuthResult{}, fmt.Errorf("通过 CloudDrive2 登录 115open 失败: %w", err)
	}
	if !result.GetSuccess() {
		return ProviderAuthResult{}, fmt.Errorf("%s", strings.TrimSpace(result.GetErrorMessage()))
	}
	after, err := client.getAllCloudAPIs(ctx)
	if err != nil {
		return ProviderAuthResult{}, err
	}
	api, err := resolveCloudAPI(before, after, "115")
	if err != nil {
		return ProviderAuthResult{}, err
	}
	return buildProviderAuthResult("115", api), nil
}

func (d *CD2115Driver) CreateQRCodeSession(ctx context.Context, channel string) (QRCodeSession, error) {
	config, err := d.service.LoadCD2GatewayConfig(ctx)
	if err != nil {
		return QRCodeSession{}, err
	}
	client, err := newCD2Client(ctx, config)
	if err != nil {
		return QRCodeSession{}, err
	}

	before, _ := client.getAllCloudAPIs(ctx)
	authCtx, err := client.authContext(ctx)
	if err != nil {
		_ = client.Close()
		return QRCodeSession{}, err
	}
	stream, err := client.client.APILogin115OpenQRCode(authCtx, &cd2pb.Login115OpenQRCodeRequest{})
	if err != nil {
		_ = client.Close()
		return QRCodeSession{}, fmt.Errorf("创建 CloudDrive2 扫码会话失败: %w", err)
	}

	session := QRCodeSession{
		ID:      buildIntegrationCode("cd2-qr"),
		Channel: channel,
		Status:  "WAITING",
		Message: "等待扫码",
	}

	d.mu.Lock()
	d.qrSessions[session.ID] = &qrSessionState{session: session}
	d.mu.Unlock()

	go d.consumeQRCodeStream(client, stream, before, session.ID)
	return session, nil
}

func (d *CD2115Driver) consumeQRCodeStream(client *cd2Client, stream cd2pb.CloudDriveFileSrv_APILogin115OpenQRCodeClient, before []*cd2pb.CloudAPI, sessionID string) {
	defer client.Close()
	defer func() {
		d.mu.Lock()
		state := d.qrSessions[sessionID]
		if state != nil && state.session.Status == "WAITING" {
			state.session.Status = "FAILED"
			state.session.Message = "扫码会话已关闭"
		}
		d.mu.Unlock()
	}()

	for {
		reply, err := stream.Recv()
		if err != nil {
			break
		}

		d.mu.Lock()
		state := d.qrSessions[sessionID]
		if state == nil {
			d.mu.Unlock()
			return
		}
		switch reply.GetMessageType() {
		case cd2pb.QRCodeScanMessageType_SHOW_IMAGE:
			state.session.ImageURL = reply.GetMessage()
		case cd2pb.QRCodeScanMessageType_SHOW_IMAGE_CONTENT:
			imageData, _ := decodeQRCodeContent(reply.GetMessage())
			state.session.ImageData = imageData
		case cd2pb.QRCodeScanMessageType_CHANGE_STATUS:
			state.session.Status = "PENDING_SCAN"
			state.session.Message = reply.GetMessage()
		case cd2pb.QRCodeScanMessageType_ERROR:
			state.session.Status = "FAILED"
			state.session.Message = reply.GetMessage()
		case cd2pb.QRCodeScanMessageType_CLOSE:
			state.session.Status = "SCANNED"
			state.session.Message = "扫码已完成，正在换取登录态"
		}
		d.mu.Unlock()
	}

	post, err := client.getAllCloudAPIs(context.Background())
	if err != nil {
		d.updateQRCodeFailure(sessionID, err.Error())
		return
	}
	api, err := resolveCloudAPI(before, post, "115")
	if err != nil {
		d.updateQRCodeFailure(sessionID, err.Error())
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	state := d.qrSessions[sessionID]
	if state == nil {
		return
	}
	result := buildProviderAuthResult("115", api)
	state.session.Status = "COMPLETED"
	state.session.Message = "扫码登录成功"
	state.session.Result = &result
}

func (d *CD2115Driver) updateQRCodeFailure(sessionID string, message string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state := d.qrSessions[sessionID]
	if state == nil {
		return
	}
	state.session.Status = "FAILED"
	state.session.Message = message
}

func (d *CD2115Driver) GetQRCodeSession(ctx context.Context, sessionID string) (QRCodeSession, error) {
	_ = ctx
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.qrSessions[sessionID]
	if !ok {
		return QRCodeSession{}, fmt.Errorf("二维码会话不存在")
	}
	return state.session, nil
}

func (d *CD2115Driver) ConsumeQRCodeSession(ctx context.Context, sessionID string) (ProviderAuthResult, error) {
	_ = ctx
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.qrSessions[sessionID]
	if !ok {
		return ProviderAuthResult{}, fmt.Errorf("二维码会话不存在")
	}
	if state.session.Result == nil || state.session.Status != "COMPLETED" {
		return ProviderAuthResult{}, fmt.Errorf("扫码登录尚未完成")
	}
	delete(d.qrSessions, sessionID)
	return *state.session.Result, nil
}

func (d *CD2115Driver) EnsureRemoteRoot(ctx context.Context, payload CloudProviderPayload, remoteRootPath string) error {
	client, err := d.openClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	segments := splitPathSegments(remoteRootPath)
	current := payload.CloudPath
	for _, segment := range segments {
		authCtx, err := client.authContext(ctx)
		if err != nil {
			return err
		}
		_, err = client.client.CreateFolder(authCtx, &cd2pb.CreateFolderRequest{
			ParentPath: current,
			FolderName: segment,
		})
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "exist") {
			return fmt.Errorf("创建 CloudDrive2 目录失败: %w", err)
		}
		current = path.Join(current, segment)
	}
	return nil
}

func (d *CD2115Driver) StartUpload(ctx context.Context, payload CloudProviderPayload, remoteRootPath string, relativePath string, source UploadSource) (string, string, error) {
	if err := d.EnsureRemoteRoot(ctx, payload, joinCloudPath(remoteRootPath, "", path.Dir(relativePath))); err != nil {
		return "", "", err
	}
	if err := d.ensureUploadChannel(ctx); err != nil {
		return "", "", err
	}

	client, err := d.uploadSessionClient()
	if err != nil {
		return "", "", err
	}
	if err := d.ensureCloudSyncEnabled(ctx, client); err != nil {
		return "", "", err
	}

	fullPath := joinCloudPath(payload.CloudPath, remoteRootPath, relativePath)
	authCtx, err := client.authContext(ctx)
	if err != nil {
		return "", "", err
	}
	started, err := client.client.StartRemoteUpload(authCtx, &cd2pb.StartRemoteUploadRequest{
		FilePath:                 fullPath,
		FileSize:                 uint64(source.Size()),
		ClientCanCalculateHashes: false,
	})
	if err != nil {
		return "", "", fmt.Errorf("创建 CloudDrive2 远程上传任务失败: %w", err)
	}

	session := &cd2UploadSession{
		id:        started.GetUploadId(),
		destPath:  fullPath,
		source:    source,
		status:    cd2pb.UploadFileInfo_Inqueue,
		done:      make(chan struct{}),
		createdAt: time.Now().UTC(),
		hashJobs:  make(map[string]context.CancelFunc),
	}
	d.mu.Lock()
	d.uploads[session.id] = session
	d.mu.Unlock()
	return session.id, fullPath, nil
}

func (d *CD2115Driver) PauseUpload(ctx context.Context, externalTaskID string) error {
	return d.controlUpload(ctx, externalTaskID, "pause")
}

func (d *CD2115Driver) AttachUpload(ctx context.Context, externalTaskID string, destinationPath string, source UploadSource) error {
	if strings.TrimSpace(externalTaskID) == "" {
		return fmt.Errorf("上传任务标识不能为空")
	}
	if err := d.ensureUploadChannel(ctx); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	session := d.uploads[externalTaskID]
	if session == nil {
		session = &cd2UploadSession{
			id:        externalTaskID,
			hashJobs:  make(map[string]context.CancelFunc),
			done:      make(chan struct{}),
			createdAt: time.Now().UTC(),
			status:    cd2pb.UploadFileInfo_Inqueue,
		}
		d.uploads[externalTaskID] = session
	}
	session.destPath = destinationPath
	session.source = source
	session.err = nil
	session.firstReadAt = nil
	if session.hashJobs == nil {
		session.hashJobs = make(map[string]context.CancelFunc)
	}
	if session.done == nil {
		session.done = make(chan struct{})
	}
	if session.createdAt.IsZero() {
		session.createdAt = time.Now().UTC()
	}
	return nil
}

func (d *CD2115Driver) WaitUpload(ctx context.Context, externalTaskID string, destinationPath string, notify func(TransferProgress)) error {
	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			progress, ok := d.lookupUploadProgress(ctx, externalTaskID, destinationPath)
			if ok && notify != nil {
				notify(progress)
			}
			if ok && progress.Terminal {
				if progress.Successful {
					return nil
				}
				if isCD2UploadCanceled(progress.ExternalStatus) {
					return context.Canceled
				}
				return buildCD2UploadTerminalError(progress)
			}
			d.mu.Lock()
			session := d.uploads[externalTaskID]
			d.mu.Unlock()
			if session != nil {
				if !d.hasUploadChannel() {
					_ = d.ensureUploadChannel(ctx)
				}
				session.hashMu.Lock()
				firstReadAt := session.firstReadAt
				createdAt := session.createdAt
				status := session.status
				session.hashMu.Unlock()
				select {
				case <-session.done:
					if session.err != nil {
						return session.err
					}
					return nil
				default:
				}
				if firstReadAt == nil && (status == cd2pb.UploadFileInfo_Preprocessing || status == cd2pb.UploadFileInfo_WaitforPreprocessing || status == cd2pb.UploadFileInfo_Inqueue) && time.Since(createdAt) > time.Minute {
					return fmt.Errorf("UPLOAD_CHANNEL_IDLE_TIMEOUT: StartRemoteUpload 后 1 分钟仍未收到 read_data/hash_data 请求")
				}
			}
		}
	}
}

func (d *CD2115Driver) ResumeUpload(ctx context.Context, externalTaskID string) error {
	return d.controlUpload(ctx, externalTaskID, "resume")
}

func (d *CD2115Driver) ResetUploadSession(ctx context.Context) error {
	_ = ctx
	d.resetUploadChannel()
	return nil
}

func (d *CD2115Driver) CancelUpload(ctx context.Context, externalTaskID string) error {
	return d.controlUpload(ctx, externalTaskID, "cancel")
}

func (d *CD2115Driver) controlUpload(ctx context.Context, externalTaskID string, control string) error {
	client, err := d.uploadSessionClient()
	if err != nil {
		return err
	}
	authCtx, err := client.authContext(ctx)
	if err != nil {
		return err
	}
	req := &cd2pb.RemoteUploadControlRequest{UploadId: externalTaskID}
	switch control {
	case "pause":
		req.Control = &cd2pb.RemoteUploadControlRequest_Pause{Pause: &cd2pb.PauseRemoteUpload{}}
	case "resume":
		req.Control = &cd2pb.RemoteUploadControlRequest_Resume{Resume: &cd2pb.ResumeRemoteUpload{}}
	default:
		req.Control = &cd2pb.RemoteUploadControlRequest_Cancel{Cancel: &cd2pb.CancelRemoteUpload{}}
	}
	_, err = client.client.RemoteUploadControl(authCtx, req)
	if err != nil {
		return fmt.Errorf("控制 CloudDrive2 上传任务失败: %w", err)
	}
	return nil
}

func (d *CD2115Driver) ResolveDownloadSource(ctx context.Context, payload CloudProviderPayload, remoteRootPath string, relativePath string) (DownloadSource, error) {
	client, err := d.openClient(ctx)
	if err != nil {
		return DownloadSource{}, err
	}
	defer client.Close()

	authCtx, err := client.authContext(ctx)
	if err != nil {
		return DownloadSource{}, err
	}
	fullPath := joinCloudPath(payload.CloudPath, remoteRootPath, relativePath)
	reply, err := client.client.GetDownloadUrlPath(authCtx, &cd2pb.GetDownloadUrlPathRequest{
		Path:         fullPath,
		Preview:      false,
		LazyRead:     false,
		GetDirectUrl: true,
	})
	if err != nil {
		return DownloadSource{}, fmt.Errorf("获取 CloudDrive2 下载地址失败: %w", err)
	}

	source := DownloadSource{
		URL:               reply.GetDirectUrl(),
		UserAgent:         reply.GetUserAgent(),
		AdditionalHeaders: reply.GetAdditionalHeaders(),
	}
	if source.URL == "" {
		source.URL = buildGatewayDownloadURL(client.serverURL, reply.GetDownloadUrlPath())
	}
	if reply.ExpiresIn != nil {
		expiresAt := time.Now().Add(time.Duration(reply.GetExpiresIn()) * time.Second)
		source.ExpiresAt = &expiresAt
	}
	return source, nil
}

func (d *CD2115Driver) DeleteFile(ctx context.Context, payload CloudProviderPayload, remoteRootPath string, relativePath string) error {
	client, err := d.openClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	authCtx, err := client.authContext(ctx)
	if err != nil {
		return err
	}
	result, err := client.client.DeleteFile(authCtx, &cd2pb.FileRequest{
		Path: joinCloudPath(payload.CloudPath, remoteRootPath, relativePath),
	})
	if err != nil {
		return fmt.Errorf("删除 CloudDrive2 文件失败: %w", err)
	}
	if !result.GetSuccess() && strings.TrimSpace(result.GetErrorMessage()) != "" {
		return fmt.Errorf("%s", result.GetErrorMessage())
	}
	return nil
}

func (d *CD2115Driver) openClient(ctx context.Context) (*cd2Client, error) {
	config, err := d.service.LoadCD2GatewayConfig(ctx)
	if err != nil {
		return nil, err
	}
	return newCD2Client(ctx, config)
}

func (d *CD2115Driver) ensureUploadChannel(ctx context.Context) error {
	d.mu.Lock()
	if d.channelCtx != nil && d.uploadStream != nil {
		d.mu.Unlock()
		return nil
	}
	if strings.TrimSpace(d.deviceID) == "" {
		deviceID, err := d.service.EnsureCD2ClientDeviceID(ctx)
		if err != nil {
			d.mu.Unlock()
			return err
		}
		d.deviceID = deviceID
	}
	client, err := d.openClient(ctx)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	channelCtx, cancel := context.WithCancel(context.Background())
	authCtx, err := client.authContext(channelCtx)
	if err != nil {
		d.mu.Unlock()
		cancel()
		_ = client.Close()
		return err
	}
	stream, err := client.client.RemoteUploadChannel(authCtx, &cd2pb.RemoteUploadChannelRequest{DeviceId: d.deviceID})
	if err != nil {
		d.mu.Unlock()
		cancel()
		_ = client.Close()
		return err
	}
	d.channelCtx = channelCtx
	d.cancelChan = cancel
	d.uploadClient = client
	d.uploadStream = stream
	d.mu.Unlock()

	go d.runUploadChannel(channelCtx, client, stream)
	return nil
}

func (d *CD2115Driver) runUploadChannel(ctx context.Context, client *cd2Client, stream cd2pb.CloudDriveFileSrv_RemoteUploadChannelClient) {
	defer client.Close()
	for {
		reply, err := stream.Recv()
		if err != nil {
			d.resetUploadChannelIfCurrent(client, stream)
			return
		}
		uploadID := reply.GetUploadId()
		switch request := reply.GetRequest().(type) {
		case *cd2pb.RemoteUploadChannelReply_ReadData:
			if request.ReadData != nil {
				d.handleRemoteReadRequest(ctx, client, uploadID, request.ReadData)
			}
		case *cd2pb.RemoteUploadChannelReply_StatusChanged:
			if request.StatusChanged != nil {
				d.handleUploadStatusChanged(uploadID, request.StatusChanged)
			}
		case *cd2pb.RemoteUploadChannelReply_HashData:
			if request.HashData != nil {
				d.handleRemoteHashRequest(ctx, client, uploadID, request.HashData)
			}
		}
	}
}

func (d *CD2115Driver) uploadSessionClient() (*cd2Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.uploadClient == nil {
		return nil, fmt.Errorf("上传通道尚未建立")
	}
	return d.uploadClient, nil
}

func (d *CD2115Driver) hasUploadChannel() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.channelCtx != nil && d.uploadStream != nil && d.uploadClient != nil
}

func (d *CD2115Driver) resetUploadChannel() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancelChan != nil {
		d.cancelChan()
	}
	d.channelCtx = nil
	d.cancelChan = nil
	d.uploadClient = nil
	d.uploadStream = nil
}

func (d *CD2115Driver) resetUploadChannelIfCurrent(client *cd2Client, stream cd2pb.CloudDriveFileSrv_RemoteUploadChannelClient) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.uploadClient != client || d.uploadStream != stream {
		return
	}
	if d.cancelChan != nil {
		d.cancelChan()
	}
	d.channelCtx = nil
	d.cancelChan = nil
	d.uploadClient = nil
	d.uploadStream = nil
}

func (d *CD2115Driver) handleRemoteReadRequest(ctx context.Context, client *cd2Client, uploadID string, request *cd2pb.RemoteReadDataRequest) {
	session := d.waitForUploadSession(uploadID, 3*time.Second)
	if session == nil {
		return
	}
	session.hashMu.Lock()
	if session.firstReadAt == nil {
		now := time.Now().UTC()
		session.firstReadAt = &now
	}
	session.hashMu.Unlock()

	data, isLast, err := session.source.ReadChunk(ctx, int64(request.GetOffset()), int64(request.GetLength()))
	if err != nil {
		d.failUpload(uploadID, err)
		return
	}

	authCtx, authErr := client.authContext(ctx)
	if authErr != nil {
		d.failUpload(uploadID, authErr)
		return
	}
	_, err = client.client.RemoteReadData(authCtx, &cd2pb.RemoteReadDataUpload{
		UploadId:    uploadID,
		Offset:      request.GetOffset(),
		Length:      request.GetLength(),
		LazyRead:    request.GetLazyRead(),
		Data:        data,
		IsLastChunk: isLast,
	})
	if err != nil {
		d.failUpload(uploadID, err)
	}
}

func (d *CD2115Driver) handleUploadStatusChanged(uploadID string, changed *cd2pb.RemoteUploadStatusChanged) {
	session := d.waitForUploadSession(uploadID, 3*time.Second)
	if session == nil {
		return
	}

	session.hashMu.Lock()
	session.status = changed.GetStatus()
	session.hashMu.Unlock()
	switch changed.GetStatus() {
	case cd2pb.UploadFileInfo_Finish:
		d.cancelHashJobs(session)
		closeSession(session)
	case cd2pb.UploadFileInfo_Cancelled:
		d.cancelHashJobs(session)
		session.err = context.Canceled
		closeSession(session)
	case cd2pb.UploadFileInfo_Skipped:
		d.cancelHashJobs(session)
		session.err = buildCD2UploadStatusError(changed.GetStatus().String(), changed.GetErrorMessage())
		closeSession(session)
	case cd2pb.UploadFileInfo_Error, cd2pb.UploadFileInfo_FatalError:
		d.cancelHashJobs(session)
		session.err = buildCD2UploadStatusError(changed.GetStatus().String(), changed.GetErrorMessage())
		closeSession(session)
	case cd2pb.UploadFileInfo_Pause:
		d.cancelHashJobs(session)
		session.firstReadAt = nil
	default:
	}
}

func (d *CD2115Driver) failUpload(uploadID string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	session := d.uploads[uploadID]
	if session == nil {
		return
	}
	session.err = err
	d.cancelHashJobs(session)
	closeSession(session)
	if isCD2UploadSessionNotFoundError(err) {
		if d.cancelChan != nil {
			d.cancelChan()
		}
		d.channelCtx = nil
		d.cancelChan = nil
		d.uploadClient = nil
		d.uploadStream = nil
	}
}

func (d *CD2115Driver) failAllUploads(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, session := range d.uploads {
		session.err = err
		d.cancelHashJobs(session)
		closeSession(session)
	}
}

func (d *CD2115Driver) handleRemoteHashRequest(ctx context.Context, client *cd2Client, uploadID string, request *cd2pb.RemoteHashDataRequest) {
	session := d.waitForUploadSession(uploadID, 3*time.Second)
	if session == nil {
		return
	}

	hashKey := fmt.Sprintf("%d", request.GetHashType())
	if blockSize := request.GetBlockSize(); blockSize > 0 {
		hashKey = fmt.Sprintf("%s:%d", hashKey, blockSize)
	}

	session.hashMu.Lock()
	if _, exists := session.hashJobs[hashKey]; exists {
		session.hashMu.Unlock()
		return
	}
	hashCtx, cancel := context.WithCancel(ctx)
	session.hashJobs[hashKey] = cancel
	session.hashMu.Unlock()

	go func() {
		defer func() {
			session.hashMu.Lock()
			delete(session.hashJobs, hashKey)
			session.hashMu.Unlock()
		}()
		if err := d.processRemoteHash(hashCtx, client, session, request); err != nil {
			d.failUpload(uploadID, err)
		}
	}()
}

func (d *CD2115Driver) processRemoteHash(ctx context.Context, client *cd2Client, session *cd2UploadSession, request *cd2pb.RemoteHashDataRequest) error {
	total := session.source.Size()
	hashType := cd2pb.CloudDriveFile_HashType(request.GetHashType())
	var (
		bytesHashed int64
		lastReport  time.Time
	)

	sendProgress := func(final bool, hashValue *string, blockHashes []string) error {
		if !final && time.Since(lastReport) < 250*time.Millisecond {
			return nil
		}
		lastReport = time.Now()
		authCtx, err := client.authContext(ctx)
		if err != nil {
			return err
		}
		_, err = client.client.RemoteHashProgress(authCtx, &cd2pb.RemoteHashProgressUpload{
			UploadId:    session.id,
			BytesHashed: uint64(bytesHashed),
			TotalBytes:  uint64(total),
			HashType:    hashType,
			HashValue:   hashValue,
			BlockHashes: blockHashes,
		})
		return err
	}

	switch hashType {
	case cd2pb.CloudDriveFile_Md5:
		return d.hashMD5(ctx, session, request, sendProgress, &bytesHashed)
	case cd2pb.CloudDriveFile_Sha1:
		return d.hashSHA1(ctx, session, sendProgress, &bytesHashed)
	case cd2pb.CloudDriveFile_PikPakSha1:
		return d.hashPikPakSHA1(ctx, session, sendProgress, &bytesHashed)
	default:
		return fmt.Errorf("当前不支持的远程哈希类型: %d", request.GetHashType())
	}
}

func (d *CD2115Driver) hashMD5(ctx context.Context, session *cd2UploadSession, request *cd2pb.RemoteHashDataRequest, report func(bool, *string, []string) error, bytesHashed *int64) error {
	blockSize := int64(request.GetBlockSize())
	if blockSize <= 0 {
		blockSize = 1 << 20
	}
	fileHasher := md5.New()
	blockHashes := make([]string, 0)
	for offset := int64(0); offset < session.source.Size(); offset += blockSize {
		select {
		case <-ctx.Done():
			return report(true, nil, nil)
		default:
		}
		chunk, _, err := session.source.ReadChunk(ctx, offset, minInt64(blockSize, session.source.Size()-offset))
		if err != nil {
			return err
		}
		_, _ = fileHasher.Write(chunk)
		if request.GetBlockSize() > 0 {
			sum := md5.Sum(chunk)
			blockHashes = append(blockHashes, strings.ToLower(hex.EncodeToString(sum[:])))
		}
		*bytesHashed += int64(len(chunk))
		if err := report(false, nil, nil); err != nil {
			return err
		}
	}
	final := strings.ToLower(hex.EncodeToString(fileHasher.Sum(nil)))
	return report(true, &final, blockHashes)
}

func (d *CD2115Driver) hashSHA1(ctx context.Context, session *cd2UploadSession, report func(bool, *string, []string) error, bytesHashed *int64) error {
	hasher := sha1.New()
	const chunkSize = 1 << 20
	for offset := int64(0); offset < session.source.Size(); offset += chunkSize {
		select {
		case <-ctx.Done():
			return report(true, nil, nil)
		default:
		}
		chunk, _, err := session.source.ReadChunk(ctx, offset, minInt64(chunkSize, session.source.Size()-offset))
		if err != nil {
			return err
		}
		_, _ = hasher.Write(chunk)
		*bytesHashed += int64(len(chunk))
		if err := report(false, nil, nil); err != nil {
			return err
		}
	}
	final := strings.ToLower(hex.EncodeToString(hasher.Sum(nil)))
	return report(true, &final, nil)
}

func (d *CD2115Driver) hashPikPakSHA1(ctx context.Context, session *cd2UploadSession, report func(bool, *string, []string) error, bytesHashed *int64) error {
	segmentSize := pikPakSegmentSize(session.source.Size())
	finalHasher := sha1.New()
	for offset := int64(0); offset < session.source.Size(); offset += segmentSize {
		select {
		case <-ctx.Done():
			return report(true, nil, nil)
		default:
		}
		chunk, _, err := session.source.ReadChunk(ctx, offset, minInt64(segmentSize, session.source.Size()-offset))
		if err != nil {
			return err
		}
		segment := sha1.Sum(chunk)
		_, _ = finalHasher.Write(segment[:])
		*bytesHashed += int64(len(chunk))
		if err := report(false, nil, nil); err != nil {
			return err
		}
	}
	final := strings.ToUpper(hex.EncodeToString(finalHasher.Sum(nil)))
	return report(true, &final, nil)
}

func pikPakSegmentSize(size int64) int64 {
	switch {
	case size <= 128<<20:
		return 256 << 10
	case size <= 256<<20:
		return 512 << 10
	case size <= 512<<20:
		return 1024 << 10
	default:
		return 2048 << 10
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (d *CD2115Driver) waitForUploadSession(uploadID string, timeout time.Duration) *cd2UploadSession {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		session := d.uploads[uploadID]
		d.mu.Unlock()
		if session != nil {
			return session
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}

func (d *CD2115Driver) cancelHashJobs(session *cd2UploadSession) {
	session.hashMu.Lock()
	defer session.hashMu.Unlock()
	for key, cancel := range session.hashJobs {
		cancel()
		delete(session.hashJobs, key)
	}
}

func (d *CD2115Driver) lookupUploadProgress(ctx context.Context, uploadID string, destPath string) (TransferProgress, bool) {
	client, err := d.openClient(ctx)
	if err != nil {
		return TransferProgress{}, false
	}
	defer client.Close()
	authCtx, err := client.authContext(ctx)
	if err != nil {
		return TransferProgress{}, false
	}
	filter := path.Base(destPath)
	const pageSize uint32 = 100
	for page := uint32(0); ; page++ {
		result, err := client.client.GetUploadFileList(authCtx, &cd2pb.GetUploadFileListRequest{
			GetAll:             false,
			ItemsPerPage:       pageSize,
			PageNumber:         page,
			Filter:             filter,
			OperatorTypeFilter: ptr(cd2pb.UploadFileInfo_RemoteUpload),
		})
		if err != nil {
			return TransferProgress{}, false
		}
		for _, item := range result.GetUploadFiles() {
			if !matchCD2UploadItem(item, uploadID, destPath) {
				continue
			}
			status := resolveCD2UploadStatus(item)
			return TransferProgress{
				BytesTotal:     int64(item.GetSize()),
				BytesDone:      int64(item.GetTransferedBytes()),
				SpeedBPS:       int64(result.GetGlobalBytesPerSecond()),
				Message:        describeCD2UploadStatus(status, item.GetErrorMessage()),
				ExternalStatus: status.String(),
				ErrorMessage:   strings.TrimSpace(item.GetErrorMessage()),
				Terminal:       isCD2UploadTerminal(status),
				Successful:     status == cd2pb.UploadFileInfo_Finish,
			}, true
		}
		if int((page+1)*pageSize) >= int(result.GetTotalCount()) || len(result.GetUploadFiles()) == 0 {
			break
		}
	}
	return TransferProgress{}, false
}

func (d *CD2115Driver) ensureCloudSyncEnabled(ctx context.Context, client *cd2Client) error {
	settings, err := client.getSystemSettings(ctx)
	if err != nil {
		return err
	}
	if settings.GetSyncWithCloud() {
		return nil
	}

	settings.SyncWithCloud = ptr(true)
	if err := client.setSystemSettings(ctx, settings); err != nil {
		return err
	}
	return nil
}

func matchCD2UploadItem(item *cd2pb.UploadFileInfo, uploadID string, destPath string) bool {
	if item == nil {
		return false
	}
	if strings.TrimSpace(uploadID) != "" && item.GetKey() == uploadID {
		return true
	}
	return item.GetDestPath() == destPath
}

func resolveCD2UploadStatus(item *cd2pb.UploadFileInfo) cd2pb.UploadFileInfo_Status {
	if item == nil {
		return cd2pb.UploadFileInfo_WaitforPreprocessing
	}
	if item.GetStatusEnum() != cd2pb.UploadFileInfo_WaitforPreprocessing || strings.EqualFold(strings.TrimSpace(item.GetStatus()), cd2pb.UploadFileInfo_WaitforPreprocessing.String()) {
		return item.GetStatusEnum()
	}
	if parsed, ok := parseCD2UploadStatus(strings.TrimSpace(item.GetStatus())); ok {
		return parsed
	}
	return item.GetStatusEnum()
}

func parseCD2UploadStatus(raw string) (cd2pb.UploadFileInfo_Status, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return cd2pb.UploadFileInfo_WaitforPreprocessing, false
	}
	number, ok := cd2pb.UploadFileInfo_Status_value[value]
	if !ok {
		return cd2pb.UploadFileInfo_WaitforPreprocessing, false
	}
	return cd2pb.UploadFileInfo_Status(number), true
}

func isCD2UploadTerminal(status cd2pb.UploadFileInfo_Status) bool {
	switch status {
	case cd2pb.UploadFileInfo_Finish,
		cd2pb.UploadFileInfo_Skipped,
		cd2pb.UploadFileInfo_Cancelled,
		cd2pb.UploadFileInfo_Error,
		cd2pb.UploadFileInfo_FatalError:
		return true
	default:
		return false
	}
}

func isCD2UploadCanceled(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), cd2pb.UploadFileInfo_Cancelled.String())
}

func describeCD2UploadStatus(status cd2pb.UploadFileInfo_Status, errMessage string) string {
	switch status {
	case cd2pb.UploadFileInfo_WaitforPreprocessing, cd2pb.UploadFileInfo_Preprocessing:
		return "文件预处理中"
	case cd2pb.UploadFileInfo_Inqueue:
		return "上传排队中"
	case cd2pb.UploadFileInfo_Transfer:
		return "上传中"
	case cd2pb.UploadFileInfo_Pause:
		return "已暂停"
	case cd2pb.UploadFileInfo_Finish:
		return "已完成"
	case cd2pb.UploadFileInfo_Skipped:
		if strings.TrimSpace(errMessage) != "" {
			return errMessage
		}
		return "已跳过"
	case cd2pb.UploadFileInfo_Cancelled:
		return "已取消"
	case cd2pb.UploadFileInfo_Error, cd2pb.UploadFileInfo_FatalError:
		if strings.TrimSpace(errMessage) != "" {
			return errMessage
		}
		return "上传失败"
	default:
		return status.String()
	}
}

func buildCD2UploadStatusError(status string, errMessage string) error {
	message := strings.TrimSpace(errMessage)
	if message == "" {
		switch strings.TrimSpace(status) {
		case cd2pb.UploadFileInfo_Skipped.String():
			message = "CloudDrive2 上传被跳过"
		case cd2pb.UploadFileInfo_Error.String(), cd2pb.UploadFileInfo_FatalError.String():
			message = "CloudDrive2 上传失败"
		default:
			message = fmt.Sprintf("CloudDrive2 上传异常: %s", strings.TrimSpace(status))
		}
	}
	return fmt.Errorf("%s", message)
}

func buildCD2UploadTerminalError(progress TransferProgress) error {
	if strings.TrimSpace(progress.ExternalStatus) == "" {
		return buildCD2UploadStatusError("", progress.ErrorMessage)
	}
	return buildCD2UploadStatusError(progress.ExternalStatus, progress.ErrorMessage)
}

func isCD2UploadSessionNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "upload session not found")
}

func decodeQRCodeContent(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "data:image") {
		parts := strings.SplitN(value, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("二维码图片格式无效")
		}
		return base64.StdEncoding.DecodeString(parts[1])
	}
	return base64.StdEncoding.DecodeString(value)
}

func resolveCloudAPI(before []*cd2pb.CloudAPI, after []*cd2pb.CloudAPI, vendor string) (*cd2pb.CloudAPI, error) {
	beforePaths := make(map[string]struct{}, len(before))
	for _, item := range before {
		beforePaths[item.GetPath()] = struct{}{}
	}
	for _, item := range after {
		if _, ok := beforePaths[item.GetPath()]; ok {
			continue
		}
		if strings.Contains(strings.ToLower(item.GetName()), strings.ToLower(vendor)) {
			return item, nil
		}
	}
	for _, item := range after {
		if strings.Contains(strings.ToLower(item.GetName()), strings.ToLower(vendor)) {
			return item, nil
		}
	}
	return nil, fmt.Errorf("未能在 CloudDrive2 中定位新接入的 %s 云盘", vendor)
}

func buildProviderAuthResult(vendor string, api *cd2pb.CloudAPI) ProviderAuthResult {
	name := strings.TrimSpace(api.GetNickName())
	if name == "" {
		name = strings.TrimSpace(api.GetName())
	}
	return ProviderAuthResult{
		ProviderVendor: vendor,
		DisplayName:    name,
		Payload: CloudProviderPayload{
			CloudName:     api.GetName(),
			CloudUserName: api.GetUserName(),
			CloudPath:     api.GetPath(),
		},
	}
}

func joinCloudPath(root string, remoteRoot string, relativePath string) string {
	base := normalizeCloudAbsolutePath(root)
	if trimmedRemoteRoot := strings.TrimSpace(strings.ReplaceAll(remoteRoot, "\\", "/")); trimmedRemoteRoot != "" && trimmedRemoteRoot != "." && trimmedRemoteRoot != "/" {
		if strings.HasPrefix(trimmedRemoteRoot, "/") {
			base = normalizeCloudAbsolutePath(trimmedRemoteRoot)
		} else {
			base = normalizeCloudAbsolutePath(path.Join(base, trimmedRemoteRoot))
		}
	}

	trimmedRelativePath := strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	if trimmedRelativePath == "" || trimmedRelativePath == "." || trimmedRelativePath == "/" {
		return base
	}
	if strings.HasPrefix(trimmedRelativePath, "/") {
		absoluteRelativePath := normalizeCloudAbsolutePath(trimmedRelativePath)
		if absoluteRelativePath == base || strings.HasPrefix(absoluteRelativePath, base+"/") {
			return absoluteRelativePath
		}
		return normalizeCloudAbsolutePath(path.Join(base, strings.Trim(absoluteRelativePath, "/")))
	}

	return normalizeCloudAbsolutePath(path.Join(base, trimmedRelativePath))
}

func splitPathSegments(value string) []string {
	clean := strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}

func normalizeCloudAbsolutePath(value string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if normalized == "" || normalized == "." || normalized == "/" {
		return "/"
	}
	cleaned := path.Clean("/" + strings.Trim(normalized, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func buildGatewayDownloadURL(serverURL *url.URL, downloadPath string) string {
	replaced := strings.ReplaceAll(downloadPath, "{SCHEME}", serverURL.Scheme)
	replaced = strings.ReplaceAll(replaced, "{HOST}", serverURL.Host)
	replaced = strings.ReplaceAll(replaced, "{PREVIEW}", "false")
	if strings.HasPrefix(replaced, "http://") || strings.HasPrefix(replaced, "https://") {
		return replaced
	}
	return strings.TrimRight(serverURL.String(), "/") + replaced
}

func closeSession(session *cd2UploadSession) {
	select {
	case <-session.done:
	default:
		close(session.done)
	}
}

func ptr[T any](value T) *T {
	return &value
}
