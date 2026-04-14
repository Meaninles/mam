package storage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/assets"
	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/integration"
	storagedto "mare/shared/contracts/dto/storage"
)

type LocalFolderService struct {
	pool        *pgxpool.Pool
	now         func() time.Time
	nas         nasConnector
	cloud       *http.Client
	cipher      credentialCipher
	integration interface {
		Provider(vendor string) (integration.CloudProviderDriver, error)
	}
	assetService interface {
		SyncMount(ctx context.Context, mountID string) error
		SyncCloudMountFirstLevel(ctx context.Context, mountID string, entries []assets.CloudMountEntry) error
	}
}

func NewLocalFolderService(pool *pgxpool.Pool) *LocalFolderService {
	return &LocalFolderService{
		pool:         pool,
		now:          time.Now,
		nas:          newSMBConnector(5 * time.Second),
		cloud:        &http.Client{Timeout: 10 * time.Second},
		cipher:       newSystemCredentialCipher(),
		assetService: assets.NewService(pool),
	}
}

func (s *LocalFolderService) SetIntegrationService(service interface {
	Provider(vendor string) (integration.CloudProviderDriver, error)
}) {
	s.integration = service
}

func (s *LocalFolderService) SetAssetService(service interface {
	SyncMount(ctx context.Context, mountID string) error
	SyncCloudMountFirstLevel(ctx context.Context, mountID string, entries []assets.CloudMountEntry) error
}) {
	s.assetService = service
}

func (s *LocalFolderService) ListLocalFolders(ctx context.Context) ([]storagedto.LocalFolderRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			m.id,
			m.name,
			m.library_id,
			m.library_name,
			sn.id,
			sn.name,
			COALESCE(sn.address, ''),
			sn.node_type,
			sn.access_mode,
			m.relative_root_path,
			m.source_path,
			m.mount_mode,
			m.enabled,
			m.heartbeat_policy,
			mr.scan_status,
			mr.last_scan_at,
			mr.next_heartbeat_at,
			mr.capacity_bytes,
			mr.available_bytes,
			mr.auth_status,
			mr.health_status,
			COALESCE(mr.last_error_message, ''),
			COALESCE(sn.description, '')
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE sn.node_type IN ('LOCAL', 'NAS', 'CLOUD')
		  AND sn.deleted_at IS NULL
		  AND m.deleted_at IS NULL
		ORDER BY m.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]storagedto.LocalFolderRecord, 0)
	for rows.Next() {
		var (
			id               string
			name             string
			libraryID        string
			libraryName      string
			nodeID           string
			nodeName         string
			nodeAddress      string
			nodeType         string
			accessMode       string
			relativePath     string
			sourcePath       string
			mountMode        string
			enabled          bool
			heartbeatPolicy  string
			scanStatus       string
			lastScanAt       *time.Time
			nextHeartbeatAt  *time.Time
			capacityBytes    *int64
			availableBytes   *int64
			authStatus       string
			healthStatus     string
			lastErrorMessage string
			notes            string
		)

		if err := rows.Scan(
			&id,
			&name,
			&libraryID,
			&libraryName,
			&nodeID,
			&nodeName,
			&nodeAddress,
			&nodeType,
			&accessMode,
			&relativePath,
			&sourcePath,
			&mountMode,
			&enabled,
			&heartbeatPolicy,
			&scanStatus,
			&lastScanAt,
			&nextHeartbeatAt,
			&capacityBytes,
			&availableBytes,
			&authStatus,
			&healthStatus,
			&lastErrorMessage,
			&notes,
		); err != nil {
			return nil, err
		}

		if nodeType == "LOCAL" && (capacityBytes == nil || availableBytes == nil) && strings.TrimSpace(sourcePath) != "" {
			if total, free, usageErr := detectDiskUsage(sourcePath); usageErr == nil {
				capacityBytes = &total
				availableBytes = &free
				_, _ = s.pool.Exec(ctx, `
					UPDATE mount_runtime
					SET capacity_bytes = $2::bigint,
					    available_bytes = $3::bigint,
					    updated_at = $4
					WHERE mount_id = $1
				`, id, total, free, s.now().UTC())
			}
		}

		items = append(items, storagedto.LocalFolderRecord{
			ID:               id,
			Name:             name,
			LibraryID:        libraryID,
			LibraryName:      libraryName,
			NodeID:           nodeID,
			NodeName:         nodeName,
			NodeRootPath:     nodeAddress,
			RelativePath:     relativePath,
			FolderType:       uiFolderType(nodeType),
			Address:          sourcePath,
			MountMode:        uiMountMode(mountMode),
			Enabled:          enabled,
			ScanStatus:       uiScanStatus(scanStatus, lastScanAt),
			ScanTone:         uiScanTone(scanStatus),
			LastScanAt:       uiScanAt(lastScanAt),
			HeartbeatPolicy:  uiHeartbeatPolicy(heartbeatPolicy),
			NextHeartbeatAt:  uiNextHeartbeatLabel(heartbeatPolicy, nextHeartbeatAt),
			CapacitySummary:  uiCapacitySummary(capacityBytes, availableBytes),
			FreeSpaceSummary: uiFreeSpaceSummary(availableBytes),
			CapacityPercent:  uiCapacityPercent(capacityBytes, availableBytes),
			RiskTags:         uiRiskTags(scanStatus, healthStatus, lastErrorMessage),
			Badges:           buildMountBadges(nodeType, accessMode, mountMode),
			AuthStatus:       uiMountAuthStatus(nodeType, authStatus),
			AuthTone:         uiMountAuthTone(nodeType, authStatus),
			Notes:            notes,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *LocalFolderService) SaveLocalFolder(ctx context.Context, request storagedto.SaveLocalFolderRequest) (storagedto.SaveLocalFolderResponse, error) {
	if strings.TrimSpace(request.Name) == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("挂载名称不能为空")
	}
	if strings.TrimSpace(request.LibraryID) == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("所属资产库不能为空")
	}
	if strings.TrimSpace(request.LibraryName) == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("所属资产库名称不能为空")
	}
	if strings.TrimSpace(request.NodeID) == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("所属节点不能为空")
	}
	if strings.TrimSpace(request.RelativePath) == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("挂载子目录不能为空")
	}

	mountMode := dbMountMode(request.MountMode)
	if mountMode == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("挂载模式无效")
	}
	heartbeatPolicy := dbHeartbeatPolicy(request.HeartbeatPolicy)
	if heartbeatPolicy == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("心跳策略无效")
	}
	scanPolicy := dbScanPolicy(request.ScanPolicy)
	if scanPolicy == "" {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("扫描策略无效")
	}

	node, err := s.loadStorageNodeForMount(ctx, request.NodeID)
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}

	relativePath := strings.Trim(strings.TrimSpace(request.RelativePath), `/\`)
	sourcePath, mountSourceType, err := buildMountSourcePath(node, relativePath)
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}

	if node.NodeType == "LOCAL" {
		if mkdirErr := os.MkdirAll(sourcePath, 0o755); mkdirErr != nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("挂载目录创建失败")
		}
	} else if node.NodeType == "NAS" {
		password, decryptErr := s.cipher.Decrypt(node.SecretCiphertext)
		if decryptErr != nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("NAS 凭据无法读取，请重新保存账号和密码")
		}
		if ensureErr := s.nas.EnsureDirectory(ctx, sourcePath, node.Username, password); ensureErr != nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest(fmt.Sprintf("NAS 挂载目录创建失败：%s", ensureErr.Error()))
		}
	} else if node.NodeType == "CLOUD" {
		if s.integration == nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("云端集成服务尚未启用")
		}
		cloudService := NewCloudNodeService(s.pool, s.integration)
		profile, profileErr := cloudService.loadCloudProfile(ctx, request.NodeID)
		if profileErr != nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest(fmt.Sprintf("网盘挂载目录创建失败：%s", profileErr.Error()))
		}
		driver, driverErr := s.integration.Provider(profile.ProviderVendor)
		if driverErr != nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest(fmt.Sprintf("网盘挂载目录创建失败：%s", driverErr.Error()))
		}
		if ensureErr := driver.EnsureRemoteRoot(ctx, profile.Payload, sourcePath); ensureErr != nil {
			return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest(fmt.Sprintf("网盘挂载目录创建失败：%s", ensureErr.Error()))
		}
	}

	now := s.now().UTC()
	nextHeartbeat := computeNextHeartbeat(now, heartbeatPolicy)
	nextScanAt := computeNextScanAt(now, scanPolicy)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at)
		VALUES ($1, $2, $3, '/', 'ACTIVE', $4, $4)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
		    updated_at = EXCLUDED.updated_at
	`, request.LibraryID, "library-"+request.LibraryID, request.LibraryName, now)
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO library_directories (
			id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
		) VALUES ($1, $2, '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $3, $3)
		ON CONFLICT (library_id, relative_path) DO UPDATE
		SET updated_at = EXCLUDED.updated_at
	`, "dir-root-"+request.LibraryID, request.LibraryID, now)
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}

	if strings.TrimSpace(request.ID) == "" {
		mountID := buildCode("mount-id", now)
		_, err = tx.Exec(ctx, `
			INSERT INTO mounts (
				id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
				source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, true, 0, $13, $13)
		`,
			mountID,
			buildCode("mount", now),
			request.LibraryID,
			request.LibraryName,
			request.NodeID,
			request.Name,
			mountSourceType,
			mountMode,
			sourcePath,
			relativePath,
			heartbeatPolicy,
			scanPolicy,
			now,
		)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO mount_runtime (
				id, mount_id, scan_status, next_scan_at, next_heartbeat_at, auth_status, health_status, created_at, updated_at
			) VALUES ($1, $2, 'IDLE', $3, $4, $5, 'UNKNOWN', $6, $6)
		`, buildCode("mount-runtime-id", now), mountID, nextScanAt, nextHeartbeat, initialMountAuthStatus(node.NodeType), now)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}

		request.ID = mountID
	} else {
		tag, err := tx.Exec(ctx, `
			UPDATE mounts
			SET name = $2,
			    library_id = $3,
			    library_name = $4,
			    storage_node_id = $5,
			    mount_source_type = $6,
			    mount_mode = $7,
			    source_path = $8,
			    relative_root_path = $9,
			    heartbeat_policy = $10,
			    scan_policy = $11,
			    updated_at = $12
			WHERE id = $1
			  AND deleted_at IS NULL
		`, request.ID, request.Name, request.LibraryID, request.LibraryName, request.NodeID, mountSourceType, mountMode, sourcePath, relativePath, heartbeatPolicy, scanPolicy, now)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}
		if tag.RowsAffected() == 0 {
			return storagedto.SaveLocalFolderResponse{}, apperrors.NotFound("挂载文件夹不存在")
		}

		_, err = tx.Exec(ctx, `
			UPDATE mount_runtime
			SET next_scan_at = $2,
			    next_heartbeat_at = $3,
			    auth_status = $4,
			    updated_at = $5
			WHERE mount_id = $1
		`, request.ID, nextScanAt, nextHeartbeat, initialMountAuthStatus(node.NodeType), now)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}

	record, err := s.loadLocalFolderByID(ctx, request.ID)
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}

	return storagedto.SaveLocalFolderResponse{
		Message: "挂载文件夹已保存",
		Record:  record,
	}, nil
}

func (s *LocalFolderService) RunLocalFolderScan(ctx context.Context, ids []string) (storagedto.RunLocalFolderScanResponse, error) {
	if len(ids) == 0 {
		return storagedto.RunLocalFolderScanResponse{}, apperrors.BadRequest("ids 不能为空")
	}

	for _, id := range ids {
		if _, err := s.runMountScan(ctx, id); err != nil {
			return storagedto.RunLocalFolderScanResponse{}, err
		}
	}

	return storagedto.RunLocalFolderScanResponse{
		Message: fmt.Sprintf("已为 %d 个挂载文件夹创建扫描任务", len(ids)),
	}, nil
}

func (s *LocalFolderService) RunLocalFolderConnectionTest(ctx context.Context, ids []string) (storagedto.RunLocalFolderConnectionTestResponse, error) {
	if len(ids) == 0 {
		return storagedto.RunLocalFolderConnectionTestResponse{}, apperrors.BadRequest("ids 不能为空")
	}

	results := make([]storagedto.ConnectionTestResult, 0, len(ids))
	now := s.now().UTC()
	for _, id := range ids {
		mount, err := s.loadMountExecutionConfig(ctx, id)
		if err != nil {
			results = append(results, storagedto.ConnectionTestResult{
				ID:          id,
				Name:        "未知挂载",
				OverallTone: "critical",
				Summary:     "挂载记录读取失败。",
				Checks: []storagedto.ConnectionCheck{
					{Label: "挂载读取", Status: "critical", Detail: err.Error()},
				},
				Suggestion: "检查挂载记录是否仍存在",
				TestedAt:   "刚刚",
			})
			continue
		}

		tone := "success"
		summary := "目录可访问，当前配置可继续使用。"
		checks := []storagedto.ConnectionCheck{}
		healthStatus := "ONLINE"
		authStatus := initialMountAuthStatus(mount.NodeType)
		lastErrorCode := ""
		lastErrorMessage := ""
		capacityBytes := int64(0)
		availableBytes := int64(0)

		switch mount.NodeType {
		case "LOCAL":
			info, statErr := os.Stat(mount.SourcePath)
			if statErr != nil {
				tone = "critical"
				summary = "目录不可访问，请检查路径是否存在。"
				healthStatus = "ERROR"
				lastErrorCode = "path_unavailable"
				lastErrorMessage = statErr.Error()
				checks = append(checks, storagedto.ConnectionCheck{Label: "可达性", Status: "critical", Detail: statErr.Error()})
			} else {
				checks = append(checks, storagedto.ConnectionCheck{Label: "可达性", Status: "success", Detail: "目录存在。"})

				if info.IsDir() {
					checks = append(checks, storagedto.ConnectionCheck{Label: "目录类型", Status: "success", Detail: "目标路径是目录。"})
				} else {
					tone = "critical"
					summary = "目标路径不是目录，请重新选择。"
					healthStatus = "ERROR"
					lastErrorCode = "not_directory"
					lastErrorMessage = "目标路径不是目录"
					checks = append(checks, storagedto.ConnectionCheck{Label: "目录类型", Status: "critical", Detail: "目标路径不是目录。"})
				}

				if _, readErr := os.ReadDir(mount.SourcePath); readErr == nil {
					checks = append(checks, storagedto.ConnectionCheck{Label: "读权限", Status: "success", Detail: "目录可读取。"})
					if total, free, usageErr := detectDiskUsage(mount.SourcePath); usageErr == nil {
						capacityBytes = total
						availableBytes = free
						checks = append(checks, storagedto.ConnectionCheck{
							Label:  "容量",
							Status: "success",
							Detail: fmt.Sprintf("总容量 %s，可用 %s。", bytesSummary(total), bytesSummary(free)),
						})
					}
				} else {
					tone = "critical"
					summary = "目录存在，但当前进程无法读取。"
					healthStatus = "ERROR"
					lastErrorCode = "read_failed"
					lastErrorMessage = readErr.Error()
					checks = append(checks, storagedto.ConnectionCheck{Label: "读权限", Status: "critical", Detail: readErr.Error()})
				}
			}
		case "NAS":
			password, decryptErr := s.cipher.Decrypt(mount.SecretCiphertext)
			if decryptErr != nil {
				tone = "critical"
				summary = "NAS 凭据无法读取，请重新保存账号和密码。"
				checks = []storagedto.ConnectionCheck{
					{Label: "凭据读取", Status: "critical", Detail: decryptErr.Error()},
				}
				healthStatus = "ERROR"
				authStatus = "FAILED"
				lastErrorCode = "credential_unreadable"
				lastErrorMessage = decryptErr.Error()
				break
			}
			probe, probeErr := s.nas.Test(ctx, mount.SourcePath, mount.Username, password)
			if probeErr != nil {
				return storagedto.RunLocalFolderConnectionTestResponse{}, probeErr
			}
			tone = probe.OverallTone
			summary = probe.Summary
			checks = probe.Checks
			healthStatus = probe.HealthStatus
			authStatus = probe.AuthStatus
			lastErrorCode = probe.LastErrorCode
			lastErrorMessage = probe.LastErrorMessage
		case "CLOUD":
			if s.integration == nil {
				tone = "critical"
				summary = "云端集成服务尚未启用。"
				checks = []storagedto.ConnectionCheck{
					{Label: "集成服务", Status: "critical", Detail: "未找到云端集成 Provider"},
				}
				healthStatus = "ERROR"
				authStatus = "FAILED"
				lastErrorCode = "integration_unavailable"
				lastErrorMessage = "云端集成服务尚未启用"
				break
			}
			cloudService := NewCloudNodeService(s.pool, s.integration)
			profile, profileErr := cloudService.loadCloudProfile(ctx, mount.NodeID)
			if profileErr != nil {
				tone = "critical"
				summary = "网盘配置读取失败。"
				checks = []storagedto.ConnectionCheck{
					{Label: "云端配置读取", Status: "critical", Detail: profileErr.Error()},
				}
				healthStatus = "ERROR"
				authStatus = "FAILED"
				lastErrorCode = "cloud_profile_unavailable"
				lastErrorMessage = profileErr.Error()
				break
			}
			driver, driverErr := s.integration.Provider(profile.ProviderVendor)
			if driverErr != nil {
				tone = "critical"
				summary = "云端驱动不可用。"
				checks = []storagedto.ConnectionCheck{
					{Label: "云端驱动", Status: "critical", Detail: driverErr.Error()},
				}
				healthStatus = "ERROR"
				authStatus = "FAILED"
				lastErrorCode = "cloud_driver_unavailable"
				lastErrorMessage = driverErr.Error()
				break
			}
			if ensureErr := driver.EnsureRemoteRoot(ctx, profile.Payload, mount.SourcePath); ensureErr != nil {
				tone = "critical"
				summary = "挂载目录访问失败。"
				checks = []storagedto.ConnectionCheck{
					{Label: "CD2 鉴权", Status: "success", Detail: "已使用保存的网盘节点配置接入 CloudDrive2"},
					{Label: "挂载目录访问", Status: "critical", Detail: ensureErr.Error()},
				}
				healthStatus = "ERROR"
				authStatus = "AUTHORIZED"
				lastErrorCode = "mount_path_unavailable"
				lastErrorMessage = ensureErr.Error()
				break
			}
			tone = "success"
			summary = "网盘挂载目录可访问，当前配置可继续使用。"
			checks = []storagedto.ConnectionCheck{
				{Label: "CD2 鉴权", Status: "success", Detail: "已使用保存的网盘节点配置接入 CloudDrive2"},
				{Label: "挂载目录访问", Status: "success", Detail: "目录存在且可访问。"},
			}
			healthStatus = "ONLINE"
			authStatus = "AUTHORIZED"
		default:
			return storagedto.RunLocalFolderConnectionTestResponse{}, apperrors.BadRequest("当前挂载类型暂不支持连接测试")
		}

		_, err = s.pool.Exec(ctx, `
			UPDATE mount_runtime
			SET auth_status = $2,
			    health_status = $3,
			    last_check_at = $4,
			    last_error_code = NULLIF($5, ''),
			    last_error_message = NULLIF($6, ''),
			    capacity_bytes = CASE WHEN $7::bigint > 0 THEN $7::bigint ELSE NULL END,
			    available_bytes = CASE WHEN $8::bigint > 0 THEN $8::bigint ELSE NULL END,
			    updated_at = $4
			WHERE mount_id = $1
		`, id, authStatus, healthStatus, now, lastErrorCode, lastErrorMessage, capacityBytes, availableBytes)
		if err != nil {
			results = append(results, storagedto.ConnectionTestResult{
				ID:          id,
				Name:        mount.Name,
				OverallTone: "critical",
				Summary:     "检测结果写入运行状态失败。",
				Checks: []storagedto.ConnectionCheck{
					{Label: "状态回写", Status: "critical", Detail: err.Error()},
				},
				Suggestion: "检查中心服务数据库状态",
				TestedAt:   "刚刚",
			})
			continue
		}

		results = append(results, storagedto.ConnectionTestResult{
			ID:          id,
			Name:        mount.Name,
			OverallTone: tone,
			Summary:     summary,
			Checks:      checks,
			Suggestion:  connectionSuggestion(tone),
			TestedAt:    "刚刚",
		})
	}

	message := "连接测试已完成"
	if len(ids) > 1 {
		message = fmt.Sprintf("已完成 %d 个挂载文件夹的连接测试", len(ids))
	}

	return storagedto.RunLocalFolderConnectionTestResponse{
		Message: message,
		Results: results,
	}, nil
}

func (s *LocalFolderService) UpdateLocalFolderHeartbeat(ctx context.Context, ids []string, heartbeatPolicy string) (storagedto.UpdateHeartbeatResponse, error) {
	if len(ids) == 0 {
		return storagedto.UpdateHeartbeatResponse{}, apperrors.BadRequest("ids 不能为空")
	}

	dbPolicy := dbHeartbeatPolicy(heartbeatPolicy)
	if dbPolicy == "" {
		return storagedto.UpdateHeartbeatResponse{}, apperrors.BadRequest("心跳策略无效")
	}

	now := s.now().UTC()
	nextHeartbeat := computeNextHeartbeat(now, dbPolicy)
	for _, id := range ids {
		if _, err := s.pool.Exec(ctx, `
			UPDATE mounts
			SET heartbeat_policy = $2,
			    updated_at = $3
			WHERE id = $1
		`, id, dbPolicy, now); err != nil {
			return storagedto.UpdateHeartbeatResponse{}, err
		}

		if _, err := s.pool.Exec(ctx, `
			UPDATE mount_runtime
			SET next_heartbeat_at = $2,
			    updated_at = $3
			WHERE mount_id = $1
		`, id, nextHeartbeat, now); err != nil {
			return storagedto.UpdateHeartbeatResponse{}, err
		}
	}

	return storagedto.UpdateHeartbeatResponse{Message: "心跳策略已更新"}, nil
}

func (s *LocalFolderService) LoadLocalFolderScanHistory(ctx context.Context, id string) (storagedto.LocalFolderScanHistoryResponse, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, started_at, finished_at, status, summary, trigger
		FROM mount_scan_histories
		WHERE mount_id = $1
		ORDER BY started_at DESC
	`, id)
	if err != nil {
		return storagedto.LocalFolderScanHistoryResponse{}, err
	}
	defer rows.Close()

	items := make([]storagedto.ScanHistoryItem, 0)
	for rows.Next() {
		var (
			historyID  string
			startedAt  time.Time
			finishedAt time.Time
			status     string
			summary    string
			trigger    string
		)
		if err := rows.Scan(&historyID, &startedAt, &finishedAt, &status, &summary, &trigger); err != nil {
			return storagedto.LocalFolderScanHistoryResponse{}, err
		}
		items = append(items, storagedto.ScanHistoryItem{
			ID:         historyID,
			StartedAt:  startedAt.Format("2006-01-02 15:04"),
			FinishedAt: finishedAt.Format("2006-01-02 15:04"),
			Status:     uiHistoryStatus(status),
			Summary:    summary,
			Trigger:    trigger,
		})
	}

	if err := rows.Err(); err != nil {
		return storagedto.LocalFolderScanHistoryResponse{}, err
	}

	return storagedto.LocalFolderScanHistoryResponse{ID: id, Items: items}, nil
}

func (s *LocalFolderService) DeleteLocalFolder(ctx context.Context, id string) (storagedto.DeleteLocalFolderResponse, error) {
	now := s.now().UTC()
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE mounts
		SET deleted_at = $2,
		    updated_at = $2
		WHERE id = $1
		  AND deleted_at IS NULL
	`, id, now)
	if err != nil {
		return storagedto.DeleteLocalFolderResponse{}, err
	}
	if commandTag.RowsAffected() == 0 {
		return storagedto.DeleteLocalFolderResponse{}, apperrors.NotFound("挂载文件夹不存在")
	}

	return storagedto.DeleteLocalFolderResponse{Message: "挂载文件夹已删除"}, nil
}

func (s *LocalFolderService) loadLocalFolderByID(ctx context.Context, id string) (storagedto.LocalFolderRecord, error) {
	items, err := s.ListLocalFolders(ctx)
	if err != nil {
		return storagedto.LocalFolderRecord{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return storagedto.LocalFolderRecord{}, apperrors.NotFound("挂载文件夹不存在")
}

func (s *LocalFolderService) loadStorageNodeForMount(ctx context.Context, id string) (storageNodeMountRef, error) {
	var node storageNodeMountRef
	err := s.pool.QueryRow(ctx, `
		SELECT
			sn.id,
			sn.name,
			sn.node_type,
			sn.access_mode,
			COALESCE(sn.address, ''),
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM storage_nodes sn
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE sn.id = $1
		  AND sn.node_type IN ('LOCAL', 'NAS', 'CLOUD')
		  AND sn.deleted_at IS NULL
	`, id).Scan(&node.ID, &node.Name, &node.NodeType, &node.AccessMode, &node.Address, &node.Username, &node.SecretCiphertext)
	if err != nil {
		if err == pgx.ErrNoRows {
			return storageNodeMountRef{}, apperrors.NotFound("所属节点不存在")
		}
		return storageNodeMountRef{}, err
	}
	return node, nil
}

func (s *LocalFolderService) loadMountExecutionConfig(ctx context.Context, id string) (mountExecutionConfig, error) {
	var mount mountExecutionConfig
	err := s.pool.QueryRow(ctx, `
		SELECT
			m.id,
			m.name,
			m.storage_node_id,
			sn.node_type,
			m.source_path,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE m.id = $1
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
	`, id).Scan(&mount.ID, &mount.Name, &mount.NodeID, &mount.NodeType, &mount.SourcePath, &mount.Username, &mount.SecretCiphertext)
	if err != nil {
		if err == pgx.ErrNoRows {
			return mountExecutionConfig{}, apperrors.NotFound("挂载文件夹不存在")
		}
		return mountExecutionConfig{}, err
	}
	return mount, nil
}

type storageNodeMountRef struct {
	ID               string
	Name             string
	NodeType         string
	AccessMode       string
	Address          string
	Username         string
	SecretCiphertext string
}

type mountExecutionConfig struct {
	ID               string
	Name             string
	NodeID           string
	NodeType         string
	SourcePath       string
	Username         string
	SecretCiphertext string
}

func buildMountSourcePath(node storageNodeMountRef, relativePath string) (string, string, error) {
	switch node.NodeType {
	case "LOCAL":
		return buildLocalMountPath(node.Address, relativePath), "LOCAL_PATH", nil
	case "NAS":
		if strings.TrimSpace(node.SecretCiphertext) == "" {
			return "", "", apperrors.BadRequest("NAS 节点凭据缺失，请先重新保存 NAS 节点")
		}
		return joinSMBPath(node.Address, relativePath), "NAS_SHARE", nil
	case "CLOUD":
		return buildCloudMountPath(node.Address, relativePath), "CLOUD_FOLDER", nil
	default:
		return "", "", apperrors.BadRequest("当前节点类型暂不支持新增挂载")
	}
}

func buildCloudMountPath(rootPath string, relativePath string) string {
	root := strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(rootPath, `\`, `/`)), `/`)
	relative := strings.Trim(strings.TrimSpace(strings.ReplaceAll(relativePath, `\`, `/`)), `/`)
	if root == "" {
		root = "/"
	}
	if relative == "" {
		return root
	}
	if root == "/" {
		return "/" + relative
	}
	return root + "/" + relative
}

func buildLocalMountPath(rootPath string, relativePath string) string {
	root := strings.TrimRight(rootPath, `\/`)
	if relativePath == "" {
		return root
	}
	return root + string(os.PathSeparator) + strings.ReplaceAll(relativePath, `\`, string(os.PathSeparator))
}

func buildCode(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%d", prefix, now.UnixNano())
}

func dbMountMode(value string) string {
	switch value {
	case "只读":
		return "READ_ONLY"
	case "可写":
		return "READ_WRITE"
	default:
		return ""
	}
}

func uiMountMode(value string) string {
	switch value {
	case "READ_ONLY":
		return "只读"
	default:
		return "可写"
	}
}

func dbHeartbeatPolicy(value string) string {
	switch value {
	case "从不":
		return "NEVER"
	case "每小时":
		return "HOURLY"
	case "每日（深夜）":
		return "DAILY"
	case "每周（深夜）":
		return "WEEKLY"
	default:
		return ""
	}
}

func uiHeartbeatPolicy(value string) string {
	switch value {
	case "HOURLY":
		return "每小时"
	case "DAILY":
		return "每日（深夜）"
	case "WEEKLY":
		return "每周（深夜）"
	default:
		return "从不"
	}
}

func dbScanPolicy(value string) string {
	switch strings.TrimSpace(value) {
	case "", "手动扫描":
		return "MANUAL"
	case "启动时扫描":
		return "ON_START"
	case "定时扫描":
		return "SCHEDULED"
	default:
		return ""
	}
}

func computeNextHeartbeat(now time.Time, policy string) *time.Time {
	switch policy {
	case "HOURLY":
		next := now.Add(time.Hour)
		return &next
	case "DAILY":
		next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		return &next
	case "WEEKLY":
		next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
		for next.Weekday() != time.Saturday || !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		return &next
	default:
		return nil
	}
}

func computeNextScanAt(now time.Time, policy string) *time.Time {
	switch policy {
	case "SCHEDULED":
		next := now.Add(time.Hour)
		return &next
	default:
		return nil
	}
}

func uiNextHeartbeatLabel(policy string, next *time.Time) string {
	if policy == "NEVER" || next == nil {
		return "—"
	}
	switch policy {
	case "HOURLY":
		return "1 小时后"
	case "DAILY":
		return "今晚 02:00"
	case "WEEKLY":
		return "周六 02:00"
	default:
		return "待首次执行"
	}
}

func uiScanStatus(status string, lastScanAt *time.Time) string {
	switch status {
	case "SUCCESS":
		return "最近扫描成功"
	case "FAILED":
		return "最近扫描失败"
	case "RUNNING":
		return "扫描中"
	default:
		if lastScanAt == nil {
			return "未扫描"
		}
		return "等待队列"
	}
}

func uiScanTone(status string) string {
	switch status {
	case "SUCCESS":
		return "success"
	case "FAILED":
		return "critical"
	case "RUNNING":
		return "warning"
	default:
		return "info"
	}
}

func uiScanAt(value *time.Time) string {
	if value == nil {
		return "未扫描"
	}
	return value.Format("2006-01-02 15:04")
}

func uiCapacitySummary(total *int64, available *int64) string {
	if total == nil || available == nil || *total <= 0 {
		return "待检测"
	}
	usedPercent := uiCapacityPercent(total, available)
	return fmt.Sprintf("已用 %d%% · %s 可用", usedPercent, bytesSummary(*available))
}

func uiFreeSpaceSummary(available *int64) string {
	if available == nil {
		return "待检测"
	}
	return fmt.Sprintf("%s 可用", bytesSummary(*available))
}

func uiCapacityPercent(total *int64, available *int64) int {
	if total == nil || available == nil || *total <= 0 {
		return 0
	}
	used := *total - *available
	if used < 0 {
		used = 0
	}
	return int((float64(used) / float64(*total)) * 100)
}

func uiRiskTags(scanStatus string, healthStatus string, lastErrorMessage string) []string {
	tags := make([]string, 0)
	if scanStatus == "FAILED" {
		tags = append(tags, "扫描失败")
	}
	if healthStatus == "ERROR" && strings.TrimSpace(lastErrorMessage) != "" {
		tags = append(tags, "连接异常")
	}
	return tags
}

func uiAuthStatus(status string) string {
	switch status {
	case "AUTHORIZED":
		return "鉴权正常"
	case "FAILED":
		return "鉴权异常"
	case "EXPIRED":
		return "鉴权过期"
	case "NOT_REQUIRED", "":
		return "无需鉴权"
	default:
		return "待检测"
	}
}

func uiMountAuthStatus(nodeType string, status string) string {
	if nodeType == "LOCAL" {
		return "无需鉴权"
	}
	return uiAuthStatus(status)
}

func uiAuthTone(status string) string {
	switch status {
	case "AUTHORIZED":
		return "success"
	case "FAILED", "EXPIRED":
		return "warning"
	case "NOT_REQUIRED", "":
		return "info"
	default:
		return "info"
	}
}

func uiMountAuthTone(nodeType string, status string) string {
	if nodeType == "LOCAL" {
		return "info"
	}
	return uiAuthTone(status)
}

func uiHistoryStatus(status string) string {
	switch status {
	case "SUCCESS":
		return "成功"
	case "FAILED":
		return "失败"
	default:
		return "进行中"
	}
}

func connectionSuggestion(tone string) string {
	if tone == "success" {
		return "可立即执行扫描"
	}
	return "检查目录路径与权限"
}

func bytesSummary(value int64) string {
	if value <= 0 {
		return "0 B"
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)
	switch {
	case value >= tb:
		return fmt.Sprintf("%.1f TB", float64(value)/float64(tb))
	case value >= gb:
		return fmt.Sprintf("%.1f GB", float64(value)/float64(gb))
	case value >= mb:
		return fmt.Sprintf("%.1f MB", float64(value)/float64(mb))
	default:
		return fmt.Sprintf("%d B", value)
	}
}

func uiFolderType(nodeType string) string {
	switch nodeType {
	case "NAS":
		return "NAS"
	case "CLOUD":
		return "网盘"
	default:
		return "本地"
	}
}

func buildMountBadges(nodeType string, accessMode string, mountMode string) []string {
	sourceBadge := "本地"
	if nodeType == "NAS" && accessMode == "SMB" {
		sourceBadge = "SMB"
	}
	if nodeType == "CLOUD" {
		sourceBadge = "115"
	}
	return []string{sourceBadge, uiMountMode(mountMode)}
}

func initialMountAuthStatus(nodeType string) string {
	if nodeType == "NAS" || nodeType == "CLOUD" {
		return "UNKNOWN"
	}
	return "NOT_REQUIRED"
}
