package integration

import (
	"context"
	"time"
)

type CloudProviderPayload struct {
	CloudName     string            `json:"cloudName"`
	CloudUserName string            `json:"cloudUserName,omitempty"`
	CloudPath     string            `json:"cloudPath"`
	Extra         map[string]string `json:"extra,omitempty"`
}

type ProviderAuthResult struct {
	ProviderVendor string
	DisplayName    string
	Payload        CloudProviderPayload
}

type QRCodeSession struct {
	ID        string
	Channel   string
	ImageURL  string
	ImageData []byte
	Status    string
	Message   string
	Result    *ProviderAuthResult
	ExpiresAt *time.Time
}

type DownloadSource struct {
	URL               string
	UserAgent         string
	AdditionalHeaders map[string]string
	ExpiresAt         *time.Time
}

type TransferProgress struct {
	BytesTotal     int64
	BytesDone      int64
	SpeedBPS       int64
	Message        string
	ExternalStatus string
	ErrorMessage   string
	Terminal       bool
	Successful     bool
}

type UploadSource interface {
	Size() int64
	ReadChunk(ctx context.Context, offset int64, length int64) ([]byte, bool, error)
	Close() error
}

type DownloadRequest struct {
	URL               string
	DestinationPath   string
	UserAgent         string
	AdditionalHeaders map[string]string
}

type DownloadTask struct {
	ID         string
	Status     string
	BytesTotal int64
	BytesDone  int64
	SpeedBPS   int64
	Message    string
}

type RuntimeComponent struct {
	Name             string
	Status           string
	Message          string
	LastCheckedAt    *time.Time
	LastErrorCode    string
	LastErrorMessage string
}

type CloudProviderDriver interface {
	Vendor() string
	AuthenticateToken(ctx context.Context, token string) (ProviderAuthResult, error)
	CreateQRCodeSession(ctx context.Context, channel string) (QRCodeSession, error)
	GetQRCodeSession(ctx context.Context, sessionID string) (QRCodeSession, error)
	ConsumeQRCodeSession(ctx context.Context, sessionID string) (ProviderAuthResult, error)
	EnsureRemoteRoot(ctx context.Context, payload CloudProviderPayload, remoteRootPath string) error
	StartUpload(ctx context.Context, payload CloudProviderPayload, remoteRootPath string, relativePath string, source UploadSource) (string, string, error)
	AttachUpload(ctx context.Context, externalTaskID string, destinationPath string, source UploadSource) error
	WaitUpload(ctx context.Context, externalTaskID string, destinationPath string, notify func(TransferProgress)) error
	PauseUpload(ctx context.Context, externalTaskID string) error
	ResumeUpload(ctx context.Context, externalTaskID string) error
	CancelUpload(ctx context.Context, externalTaskID string) error
	ResolveDownloadSource(ctx context.Context, payload CloudProviderPayload, remoteRootPath string, relativePath string) (DownloadSource, error)
	DeleteFile(ctx context.Context, payload CloudProviderPayload, remoteRootPath string, relativePath string) error
}

type DownloadEngine interface {
	Name() string
	Start(ctx context.Context)
	RuntimeStatus() RuntimeComponent
	Enqueue(ctx context.Context, request DownloadRequest) (string, error)
	Recover(ctx context.Context, taskID string, request DownloadRequest) (string, error)
	Wait(ctx context.Context, taskID string, notify func(TransferProgress)) error
	Pause(ctx context.Context, taskID string) error
	Resume(ctx context.Context, taskID string) error
	Cancel(ctx context.Context, taskID string) error
}

type CD2GatewayConfig struct {
	ID               string
	BaseURL          string
	Username         string
	Password         string
	ClientDeviceID   string
	Enabled          bool
	RuntimeStatus    string
	LastTestAt       *time.Time
	LastErrorCode    string
	LastErrorMessage string
}
