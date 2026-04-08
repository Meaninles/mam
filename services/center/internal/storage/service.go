package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	storagedto "mare/shared/contracts/dto/storage"
)

type LocalFolderService struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewLocalFolderService(pool *pgxpool.Pool) *LocalFolderService {
	return &LocalFolderService{
		pool: pool,
		now:  time.Now,
	}
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
			sn.address,
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
		WHERE sn.node_type = 'LOCAL'
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
			nodeRootPath     string
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
			&nodeRootPath,
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

		if (capacityBytes == nil || availableBytes == nil) && strings.TrimSpace(sourcePath) != "" {
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
			NodeRootPath:     nodeRootPath,
			RelativePath:     relativePath,
			FolderType:       "本地",
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
			Badges:           []string{"本地", uiMountMode(mountMode)},
			AuthStatus:       uiAuthStatus(authStatus),
			AuthTone:         uiAuthTone(authStatus),
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

	now := s.now().UTC()
	nextHeartbeat := computeNextHeartbeat(now, heartbeatPolicy)
	nodeRootPath, _, err := s.loadLocalNodePath(ctx, request.NodeID)
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}
	relativePath := strings.TrimLeft(strings.TrimSpace(request.RelativePath), `/\`)
	sourcePath := buildLocalMountPath(nodeRootPath, relativePath)
	if mkdirErr := os.MkdirAll(sourcePath, 0o755); mkdirErr != nil {
		return storagedto.SaveLocalFolderResponse{}, apperrors.BadRequest("挂载目录创建失败")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return storagedto.SaveLocalFolderResponse{}, err
	}
	defer tx.Rollback(ctx)

	if strings.TrimSpace(request.ID) == "" {
		mountID := buildCode("local-mount-id", now)
		_, err = tx.Exec(ctx, `
			INSERT INTO mounts (
				id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
				source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'LOCAL_PATH', $7, $8, '/', $9, 'MANUAL', true, 0, $10, $10)
		`,
			mountID,
			buildCode("local-mount", now),
			request.LibraryID,
			request.LibraryName,
			request.NodeID,
			request.Name,
			mountMode,
			sourcePath,
			heartbeatPolicy,
			now,
		)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO mount_runtime (
				id, mount_id, scan_status, next_heartbeat_at, auth_status, health_status, created_at, updated_at
			) VALUES ($1, $2, 'IDLE', $3, 'NOT_REQUIRED', 'UNKNOWN', $4, $4)
		`, buildCode("mount-runtime-id", now), mountID, nextHeartbeat, now)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}

		request.ID = mountID
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE mounts
			SET name = $2,
			    library_id = $3,
			    library_name = $4,
			    storage_node_id = $5,
			    mount_mode = $6,
			    source_path = $7,
			    relative_root_path = $8,
			    heartbeat_policy = $9,
			    updated_at = $10
			WHERE id = $1
			  AND deleted_at IS NULL
		`, request.ID, request.Name, request.LibraryID, request.LibraryName, request.NodeID, mountMode, sourcePath, relativePath, heartbeatPolicy, now)
		if err != nil {
			return storagedto.SaveLocalFolderResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE mount_runtime
			SET next_heartbeat_at = $2,
			    updated_at = $3
			WHERE mount_id = $1
		`, request.ID, nextHeartbeat, now)
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

	now := s.now().UTC()
	for _, id := range ids {
		path, name, err := s.loadLocalFolderPath(ctx, id)
		if err != nil {
			return storagedto.RunLocalFolderScanResponse{}, err
		}

		startedAt := now
		entries, readErr := os.ReadDir(path)
		finishedAt := s.now().UTC()
		scanStatus := "SUCCESS"
		summary := fmt.Sprintf("%s 扫描完成，发现 %d 个直接子项。", name, len(entries))
		lastErrorCode := ""
		lastErrorMessage := ""
		healthStatus := "ONLINE"
		capacityBytes, availableBytes, usageErr := detectDiskUsage(path)
		if readErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：%s", name, readErr.Error())
			lastErrorCode = "scan_failed"
			lastErrorMessage = readErr.Error()
			healthStatus = "ERROR"
		}
		if usageErr != nil {
			capacityBytes = 0
			availableBytes = 0
		}

		_, err = s.pool.Exec(ctx, `
			UPDATE mount_runtime
			SET scan_status = $2,
			    last_scan_at = $3,
			    last_scan_summary = $4,
			    health_status = $5,
			    last_error_code = NULLIF($6, ''),
			    last_error_message = NULLIF($7, ''),
			    capacity_bytes = CASE WHEN $8::bigint > 0 THEN $8::bigint ELSE NULL END,
			    available_bytes = CASE WHEN $9::bigint > 0 THEN $9::bigint ELSE NULL END,
			    updated_at = $3
			WHERE mount_id = $1
		`, id, scanStatus, finishedAt, summary, healthStatus, lastErrorCode, lastErrorMessage, capacityBytes, availableBytes)
		if err != nil {
			return storagedto.RunLocalFolderScanResponse{}, err
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO mount_scan_histories (
				id, mount_id, started_at, finished_at, status, summary, trigger
			) VALUES ($1, $2, $3, $4, $5, $6, '手动扫描')
		`, buildCode("scan-history-id", finishedAt), id, startedAt, finishedAt, scanStatus, summary)
		if err != nil {
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
		path, name, err := s.loadLocalFolderPath(ctx, id)
		if err != nil {
			results = append(results, storagedto.ConnectionTestResult{
				ID:          id,
				Name:        "未知挂载",
				OverallTone: "critical",
				Summary:     "挂载记录读取失败。",
				Checks: []storagedto.ConnectionCheck{
					{
						Label:  "挂载读取",
						Status: "critical",
						Detail: err.Error(),
					},
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
		lastErrorCode := ""
		lastErrorMessage := ""
		capacityBytes := int64(0)
		availableBytes := int64(0)

		info, statErr := os.Stat(path)
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

			if _, readErr := os.ReadDir(path); readErr == nil {
				checks = append(checks, storagedto.ConnectionCheck{Label: "读权限", Status: "success", Detail: "目录可读取。"})
				if total, free, usageErr := detectDiskUsage(path); usageErr == nil {
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

		_, err = s.pool.Exec(ctx, `
			UPDATE mount_runtime
			SET auth_status = 'NOT_REQUIRED',
			    health_status = $2,
			    last_check_at = $3,
			    last_error_code = NULLIF($4, ''),
			    last_error_message = NULLIF($5, ''),
			    capacity_bytes = CASE WHEN $6::bigint > 0 THEN $6::bigint ELSE NULL END,
			    available_bytes = CASE WHEN $7::bigint > 0 THEN $7::bigint ELSE NULL END,
			    updated_at = $3
			WHERE mount_id = $1
		`, id, healthStatus, now, lastErrorCode, lastErrorMessage, capacityBytes, availableBytes)
		if err != nil {
			results = append(results, storagedto.ConnectionTestResult{
				ID:          id,
				Name:        name,
				OverallTone: "critical",
				Summary:     "检测结果生成后写入运行状态失败。",
				Checks: []storagedto.ConnectionCheck{
					{
						Label:  "状态写回",
						Status: "critical",
						Detail: err.Error(),
					},
				},
				Suggestion: "检查中心服务数据库状态",
				TestedAt:   "刚刚",
			})
			continue
		}

		results = append(results, storagedto.ConnectionTestResult{
			ID:          id,
			Name:        name,
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

func (s *LocalFolderService) loadLocalFolderPath(ctx context.Context, id string) (string, string, error) {
	var (
		path string
		name string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT source_path, name
		FROM mounts
		WHERE id = $1
		  AND deleted_at IS NULL
	`, id).Scan(&path, &name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", apperrors.NotFound("挂载文件夹不存在")
		}
		return "", "", err
	}
	return path, name, nil
}

func (s *LocalFolderService) loadLocalNodePath(ctx context.Context, id string) (string, string, error) {
	var (
		path string
		name string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT address, name
		FROM storage_nodes
		WHERE id = $1
		  AND node_type = 'LOCAL'
		  AND deleted_at IS NULL
	`, id).Scan(&path, &name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", apperrors.NotFound("所属节点不存在")
		}
		return "", "", err
	}
	return path, name, nil
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
	if status == "NOT_REQUIRED" || status == "" {
		return "无需鉴权"
	}
	return status
}

func uiAuthTone(status string) string {
	if status == "NOT_REQUIRED" || status == "" {
		return "info"
	}
	return "warning"
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
