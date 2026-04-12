package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const aria2DownloadURL = "https://github.com/aria2/aria2/releases/download/release-1.37.0/aria2-1.37.0-win-64bit-build1.zip"

type Aria2Manager struct {
	now        func() time.Time
	httpClient *http.Client

	mu        sync.Mutex
	status    RuntimeComponent
	startOnce sync.Once
	process   *exec.Cmd
	rpcURL    string
	rpcSecret string
}

func NewAria2Manager() *Aria2Manager {
	return &Aria2Manager{
		now:        time.Now,
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		status: RuntimeComponent{
			Name:    "aria2",
			Status:  "UNKNOWN",
			Message: "aria2 尚未启动",
		},
	}
}

func (a *Aria2Manager) Name() string {
	return "ARIA2"
}

func (a *Aria2Manager) Start(ctx context.Context) {
	a.startOnce.Do(func() {
		go a.startProcess(ctx)
	})
}

func (a *Aria2Manager) RuntimeStatus() RuntimeComponent {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

func (a *Aria2Manager) Enqueue(ctx context.Context, request DownloadRequest) (string, error) {
	if err := a.ensureReady(ctx); err != nil {
		return "", err
	}
	destinationDir := filepath.Dir(request.DestinationPath)
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return "", err
	}

	options := map[string]any{
		"dir":                       destinationDir,
		"out":                       filepath.Base(request.DestinationPath),
		"allow-overwrite":           "true",
		"auto-file-renaming":        "false",
		"continue":                  "true",
		"max-connection-per-server": "8",
	}
	if strings.TrimSpace(request.UserAgent) != "" {
		options["user-agent"] = request.UserAgent
	}
	if len(request.AdditionalHeaders) > 0 {
		headers := make([]string, 0, len(request.AdditionalHeaders))
		for key, value := range request.AdditionalHeaders {
			headers = append(headers, key+": "+value)
		}
		options["header"] = headers
	}

	var gid string
	if err := a.rpc(ctx, "aria2.addUri", []any{"token:" + a.rpcSecret, []string{request.URL}, options}, &gid); err != nil {
		return "", err
	}
	return gid, nil
}

func (a *Aria2Manager) Recover(ctx context.Context, taskID string, request DownloadRequest) (string, error) {
	if err := a.ensureReady(ctx); err != nil {
		return "", err
	}
	if strings.TrimSpace(taskID) != "" {
		if _, found, err := a.tellStatus(ctx, taskID); err == nil && found {
			return taskID, nil
		}
	}
	gid, err := a.findTaskByDestination(ctx, request.DestinationPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(gid) != "" {
		return gid, nil
	}
	return a.Enqueue(ctx, request)
}

func (a *Aria2Manager) Wait(ctx context.Context, taskID string, notify func(TransferProgress)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		status, found, err := a.tellStatus(ctx, taskID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("aria2 任务不存在或已丢失")
		}
		progress := TransferProgress{
			BytesTotal: parseAria2Int(status.TotalLength),
			BytesDone:  parseAria2Int(status.CompletedLength),
			SpeedBPS:   parseAria2Int(status.DownloadSpeed),
			Message:    status.Status,
		}
		if strings.TrimSpace(status.ErrorMessage) != "" {
			progress.Message = status.ErrorMessage
		}
		if notify != nil {
			notify(progress)
		}
		switch status.Status {
		case "complete":
			return nil
		case "error", "removed":
			if strings.TrimSpace(status.ErrorMessage) != "" {
				return fmt.Errorf("%s", status.ErrorMessage)
			}
			return fmt.Errorf("aria2 下载失败")
		}
	}
}

func (a *Aria2Manager) Pause(ctx context.Context, taskID string) error {
	return a.rpc(ctx, "aria2.pause", []any{"token:" + a.rpcSecret, taskID}, nil)
}

func (a *Aria2Manager) Resume(ctx context.Context, taskID string) error {
	return a.rpc(ctx, "aria2.unpause", []any{"token:" + a.rpcSecret, taskID}, nil)
}

func (a *Aria2Manager) Cancel(ctx context.Context, taskID string) error {
	return a.rpc(ctx, "aria2.forceRemove", []any{"token:" + a.rpcSecret, taskID}, nil)
}

func (a *Aria2Manager) ensureReady(ctx context.Context) error {
	a.mu.Lock()
	status := a.status
	a.mu.Unlock()
	if status.Status == "ONLINE" {
		return nil
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		status = a.status
		a.mu.Unlock()
		if status.Status == "ONLINE" {
			return nil
		}
		if status.Status == "ERROR" {
			return fmt.Errorf("%s", status.Message)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("aria2 未在预期时间内就绪")
}

func (a *Aria2Manager) startProcess(ctx context.Context) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		a.setStatus("ERROR", "当前平台暂不支持 aria2 集成", "unsupported_platform", "仅支持 Windows x64")
		return
	}
	binaryPath, stateDir, err := a.ensureBinary(ctx)
	if err != nil {
		a.setStatus("ERROR", "下载 aria2 二进制失败", "download_failed", err.Error())
		return
	}

	rpcSecret, err := a.ensureRPCSecret(stateDir)
	if err != nil {
		a.setStatus("ERROR", "初始化 aria2 运行状态失败", "state_init_failed", err.Error())
		return
	}

	a.mu.Lock()
	a.rpcURL = "http://127.0.0.1:16818/jsonrpc"
	a.rpcSecret = rpcSecret
	a.mu.Unlock()

	if err := a.rpc(context.Background(), "aria2.getVersion", []any{"token:" + rpcSecret}, nil); err == nil {
		a.setStatus("ONLINE", "aria2 运行正常", "", "")
		return
	}
	if aria2PortReachable() {
		_ = stopExistingAria2Processes()
		time.Sleep(1 * time.Second)
	}

	sessionFile := filepath.Join(stateDir, "aria2.session")
	if err := ensureAria2SessionFile(sessionFile); err != nil {
		a.setStatus("ERROR", "初始化 aria2 会话文件失败", "session_file_failed", err.Error())
		return
	}
	cmd := exec.Command(
		binaryPath,
		"--enable-rpc=true",
		"--rpc-listen-all=false",
		"--rpc-listen-port=16818",
		"--rpc-secret="+rpcSecret,
		"--check-certificate=false",
		"--continue=true",
		"--auto-file-renaming=false",
		"--input-file="+sessionFile,
		"--save-session="+sessionFile,
		"--save-session-interval=1",
	)
	if err := cmd.Start(); err != nil {
		a.setStatus("ERROR", "启动 aria2 失败", "start_failed", err.Error())
		return
	}

	a.mu.Lock()
	a.process = cmd
	a.mu.Unlock()

	for i := 0; i < 40; i++ {
		if err := a.rpc(context.Background(), "aria2.getVersion", []any{"token:" + rpcSecret}, nil); err == nil {
			a.setStatus("ONLINE", "aria2 运行正常", "", "")
			go func() {
				_ = cmd.Wait()
				a.setStatus("ERROR", "aria2 进程已退出", "process_exited", "aria2 进程异常退出")
			}()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	a.setStatus("ERROR", "aria2 未能就绪", "not_ready", "aria2 启动后未能通过 RPC 健康检查")
}

func (a *Aria2Manager) ensureBinary(ctx context.Context) (string, string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", "", err
	}
	stateDir := filepath.Join(binDir, "aria2")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", "", err
	}
	target := filepath.Join(binDir, "aria2c.exe")
	if _, err := os.Stat(target); err == nil {
		return target, stateDir, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, aria2DownloadURL, nil)
	if err != nil {
		return "", "", err
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return "", "", fmt.Errorf("下载 aria2 失败: HTTP %d", response.StatusCode)
	}

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", err
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", "", err
	}
	for _, file := range archive.File {
		if !strings.EqualFold(filepath.Base(file.Name), "aria2c.exe") {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return "", "", err
		}
		defer reader.Close()
		output, err := os.Create(target)
		if err != nil {
			return "", "", err
		}
		if _, err := io.Copy(output, reader); err != nil {
			output.Close()
			return "", "", err
		}
		if err := output.Close(); err != nil {
			return "", "", err
		}
		return target, stateDir, nil
	}
	return "", "", fmt.Errorf("压缩包中未找到 aria2c.exe")
}

func (a *Aria2Manager) ensureRPCSecret(stateDir string) (string, error) {
	secretPath := filepath.Join(stateDir, "rpc-secret.txt")
	if raw, err := os.ReadFile(secretPath); err == nil {
		value := strings.TrimSpace(string(raw))
		if value != "" {
			return value, nil
		}
	}
	value := buildIntegrationCode("aria2-secret")
	if err := os.WriteFile(secretPath, []byte(value), 0o600); err != nil {
		return "", err
	}
	return value, nil
}

func ensureAria2SessionFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func aria2PortReachable() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:16818", 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func stopExistingAria2Processes() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	command := exec.Command("taskkill", "/IM", "aria2c.exe", "/F")
	return command.Run()
}

func (a *Aria2Manager) tellStatus(ctx context.Context, taskID string) (aria2TellStatusResponse, bool, error) {
	var status aria2TellStatusResponse
	err := a.rpc(ctx, "aria2.tellStatus", []any{"token:" + a.rpcSecret, taskID, []string{"status", "totalLength", "completedLength", "downloadSpeed", "errorMessage"}}, &status)
	if err == nil {
		return status, true, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return aria2TellStatusResponse{}, false, nil
	}
	return aria2TellStatusResponse{}, false, err
}

func (a *Aria2Manager) findTaskByDestination(ctx context.Context, destinationPath string) (string, error) {
	target := filepath.Clean(destinationPath)
	lists := []struct {
		method string
		params []any
	}{
		{method: "aria2.tellActive", params: []any{"token:" + a.rpcSecret, []string{"gid", "dir", "files"}}},
		{method: "aria2.tellWaiting", params: []any{"token:" + a.rpcSecret, 0, 1000, []string{"gid", "dir", "files"}}},
		{method: "aria2.tellStopped", params: []any{"token:" + a.rpcSecret, 0, 1000, []string{"gid", "dir", "files"}}},
	}
	for _, call := range lists {
		var tasks []aria2TaskRecord
		if err := a.rpc(ctx, call.method, call.params, &tasks); err != nil {
			return "", err
		}
		for _, task := range tasks {
			if filepath.Clean(task.destinationPath()) == target {
				return task.GID, nil
			}
		}
	}
	return "", nil
}

func (a *Aria2Manager) rpc(ctx context.Context, method string, params []any, result any) error {
	a.mu.Lock()
	rpcURL := a.rpcURL
	a.mu.Unlock()
	if strings.TrimSpace(rpcURL) == "" {
		return fmt.Errorf("aria2 RPC 尚未就绪")
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      hex.EncodeToString(randomBytes(6)),
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return fmt.Errorf("aria2 RPC 返回状态 %d", response.StatusCode)
	}
	var reply struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		return err
	}
	if reply.Error != nil {
		return fmt.Errorf("aria2 RPC 失败: %s", reply.Error.Message)
	}
	if result != nil && len(reply.Result) > 0 {
		if err := json.Unmarshal(reply.Result, result); err != nil {
			return err
		}
	}
	return nil
}

func (a *Aria2Manager) setStatus(status string, message string, errorCode string, errorMessage string) {
	now := a.now().UTC()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = RuntimeComponent{
		Name:             "aria2",
		Status:           status,
		Message:          message,
		LastCheckedAt:    &now,
		LastErrorCode:    errorCode,
		LastErrorMessage: errorMessage,
	}
}

type aria2TellStatusResponse struct {
	Status          string `json:"status"`
	TotalLength     string `json:"totalLength"`
	CompletedLength string `json:"completedLength"`
	DownloadSpeed   string `json:"downloadSpeed"`
	ErrorMessage    string `json:"errorMessage"`
}

type aria2TaskRecord struct {
	GID   string `json:"gid"`
	Dir   string `json:"dir"`
	Files []struct {
		Path string `json:"path"`
	} `json:"files"`
}

func (t aria2TaskRecord) destinationPath() string {
	if len(t.Files) > 0 && strings.TrimSpace(t.Files[0].Path) != "" {
		return t.Files[0].Path
	}
	return t.Dir
}

func parseAria2Int(value string) int64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	var result int64
	_, _ = fmt.Sscan(value, &result)
	return result
}

func randomBytes(size int) []byte {
	buffer := make([]byte, size)
	_, _ = rand.Read(buffer)
	return buffer
}
