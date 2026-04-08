package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	apperrors "mare/services/center/internal/errors"
	storagedto "mare/shared/contracts/dto/storage"
)

func (s *LocalFolderService) ListLocalNodes(ctx context.Context) ([]storagedto.LocalNodeRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			sn.id,
			sn.name,
			sn.address,
			sn.enabled,
			COALESCE(snr.health_status, 'UNKNOWN'),
			snr.last_check_at,
			COALESCE(sn.description, ''),
			COUNT(m.id) FILTER (WHERE m.deleted_at IS NULL)
		FROM storage_nodes sn
		LEFT JOIN storage_node_runtime snr ON snr.storage_node_id = sn.id
		LEFT JOIN mounts m ON m.storage_node_id = sn.id
		WHERE sn.node_type = 'LOCAL'
		  AND sn.deleted_at IS NULL
		GROUP BY sn.id, sn.name, sn.address, sn.enabled, snr.health_status, snr.last_check_at, sn.description
		ORDER BY sn.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]storagedto.LocalNodeRecord, 0)
	for rows.Next() {
		var (
			id          string
			name        string
			rootPath    string
			enabled     bool
			health      string
			lastCheckAt *time.Time
			notes       string
			mountCount  int
		)
		if err := rows.Scan(&id, &name, &rootPath, &enabled, &health, &lastCheckAt, &notes, &mountCount); err != nil {
			return nil, err
		}

		total, free, usageErr := detectDiskUsage(rootPath)
		var totalPtr *int64
		var freePtr *int64
		if usageErr == nil {
			totalPtr = &total
			freePtr = &free
		}

		items = append(items, storagedto.LocalNodeRecord{
			ID:               id,
			Name:             name,
			RootPath:         rootPath,
			Enabled:          enabled,
			HealthStatus:     uiNodeHealthStatus(health),
			HealthTone:       uiNodeHealthTone(health),
			LastCheckAt:      uiNodeLastCheckAt(lastCheckAt),
			CapacitySummary:  uiCapacitySummary(totalPtr, freePtr),
			FreeSpaceSummary: uiFreeSpaceSummary(freePtr),
			CapacityPercent:  uiCapacityPercent(totalPtr, freePtr),
			MountCount:       mountCount,
			Notes:            notes,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *LocalFolderService) SaveLocalNode(ctx context.Context, request storagedto.SaveLocalNodeRequest) (storagedto.SaveLocalNodeResponse, error) {
	if strings.TrimSpace(request.Name) == "" {
		return storagedto.SaveLocalNodeResponse{}, apperrors.BadRequest("本地文件夹名称不能为空")
	}
	if strings.TrimSpace(request.RootPath) == "" {
		return storagedto.SaveLocalNodeResponse{}, apperrors.BadRequest("本地文件夹根目录不能为空")
	}

	now := s.now().UTC()
	if strings.TrimSpace(request.ID) == "" {
		nodeID := buildCode("local-node-id", now)
		_, err := s.pool.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, address, access_mode, enabled, description, created_at, updated_at
			) VALUES ($1, $2, $3, 'LOCAL', $4, 'DIRECT', true, $5, $6, $6)
		`, nodeID, buildCode("local-node", now), request.Name, request.RootPath, request.Notes, now)
		if err != nil {
			return storagedto.SaveLocalNodeResponse{}, err
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO storage_node_runtime (
				id, storage_node_id, health_status, auth_status, created_at, updated_at
			) VALUES ($1, $2, 'UNKNOWN', 'NOT_REQUIRED', $3, $3)
		`, buildCode("local-node-runtime", now), nodeID, now)
		if err != nil {
			return storagedto.SaveLocalNodeResponse{}, err
		}
		request.ID = nodeID
	} else {
		tag, err := s.pool.Exec(ctx, `
			UPDATE storage_nodes
			SET name = $2,
			    address = $3,
			    description = $4,
			    updated_at = $5
			WHERE id = $1
			  AND node_type = 'LOCAL'
			  AND deleted_at IS NULL
		`, request.ID, request.Name, request.RootPath, request.Notes, now)
		if err != nil {
			return storagedto.SaveLocalNodeResponse{}, err
		}
		if tag.RowsAffected() == 0 {
			return storagedto.SaveLocalNodeResponse{}, apperrors.NotFound("本地文件夹节点不存在")
		}
	}

	record, err := s.loadLocalNodeByID(ctx, request.ID)
	if err != nil {
		return storagedto.SaveLocalNodeResponse{}, err
	}

	return storagedto.SaveLocalNodeResponse{
		Message: "本地文件夹已保存",
		Record:  record,
	}, nil
}

func (s *LocalFolderService) RunLocalNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunLocalNodeConnectionTestResponse, error) {
	if len(ids) == 0 {
		return storagedto.RunLocalNodeConnectionTestResponse{}, apperrors.BadRequest("ids 不能为空")
	}

	now := s.now().UTC()
	results := make([]storagedto.ConnectionTestResult, 0, len(ids))
	for _, id := range ids {
		node, err := s.loadLocalNodeByID(ctx, id)
		if err != nil {
			return storagedto.RunLocalNodeConnectionTestResponse{}, err
		}

		tone := "success"
		summary := "节点根目录可访问，当前配置可继续使用。"
		checks := []storagedto.ConnectionCheck{}
		healthStatus := "ONLINE"
		lastErrorCode := ""
		lastErrorMessage := ""

		info, statErr := os.Stat(node.RootPath)
		if statErr != nil {
			tone = "critical"
			summary = "节点根目录不可访问，请检查路径是否存在。"
			healthStatus = "ERROR"
			lastErrorCode = "path_unavailable"
			lastErrorMessage = statErr.Error()
			checks = append(checks, storagedto.ConnectionCheck{Label: "可达性", Status: "critical", Detail: statErr.Error()})
		} else {
			checks = append(checks, storagedto.ConnectionCheck{Label: "可达性", Status: "success", Detail: "根目录存在。"})
			if info.IsDir() {
				checks = append(checks, storagedto.ConnectionCheck{Label: "目录类型", Status: "success", Detail: "目标路径是目录。"})
			} else {
				tone = "critical"
				summary = "节点路径不是目录，请重新选择。"
				healthStatus = "ERROR"
				lastErrorCode = "not_directory"
				lastErrorMessage = "目标路径不是目录"
				checks = append(checks, storagedto.ConnectionCheck{Label: "目录类型", Status: "critical", Detail: "目标路径不是目录。"})
			}

			if _, readErr := os.ReadDir(node.RootPath); readErr == nil {
				checks = append(checks, storagedto.ConnectionCheck{Label: "读权限", Status: "success", Detail: "根目录可读取。"})
				if total, free, usageErr := detectDiskUsage(node.RootPath); usageErr == nil {
					checks = append(checks, storagedto.ConnectionCheck{
						Label:  "容量",
						Status: "success",
						Detail: fmt.Sprintf("总容量 %s，可用 %s。", bytesSummary(total), bytesSummary(free)),
					})
				}
			} else {
				tone = "critical"
				summary = "根目录存在，但当前进程无法读取。"
				healthStatus = "ERROR"
				lastErrorCode = "read_failed"
				lastErrorMessage = readErr.Error()
				checks = append(checks, storagedto.ConnectionCheck{Label: "读权限", Status: "critical", Detail: readErr.Error()})
			}
		}

		_, err = s.pool.Exec(ctx, `
			UPDATE storage_node_runtime
			SET health_status = $2,
			    auth_status = 'NOT_REQUIRED',
			    last_check_at = $3,
			    last_error_code = NULLIF($4, ''),
			    last_error_message = NULLIF($5, ''),
			    updated_at = $3
			WHERE storage_node_id = $1
		`, id, healthStatus, now, lastErrorCode, lastErrorMessage)
		if err != nil {
			return storagedto.RunLocalNodeConnectionTestResponse{}, err
		}

		results = append(results, storagedto.ConnectionTestResult{
			ID:          node.ID,
			Name:        node.Name,
			OverallTone: tone,
			Summary:     summary,
			Checks:      checks,
			Suggestion:  connectionSuggestion(tone),
			TestedAt:    "刚刚",
		})
	}

	message := "连接测试已完成"
	if len(ids) > 1 {
		message = fmt.Sprintf("已完成 %d 个本地文件夹节点的连接测试", len(ids))
	}
	return storagedto.RunLocalNodeConnectionTestResponse{
		Message: message,
		Results: results,
	}, nil
}

func (s *LocalFolderService) DeleteLocalNode(ctx context.Context, id string) (storagedto.DeleteLocalNodeResponse, error) {
	var mountCount int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM mounts
		WHERE storage_node_id = $1
		  AND deleted_at IS NULL
	`, id).Scan(&mountCount); err != nil {
		return storagedto.DeleteLocalNodeResponse{}, err
	}
	if mountCount > 0 {
		return storagedto.DeleteLocalNodeResponse{}, apperrors.BadRequest("当前节点下仍存在挂载，请先删除挂载")
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE storage_nodes
		SET deleted_at = $2,
		    updated_at = $2
		WHERE id = $1
		  AND node_type = 'LOCAL'
		  AND deleted_at IS NULL
	`, id, s.now().UTC())
	if err != nil {
		return storagedto.DeleteLocalNodeResponse{}, err
	}
	if tag.RowsAffected() == 0 {
		return storagedto.DeleteLocalNodeResponse{}, apperrors.NotFound("本地文件夹节点不存在")
	}

	return storagedto.DeleteLocalNodeResponse{Message: "本地文件夹已删除"}, nil
}

func (s *LocalFolderService) loadLocalNodeByID(ctx context.Context, id string) (storagedto.LocalNodeRecord, error) {
	items, err := s.ListLocalNodes(ctx)
	if err != nil {
		return storagedto.LocalNodeRecord{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return storagedto.LocalNodeRecord{}, apperrors.NotFound("本地文件夹节点不存在")
}

func uiNodeHealthStatus(status string) string {
	switch status {
	case "ONLINE":
		return "可用"
	case "ERROR":
		return "异常"
	case "OFFLINE":
		return "离线"
	default:
		return "待检测"
	}
}

func uiNodeHealthTone(status string) string {
	switch status {
	case "ONLINE":
		return "success"
	case "ERROR":
		return "critical"
	case "OFFLINE", "DEGRADED":
		return "warning"
	default:
		return "info"
	}
}

func uiNodeLastCheckAt(value *time.Time) string {
	if value == nil {
		return "尚未检测"
	}
	return value.Format("2006-01-02 15:04")
}
