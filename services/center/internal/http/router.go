package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mare/services/center/internal/agentregistry"
	"mare/services/center/internal/assets"
	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/issues"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/notifications"
	"mare/services/center/internal/response"
	"mare/services/center/internal/runtime"
	"mare/services/center/internal/storage"
	assetdto "mare/shared/contracts/dto/asset"
	issuedto "mare/shared/contracts/dto/issue"
	importdto "mare/shared/contracts/dto/importing"
	integrationdto "mare/shared/contracts/dto/integration"
	jobdto "mare/shared/contracts/dto/job"
	notificationdto "mare/shared/contracts/dto/notification"
	storagedto "mare/shared/contracts/dto/storage"
	tagdto "mare/shared/contracts/dto/tag"
)

type RuntimeService interface {
	Health() runtime.HealthPayload
	Ready(ctx context.Context) (runtime.ReadinessPayload, error)
	Status(ctx context.Context) (runtime.RuntimeStatusPayload, error)
}

type AgentService interface {
	Register(ctx context.Context, registration agentregistry.Registration) (agentregistry.Agent, error)
	Heartbeat(ctx context.Context, heartbeat agentregistry.Heartbeat) (agentregistry.Agent, error)
}

type LocalFolderService interface {
	ListLocalFolders(ctx context.Context) ([]storagedto.LocalFolderRecord, error)
	SaveLocalFolder(ctx context.Context, request storagedto.SaveLocalFolderRequest) (storagedto.SaveLocalFolderResponse, error)
	RunLocalFolderScan(ctx context.Context, ids []string) (storagedto.RunLocalFolderScanResponse, error)
	PrepareMountScanPlan(ctx context.Context, ids []string) (storage.MountScanPlan, error)
	RunLocalFolderConnectionTest(ctx context.Context, ids []string) (storagedto.RunLocalFolderConnectionTestResponse, error)
	UpdateLocalFolderHeartbeat(ctx context.Context, ids []string, heartbeatPolicy string) (storagedto.UpdateHeartbeatResponse, error)
	LoadLocalFolderScanHistory(ctx context.Context, id string) (storagedto.LocalFolderScanHistoryResponse, error)
	DeleteLocalFolder(ctx context.Context, id string) (storagedto.DeleteLocalFolderResponse, error)
}

type LocalNodeService interface {
	ListLocalNodes(ctx context.Context) ([]storagedto.LocalNodeRecord, error)
	SaveLocalNode(ctx context.Context, request storagedto.SaveLocalNodeRequest) (storagedto.SaveLocalNodeResponse, error)
	RunLocalNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunLocalNodeConnectionTestResponse, error)
	DeleteLocalNode(ctx context.Context, id string) (storagedto.DeleteLocalNodeResponse, error)
}

type NASNodeService interface {
	ListNasNodes(ctx context.Context) ([]storagedto.NasRecord, error)
	SaveNasNode(ctx context.Context, request storagedto.SaveNasNodeRequest) (storagedto.SaveNasNodeResponse, error)
	RunNasNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunNasNodeConnectionTestResponse, error)
	DeleteNasNode(ctx context.Context, id string) (storagedto.DeleteNasNodeResponse, error)
}

type CloudNodeService interface {
	ListCloudNodes(ctx context.Context) ([]storagedto.CloudNodeRecord, error)
	SaveCloudNode(ctx context.Context, request storagedto.SaveCloudNodeRequest) (storagedto.SaveCloudNodeResponse, error)
	RunCloudNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunCloudNodeConnectionTestResponse, error)
	DeleteCloudNode(ctx context.Context, id string) (storagedto.DeleteCloudNodeResponse, error)
	CreateCloudQRCodeSession(ctx context.Context, channel string) (storagedto.CloudQRCodeSession, error)
	GetCloudQRCodeStatus(ctx context.Context, session storagedto.CloudQRCodeSession) (storagedto.CloudQRCodeStatusResponse, error)
	FetchCloudQRCodeImage(ctx context.Context, session storagedto.CloudQRCodeSession) ([]byte, string, error)
}

type AssetService interface {
	ListLibraries(ctx context.Context) ([]assetdto.LibraryRecord, error)
	CreateLibrary(ctx context.Context, request assetdto.CreateLibraryRequest) (assetdto.CreateLibraryResponse, error)
	CreateDirectory(ctx context.Context, libraryID string, request assetdto.CreateDirectoryRequest) (assetdto.CreateDirectoryResponse, error)
	PrepareUploadPlan(ctx context.Context, libraryID string, request assetdto.UploadSelectionRequest) (assets.UploadPlan, error)
	UpdateAnnotations(ctx context.Context, id string, request assetdto.UpdateAnnotationsRequest) (assetdto.UpdateAnnotationsResponse, error)
	DeleteEntry(ctx context.Context, id string) (assetdto.DeleteEntryResponse, error)
	PrepareReplicatePlan(ctx context.Context, request assetdto.CreateReplicateJobRequest) (assets.ReplicatePlan, error)
	PrepareDeleteReplicaPlan(ctx context.Context, request assetdto.CreateDeleteReplicaJobRequest) (assets.DeleteReplicaPlan, error)
	PrepareDeleteAssetPlan(ctx context.Context, request assetdto.CreateDeleteAssetJobRequest) (assets.DeleteAssetPlan, error)
	BrowseLibrary(ctx context.Context, libraryID string, query assetdto.BrowseQuery) (assetdto.BrowseLibraryResponse, error)
	LoadEntry(ctx context.Context, id string) (*assetdto.EntryRecord, error)
	ScanDirectory(ctx context.Context, libraryID string, request assetdto.ScanDirectoryRequest) (assetdto.ScanDirectoryResponse, error)
	PrepareDirectoryScanPlan(ctx context.Context, libraryID string, request assetdto.ScanDirectoryRequest) (assets.DirectoryScanPlan, error)
}

type TagService interface {
	LoadManagementSnapshot(ctx context.Context, searchText string) (tagdto.ManagementSnapshot, error)
	ListSuggestions(ctx context.Context, searchText string, libraryID *string) ([]tagdto.Suggestion, error)
	CreateGroup(ctx context.Context, request tagdto.CreateGroupRequest) (tagdto.CreateGroupResponse, error)
	UpdateGroup(ctx context.Context, id string, request tagdto.UpdateGroupRequest) (tagdto.MutationResponse, error)
	MoveGroup(ctx context.Context, id string, request tagdto.MoveRequest) (tagdto.MutationResponse, error)
	CreateTag(ctx context.Context, request tagdto.CreateTagRequest) (tagdto.CreateTagResponse, error)
	UpdateTag(ctx context.Context, id string, request tagdto.UpdateTagRequest) (tagdto.MutationResponse, error)
	MoveTag(ctx context.Context, id string, request tagdto.MoveRequest) (tagdto.MutationResponse, error)
	MergeTag(ctx context.Context, id string, request tagdto.MergeTagRequest) (tagdto.MutationResponse, error)
	DeleteTag(ctx context.Context, id string) (tagdto.MutationResponse, error)
}

type JobService interface {
	CreateMountScanJob(ctx context.Context, plan storage.MountScanPlan) (jobdto.CreateResponse, error)
	CreateDirectoryScanJob(ctx context.Context, plan assets.DirectoryScanPlan) (jobdto.CreateResponse, error)
	CreateUploadJob(ctx context.Context, plan assets.UploadPlan) (jobdto.CreateResponse, error)
	CreateReplicateJob(ctx context.Context, plan assets.ReplicatePlan) (jobdto.CreateResponse, error)
	CreateDeleteReplicaJob(ctx context.Context, plan assets.DeleteReplicaPlan) (jobdto.CreateResponse, error)
	CreateDeleteAssetJob(ctx context.Context, plan assets.DeleteAssetPlan) (jobdto.CreateResponse, error)
	ListJobs(ctx context.Context, query jobs.ListQuery) (jobdto.ListResponse, error)
	LoadJobDetail(ctx context.Context, id string) (jobdto.Detail, error)
	ListJobEvents(ctx context.Context, id string) (jobdto.EventListResponse, error)
	PauseJob(ctx context.Context, id string) (jobdto.MutationResponse, error)
	ResumeJob(ctx context.Context, id string) (jobdto.MutationResponse, error)
	CancelJob(ctx context.Context, id string) (jobdto.MutationResponse, error)
	PauseJobItem(ctx context.Context, id string) (jobdto.ItemMutationResponse, error)
	ResumeJobItem(ctx context.Context, id string) (jobdto.ItemMutationResponse, error)
	CancelJobItem(ctx context.Context, id string) (jobdto.ItemMutationResponse, error)
	RetryJob(ctx context.Context, id string) (jobdto.MutationResponse, error)
	UpdatePriority(ctx context.Context, id string, priority string) (jobdto.MutationResponse, error)
	Subscribe(jobID string) (<-chan jobdto.StreamEvent, func())
}

type IssueService interface {
	ListIssues(ctx context.Context, query issues.ListQuery) (issuedto.ListResponse, error)
	ListByJobIDs(ctx context.Context, jobIDs []string) ([]issuedto.Record, error)
	ApplyAction(ctx context.Context, request issuedto.ActionRequest) (issuedto.ActionResponse, error)
	ClearHistory(ctx context.Context, request issuedto.ClearHistoryRequest) (issuedto.ClearHistoryResponse, error)
}

type NotificationService interface {
	ListNotifications(ctx context.Context, query notifications.ListQuery) (notificationdto.ListResponse, error)
	Subscribe() (<-chan notificationdto.StreamEvent, func())
}

type ImportService interface {
	RefreshDashboard(ctx context.Context) (importdto.DashboardResponse, error)
	LoadDashboard(ctx context.Context) (importdto.DashboardResponse, error)
	BrowseSession(ctx context.Context, sessionID string, path string, limit int, offset int) (importdto.BrowseSessionResponse, error)
	SetDraftLibrary(ctx context.Context, draftID string, libraryID string) (importdto.MutationResponse, error)
	ApplyTargetToAll(ctx context.Context, sessionID string, targetID string) (importdto.MutationResponse, error)
	RemoveTargetFromAll(ctx context.Context, sessionID string, targetID string) (importdto.MutationResponse, error)
	SaveSelectionTargets(ctx context.Context, sessionID string, entryType string, name string, relativePath string, targetIDs []string) (importdto.MutationResponse, error)
	RefreshPrecheck(ctx context.Context, draftID string) (importdto.MutationResponse, error)
	Submit(ctx context.Context, sessionID string) (importdto.SubmitResponse, error)
}

type IntegrationService interface {
	ListGateways(ctx context.Context) (integrationdto.GatewayListResponse, error)
	SaveCD2Gateway(ctx context.Context, request integrationdto.SaveCD2GatewayRequest) (integrationdto.SaveCD2GatewayResponse, error)
	TestCD2Gateway(ctx context.Context, request integrationdto.TestCD2GatewayRequest) (integrationdto.TestCD2GatewayResponse, error)
	RuntimeStatus(ctx context.Context) (integrationdto.RuntimeStatusResponse, error)
}

type Dependencies struct {
	Logger        *slog.Logger
	Runtime       RuntimeService
	Agents        AgentService
	Jobs          JobService
	Issues        IssueService
	Notifications NotificationService
	Imports       ImportService
	Integrations  IntegrationService
	LocalNodes    LocalNodeService
	NasNodes      NASNodeService
	CloudNodes    CloudNodeService
	LocalFolders  LocalFolderService
	Assets        AssetService
	Tags          TagService
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		response.WriteSuccess(w, http.StatusOK, deps.Runtime.Health())
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Runtime.Ready(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/runtime/status", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Runtime.Status(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/integrations/gateways", func(w http.ResponseWriter, r *http.Request) {
		if deps.Integrations == nil {
			writeError(deps.Logger, w, apperrors.NotFound("集成服务尚未启用"))
			return
		}
		payload, err := deps.Integrations.ListGateways(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("PUT /api/integrations/gateways/cd2", func(w http.ResponseWriter, r *http.Request) {
		if deps.Integrations == nil {
			writeError(deps.Logger, w, apperrors.NotFound("集成服务尚未启用"))
			return
		}
		var payload integrationdto.SaveCD2GatewayRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("CloudDrive2 配置请求格式无效"))
			return
		}
		result, err := deps.Integrations.SaveCD2Gateway(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/integrations/gateways/cd2/test", func(w http.ResponseWriter, r *http.Request) {
		if deps.Integrations == nil {
			writeError(deps.Logger, w, apperrors.NotFound("集成服务尚未启用"))
			return
		}
		var payload integrationdto.TestCD2GatewayRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("CloudDrive2 连接测试请求格式无效"))
			return
		}
		result, err := deps.Integrations.TestCD2Gateway(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/integrations/runtime", func(w http.ResponseWriter, r *http.Request) {
		if deps.Integrations == nil {
			writeError(deps.Logger, w, apperrors.NotFound("集成服务尚未启用"))
			return
		}
		payload, err := deps.Integrations.RuntimeStatus(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /agent/register", func(w http.ResponseWriter, r *http.Request) {
		var payload agentregistry.Registration
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("注册请求格式无效"))
			return
		}

		agent, err := deps.Agents.Register(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}

		response.WriteSuccess(w, http.StatusOK, agent)
	})

	mux.HandleFunc("POST /agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var payload agentregistry.Heartbeat
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("心跳请求格式无效"))
			return
		}

		agent, err := deps.Agents.Heartbeat(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}

		response.WriteSuccess(w, http.StatusOK, agent)
	})

	mux.HandleFunc("GET /api/storage/local-folders", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.LocalFolders.ListLocalFolders(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/storage/local-nodes", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.LocalNodes.ListLocalNodes(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/storage/local-nodes", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.SaveLocalNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("本地文件夹保存请求格式无效"))
			return
		}

		result, err := deps.LocalNodes.SaveLocalNode(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-nodes/connection-test", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunLocalNodeConnectionTestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("本地文件夹连接测试请求格式无效"))
			return
		}

		result, err := deps.LocalNodes.RunLocalNodeConnectionTest(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/storage/local-nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.LocalNodes.DeleteLocalNode(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/storage/nas-nodes", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.NasNodes.ListNasNodes(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/storage/nas-nodes", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.SaveNasNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("NAS 保存请求格式无效"))
			return
		}

		result, err := deps.NasNodes.SaveNasNode(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/nas-nodes/connection-test", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunNasNodeConnectionTestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("NAS 连接测试请求格式无效"))
			return
		}

		result, err := deps.NasNodes.RunNasNodeConnectionTest(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/storage/nas-nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.NasNodes.DeleteNasNode(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/storage/cloud-nodes", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.CloudNodes.ListCloudNodes(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/storage/cloud-nodes", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.SaveCloudNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘保存请求格式无效"))
			return
		}

		result, err := deps.CloudNodes.SaveCloudNode(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/cloud-nodes/connection-test", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunCloudNodeConnectionTestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘连接测试请求格式无效"))
			return
		}

		result, err := deps.CloudNodes.RunCloudNodeConnectionTest(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/storage/cloud-nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.CloudNodes.DeleteCloudNode(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/cloud-nodes/qr-session", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.CloudQRCodeSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘二维码会话请求格式无效"))
			return
		}

		result, err := deps.CloudNodes.CreateCloudQRCodeSession(r.Context(), payload.Channel)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/cloud-nodes/qr-session/status", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.CloudQRCodeSession
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘二维码状态请求格式无效"))
			return
		}

		result, err := deps.CloudNodes.GetCloudQRCodeStatus(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/storage/cloud-nodes/qr-session/image", func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("payload")
		if raw == "" {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘二维码图片请求格式无效"))
			return
		}
		decoded, err := url.QueryUnescape(raw)
		if err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘二维码图片请求格式无效"))
			return
		}

		var payload storagedto.CloudQRCodeSession
		if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("网盘二维码图片请求格式无效"))
			return
		}

		image, contentType, err := deps.CloudNodes.FetchCloudQRCodeImage(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(image)
	})

	mux.HandleFunc("GET /api/libraries", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Assets.ListLibraries(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/libraries", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.CreateLibraryRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("资产库创建请求格式无效"))
			return
		}
		result, err := deps.Assets.CreateLibrary(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/libraries/{libraryId}/directories", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.CreateDirectoryRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("目录创建请求格式无效"))
			return
		}

		result, err := deps.Assets.CreateDirectory(r.Context(), r.PathValue("libraryId"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/libraries/{libraryId}/uploads", func(w http.ResponseWriter, r *http.Request) {
		payload, err := parseUploadSelectionRequest(r)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}

		plan, err := deps.Assets.PrepareUploadPlan(r.Context(), r.PathValue("libraryId"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		jobResult, err := deps.Jobs.CreateUploadJob(r.Context(), plan)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		message := "已提交上传作业，任务已加入队列"
		if strings.TrimSpace(jobResult.Message) != "" {
			message = jobResult.Message
		}
		response.WriteSuccess(w, http.StatusAccepted, assetdto.UploadSelectionResponse{
			Message:      message,
			CreatedCount: len(plan.Items),
			JobID:        jobResult.JobID,
		})
	})

	mux.HandleFunc("GET /api/libraries/{libraryId}/browse", func(w http.ResponseWriter, r *http.Request) {
		query, err := parseBrowseQuery(r.URL)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		payload, err := deps.Assets.BrowseLibrary(r.Context(), r.PathValue("libraryId"), query)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/libraries/{libraryId}/scan", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.ScanDirectoryRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("目录扫描请求格式无效"))
			return
		}

		plan, err := deps.Assets.PrepareDirectoryScanPlan(r.Context(), r.PathValue("libraryId"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		result, err := deps.Jobs.CreateDirectoryScanJob(r.Context(), plan)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusAccepted, result)
	})

	mux.HandleFunc("GET /api/file-entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Assets.LoadEntry(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("PATCH /api/file-entries/{id}/annotations", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.UpdateAnnotationsRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标记更新请求格式无效"))
			return
		}

		result, err := deps.Assets.UpdateAnnotations(r.Context(), r.PathValue("id"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/file-entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Assets.DeleteEntry(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/import-center", func(w http.ResponseWriter, r *http.Request) {
		if deps.Imports == nil {
			writeError(deps.Logger, w, apperrors.NotFound("导入中心服务尚未启用"))
			return
		}
		payload, err := deps.Imports.LoadDashboard(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/import-center/refresh", func(w http.ResponseWriter, r *http.Request) {
		if deps.Imports == nil {
			writeError(deps.Logger, w, apperrors.NotFound("导入中心服务尚未启用"))
			return
		}
		payload, err := deps.Imports.RefreshDashboard(r.Context())
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/import-sessions/{id}/browse", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
		offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
		payload, err := deps.Imports.BrowseSession(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"), limit, offset)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("PATCH /api/import-drafts/{id}/library", func(w http.ResponseWriter, r *http.Request) {
		var payload importdto.SetDraftLibraryRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("导入资产库请求格式无效"))
			return
		}
		result, err := deps.Imports.SetDraftLibrary(r.Context(), r.PathValue("id"), payload.LibraryID)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/import-sessions/{id}/targets/{targetId}/apply-all", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Imports.ApplyTargetToAll(r.Context(), r.PathValue("id"), r.PathValue("targetId"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/import-sessions/{id}/targets/{targetId}/remove-all", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Imports.RemoveTargetFromAll(r.Context(), r.PathValue("id"), r.PathValue("targetId"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("PATCH /api/import-sessions/{id}/selections", func(w http.ResponseWriter, r *http.Request) {
		var payload importdto.SetSourceTargetsRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("导入文件目标端请求格式无效"))
			return
		}
		result, err := deps.Imports.SaveSelectionTargets(
			r.Context(),
			r.PathValue("id"),
			payload.EntryType,
			payload.Name,
			payload.RelativePath,
			payload.TargetEndpointIDs,
		)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/import-drafts/{id}/precheck", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Imports.RefreshPrecheck(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/import-sessions/{id}/submit", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Imports.Submit(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/file-entries/replicate", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.CreateReplicateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("同步请求格式无效"))
			return
		}

		plan, err := deps.Assets.PrepareReplicatePlan(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		result, err := deps.Jobs.CreateReplicateJob(r.Context(), plan)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusAccepted, result)
	})

	mux.HandleFunc("POST /api/file-entries/delete-replicas", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.CreateDeleteReplicaJobRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("副本删除请求格式无效"))
			return
		}

		plan, err := deps.Assets.PrepareDeleteReplicaPlan(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		result, err := deps.Jobs.CreateDeleteReplicaJob(r.Context(), plan)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusAccepted, result)
	})

	mux.HandleFunc("POST /api/file-entries/delete-assets", func(w http.ResponseWriter, r *http.Request) {
		var payload assetdto.CreateDeleteAssetJobRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("资产删除请求格式无效"))
			return
		}

		plan, err := deps.Assets.PrepareDeleteAssetPlan(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		result, err := deps.Jobs.CreateDeleteAssetJob(r.Context(), plan)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusAccepted, result)
	})

	mux.HandleFunc("GET /api/tags/management", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Tags.LoadManagementSnapshot(r.Context(), strings.TrimSpace(r.URL.Query().Get("searchText")))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/tags/suggestions", func(w http.ResponseWriter, r *http.Request) {
		var libraryID *string
		if value := strings.TrimSpace(r.URL.Query().Get("libraryId")); value != "" {
			libraryID = &value
		}
		payload, err := deps.Tags.ListSuggestions(r.Context(), strings.TrimSpace(r.URL.Query().Get("searchText")), libraryID)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/tags/groups", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.CreateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签分组创建请求格式无效"))
			return
		}
		result, err := deps.Tags.CreateGroup(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("PATCH /api/tags/groups/{id}", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.UpdateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签分组更新请求格式无效"))
			return
		}
		result, err := deps.Tags.UpdateGroup(r.Context(), r.PathValue("id"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/tags/groups/{id}/move", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.MoveRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签分组排序请求格式无效"))
			return
		}
		result, err := deps.Tags.MoveGroup(r.Context(), r.PathValue("id"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/tags", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.CreateTagRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签创建请求格式无效"))
			return
		}
		result, err := deps.Tags.CreateTag(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("PATCH /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.UpdateTagRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签更新请求格式无效"))
			return
		}
		result, err := deps.Tags.UpdateTag(r.Context(), r.PathValue("id"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/tags/{id}/move", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.MoveRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签排序请求格式无效"))
			return
		}
		result, err := deps.Tags.MoveTag(r.Context(), r.PathValue("id"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/tags/{id}/merge", func(w http.ResponseWriter, r *http.Request) {
		var payload tagdto.MergeTagRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("标签合并请求格式无效"))
			return
		}
		result, err := deps.Tags.MergeTag(r.Context(), r.PathValue("id"), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Tags.DeleteTag(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-folders", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.SaveLocalFolderRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("挂载文件夹保存请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.SaveLocalFolder(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/storage/local-folders/scan", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunLocalFolderScanRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("扫描请求格式无效"))
			return
		}

		plan, err := deps.LocalFolders.PrepareMountScanPlan(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		result, err := deps.Jobs.CreateMountScanJob(r.Context(), plan)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusAccepted, result)
	})

	mux.HandleFunc("POST /api/storage/local-folders/connection-test", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.RunLocalFolderConnectionTestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("连接测试请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.RunLocalFolderConnectionTest(r.Context(), payload.IDs)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("PATCH /api/storage/local-folders/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var payload storagedto.UpdateHeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("心跳更新请求格式无效"))
			return
		}

		result, err := deps.LocalFolders.UpdateLocalFolderHeartbeat(r.Context(), payload.IDs, payload.HeartbeatPolicy)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/storage/local-folders/{id}/scan-history", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.LocalFolders.LoadLocalFolderScanHistory(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /api/storage/local-folders/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.LocalFolders.DeleteLocalFolder(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		query, err := parseJobListQuery(r.URL)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		payload, err := deps.Jobs.ListJobs(r.Context(), query)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.LoadJobDetail(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/jobs/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.ListJobEvents(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/jobs/{id}/pause", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.PauseJob(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/jobs/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.ResumeJob(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.CancelJob(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/job-items/{id}/pause", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.PauseJobItem(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/job-items/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.ResumeJobItem(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/job-items/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.CancelJobItem(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /api/jobs/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Jobs.RetryJob(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("PATCH /api/jobs/{id}/priority", func(w http.ResponseWriter, r *http.Request) {
		var payload jobdto.UpdatePriorityRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("优先级更新请求格式无效"))
			return
		}
		result, err := deps.Jobs.UpdatePriority(r.Context(), r.PathValue("id"), payload.Priority)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/issues", func(w http.ResponseWriter, r *http.Request) {
		query := issues.ListQuery{
			Page:          parsePositiveInt(r.URL.Query().Get("page"), 1),
			PageSize:      parsePositiveInt(r.URL.Query().Get("pageSize"), 20),
			SearchText:    strings.TrimSpace(r.URL.Query().Get("searchText")),
			IssueCategory: strings.TrimSpace(r.URL.Query().Get("issueCategory")),
			SourceDomain:  strings.TrimSpace(r.URL.Query().Get("sourceDomain")),
			LibraryID:     strings.TrimSpace(r.URL.Query().Get("libraryId")),
			Status:        strings.TrimSpace(r.URL.Query().Get("status")),
			Severity:      strings.TrimSpace(r.URL.Query().Get("severity")),
			Nature:        strings.TrimSpace(r.URL.Query().Get("nature")),
			SortValue:     strings.TrimSpace(r.URL.Query().Get("sortValue")),
			EndpointID:    strings.TrimSpace(r.URL.Query().Get("endpointId")),
			Path:          strings.TrimSpace(r.URL.Query().Get("path")),
			JobIDs:        r.URL.Query()["jobId"],
		}
		payload, err := deps.Issues.ListIssues(r.Context(), query)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/issues/by-jobs", func(w http.ResponseWriter, r *http.Request) {
		payload, err := deps.Issues.ListByJobIDs(r.Context(), r.URL.Query()["jobId"])
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, map[string]any{"items": payload})
	})

	mux.HandleFunc("POST /api/issues/actions", func(w http.ResponseWriter, r *http.Request) {
		var payload issuedto.ActionRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("异常动作请求格式无效"))
			return
		}
		result, err := deps.Issues.ApplyAction(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/issues/history/clear", func(w http.ResponseWriter, r *http.Request) {
		var payload issuedto.ClearHistoryRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(deps.Logger, w, apperrors.BadRequest("清理异常历史请求格式无效"))
			return
		}
		result, err := deps.Issues.ClearHistory(r.Context(), payload)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/notifications", func(w http.ResponseWriter, r *http.Request) {
		if deps.Notifications == nil {
			writeError(deps.Logger, w, apperrors.Internal("通知服务未启用"))
			return
		}
		query := notifications.ListQuery{
			Page:         parsePositiveInt(r.URL.Query().Get("page"), 1),
			PageSize:     parsePositiveInt(r.URL.Query().Get("pageSize"), 50),
			Kind:         strings.TrimSpace(r.URL.Query().Get("kind")),
			SearchText:   strings.TrimSpace(r.URL.Query().Get("searchText")),
			IncludeStale: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeStale")), "true"),
		}
		payload, err := deps.Notifications.ListNotifications(r.Context(), query)
		if err != nil {
			writeError(deps.Logger, w, err)
			return
		}
		response.WriteSuccess(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /api/notifications/stream", func(w http.ResponseWriter, r *http.Request) {
		if deps.Notifications == nil {
			writeError(deps.Logger, w, apperrors.Internal("通知服务未启用"))
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(deps.Logger, w, apperrors.Internal("当前连接不支持事件流"))
			return
		}

		events, cancel := deps.Notifications.Subscribe()
		defer cancel()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				payload, err := json.Marshal(event)
				if err != nil {
					continue
				}
				_, _ = io.WriteString(w, "event: "+event.EventType+"\n")
				_, _ = io.WriteString(w, "data: "+string(payload)+"\n\n")
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("GET /api/events/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(deps.Logger, w, apperrors.Internal("当前连接不支持事件流"))
			return
		}

		jobID := strings.TrimSpace(r.URL.Query().Get("jobId"))
		events, cancel := deps.Jobs.Subscribe(jobID)
		defer cancel()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				_, _ = w.Write([]byte(": keep-alive\n\n"))
				flusher.Flush()
			case event, ok := <-events:
				if !ok {
					return
				}
				raw, err := json.Marshal(event)
				if err != nil {
					continue
				}
				_, _ = w.Write([]byte("event: " + event.EventType + "\n"))
				_, _ = w.Write([]byte("data: " + string(raw) + "\n\n"))
				flusher.Flush()
			}
		}
	})

	return withCORS(mux)
}

type uploadManifestItem struct {
	Field        string `json:"field"`
	Name         string `json:"name"`
	RelativePath string `json:"relativePath"`
}

func parseBrowseQuery(raw *url.URL) (assetdto.BrowseQuery, error) {
	values := raw.Query()
	page := 1
	if value := strings.TrimSpace(values.Get("page")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return assetdto.BrowseQuery{}, apperrors.BadRequest("page 参数无效")
		}
		page = parsed
	}

	pageSize := 20
	if value := strings.TrimSpace(values.Get("pageSize")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return assetdto.BrowseQuery{}, apperrors.BadRequest("pageSize 参数无效")
		}
		pageSize = parsed
	}

	var parentID *string
	if value := strings.TrimSpace(values.Get("parentId")); value != "" {
		parentID = &value
	}

	return assetdto.BrowseQuery{
		ParentID:                 parentID,
		Page:                     page,
		PageSize:                 pageSize,
		SearchText:               values.Get("searchText"),
		FileType:                 values.Get("fileTypeFilter"),
		StatusFilter:             values.Get("statusFilter"),
		SortValue:                values.Get("sortValue"),
		SortDirection:            values.Get("sortDirection"),
		PartialSyncEndpointNames: values["partialSyncEndpointName"],
	}, nil
}

func parsePositiveInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseJobListQuery(raw *url.URL) (jobs.ListQuery, error) {
	values := raw.Query()

	page := 1
	if value := strings.TrimSpace(values.Get("page")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return jobs.ListQuery{}, apperrors.BadRequest("page 参数无效")
		}
		page = parsed
	}

	pageSize := 20
	if value := strings.TrimSpace(values.Get("pageSize")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return jobs.ListQuery{}, apperrors.BadRequest("pageSize 参数无效")
		}
		pageSize = parsed
	}

	return jobs.ListQuery{
		Page:         page,
		PageSize:     pageSize,
		SearchText:   strings.TrimSpace(values.Get("searchText")),
		Status:       strings.TrimSpace(values.Get("status")),
		JobFamily:    strings.TrimSpace(values.Get("jobFamily")),
		SourceDomain: strings.TrimSpace(values.Get("sourceDomain")),
		LibraryID:    strings.TrimSpace(values.Get("libraryId")),
	}, nil
}

func parseUploadSelectionRequest(r *http.Request) (assetdto.UploadSelectionRequest, error) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		return assetdto.UploadSelectionRequest{}, apperrors.BadRequest("上传请求格式无效")
	}

	mode := strings.TrimSpace(r.FormValue("mode"))
	var parentID *string
	if value := strings.TrimSpace(r.FormValue("parentId")); value != "" {
		parentID = &value
	}

	var manifest []uploadManifestItem
	if err := json.Unmarshal([]byte(r.FormValue("manifest")), &manifest); err != nil {
		return assetdto.UploadSelectionRequest{}, apperrors.BadRequest("上传清单格式无效")
	}
	if len(manifest) == 0 {
		return assetdto.UploadSelectionRequest{}, apperrors.BadRequest("未选择任何上传内容")
	}

	files := make([]assetdto.UploadSelectionFile, 0, len(manifest))
	for _, item := range manifest {
		headers := r.MultipartForm.File[item.Field]
		if len(headers) == 0 {
			return assetdto.UploadSelectionRequest{}, apperrors.BadRequest("上传文件内容缺失")
		}
		file, err := headers[0].Open()
		if err != nil {
			return assetdto.UploadSelectionRequest{}, apperrors.BadRequest("上传文件读取失败")
		}
		content, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			return assetdto.UploadSelectionRequest{}, apperrors.BadRequest("上传文件读取失败")
		}
		files = append(files, assetdto.UploadSelectionFile{
			Name:         item.Name,
			RelativePath: item.RelativePath,
			Size:         int64(len(content)),
			Content:      content,
		})
	}

	return assetdto.UploadSelectionRequest{
		ParentID: parentID,
		Mode:     mode,
		Files:    files,
	}, nil
}

func writeError(logger *slog.Logger, w http.ResponseWriter, err error) {
	if logger != nil {
		logger.Error("request failed", slog.String("error", err.Error()))
	}

	statusCode, code, message := apperrors.ToHTTP(err)
	response.WriteError(w, statusCode, code, message)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
