package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/integration"
	storagedto "mare/shared/contracts/dto/storage"
)

type CloudNodeService struct {
	pool   *pgxpool.Pool
	now    func() time.Time
	cipher credentialCipher
	client *http.Client
	integration interface {
		Provider(vendor string) (integration.CloudProviderDriver, error)
	}
}

type cloudCredentialProbeResult struct {
	Authenticated bool
	Message       string
}

type openOAuthAuthenticator interface {
	AuthenticateOpenToken(ctx context.Context, token integration.OpenOAuthToken) (integration.ProviderAuthResult, error)
}

const default115OpenAppID = "100195125"

func NewCloudNodeService(pool *pgxpool.Pool, integrations ...interface {
	Provider(vendor string) (integration.CloudProviderDriver, error)
}) *CloudNodeService {
	var integrationService interface {
		Provider(vendor string) (integration.CloudProviderDriver, error)
	}
	if len(integrations) > 0 {
		integrationService = integrations[0]
	}
	return &CloudNodeService{
		pool:   pool,
		now:    time.Now,
		cipher: newSystemCredentialCipher(),
		client: &http.Client{Timeout: 10 * time.Second},
		integration: integrationService,
	}
}

func (s *CloudNodeService) ListCloudNodes(ctx context.Context) ([]storagedto.CloudNodeRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			sn.id,
			sn.name,
			COALESCE(cp.provider_vendor, COALESCE(sn.vendor, '115')),
			COALESCE(cp.auth_method, COALESCE(sn.access_mode, 'QR')),
			COALESCE(snc.secret_ref, ''),
			COALESCE(cp.remote_root_path, COALESCE(sn.address, '')),
			COALESCE(snc.token_status, 'UNKNOWN'),
			COALESCE(snc.secret_ciphertext, ''),
			snr.last_check_at,
			COALESCE(snr.auth_status, 'UNKNOWN'),
			COALESCE(snr.health_status, 'UNKNOWN'),
			COALESCE(sn.description, ''),
			COUNT(m.id) FILTER (WHERE m.deleted_at IS NULL)
		FROM storage_nodes sn
		LEFT JOIN cloud_node_profiles cp ON cp.storage_node_id = sn.id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		LEFT JOIN storage_node_runtime snr ON snr.storage_node_id = sn.id
		LEFT JOIN mounts m ON m.storage_node_id = sn.id
		WHERE sn.node_type = 'CLOUD'
		  AND sn.deleted_at IS NULL
		GROUP BY
			sn.id, sn.name, cp.provider_vendor, cp.auth_method, snc.secret_ref,
			cp.remote_root_path, snc.token_status, snc.secret_ciphertext, snr.last_check_at,
			snr.auth_status, snr.health_status, sn.description
		ORDER BY sn.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]storagedto.CloudNodeRecord, 0)
	for rows.Next() {
		var (
			id           string
			name         string
			vendor       string
			accessMode   string
			qrChannel    string
			mountPath    string
			tokenStatus  string
			ciphertext   string
			lastCheckAt  *time.Time
			authStatus   string
			healthStatus string
			notes        string
			mountCount   int
		)
		if err := rows.Scan(&id, &name, &vendor, &accessMode, &qrChannel, &mountPath, &tokenStatus, &ciphertext, &lastCheckAt, &authStatus, &healthStatus, &notes, &mountCount); err != nil {
			return nil, err
		}
		savedToken := ""
		if strings.TrimSpace(ciphertext) != "" {
			plaintext, decryptErr := s.cipher.Decrypt(ciphertext)
			if decryptErr == nil {
				savedToken = strings.TrimSpace(plaintext)
			}
		}

		items = append(items, storagedto.CloudNodeRecord{
			ID:           id,
			Name:         name,
			Vendor:       vendor,
			AccessMethod: uiCloudAccessMethod(accessMode),
			QRChannel:    qrChannel,
			MountPath:    mountPath,
			TokenStatus:  uiCloudTokenStatus(tokenStatus),
			Token:        savedToken,
			LastTestAt:   uiNodeLastCheckAt(lastCheckAt),
			Status:       uiCloudStatus(authStatus, healthStatus, tokenStatus),
			Tone:         uiCloudTone(authStatus, healthStatus, tokenStatus),
			MountCount:   mountCount,
			Notes:        notes,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *CloudNodeService) SaveCloudNode(ctx context.Context, request storagedto.SaveCloudNodeRequest) (storagedto.SaveCloudNodeResponse, error) {
	if s.integration == nil {
		return s.saveCloudNodeLegacy(ctx, request)
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Vendor = strings.TrimSpace(request.Vendor)
	request.AccessMethod = strings.TrimSpace(request.AccessMethod)
	request.QRChannel = strings.TrimSpace(request.QRChannel)
	request.MountPath = strings.TrimSpace(request.MountPath)
	request.Token = strings.TrimSpace(request.Token)
	request.Notes = strings.TrimSpace(request.Notes)
	request.AccessMethod = normalizeCloudAccessMethod(request.AccessMethod, request.QRSession != nil, request.Token != "")
	if request.QRChannel != "" {
		request.QRChannel = normalizeCloudQRChannel(request.QRChannel)
	}
	if request.QRChannel == "" && request.QRSession != nil {
		request.QRChannel = normalizeCloudQRChannel(request.QRSession.Channel)
	}

	if request.Name == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("网盘名称不能为空")
	}
	if request.Vendor == "" {
		request.Vendor = "115"
	}
	if request.MountPath == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("挂载目录不能为空")
	}
	if request.AccessMethod == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("接入方式无效")
	}
	if request.AccessMethod == "QR" && request.QRChannel == "" {
		request.QRChannel = "wechatmini"
	}
	driver, err := s.integration.Provider(request.Vendor)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	existingCiphertext := ""
	existingSecretRef := ""
	existingTokenStatus := "UNKNOWN"
	existingPlainCredential := ""
	if request.ID != "" {
		existingCiphertext, existingSecretRef, existingTokenStatus, err = s.loadCloudCredentialEnvelope(ctx, request.ID)
		if err != nil && !isStorageNotFound(err) {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		if err == nil && strings.TrimSpace(existingCiphertext) != "" {
			if plaintext, decryptErr := s.cipher.Decrypt(existingCiphertext); decryptErr == nil {
				existingPlainCredential = strings.TrimSpace(plaintext)
			}
		}
	}

	var (
		authResult      integration.ProviderAuthResult
		plainCredential string
		hasAuthResult   bool
	)
	if strings.TrimSpace(request.Token) != "" {
		plainCredential = strings.TrimSpace(request.Token)
		if request.ID != "" && existingPlainCredential != "" && plainCredential == existingPlainCredential {
			plainCredential = existingPlainCredential
		} else {
			if isCloudOAuthToken(plainCredential) {
				oauthDriver, ok := driver.(openOAuthAuthenticator)
				if !ok {
					return storagedto.SaveCloudNodeResponse{}, apperrors.Internal("当前 CD2 驱动不支持 115open Token 鉴权")
				}
				openToken, tokenErr := exchange115OpenRefreshToken(ctx, s.client, plainCredential)
				if tokenErr != nil {
					return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(tokenErr.Error())
				}
				authResult, err = oauthDriver.AuthenticateOpenToken(ctx, openToken)
				if err != nil {
					return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(err.Error())
				}
			} else {
				authResult, err = driver.AuthenticateToken(ctx, plainCredential)
				if err != nil {
					return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(err.Error())
				}
			}
			hasAuthResult = true
		}
	} else if request.QRSession != nil {
		openToken, tokenErr := exchangeQRCodeSessionToOpenToken(ctx, s.client, *request.QRSession)
		if tokenErr != nil {
			return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(tokenErr.Error())
		}
		oauthDriver, ok := driver.(openOAuthAuthenticator)
		if !ok {
			return storagedto.SaveCloudNodeResponse{}, apperrors.Internal("当前 CD2 驱动不支持 115open Token 鉴权")
		}
		authResult, err = oauthDriver.AuthenticateOpenToken(ctx, openToken)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(err.Error())
		}
		plainCredential = openToken.RefreshToken
		hasAuthResult = true
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}
	defer tx.Rollback(ctx)

	accessMode := dbCloudAccessMode(request.AccessMethod)
	tokenStatus := existingTokenStatus
	ciphertext := existingCiphertext
	secretRef := ""
	authStatus := "UNKNOWN"
	healthStatus := "UNKNOWN"
	if request.AccessMethod == "QR" && request.QRSession == nil && request.ID == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("请先获取扫码二维码")
	}
	if request.AccessMethod == "TOKEN" && strings.TrimSpace(request.Token) == "" && request.ID == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("Token 不能为空")
	}

	profile, profileErr := s.loadCloudProfile(ctx, request.ID)
	if profileErr != nil && request.ID != "" {
		return storagedto.SaveCloudNodeResponse{}, profileErr
	}
	secretRef = existingSecretRef
	if !hasAuthResult {
		authResult = integration.ProviderAuthResult{
			ProviderVendor: request.Vendor,
			Payload:        profile.Payload,
		}
	}
	if request.AccessMethod == "QR" && !hasAuthResult && request.ID == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("扫码登录尚未完成")
	}
	if request.AccessMethod == "TOKEN" && !hasAuthResult && request.ID == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("Token 无效")
	}
	if strings.TrimSpace(plainCredential) != "" {
		ciphertext, err = s.cipher.Encrypt(plainCredential)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		tokenStatus = "CONFIGURED"
	}
	if request.AccessMethod == "QR" {
		secretRef = request.QRChannel
	} else {
		secretRef = ""
	}
	if err := driver.EnsureRemoteRoot(ctx, authResult.Payload, request.MountPath); err != nil {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(err.Error())
	}
	authStatus = "AUTHORIZED"
	healthStatus = "ONLINE"

	if request.ID == "" {
		nodeID := buildCode("cloud-node-id", now)
		_, err = tx.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, description, created_at, updated_at
			) VALUES ($1, $2, $3, 'CLOUD', $4, $5, $6, $7, true, $8, $9, $9)
		`, nodeID, buildCode("cloud-node", now), request.Name, request.Vendor, request.MountPath, accessMode, request.Name, request.Notes, now)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO storage_node_credentials (
				id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
			) VALUES ($1, $2, 'TOKEN', NULLIF($3, ''), NULLIF($4, ''), $5, $6, $6)
		`, buildCode("cloud-node-credential", now), nodeID, ciphertext, secretRef, tokenStatus, now)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO storage_node_runtime (
				id, storage_node_id, health_status, auth_status, last_check_at, last_success_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $5, $5, $5)
		`, buildCode("cloud-node-runtime", now), nodeID, healthStatus, authStatus, now)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}

		request.ID = nodeID
	} else {
		tag, err := tx.Exec(ctx, `
			UPDATE storage_nodes
			SET name = $2,
			    vendor = $3,
			    address = $4,
			    access_mode = $5,
			    account_alias = $6,
			    description = $7,
			    updated_at = $8
			WHERE id = $1
			  AND node_type = 'CLOUD'
			  AND deleted_at IS NULL
		`, request.ID, request.Name, request.Vendor, request.MountPath, accessMode, request.Name, request.Notes, now)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		if tag.RowsAffected() == 0 {
			return storagedto.SaveCloudNodeResponse{}, apperrors.NotFound("网盘节点不存在")
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO storage_node_credentials (
				id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
			) VALUES ($1, $2, 'TOKEN', NULLIF($3, ''), NULLIF($4, ''), $5, $6, $6)
			ON CONFLICT (storage_node_id) DO UPDATE SET
				credential_kind = EXCLUDED.credential_kind,
				secret_ciphertext = EXCLUDED.secret_ciphertext,
				secret_ref = EXCLUDED.secret_ref,
				token_status = EXCLUDED.token_status,
				updated_at = EXCLUDED.updated_at
		`, buildCode("cloud-node-credential", now), request.ID, ciphertext, secretRef, tokenStatus, now)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE storage_node_runtime
			SET health_status = $2,
			    auth_status = $3,
			    last_check_at = $4,
			    last_success_at = $4,
			    last_error_code = NULL,
			    last_error_message = NULL,
			    updated_at = $4
			WHERE storage_node_id = $1
		`, request.ID, healthStatus, authStatus, now)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
	}

	providerPayload, err := integration.MarshalProviderPayload(authResult.Payload)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}
	providerPayloadJSON, err := json.Marshal(providerPayload)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload,
			last_auth_at, last_auth_error_code, last_auth_error_message, updated_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, NULL, NULL, $7, $7
		)
		ON CONFLICT (storage_node_id) DO UPDATE SET
			provider_vendor = EXCLUDED.provider_vendor,
			auth_method = EXCLUDED.auth_method,
			remote_root_path = EXCLUDED.remote_root_path,
			provider_payload = EXCLUDED.provider_payload,
			last_auth_at = EXCLUDED.last_auth_at,
			last_auth_error_code = NULL,
			last_auth_error_message = NULL,
			updated_at = EXCLUDED.updated_at
	`, buildCode("cloud-node-profile", now), request.ID, request.Vendor, request.AccessMethod, request.MountPath, providerPayloadJSON, now)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	record, err := s.loadCloudNodeByID(ctx, request.ID)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	return storagedto.SaveCloudNodeResponse{
		Message: "网盘已保存",
		Record:  record,
	}, nil
}

func (s *CloudNodeService) RunCloudNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunCloudNodeConnectionTestResponse, error) {
	if s.integration == nil {
		return s.runCloudNodeConnectionTestLegacy(ctx, ids)
	}
	if len(ids) == 0 {
		return storagedto.RunCloudNodeConnectionTestResponse{}, apperrors.BadRequest("ids 不能为空")
	}

	now := s.now().UTC()
	results := make([]storagedto.ConnectionTestResult, 0, len(ids))
	for _, id := range ids {
		record, err := s.loadCloudNodeByID(ctx, id)
		if err != nil {
			return storagedto.RunCloudNodeConnectionTestResponse{}, err
		}
		profile, err := s.loadCloudProfile(ctx, id)
		if err != nil {
			return storagedto.RunCloudNodeConnectionTestResponse{}, err
		}
		driver, err := s.integration.Provider(profile.ProviderVendor)
		if err != nil {
			return storagedto.RunCloudNodeConnectionTestResponse{}, err
		}

		overallTone := "critical"
		summary := "未能通过 115 登录态校验"
		authStatus := "FAILED"
		healthStatus := "ERROR"
		lastErrorCode := ""
		lastErrorMessage := ""

		checks := []storagedto.ConnectionCheck{
			{Label: "网盘类型", Status: "success", Detail: record.Vendor},
			{Label: "挂载根目录", Status: "success", Detail: record.MountPath},
		}

		if err := driver.EnsureRemoteRoot(ctx, profile.Payload, record.MountPath); err != nil {
			lastErrorCode = "cloud_root_check_failed"
			lastErrorMessage = err.Error()
			checks = append(checks, storagedto.ConnectionCheck{Label: "远端根目录校验", Status: "critical", Detail: err.Error()})
		} else {
			overallTone = "success"
			summary = "已通过 CloudDrive2 云端连通性校验"
			authStatus = "AUTHORIZED"
			healthStatus = "ONLINE"
			checks = append(checks, storagedto.ConnectionCheck{Label: "远端根目录校验", Status: "success", Detail: "CloudDrive2 已确认云端路径可用"})
		}

		_, err = s.pool.Exec(ctx, `
			UPDATE storage_node_runtime
			SET health_status = $2,
			    auth_status = $3,
			    last_check_at = $4,
			    last_success_at = CASE WHEN $5 THEN $4 ELSE last_success_at END,
			    last_error_code = NULLIF($6, ''),
			    last_error_message = NULLIF($7, ''),
			    updated_at = $4
			WHERE storage_node_id = $1
		`, id, healthStatus, authStatus, now, overallTone == "success", lastErrorCode, lastErrorMessage)
		if err != nil {
			return storagedto.RunCloudNodeConnectionTestResponse{}, err
		}

		results = append(results, storagedto.ConnectionTestResult{
			ID:          id,
			Name:        record.Name,
			OverallTone: overallTone,
			Summary:     summary,
			Checks:      checks,
			Suggestion:  uiCloudSuggestion(overallTone, record.AccessMethod),
			TestedAt:    "刚刚",
		})
	}

	message := "连接测试已完成"
	if len(ids) > 1 {
		message = fmt.Sprintf("已完成 %d 个网盘节点的连接测试", len(ids))
	}
	return storagedto.RunCloudNodeConnectionTestResponse{
		Message: message,
		Results: results,
	}, nil
}

func (s *CloudNodeService) DeleteCloudNode(ctx context.Context, id string) (storagedto.DeleteCloudNodeResponse, error) {
	var mountCount int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM mounts
		WHERE storage_node_id = $1
		  AND deleted_at IS NULL
	`, id).Scan(&mountCount); err != nil {
		return storagedto.DeleteCloudNodeResponse{}, err
	}
	if mountCount > 0 {
		return storagedto.DeleteCloudNodeResponse{}, apperrors.BadRequest("当前网盘节点下仍存在挂载，请先删除挂载")
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE storage_nodes
		SET deleted_at = $2,
		    updated_at = $2
		WHERE id = $1
		  AND node_type = 'CLOUD'
		  AND deleted_at IS NULL
	`, id, s.now().UTC())
	if err != nil {
		return storagedto.DeleteCloudNodeResponse{}, err
	}
	if tag.RowsAffected() == 0 {
		return storagedto.DeleteCloudNodeResponse{}, apperrors.NotFound("网盘节点不存在")
	}

	return storagedto.DeleteCloudNodeResponse{Message: "网盘已删除"}, nil
}

func (s *CloudNodeService) loadCloudNodeByID(ctx context.Context, id string) (storagedto.CloudNodeRecord, error) {
	items, err := s.ListCloudNodes(ctx)
	if err != nil {
		return storagedto.CloudNodeRecord{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return storagedto.CloudNodeRecord{}, apperrors.NotFound("网盘节点不存在")
}

type cloudProfile struct {
	ProviderVendor string
	AuthMethod     string
	RemoteRootPath string
	Payload        integration.CloudProviderPayload
}

func (s *CloudNodeService) loadCloudProfile(ctx context.Context, nodeID string) (cloudProfile, error) {
	if strings.TrimSpace(nodeID) == "" {
		return cloudProfile{}, nil
	}
	var (
		vendor string
		authMethod string
		remoteRoot string
		payload []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT provider_vendor, auth_method, remote_root_path, provider_payload
		FROM cloud_node_profiles
		WHERE storage_node_id = $1
	`, nodeID).Scan(&vendor, &authMethod, &remoteRoot, &payload)
	if err != nil {
		if err == pgx.ErrNoRows {
			return cloudProfile{}, apperrors.NotFound("网盘节点配置不存在")
		}
		return cloudProfile{}, err
	}
	decoded, err := integration.UnmarshalProviderPayload(payload)
	if err != nil {
		return cloudProfile{}, err
	}
	return cloudProfile{
		ProviderVendor: vendor,
		AuthMethod:     authMethod,
		RemoteRootPath: remoteRoot,
		Payload:        decoded,
	}, nil
}

func dbCloudAccessMode(value string) string {
	switch value {
	case "TOKEN":
		return "TOKEN"
	default:
		return "QR"
	}
}

func normalizeCloudAccessMethod(value string, hasQRSession bool, hasToken bool) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if hasQRSession && !hasToken {
		return "QR"
	}
	if hasToken && !hasQRSession && normalized == "" {
		return "TOKEN"
	}
	switch normalized {
	case "TOKEN":
		return "TOKEN"
	case "QR":
		return "QR"
	}

	if strings.Contains(normalized, "TOKEN") {
		if strings.Contains(normalized, "SCAN") || strings.Contains(normalized, "LOGIN") || strings.Contains(normalized, "QR") {
			return "QR"
		}
		return "TOKEN"
	}
	if strings.Contains(value, "扫码") || strings.Contains(value, "登录") || strings.Contains(value, "閹殿偆鐖") {
		return "QR"
	}
	if strings.Contains(value, "填入") || strings.Contains(value, "婵夘偄鍙") {
		return "TOKEN"
	}
	if hasToken {
		return "TOKEN"
	}
	return ""
}

func normalizeCloudQRChannel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "wechatmini", "alipaymini", "tv":
		return normalized
	}

	if strings.Contains(value, "微信") || strings.Contains(value, "寰俊") {
		return "wechatmini"
	}
	if strings.Contains(value, "支付宝") || strings.Contains(value, "鏀粯瀹") || strings.Contains(value, "閺€顖欑帛") {
		return "alipaymini"
	}
	if strings.Contains(value, "电视") || strings.Contains(value, "鐢佃") {
		return "tv"
	}
	return ""
}

func uiCloudAccessMethod(value string) string {
	switch value {
	case "TOKEN":
		return "填入 Token"
	default:
		return "扫码登录获取 Token"
	}
}

func uiCloudTokenStatus(tokenStatus string) string {
	switch tokenStatus {
	case "CONFIGURED":
		return "已配置"
	case "PENDING_SCAN":
		return "待扫码"
	default:
		return "未知"
	}
}

func uiCloudStatus(authStatus string, healthStatus string, tokenStatus string) string {
	switch {
	case authStatus == "AUTHORIZED" && healthStatus == "ONLINE":
		return "鉴权正常"
	case tokenStatus == "PENDING_SCAN":
		return "待扫码"
	case authStatus == "FAILED" || healthStatus == "ERROR":
		return "鉴权异常"
	default:
		return "待检测"
	}
}

func uiCloudTone(authStatus string, healthStatus string, tokenStatus string) string {
	switch {
	case authStatus == "AUTHORIZED" && healthStatus == "ONLINE":
		return "success"
	case tokenStatus == "PENDING_SCAN":
		return "warning"
	case authStatus == "FAILED" || healthStatus == "ERROR":
		return "critical"
	default:
		return "info"
	}
}

func uiCloudSuggestion(tone string, accessMethod string) string {
	if tone == "success" {
		return "可继续创建挂载"
	}
	if accessMethod == "QR" || accessMethod == "扫码登录获取 Token" {
		return "请重新扫码登录并确保已成功换取凭据"
	}
	return "请检查 Token 是否正确"
}

func (s *CloudNodeService) CreateCloudQRCodeSession(ctx context.Context, channel string) (storagedto.CloudQRCodeSession, error) {
	return s.createCloudQRCodeSessionLegacy(ctx, channel)
}

func (s *CloudNodeService) GetCloudQRCodeStatus(ctx context.Context, session storagedto.CloudQRCodeSession) (storagedto.CloudQRCodeStatusResponse, error) {
	return s.getCloudQRCodeStatusLegacy(ctx, session)
}

func (s *CloudNodeService) FetchCloudQRCodeImage(ctx context.Context, session storagedto.CloudQRCodeSession) ([]byte, string, error) {
	return s.fetchCloudQRCodeImageLegacy(ctx, session)
}

func (s *CloudNodeService) saveCloudNodeLegacy(ctx context.Context, request storagedto.SaveCloudNodeRequest) (storagedto.SaveCloudNodeResponse, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Vendor = strings.TrimSpace(request.Vendor)
	request.AccessMethod = normalizeCloudAccessMethod(strings.TrimSpace(request.AccessMethod), request.QRSession != nil, strings.TrimSpace(request.Token) != "")
	request.QRChannel = normalizeCloudQRChannel(strings.TrimSpace(request.QRChannel))
	request.MountPath = strings.TrimSpace(request.MountPath)
	request.Token = strings.TrimSpace(request.Token)
	request.Notes = strings.TrimSpace(request.Notes)
	if request.Name == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("网盘名称不能为空")
	}
	if request.Vendor == "" {
		request.Vendor = "115"
	}
	if request.MountPath == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("挂载目录不能为空")
	}
	if request.AccessMethod == "QR" && request.QRSession == nil {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("请先获取扫码二维码")
	}
	if request.AccessMethod == "TOKEN" && request.Token == "" {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest("Token 不能为空")
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}
	defer tx.Rollback(ctx)

	accessMode := dbCloudAccessMode(request.AccessMethod)
	tokenStatus := "PENDING_SCAN"
	ciphertext := ""
	authStatus := "UNKNOWN"
	healthStatus := "UNKNOWN"
	plainCredential := ""
	if request.AccessMethod == "QR" {
		cookie, err := s.consumeQRCodeSession(ctx, *request.QRSession)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		plainCredential = cookie
		ciphertext, err = s.cipher.Encrypt(cookie)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		tokenStatus = "CONFIGURED"
		authStatus = "AUTHORIZED"
		healthStatus = "ONLINE"
	} else {
		probe, err := s.probeCloudCredential(ctx, request.Token)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		if !probe.Authenticated {
			return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(probe.Message)
		}
		plainCredential = request.Token
		ciphertext, err = s.cipher.Encrypt(request.Token)
		if err != nil {
			return storagedto.SaveCloudNodeResponse{}, err
		}
		tokenStatus = "CONFIGURED"
		authStatus = "AUTHORIZED"
		healthStatus = "ONLINE"
	}
	if ensureErr := s.ensureCloudMountDirectory(ctx, plainCredential, request.MountPath); ensureErr != nil {
		return storagedto.SaveCloudNodeResponse{}, apperrors.BadRequest(ensureErr.Error())
	}

	nodeID := request.ID
	if nodeID == "" {
		nodeID = buildCode("cloud-node-id", now)
		_, err = tx.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, description, created_at, updated_at
			) VALUES ($1, $2, $3, 'CLOUD', $4, $5, $6, $7, true, $8, $9, $9)
		`, nodeID, buildCode("cloud-node", now), request.Name, request.Vendor, request.MountPath, accessMode, request.Name, request.Notes, now)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE storage_nodes
			SET name = $2,
			    vendor = $3,
			    address = $4,
			    access_mode = $5,
			    account_alias = $6,
			    description = $7,
			    updated_at = $8
			WHERE id = $1
		`, nodeID, request.Name, request.Vendor, request.MountPath, accessMode, request.Name, request.Notes, now)
	}
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO storage_node_credentials (
			id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
		) VALUES ($1, $2, 'TOKEN', NULLIF($3, ''), NULLIF($4, ''), $5, $6, $6)
		ON CONFLICT (storage_node_id) DO UPDATE SET
			secret_ciphertext = EXCLUDED.secret_ciphertext,
			secret_ref = EXCLUDED.secret_ref,
			token_status = EXCLUDED.token_status,
			updated_at = EXCLUDED.updated_at
	`, buildCode("cloud-node-credential", now), nodeID, ciphertext, request.QRChannel, tokenStatus, now)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO storage_node_runtime (
			id, storage_node_id, health_status, auth_status, last_check_at, last_success_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $5, $5, $5)
		ON CONFLICT (storage_node_id) DO UPDATE SET
			health_status = EXCLUDED.health_status,
			auth_status = EXCLUDED.auth_status,
			last_check_at = EXCLUDED.last_check_at,
			last_success_at = EXCLUDED.last_success_at,
			last_error_code = NULL,
			last_error_message = NULL,
			updated_at = EXCLUDED.updated_at
	`, buildCode("cloud-node-runtime", now), nodeID, healthStatus, authStatus, now)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO cloud_node_profiles (
			id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
		) VALUES ($1, $2, $3, $4, $5, '{}'::jsonb, $6, $6, $6)
		ON CONFLICT (storage_node_id) DO UPDATE SET
			provider_vendor = EXCLUDED.provider_vendor,
			auth_method = EXCLUDED.auth_method,
			remote_root_path = EXCLUDED.remote_root_path,
			last_auth_at = EXCLUDED.last_auth_at,
			updated_at = EXCLUDED.updated_at
	`, buildCode("cloud-node-profile", now), nodeID, request.Vendor, request.AccessMethod, request.MountPath, now)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}
	record, err := s.loadCloudNodeByID(ctx, nodeID)
	if err != nil {
		return storagedto.SaveCloudNodeResponse{}, err
	}
	return storagedto.SaveCloudNodeResponse{Message: "网盘已保存", Record: record}, nil
}

func (s *CloudNodeService) runCloudNodeConnectionTestLegacy(ctx context.Context, ids []string) (storagedto.RunCloudNodeConnectionTestResponse, error) {
	if len(ids) == 0 {
		return storagedto.RunCloudNodeConnectionTestResponse{}, apperrors.BadRequest("ids 不能为空")
	}
	now := s.now().UTC()
	results := make([]storagedto.ConnectionTestResult, 0, len(ids))
	for _, id := range ids {
		record, err := s.loadCloudNodeByID(ctx, id)
		if err != nil {
			return storagedto.RunCloudNodeConnectionTestResponse{}, err
		}
		cookie, err := s.loadCloudCredential(ctx, id)
		overallTone := "success"
		summary := "已通过 115 登录态校验，当前 cookie 可用"
		authStatus := "AUTHORIZED"
		healthStatus := "ONLINE"
		lastErrorCode := ""
		lastErrorMessage := ""
		checks := []storagedto.ConnectionCheck{
			{Label: "网盘类型", Status: "success", Detail: record.Vendor},
			{Label: "挂载根目录", Status: "success", Detail: record.MountPath},
		}
		if err != nil {
			overallTone = "critical"
			summary = "未能通过 115 登录态校验"
			authStatus = "FAILED"
			healthStatus = "ERROR"
			lastErrorCode = "credential_load_failed"
			lastErrorMessage = err.Error()
			checks = append(checks, storagedto.ConnectionCheck{Label: "凭据加载", Status: "critical", Detail: err.Error()})
		} else {
			checks = append(checks, storagedto.ConnectionCheck{Label: "凭据加载", Status: "success", Detail: "已读取已保存凭据"})
			probe, probeErr := s.probeCloudCredential(ctx, cookie)
			if probeErr != nil || !probe.Authenticated {
				overallTone = "critical"
				summary = "115 接口校验失败"
				authStatus = "FAILED"
				healthStatus = "ERROR"
				lastErrorCode = "credential_probe_failed"
				if probeErr != nil {
					lastErrorMessage = probeErr.Error()
					checks = append(checks, storagedto.ConnectionCheck{Label: "115 登录态校验", Status: "critical", Detail: probeErr.Error()})
				} else {
					lastErrorMessage = probe.Message
					checks = append(checks, storagedto.ConnectionCheck{Label: "115 登录态校验", Status: "critical", Detail: probe.Message})
				}
			} else {
				checks = append(checks, storagedto.ConnectionCheck{Label: "115 登录态校验", Status: "success", Detail: probe.Message})
			}
		}
		_, _ = s.pool.Exec(ctx, `
			UPDATE storage_node_runtime
			SET health_status = $2,
			    auth_status = $3,
			    last_check_at = $4,
			    last_success_at = CASE WHEN $5 THEN $4 ELSE last_success_at END,
			    last_error_code = NULLIF($6, ''),
			    last_error_message = NULLIF($7, ''),
			    updated_at = $4
			WHERE storage_node_id = $1
		`, id, healthStatus, authStatus, now, overallTone == "success", lastErrorCode, lastErrorMessage)
		results = append(results, storagedto.ConnectionTestResult{
			ID:          id,
			Name:        record.Name,
			OverallTone: overallTone,
			Summary:     summary,
			Checks:      checks,
			Suggestion:  uiCloudSuggestion(overallTone, record.AccessMethod),
			TestedAt:    "刚刚",
		})
	}
	return storagedto.RunCloudNodeConnectionTestResponse{Message: "连接测试已完成", Results: results}, nil
}

func (s *CloudNodeService) createCloudQRCodeSessionLegacy(ctx context.Context, channel string) (storagedto.CloudQRCodeSession, error) {
	channel = normalizeCloudQRChannel(channel)
	if !isSupportedCloudQRChannel(channel) {
		return storagedto.CloudQRCodeSession{}, apperrors.BadRequest("扫码登录类型无效")
	}
	codeVerifier, codeChallenge, err := generate115OpenCodeVerifier()
	if err != nil {
		return storagedto.CloudQRCodeSession{}, err
	}
	form := url.Values{}
	form.Set("client_id", default115OpenAppID)
	form.Set("code_challenge", codeChallenge)
	form.Set("code_challenge_method", "sha256")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://passportapi.115.com/open/authDeviceCode", strings.NewReader(form.Encode()))
	if err != nil {
		return storagedto.CloudQRCodeSession{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")
	response, err := s.client.Do(request)
	if err != nil {
		return storagedto.CloudQRCodeSession{}, err
	}
	defer response.Body.Close()
	var payload struct {
		Data struct {
			UID    string `json:"uid"`
			Time   int64  `json:"time"`
			Sign   string `json:"sign"`
			QRCode string `json:"qrcode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return storagedto.CloudQRCodeSession{}, err
	}
	return storagedto.CloudQRCodeSession{
		UID:     payload.Data.UID,
		Time:    payload.Data.Time,
		Sign:    payload.Data.Sign,
		QRCode:  payload.Data.QRCode,
		Channel: channel,
		CodeVerifier: codeVerifier,
	}, nil
}

func (s *CloudNodeService) getCloudQRCodeStatusLegacy(ctx context.Context, session storagedto.CloudQRCodeSession) (storagedto.CloudQRCodeStatusResponse, error) {
	values := url.Values{}
	values.Set("uid", session.UID)
	values.Set("time", fmt.Sprintf("%d", session.Time))
	values.Set("sign", session.Sign)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://qrcodeapi.115.com/get/status/?"+values.Encode(), nil)
	if err != nil {
		return storagedto.CloudQRCodeStatusResponse{}, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return storagedto.CloudQRCodeStatusResponse{}, err
	}
	defer response.Body.Close()
	var payload struct {
		Data struct {
			Status int `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return storagedto.CloudQRCodeStatusResponse{}, err
	}
	return storagedto.CloudQRCodeStatusResponse{Status: mapCloudQRStatus(payload.Data.Status), Message: mapCloudQRMessage(payload.Data.Status)}, nil
}

func (s *CloudNodeService) fetchCloudQRCodeImageLegacy(ctx context.Context, session storagedto.CloudQRCodeSession) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizeCloudQRCodeURL(session.QRCode), nil)
	if err != nil {
		return nil, "", err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}
	contentType := response.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	return body, contentType, nil
}

func (s *CloudNodeService) consumeQRCodeSession(ctx context.Context, session storagedto.CloudQRCodeSession) (string, error) {
	appName := mapCloudQRApp(session.Channel)
	form := url.Values{}
	form.Set("account", session.UID)
	form.Set("app", appName)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://passportapi.115.com/app/1.0/"+appName+"/1.0/login/qrcode/", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")

	response, err := s.client.Do(request)
	if err != nil {
		if isCloudQRCodeTimeout(err) {
			return "", apperrors.BadRequest("閹殿偆鐖滈惂璇茬秿缂佹挻鐏夐懢宄板絿鐡掑懏妞傞敍宀冾嚞缁嬪秴鎮楅柌宥堢槸")
		}
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return "", apperrors.BadRequest("扫码登录结果换取失败")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var payload struct {
		State   int    `json:"state"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Data    struct {
			Cookie map[string]string `json:"cookie"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Data.Cookie) == 0 {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Error)
		}
		if message == "" {
			message = "鎵爜鐧诲綍灏氭湭瀹屾垚纭"
		}
		return "", apperrors.BadRequest(message)
	}

	cookie := joinCloudCookies(payload.Data.Cookie)
	probe, err := s.probeCloudCredential(ctx, cookie)
	if err != nil {
		return "", err
	}
	if !probe.Authenticated {
		return "", apperrors.BadRequest(probe.Message)
	}

	return cookie, nil
}

func (s *CloudNodeService) loadCloudCredential(ctx context.Context, id string) (string, error) {
	ciphertext, _, _, err := s.loadCloudCredentialEnvelope(ctx, id)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ciphertext) == "" {
		return "", fmt.Errorf("鏈壘鍒板凡淇濆瓨鍑嵁")
	}

	plaintext, err := s.cipher.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", fmt.Errorf("已保存凭据为空")
	}
	return plaintext, nil
}

func (s *CloudNodeService) loadCloudCredentialEnvelope(ctx context.Context, id string) (string, string, string, error) {
	var (
		ciphertext string
		secretRef string
		tokenStatus string
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(secret_ciphertext, ''), COALESCE(secret_ref, ''), COALESCE(token_status, 'UNKNOWN')
		FROM storage_node_credentials
		WHERE storage_node_id = $1
	`, id).Scan(&ciphertext, &secretRef, &tokenStatus); err != nil {
		if err == pgx.ErrNoRows {
			return "", "", "", apperrors.NotFound("网盘凭据不存在")
		}
		return "", "", "", err
	}
	return ciphertext, secretRef, tokenStatus, nil
}

func isStorageNotFound(err error) bool {
	appErr, ok := err.(*apperrors.AppError)
	return ok && appErr.Code == "not_found"
}

func (s *CloudNodeService) probeCloudCredential(ctx context.Context, cookie string) (cloudCredentialProbeResult, error) {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return cloudCredentialProbeResult{Message: "缺少可用凭据"}, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://qrcodeapi.115.com/app/1.0/web/1.0/login_log/login_devices", nil)
	if err != nil {
		return cloudCredentialProbeResult{}, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")
	request.Header.Set("Cookie", cookie)

	response, err := s.client.Do(request)
	if err != nil {
		if isCloudQRCodeTimeout(err) {
			return cloudCredentialProbeResult{}, fmt.Errorf("115 登录态校验超时，请稍后重试")
		}
		return cloudCredentialProbeResult{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return cloudCredentialProbeResult{}, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return cloudCredentialProbeResult{}, err
	}

	if code := numericJSONValue(payload["code"]); code != 0 {
		return cloudCredentialProbeResult{Message: fallbackCloudProbeMessage(payload)}, nil
	}
	if errno := numericJSONValue(payload["errno"]); errno != 0 {
		return cloudCredentialProbeResult{Message: fallbackCloudProbeMessage(payload)}, nil
	}
	if state, ok := payload["state"]; ok && !isTruthyJSONValue(state) {
		return cloudCredentialProbeResult{Message: fallbackCloudProbeMessage(payload)}, nil
	}

	return cloudCredentialProbeResult{Authenticated: true, Message: "115 接口返回已登录状态"}, nil
}

func joinCloudCookies(cookieMap map[string]string) string {
	keys := make([]string, 0, len(cookieMap))
	for key := range cookieMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+cookieMap[key])
	}
	return strings.Join(parts, "; ")
}

func (s *CloudNodeService) ensureCloudMountDirectory(ctx context.Context, cookie string, mountPath string) error {
	segments := splitCloudMountPath(mountPath)
	if len(segments) == 0 {
		return nil
	}

	parentID := "0"
	for _, segment := range segments {
		children, err := s.listCloudDirectories(ctx, cookie, parentID)
		if err != nil {
			return err
		}
		if nextID, ok := children[segment]; ok {
			parentID = nextID
			continue
		}

		nextID, err := s.createCloudDirectory(ctx, cookie, parentID, segment)
		if err != nil {
			return err
		}
		parentID = nextID
	}

	return nil
}

func (s *CloudNodeService) resolveCloudMountDirectory(ctx context.Context, cookie string, mountPath string) (string, error) {
	segments := splitCloudMountPath(mountPath)
	if len(segments) == 0 {
		return "0", nil
	}

	parentID := "0"
	for _, segment := range segments {
		children, err := s.listCloudDirectories(ctx, cookie, parentID)
		if err != nil {
			return "", err
		}
		nextID, ok := children[segment]
		if !ok {
			return "", fmt.Errorf("网盘目录不存在：%s", segment)
		}
		parentID = nextID
	}

	return parentID, nil
}

func splitCloudMountPath(mountPath string) []string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(mountPath, "\\", "/"))
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "" {
		return nil
	}

	rawParts := strings.Split(cleaned, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func (s *CloudNodeService) listCloudDirectories(ctx context.Context, cookie string, parentID string) (map[string]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://webapi.115.com/files", nil)
	if err != nil {
		return nil, err
	}

	query := request.URL.Query()
	query.Set("aid", "1")
	query.Set("cid", parentID)
	query.Set("limit", "10000")
	query.Set("show_dir", "1")
	request.URL.RawQuery = query.Encode()
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")
	request.Header.Set("Cookie", cookie)

	response, err := s.client.Do(request)
	if err != nil {
		if isCloudQRCodeTimeout(err) {
			return nil, fmt.Errorf("网盘目录读取超时，请稍后重试")
		}
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if code := numericJSONValue(payload["code"]); code != 0 {
		return nil, fmt.Errorf("%s", fallbackCloudProbeMessage(payload))
	}
	if errno := numericJSONValue(payload["errno"]); errno != 0 {
		return nil, fmt.Errorf("%s", fallbackCloudProbeMessage(payload))
	}
	if state, ok := payload["state"]; ok && !isTruthyJSONValue(state) {
		return nil, fmt.Errorf("%s", fallbackCloudProbeMessage(payload))
	}

	entries := make(map[string]string)
	items, _ := payload["data"].([]any)
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := firstNonEmptyString(record["n"], record["name"])
		cid := stringJSONValue(record["cid"], record["file_id"], record["id"])
		if name != "" && cid != "" {
			entries[name] = cid
		}
	}
	return entries, nil
}

func (s *CloudNodeService) createCloudDirectory(ctx context.Context, cookie string, parentID string, name string) (string, error) {
	form := url.Values{}
	form.Set("aid", "1")
	form.Set("pid", parentID)
	form.Set("cname", name)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://webapi.115.com/files/add", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")
	request.Header.Set("Cookie", cookie)

	response, err := s.client.Do(request)
	if err != nil {
		if isCloudQRCodeTimeout(err) {
			return "", fmt.Errorf("网盘目录创建超时，请稍后重试")
		}
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if code := numericJSONValue(payload["code"]); code != 0 {
		return "", fmt.Errorf("%s", fallbackCloudProbeMessage(payload))
	}
	if errno := numericJSONValue(payload["errno"]); errno != 0 {
		return "", fmt.Errorf("%s", fallbackCloudProbeMessage(payload))
	}
	if state, ok := payload["state"]; ok && !isTruthyJSONValue(state) {
		return "", fmt.Errorf("%s", fallbackCloudProbeMessage(payload))
	}

	cid := stringJSONValue(payload["cid"])
	if cid == "" {
		if data, ok := payload["data"].(map[string]any); ok {
			cid = stringJSONValue(data["cid"], data["file_id"], data["id"])
		}
	}
	if cid == "" {
		return "", fmt.Errorf("网盘目录创建成功但未返回目录 ID")
	}
	return cid, nil
}

func fallbackCloudProbeMessage(payload map[string]any) string {
	if message := firstNonEmptyString(payload["message"], payload["error"]); message != "" {
		return message
	}
	return "115 登录态校验失败"
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func numericJSONValue(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	default:
		return 0
	}
}

func stringJSONValue(values ...any) string {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text != "" {
				return text
			}
		case float64:
			return fmt.Sprintf("%.0f", typed)
		case int:
			return fmt.Sprintf("%d", typed)
		case int64:
			return fmt.Sprintf("%d", typed)
		}
	}
	return ""
}

func isTruthyJSONValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case string:
		return typed != "" && typed != "0" && !strings.EqualFold(typed, "false")
	default:
		return false
	}
}

func isSupportedCloudQRChannel(value string) bool {
	return normalizeCloudQRChannel(value) != ""
}

func mapCloudQRStatus(status int) string {
	switch status {
	case 0:
		return "WAITING"
	case 1:
		return "SCANNED"
	case 2:
		return "CONFIRMED"
	case -1:
		return "EXPIRED"
	case -2:
		return "CANCELED"
	default:
		return "UNKNOWN"
	}
}

func mapCloudQRMessage(status int) string {
	switch status {
	case 0:
		return "等待扫码"
	case 1:
		return "已扫码，请在 115 中确认登录"
	case 2:
		return "扫码登录已确认"
	case -1:
		return "娴滃瞼娣惍浣稿嚒鏉╁洦婀￠敍宀冾嚞闁插秵鏌婇悽鐔稿灇"
	case -2:
		return "扫码登录已取消"
	default:
		return "二维码状态未知"
	}
}

func mapCloudQRApp(channel string) string {
	switch normalizeCloudQRChannel(channel) {
	case "wechatmini":
		return "wechatmini"
	case "alipaymini":
		return "alipaymini"
	default:
		return "tv"
	}
}

func isCloudQRCodeTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func normalizeCloudQRCodeURL(value string) string {
	raw := strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(raw, "http://"), strings.HasPrefix(raw, "https://"):
		return raw
	case strings.HasPrefix(raw, "//"):
		return "https:" + raw
	case strings.HasPrefix(raw, "/"):
		return "https://qrcodeapi.115.com" + raw
	default:
		return "https://qrcodeapi.115.com/" + strings.TrimPrefix(raw, "./")
	}
}

func isCloudOAuthToken(value string) bool {
	token := strings.TrimSpace(value)
	return token != "" && !strings.Contains(token, "=") && !strings.Contains(token, ";")
}

func exchangeQRCodeSessionToOpenToken(ctx context.Context, client *http.Client, session storagedto.CloudQRCodeSession) (integration.OpenOAuthToken, error) {
	if strings.TrimSpace(session.UID) == "" {
		return integration.OpenOAuthToken{}, fmt.Errorf("二维码会话无效")
	}
	codeVerifier := strings.TrimSpace(session.CodeVerifier)
	if codeVerifier == "" {
		return integration.OpenOAuthToken{}, fmt.Errorf("二维码会话缺少 code verifier")
	}

	form := url.Values{}
	form.Set("uid", session.UID)
	form.Set("code_verifier", codeVerifier)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://passportapi.115.com/open/deviceCodeToToken", strings.NewReader(form.Encode()))
	if err != nil {
		return integration.OpenOAuthToken{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")

	response, err := client.Do(request)
	if err != nil {
		if isCloudQRCodeTimeout(err) {
			return integration.OpenOAuthToken{}, fmt.Errorf("115open 换取 Token 超时，请稍后重试")
		}
		return integration.OpenOAuthToken{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return integration.OpenOAuthToken{}, err
	}

	return parse115OpenTokenResponse(body)
}

func exchange115OpenRefreshToken(ctx context.Context, client *http.Client, refreshToken string) (integration.OpenOAuthToken, error) {
	form := url.Values{}
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://passportapi.115.com/open/refreshToken", strings.NewReader(form.Encode()))
	if err != nil {
		return integration.OpenOAuthToken{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "application/json,text/plain,*/*")

	response, err := client.Do(request)
	if err != nil {
		if isCloudQRCodeTimeout(err) {
			return integration.OpenOAuthToken{}, fmt.Errorf("115open 刷新 Token 超时，请稍后重试")
		}
		return integration.OpenOAuthToken{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return integration.OpenOAuthToken{}, err
	}

	return parse115OpenTokenResponse(body)
}

func parse115OpenTokenResponse(body []byte) (integration.OpenOAuthToken, error) {
	var payload struct {
		State   any    `json:"state"`
		Code    any    `json:"code"`
		ErrNo   any    `json:"errno"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    uint64 `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return integration.OpenOAuthToken{}, err
	}
	if code := numericJSONValue(payload.Code); code != 0 {
		return integration.OpenOAuthToken{}, fmt.Errorf("%s", fallbackCloudProbeMessage(map[string]any{
			"message": payload.Message,
			"error":   payload.Error,
			"code":    payload.Code,
			"errno":   payload.ErrNo,
		}))
	}
	if errno := numericJSONValue(payload.ErrNo); errno != 0 {
		return integration.OpenOAuthToken{}, fmt.Errorf("%s", fallbackCloudProbeMessage(map[string]any{
			"message": payload.Message,
			"error":   payload.Error,
			"code":    payload.Code,
			"errno":   payload.ErrNo,
		}))
	}
	if payload.Data.RefreshToken == "" || payload.Data.AccessToken == "" {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Error)
		}
		if message == "" {
			message = "115open 未返回有效 Token"
		}
		return integration.OpenOAuthToken{}, fmt.Errorf("%s", message)
	}
	return integration.OpenOAuthToken{
		RefreshToken: payload.Data.RefreshToken,
		AccessToken:  payload.Data.AccessToken,
		ExpiresIn:    payload.Data.ExpiresIn,
	}, nil
}

func generate115OpenCodeVerifier() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier := base64.StdEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.StdEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}
