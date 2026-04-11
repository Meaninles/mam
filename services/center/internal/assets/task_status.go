package assets

import (
	"context"
	"fmt"
)

func (s *Service) loadLatestTaskStatusForAsset(ctx context.Context, assetID string) (string, string, error) {
	type taskSnapshot struct {
		title      *string
		summary    *string
		status     string
		intent     string
		targetName *string
	}

	var snapshot taskSnapshot
	err := s.pool.QueryRow(ctx, `
		SELECT
			ji.title,
			ji.summary,
			j.status,
			j.job_intent,
			m.name
		FROM job_object_links asset_link
		INNER JOIN jobs j ON j.id = asset_link.job_id
		LEFT JOIN job_items ji ON ji.id = asset_link.job_item_id
		LEFT JOIN job_object_links mount_link
			ON mount_link.job_id = asset_link.job_id
		   AND mount_link.job_item_id IS NOT DISTINCT FROM asset_link.job_item_id
		   AND mount_link.link_role = 'TARGET_MOUNT'
		LEFT JOIN mounts m ON m.id = mount_link.mount_id
		WHERE asset_link.asset_id = $1
		ORDER BY j.updated_at DESC, j.created_at DESC
		LIMIT 1
	`, assetID).Scan(&snapshot.title, &snapshot.summary, &snapshot.status, &snapshot.intent, &snapshot.targetName)
	if err != nil {
		return "暂无任务", "info", nil
	}

	baseText := ""
	if snapshot.summary != nil && *snapshot.summary != "" {
		baseText = *snapshot.summary
	} else if snapshot.title != nil && *snapshot.title != "" {
		baseText = *snapshot.title
	} else {
		baseText = fallbackTaskSummary(snapshot.intent, snapshot.targetName)
	}

	return fmt.Sprintf("%s（%s）", baseText, mapJobStatusToTaskLabel(snapshot.status)), mapJobStatusToTaskTone(snapshot.status), nil
}

func (s *Service) loadPendingEndpointOperations(ctx context.Context, assetID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT mount_link.mount_id
		FROM job_object_links asset_link
		INNER JOIN jobs j ON j.id = asset_link.job_id
		INNER JOIN job_object_links mount_link
			ON mount_link.job_id = asset_link.job_id
		   AND mount_link.job_item_id IS NOT DISTINCT FROM asset_link.job_item_id
		   AND mount_link.link_role = 'TARGET_MOUNT'
		WHERE asset_link.asset_id = $1
		  AND j.status IN ('PENDING', 'QUEUED', 'RUNNING', 'WAITING_RETRY', 'PAUSED')
		  AND mount_link.mount_id IS NOT NULL
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]struct{})
	for rows.Next() {
		var mountID string
		if err := rows.Scan(&mountID); err != nil {
			return nil, err
		}
		results[mountID] = struct{}{}
	}
	return results, rows.Err()
}

func fallbackTaskSummary(intent string, targetName *string) string {
	switch intent {
	case "REPLICATE":
		if targetName != nil && *targetName != "" {
			return "同步到 " + *targetName
		}
		return "同步副本"
	case "DELETE_REPLICA":
		if targetName != nil && *targetName != "" {
			return "删除 " + *targetName + " 副本"
		}
		return "删除副本"
	case "DELETE_ASSET":
		return "删除资产"
	default:
		return "最近任务"
	}
}

func mapJobStatusToTaskLabel(status string) string {
	switch status {
	case "RUNNING":
		return "运行中"
	case "PAUSED":
		return "已暂停"
	case "WAITING_CONFIRMATION", "WAITING_RETRY", "PENDING", "QUEUED":
		return "待执行"
	case "PARTIAL_SUCCESS":
		return "部分成功"
	case "FAILED":
		return "失败"
	case "COMPLETED":
		return "已完成"
	case "CANCELED":
		return "已取消"
	default:
		return status
	}
}

func mapJobStatusToTaskTone(status string) string {
	switch status {
	case "COMPLETED":
		return "success"
	case "FAILED":
		return "critical"
	case "PARTIAL_SUCCESS", "RUNNING":
		return "warning"
	default:
		return "info"
	}
}
