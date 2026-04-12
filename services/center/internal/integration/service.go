package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	integrationdto "mare/shared/contracts/dto/integration"
)

type Service struct {
	pool        *pgxpool.Pool
	now         func() time.Time
	cipher      credentialCipher
	providers   map[string]CloudProviderDriver
	downloaders map[string]DownloadEngine
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:        pool,
		now:         time.Now,
		cipher:      newSystemCredentialCipher(),
		providers:   make(map[string]CloudProviderDriver),
		downloaders: make(map[string]DownloadEngine),
	}
}

func (s *Service) RegisterProvider(driver CloudProviderDriver) {
	s.providers[strings.ToUpper(strings.TrimSpace(driver.Vendor()))] = driver
}

func (s *Service) RegisterDownloader(engine DownloadEngine) {
	s.downloaders[strings.ToUpper(strings.TrimSpace(engine.Name()))] = engine
}

func (s *Service) Start(ctx context.Context) {
	for _, engine := range s.downloaders {
		engine.Start(ctx)
	}
}

func (s *Service) Provider(vendor string) (CloudProviderDriver, error) {
	driver, ok := s.providers[strings.ToUpper(strings.TrimSpace(vendor))]
	if !ok {
		return nil, apperrors.BadRequest("当前云厂商暂未接入")
	}
	return driver, nil
}

func (s *Service) Downloader(name string) (DownloadEngine, error) {
	engine, ok := s.downloaders[strings.ToUpper(strings.TrimSpace(name))]
	if !ok {
		return nil, apperrors.BadRequest("当前下载引擎暂未接入")
	}
	return engine, nil
}

func (s *Service) ListGateways(ctx context.Context) (integrationdto.GatewayListResponse, error) {
	row, err := s.loadCD2Gateway(ctx)
	if err != nil {
		if isNotFound(err) {
			return integrationdto.GatewayListResponse{Items: []integrationdto.GatewayRecord{}}, nil
		}
		return integrationdto.GatewayListResponse{}, err
	}
	return integrationdto.GatewayListResponse{Items: []integrationdto.GatewayRecord{mapGatewayRecord(row)}}, nil
}

func (s *Service) SaveCD2Gateway(ctx context.Context, request integrationdto.SaveCD2GatewayRequest) (integrationdto.SaveCD2GatewayResponse, error) {
	request.BaseURL = strings.TrimSpace(request.BaseURL)
	request.Username = strings.TrimSpace(request.Username)
	request.Password = strings.TrimSpace(request.Password)
	if request.BaseURL == "" {
		return integrationdto.SaveCD2GatewayResponse{}, apperrors.BadRequest("CD2 地址不能为空")
	}
	if request.Username == "" {
		return integrationdto.SaveCD2GatewayResponse{}, apperrors.BadRequest("CD2 账号不能为空")
	}

	existing, err := s.loadCD2Gateway(ctx)
	if err != nil && !isNotFound(err) {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}

	password := request.Password
	if password == "" && existing.Password != "" {
		password = existing.Password
	}
	if password == "" {
		return integrationdto.SaveCD2GatewayResponse{}, apperrors.BadRequest("CD2 密码不能为空")
	}

	ciphertext, err := s.cipher.Encrypt(password)
	if err != nil {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}
	defer tx.Rollback(ctx)

	gatewayID := "integration-gateway-cd2"
	deviceID := existing.ClientDeviceID
	if strings.TrimSpace(deviceID) == "" {
		deviceID = buildIntegrationCode("cd2-device")
	}
	gatewayPayload := mustMarshalJSONB(map[string]any{
		"clientDeviceId": deviceID,
	})
	_, err = tx.Exec(ctx, `
		INSERT INTO integration_gateways (
			id, gateway_type, display_name, base_url, enabled, runtime_status, gateway_payload, updated_at, created_at
		) VALUES (
			$1, 'CD2', 'CloudDrive2', $2, $3, $4, $5::jsonb, $6, $6
		)
		ON CONFLICT (gateway_type) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			base_url = EXCLUDED.base_url,
			enabled = EXCLUDED.enabled,
			gateway_payload = jsonb_set(
				COALESCE(integration_gateways.gateway_payload, '{}'::jsonb),
				'{clientDeviceId}',
				COALESCE(EXCLUDED.gateway_payload->'clientDeviceId', integration_gateways.gateway_payload->'clientDeviceId', to_jsonb(''::text)),
				true
			),
			runtime_status = CASE WHEN EXCLUDED.enabled THEN integration_gateways.runtime_status ELSE 'DISABLED' END,
			updated_at = EXCLUDED.updated_at
	`, gatewayID, request.BaseURL, request.Enabled, defaultGatewayRuntime(request.Enabled), gatewayPayload, now)
	if err != nil {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO integration_gateway_credentials (
			id, gateway_id, username, password_ciphertext, updated_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (gateway_id) DO UPDATE SET
			username = EXCLUDED.username,
			password_ciphertext = EXCLUDED.password_ciphertext,
			updated_at = EXCLUDED.updated_at
	`, "integration-gateway-credential-cd2", gatewayID, request.Username, ciphertext, now)
	if err != nil {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}

	record, err := s.loadCD2Gateway(ctx)
	if err != nil {
		return integrationdto.SaveCD2GatewayResponse{}, err
	}
	return integrationdto.SaveCD2GatewayResponse{
		Message: "CloudDrive2 配置已保存",
		Record:  mapGatewayRecord(record),
	}, nil
}

func (s *Service) TestCD2Gateway(ctx context.Context, request integrationdto.TestCD2GatewayRequest) (integrationdto.TestCD2GatewayResponse, error) {
	config, err := s.resolveGatewayConfigForTest(ctx, request)
	if err != nil {
		return integrationdto.TestCD2GatewayResponse{}, err
	}
	driver, err := s.Provider("115")
	if err != nil {
		return integrationdto.TestCD2GatewayResponse{}, err
	}
	tester, ok := driver.(interface {
		TestGateway(ctx context.Context, config CD2GatewayConfig) error
	})
	if !ok {
		return integrationdto.TestCD2GatewayResponse{}, apperrors.Internal("CD2 驱动不支持连接测试")
	}
	testErr := tester.TestGateway(ctx, config)
	now := s.now().UTC()
	newStatus := gatewayRuntimeStatus(testErr, config.Enabled)
	_, updateErr := s.pool.Exec(ctx, `
		UPDATE integration_gateways
		SET runtime_status = $1,
		    last_test_at = $2,
		    last_error_code = $3,
		    last_error_message = $4,
		    updated_at = $2
		WHERE gateway_type = 'CD2'
	`, newStatus, now, gatewayErrorCode(testErr), gatewayErrorMessage(testErr))
	if updateErr != nil {
		return integrationdto.TestCD2GatewayResponse{}, updateErr
	}
	record, loadErr := s.loadCD2Gateway(ctx)
	if loadErr != nil {
		record = config
		record.LastTestAt = &now
		record.RuntimeStatus = newStatus
		record.LastErrorCode = gatewayErrorCode(testErr)
		record.LastErrorMessage = gatewayErrorMessage(testErr)
	} else {
		record.RuntimeStatus = newStatus
		record.LastTestAt = &now
		record.LastErrorCode = gatewayErrorCode(testErr)
		record.LastErrorMessage = gatewayErrorMessage(testErr)
	}
	if testErr != nil {
		return integrationdto.TestCD2GatewayResponse{}, apperrors.BadRequest(testErr.Error())
	}
	return integrationdto.TestCD2GatewayResponse{
		Message: "CloudDrive2 连接测试通过",
		Record:  mapGatewayRecord(record),
	}, nil
}

func (s *Service) RuntimeStatus(ctx context.Context) (integrationdto.RuntimeStatusResponse, error) {
	components := make([]integrationdto.RuntimeComponentRecord, 0, 2)
	if gateway, err := s.loadCD2Gateway(ctx); err == nil {
		components = append(components, integrationdto.RuntimeComponentRecord{
			Name:             "CloudDrive2",
			Status:           gateway.RuntimeStatus,
			Message:          gatewayRuntimeMessage(gateway),
			LastCheckedAt:    formatOptionalRFC3339(gateway.LastTestAt),
			LastErrorCode:    gateway.LastErrorCode,
			LastErrorMessage: gateway.LastErrorMessage,
		})
	}
	if engine, ok := s.downloaders["ARIA2"]; ok {
		status := engine.RuntimeStatus()
		components = append(components, integrationdto.RuntimeComponentRecord{
			Name:             status.Name,
			Status:           status.Status,
			Message:          status.Message,
			LastCheckedAt:    formatOptionalRFC3339(status.LastCheckedAt),
			LastErrorCode:    status.LastErrorCode,
			LastErrorMessage: status.LastErrorMessage,
		})
	}
	return integrationdto.RuntimeStatusResponse{Components: components}, nil
}

func (s *Service) LoadCD2GatewayConfig(ctx context.Context) (CD2GatewayConfig, error) {
	return s.loadCD2Gateway(ctx)
}

func (s *Service) loadCD2Gateway(ctx context.Context) (CD2GatewayConfig, error) {
	var (
		row CD2GatewayConfig
		passwordCiphertext string
		gatewayPayload []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT
			g.id,
			g.base_url,
			COALESCE(c.username, ''),
			COALESCE(c.password_ciphertext, ''),
			COALESCE(g.gateway_payload, '{}'::jsonb),
			g.enabled,
			g.runtime_status,
			g.last_test_at,
			COALESCE(g.last_error_code, ''),
			COALESCE(g.last_error_message, '')
		FROM integration_gateways g
		LEFT JOIN integration_gateway_credentials c ON c.gateway_id = g.id
		WHERE g.gateway_type = 'CD2'
	`).Scan(
		&row.ID,
		&row.BaseURL,
		&row.Username,
		&passwordCiphertext,
		&gatewayPayload,
		&row.Enabled,
		&row.RuntimeStatus,
		&row.LastTestAt,
		&row.LastErrorCode,
		&row.LastErrorMessage,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return CD2GatewayConfig{}, apperrors.NotFound("CloudDrive2 配置不存在")
		}
		return CD2GatewayConfig{}, err
	}
	password, err := s.cipher.Decrypt(passwordCiphertext)
	if err != nil {
		return CD2GatewayConfig{}, err
	}
	row.Password = password
	var payload struct {
		ClientDeviceID string `json:"clientDeviceId"`
	}
	if err := json.Unmarshal(gatewayPayload, &payload); err == nil {
		row.ClientDeviceID = strings.TrimSpace(payload.ClientDeviceID)
	}
	return row, nil
}

func (s *Service) resolveGatewayConfigForTest(ctx context.Context, request integrationdto.TestCD2GatewayRequest) (CD2GatewayConfig, error) {
	existing, err := s.loadCD2Gateway(ctx)
	if err != nil && !isNotFound(err) {
		return CD2GatewayConfig{}, err
	}
	config := existing
	if strings.TrimSpace(request.BaseURL) != "" {
		config.BaseURL = strings.TrimSpace(request.BaseURL)
	}
	if strings.TrimSpace(request.Username) != "" {
		config.Username = strings.TrimSpace(request.Username)
	}
	if strings.TrimSpace(request.Password) != "" {
		config.Password = strings.TrimSpace(request.Password)
	}
	if request.Enabled != nil {
		config.Enabled = *request.Enabled
	}
	if config.BaseURL == "" || config.Username == "" || config.Password == "" {
		return CD2GatewayConfig{}, apperrors.BadRequest("CloudDrive2 配置不完整")
	}
	return config, nil
}

func defaultGatewayRuntime(enabled bool) string {
	if !enabled {
		return "DISABLED"
	}
	return "UNKNOWN"
}

func gatewayRuntimeStatus(err error, enabled bool) string {
	if !enabled {
		return "DISABLED"
	}
	if err != nil {
		return "ERROR"
	}
	return "ONLINE"
}

func gatewayErrorCode(err error) string {
	if err == nil {
		return ""
	}
	return "gateway_test_failed"
}

func gatewayErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func gatewayRuntimeMessage(config CD2GatewayConfig) string {
	switch config.RuntimeStatus {
	case "ONLINE":
		return "CloudDrive2 连接正常"
	case "DISABLED":
		return "CloudDrive2 已禁用"
	case "ERROR":
		if strings.TrimSpace(config.LastErrorMessage) != "" {
			return config.LastErrorMessage
		}
		return "CloudDrive2 连接异常"
	default:
		return "CloudDrive2 尚未检测"
	}
}

func mapGatewayRecord(config CD2GatewayConfig) integrationdto.GatewayRecord {
	return integrationdto.GatewayRecord{
		ID:               config.ID,
		GatewayType:      "CD2",
		DisplayName:      "CloudDrive2",
		BaseURL:          config.BaseURL,
		Enabled:          config.Enabled,
		RuntimeStatus:    config.RuntimeStatus,
		ClientDeviceID:   config.ClientDeviceID,
		LastTestAt:       formatOptionalRFC3339(config.LastTestAt),
		LastErrorCode:    config.LastErrorCode,
		LastErrorMessage: config.LastErrorMessage,
		HasPassword:      strings.TrimSpace(config.Password) != "",
		Username:         config.Username,
	}
}

func (s *Service) EnsureCD2ClientDeviceID(ctx context.Context) (string, error) {
	config, err := s.loadCD2Gateway(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(config.ClientDeviceID) != "" {
		return config.ClientDeviceID, nil
	}
	deviceID := buildIntegrationCode("cd2-device")
	now := s.now().UTC()
	payload := mustMarshalJSONB(map[string]any{"clientDeviceId": deviceID})
	_, err = s.pool.Exec(ctx, `
		UPDATE integration_gateways
		SET gateway_payload = $1::jsonb,
		    updated_at = $2
		WHERE gateway_type = 'CD2'
	`, payload, now)
	if err != nil {
		return "", err
	}
	return deviceID, nil
}

func mustMarshalJSONB(value map[string]any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func MarshalProviderPayload(payload CloudProviderPayload) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal provider payload: %w", err)
	}
	result := make(map[string]any)
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal provider payload: %w", err)
	}
	return result, nil
}

func UnmarshalProviderPayload(value []byte) (CloudProviderPayload, error) {
	if len(value) == 0 {
		return CloudProviderPayload{}, nil
	}
	var payload CloudProviderPayload
	if err := json.Unmarshal(value, &payload); err != nil {
		return CloudProviderPayload{}, fmt.Errorf("decode provider payload: %w", err)
	}
	return payload, nil
}

func formatOptionalRFC3339(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func isNotFound(err error) bool {
	if appErr, ok := err.(*apperrors.AppError); ok {
		return appErr.Code == "not_found"
	}
	return false
}
