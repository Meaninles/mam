package issues

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	issuedto "mare/shared/contracts/dto/issue"
)

type missingReplicaRow struct {
	ReplicaID          string
	AssetID            string
	LibraryID          *string
	AssetName          string
	PhysicalPath       string
	MountID            string
	MountName          string
	StorageNodeID      string
	StorageNodeName    string
	MissingDetectedAt  time.Time
	LastDetectionKey   string
}

func (s *Service) SyncMissingReplicaIssues(ctx context.Context) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := s.loadMissingReplicaRows(ctx, tx)
	if err != nil {
		return err
	}
	candidates := make([]issueCandidate, 0, len(rows))
	dedupeKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		candidate := buildMissingReplicaCandidate(row)
		candidates = append(candidates, candidate)
		dedupeKeys = append(dedupeKeys, candidate.DedupeKey)
	}

	existing, err := s.loadExistingIssuesByDedupe(ctx, tx, dedupeKeys)
	if err != nil {
		return err
	}

	changedIssueIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if current := existing[candidate.DedupeKey]; current != nil {
			if err := s.upsertExistingIssue(ctx, tx, *current, candidate, jobSnapshot{Status: "FAILED"}); err != nil {
				return err
			}
			changedIssueIDs = append(changedIssueIDs, current.ID)
			continue
		}
		if err := s.insertIssue(ctx, tx, candidate); err != nil {
			return err
		}
		inserted, err := s.loadExistingIssuesByDedupe(ctx, tx, []string{candidate.DedupeKey})
		if err != nil {
			return err
		}
		if current := inserted[candidate.DedupeKey]; current != nil {
			changedIssueIDs = append(changedIssueIDs, current.ID)
		}
	}

	resolvedIssueIDs, err := s.resolveRecoveredMissingReplicaIssues(ctx, tx, dedupeStrings(dedupeKeys))
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	for _, issueID := range append(changedIssueIDs, resolvedIssueIDs...) {
		s.syncIssueNotification(ctx, issueID)
	}
	return nil
}

func (s *Service) loadMissingReplicaRows(ctx context.Context, tx pgx.Tx) ([]missingReplicaRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			ar.id,
			ar.asset_id,
			a.library_id,
			a.name,
			ar.physical_path,
			m.id,
			m.name,
			sn.id,
			sn.name,
			ar.missing_detected_at
		FROM asset_replicas ar
		INNER JOIN assets a ON a.id = ar.asset_id
		INNER JOIN mounts m ON m.id = ar.mount_id
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		WHERE ar.replica_state = 'MISSING'
		  AND ar.missing_detected_at IS NOT NULL
		  AND a.deleted_at IS NULL
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
		ORDER BY ar.missing_detected_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]missingReplicaRow, 0)
	for rows.Next() {
		var item missingReplicaRow
		if err := rows.Scan(
			&item.ReplicaID,
			&item.AssetID,
			&item.LibraryID,
			&item.AssetName,
			&item.PhysicalPath,
			&item.MountID,
			&item.MountName,
			&item.StorageNodeID,
			&item.StorageNodeName,
			&item.MissingDetectedAt,
		); err != nil {
			return nil, err
		}
		item.LastDetectionKey = fmt.Sprintf("%s:%s", item.ReplicaID, item.MissingDetectedAt.UTC().Format(time.RFC3339Nano))
		items = append(items, item)
	}
	return items, rows.Err()
}

func buildMissingReplicaCandidate(row missingReplicaRow) issueCandidate {
	issueType := "REPLICA_MISSING"
	suggestion := "请确认原文件是否已被移动、删除，或重新触发扫描/同步后再继续处理。"
	sourceLabel := row.StorageNodeName
	endpointLabel := row.MountName
	title := fmt.Sprintf("%s 文件缺失", row.AssetName)
	summary := "扫描发现副本文件已不存在，请确认是否被移动、删除或暂时离线。"
	return issueCandidate{
		LibraryID:            row.LibraryID,
		IssueCategory:        CategoryScanParse,
		IssueType:            issueType,
		Nature:               NatureBlocking,
		SourceDomain:         SourceDomainGovernance,
		Severity:             SeverityWarning,
		DedupeKey:            "missing-replica:" + row.ReplicaID,
		Title:                title,
		Summary:              summary,
		ObjectLabel:          row.PhysicalPath,
		AssetLabel:           &row.AssetName,
		SuggestedActionLabel: ptr("打开存储节点"),
		Suggestion:           &suggestion,
		Detail:               ptr("后台巡检检测到副本文件缺失，已转为可治理异常。"),
		DetectionKeys:        []string{row.LastDetectionKey},
		SourceSnapshot: issuedto.SourceContext{
			AssetID:       &row.AssetID,
			EntryID:       &row.AssetID,
			EndpointID:    &row.MountID,
			EndpointLabel: &endpointLabel,
			Path:          &row.PhysicalPath,
			SourceLabel:   &sourceLabel,
		},
		ImpactSnapshot: issuedto.ImpactSummary{
			AssetCount:          1,
			ReplicaCount:        1,
			EndpointCount:       1,
			BlocksStatusCommit:  false,
			BlocksTaskExecution: false,
		},
		LatestErrorCode:    ptr("replica_missing"),
		LatestErrorMessage: ptr("扫描发现副本文件已不存在"),
		Links: []issueObjectLink{
			{LinkRole: "AFFECTED_ASSET", ObjectType: "ASSET", AssetID: &row.AssetID, ObjectLabel: &row.AssetName},
			{LinkRole: "AFFECTED_REPLICA", ObjectType: "ASSET_REPLICA", AssetReplicaID: &row.ReplicaID, ObjectLabel: &row.PhysicalPath},
			{LinkRole: "AFFECTED_MOUNT", ObjectType: "MOUNT", MountID: &row.MountID, ObjectLabel: &row.MountName},
			{LinkRole: "AFFECTED_STORAGE_NODE", ObjectType: "STORAGE_NODE", StorageNodeID: &row.StorageNodeID, ObjectLabel: &row.StorageNodeName},
		},
	}
}

func (s *Service) resolveRecoveredMissingReplicaIssues(ctx context.Context, tx pgx.Tx, activeDedupeKeys []string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id, code, library_id, issue_category, issue_type, nature, source_domain, severity, status, dedupe_key,
			title, summary, object_label, asset_label, suggested_action, suggested_action_label, suggestion, detail, occurrence_count, last_detection_key,
			source_snapshot, impact_snapshot, first_detected_at, last_detected_at, last_status_changed_at,
			resolved_at, archived_at, latest_event_at, latest_error_code, latest_error_message, created_at, updated_at
		FROM issues
		WHERE source_domain = 'SYSTEM_GOVERNANCE'
		  AND issue_type = 'REPLICA_MISSING'
		  AND status IN ('OPEN', 'AWAITING_CONFIRMATION', 'IN_PROGRESS')
		  AND NOT (dedupe_key = ANY($1))
		FOR UPDATE
	`, activeDedupeKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := s.now().UTC()
	resolved := make([]string, 0)
	for rows.Next() {
		row, err := scanIssueRow(rows)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE issues
			SET status = 'RESOLVED',
			    resolved_at = COALESCE(resolved_at, $2),
			    last_status_changed_at = $2,
			    latest_event_at = $2,
			    updated_at = $2
			WHERE id = $1
		`, row.ID, now); err != nil {
			return nil, err
		}
		if err := s.insertIssueEvent(ctx, tx, row.ID, issueEventInput{
			EventType:     EventResolved,
			FromStatus:    ptr(row.Status),
			ToStatus:      ptr(StatusResolved),
			ActorType:     "SCHEDULER",
			OperatorLabel: ptr("后台巡检"),
			Message:       ptr("缺失副本已恢复，异常已自动关闭。"),
			CreatedAt:     now,
		}); err != nil {
			return nil, err
		}
		resolved = append(resolved, row.ID)
	}
	return resolved, rows.Err()
}
