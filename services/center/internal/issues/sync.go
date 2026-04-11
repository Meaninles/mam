package issues

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	issuedto "mare/shared/contracts/dto/issue"
)

func (s *Service) SyncJobIssues(ctx context.Context, jobID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	job, err := s.loadJobSnapshot(ctx, tx, jobID)
	if err != nil {
		return err
	}
	items, err := s.loadJobItems(ctx, tx, jobID)
	if err != nil {
		return err
	}
	jobLinks, err := s.loadLinks(ctx, tx, "job_id = $1 AND job_item_id IS NULL", jobID)
	if err != nil {
		return err
	}

	candidates := make([]issueCandidate, 0)
	for _, item := range items {
		if item.Status != "FAILED" {
			continue
		}
		itemLinks, err := s.loadLinks(ctx, tx, "job_item_id = $1", item.ID)
		if err != nil {
			return err
		}
		candidate, err := s.buildItemIssueCandidate(ctx, tx, job, item, jobLinks, itemLinks)
		if err != nil {
			return err
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 && job.Status == "FAILED" && strings.TrimSpace(nullableString(job.LatestErrorMessage)) != "" {
		candidate, err := s.buildJobIssueCandidate(ctx, tx, job, jobLinks)
		if err != nil {
			return err
		}
		candidates = append(candidates, candidate)
	}

	existingByDedupe, err := s.loadExistingIssuesByDedupe(ctx, tx, collectDedupeKeys(candidates))
	if err != nil {
		return err
	}

	desired := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		desired[candidate.DedupeKey] = struct{}{}
		existing := existingByDedupe[candidate.DedupeKey]
		if existing == nil {
			if err := s.insertIssue(ctx, tx, candidate); err != nil {
				return err
			}
			continue
		}
		if err := s.upsertExistingIssue(ctx, tx, *existing, candidate, job); err != nil {
			return err
		}
	}

	if err := s.resolveNoLongerDetectedIssues(ctx, tx, job, desired); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.refreshJobIssueCounters(ctx, jobID)
}

func (s *Service) transitionIssues(ctx context.Context, ids []string, nextStatus string, eventType string, message string) ([]string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT
			id, code, library_id, issue_category, issue_type, nature, source_domain, severity, status, dedupe_key,
			title, summary, object_label, asset_label, suggested_action, suggested_action_label, suggestion, detail,
			source_snapshot, impact_snapshot, first_detected_at, last_detected_at, last_status_changed_at,
			resolved_at, archived_at, latest_event_at, latest_error_code, latest_error_message, created_at, updated_at
		FROM issues
		WHERE id = ANY($1)
		FOR UPDATE
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	loaded := make([]issueRow, 0)
	for rows.Next() {
		row, err := scanIssueRow(rows)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(loaded) == 0 {
		return nil, apperrors.NotFound("异常不存在")
	}

	now := s.now().UTC()
	jobIDs := make(map[string]struct{})
	for _, row := range loaded {
		if taskID := extractTaskID(row.SourceSnapshot); taskID != "" {
			jobIDs[taskID] = struct{}{}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE issues
			SET status = $2,
			    resolved_at = CASE WHEN $2 = 'RESOLVED' THEN COALESCE(resolved_at, $3) ELSE resolved_at END,
			    archived_at = CASE WHEN $2 = 'ARCHIVED' THEN COALESCE(archived_at, $3) ELSE archived_at END,
			    last_status_changed_at = $3,
			    latest_event_at = $3,
			    updated_at = $3
			WHERE id = $1
		`, row.ID, nextStatus, now); err != nil {
			return nil, err
		}
		if err := s.insertIssueEvent(ctx, tx, row.ID, issueEventInput{
			EventType:     eventType,
			ActionKey:     &nextStatus,
			FromStatus:    statusMaybePtr(row.Status),
			ToStatus:      statusMaybePtr(nextStatus),
			ActorType:     "USER",
			OperatorLabel: ptr("异常中心"),
			Message:       ptr(message),
			CreatedAt:     now,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(jobIDs))
	for jobID := range jobIDs {
		if err := s.refreshJobIssueCounters(ctx, jobID); err != nil {
			return nil, err
		}
		result = append(result, jobID)
	}
	return result, nil
}

func (s *Service) buildItemIssueCandidate(ctx context.Context, tx pgx.Tx, job jobSnapshot, item jobItemSnapshot, jobLinks []issueLinkRow, itemLinks []issueLinkRow) (issueCandidate, error) {
	linkContext, err := s.buildLinkContext(ctx, tx, jobLinks, itemLinks)
	if err != nil {
		return issueCandidate{}, err
	}

	detail := strings.TrimSpace(nullableString(item.LatestErrorMessage))
	if detail == "" {
		detail = strings.TrimSpace(nullableString(job.LatestErrorMessage))
	}
	if detail == "" {
		detail = "任务执行失败"
	}

	suggestion, suggestedAction, suggestedActionLabel := deriveSuggestion(linkContext)
	sourceSnapshot := issuedto.SourceContext{
		TaskID:        ptr(job.ID),
		TaskTitle:     ptr(job.Title),
		TaskItemID:    ptr(item.ID),
		TaskItemTitle: ptr(item.Title),
		AssetID:       linkContext.AssetID,
		EntryID:       linkContext.EntryID,
		EndpointID:    linkContext.EndpointID,
		EndpointLabel: linkContext.EndpointLabel,
		Path:          coalesceStringPtr(item.TargetPath, item.SourcePath, linkContext.Path),
		SourceLabel:   linkContext.SourceLabel,
		RouteLabel:    linkContext.RouteLabel,
	}

	return issueCandidate{
		LibraryID:            job.LibraryID,
		IssueCategory:        deriveIssueCategory(job, item),
		IssueType:            deriveIssueType(job, item),
		Nature:               NatureBlocking,
		SourceDomain:         deriveSourceDomain(job),
		Severity:             deriveSeverity(job, deriveIssueCategory(job, item)),
		DedupeKey:            "job-item:" + item.ID,
		Title:                deriveIssueTitle(job, item, linkContext),
		Summary:              detail,
		ObjectLabel:          deriveObjectLabel(linkContext, item, job),
		AssetLabel:           linkContext.AssetLabel,
		SuggestedAction:      suggestedAction,
		SuggestedActionLabel: suggestedActionLabel,
		Suggestion:           suggestion,
		Detail:               ptr(detail),
		SourceSnapshot:       sourceSnapshot,
		ImpactSnapshot:       buildImpactSummary(linkContext, true),
		LatestErrorCode:      item.LatestErrorCode,
		LatestErrorMessage:   ptr(detail),
		Links:                buildIssueLinks(job, &item, linkContext),
	}, nil
}

func (s *Service) buildJobIssueCandidate(ctx context.Context, tx pgx.Tx, job jobSnapshot, jobLinks []issueLinkRow) (issueCandidate, error) {
	linkContext, err := s.buildLinkContext(ctx, tx, jobLinks, nil)
	if err != nil {
		return issueCandidate{}, err
	}

	detail := strings.TrimSpace(nullableString(job.LatestErrorMessage))
	if detail == "" {
		detail = "任务执行失败"
	}
	suggestion, suggestedAction, suggestedActionLabel := deriveSuggestion(linkContext)
	sourceSnapshot := issuedto.SourceContext{
		TaskID:        ptr(job.ID),
		TaskTitle:     ptr(job.Title),
		AssetID:       linkContext.AssetID,
		EntryID:       linkContext.EntryID,
		EndpointID:    linkContext.EndpointID,
		EndpointLabel: linkContext.EndpointLabel,
		Path:          linkContext.Path,
		SourceLabel:   linkContext.SourceLabel,
		RouteLabel:    linkContext.RouteLabel,
	}
	return issueCandidate{
		LibraryID:            job.LibraryID,
		IssueCategory:        deriveIssueCategory(job, jobItemSnapshot{}),
		IssueType:            strings.ToUpper(job.JobIntent) + "_FAILED",
		Nature:               NatureBlocking,
		SourceDomain:         deriveSourceDomain(job),
		Severity:             deriveSeverity(job, deriveIssueCategory(job, jobItemSnapshot{})),
		DedupeKey:            "job:" + job.ID,
		Title:                job.Title + " 执行失败",
		Summary:              detail,
		ObjectLabel:          deriveObjectLabel(linkContext, jobItemSnapshot{}, job),
		AssetLabel:           linkContext.AssetLabel,
		SuggestedAction:      suggestedAction,
		SuggestedActionLabel: suggestedActionLabel,
		Suggestion:           suggestion,
		Detail:               ptr(detail),
		SourceSnapshot:       sourceSnapshot,
		ImpactSnapshot:       buildImpactSummary(linkContext, true),
		LatestErrorCode:      job.LatestErrorCode,
		LatestErrorMessage:   ptr(detail),
		Links:                buildIssueLinks(job, nil, linkContext),
	}, nil
}

func (s *Service) buildLinkContext(ctx context.Context, tx pgx.Tx, jobLinks []issueLinkRow, itemLinks []issueLinkRow) (issueLinkContext, error) {
	result := issueLinkContext{
		AssetIDs:       make(map[string]struct{}),
		ReplicaIDs:     make(map[string]struct{}),
		DirectoryIDs:   make(map[string]struct{}),
		EndpointIDs:    make(map[string]struct{}),
		StorageNodeIDs: make(map[string]struct{}),
	}

	allLinks := append([]issueLinkRow{}, jobLinks...)
	allLinks = append(allLinks, itemLinks...)
	for _, link := range allLinks {
		switch {
		case link.AssetID != nil:
			result.AssetIDs[*link.AssetID] = struct{}{}
			if result.AssetID == nil {
				result.AssetID = link.AssetID
			}
		case link.AssetReplicaID != nil:
			result.ReplicaIDs[*link.AssetReplicaID] = struct{}{}
		case link.DirectoryID != nil:
			result.DirectoryIDs[*link.DirectoryID] = struct{}{}
			if result.DirectoryID == nil {
				result.DirectoryID = link.DirectoryID
			}
		case link.MountID != nil:
			result.EndpointIDs[*link.MountID] = struct{}{}
			if result.EndpointID == nil {
				result.EndpointID = link.MountID
			}
		case link.StorageNodeID != nil:
			result.StorageNodeIDs[*link.StorageNodeID] = struct{}{}
		}
	}

	if result.AssetID != nil {
		var name string
		var relativePath string
		var directoryID string
		if err := tx.QueryRow(ctx, `SELECT name, relative_path, directory_id FROM assets WHERE id = $1`, *result.AssetID).Scan(&name, &relativePath, &directoryID); err == nil {
			result.AssetLabel = ptr(name)
			result.EntryID = result.AssetID
			if result.Path == nil && strings.TrimSpace(relativePath) != "" {
				result.Path = ptr(relativePath)
			}
			if result.DirectoryID == nil && strings.TrimSpace(directoryID) != "" {
				result.DirectoryID = &directoryID
				result.DirectoryIDs[directoryID] = struct{}{}
			}
		}
	}

	if result.DirectoryID != nil {
		var relativePath string
		if err := tx.QueryRow(ctx, `SELECT relative_path FROM library_directories WHERE id = $1`, *result.DirectoryID).Scan(&relativePath); err == nil {
			if result.Path == nil && strings.TrimSpace(relativePath) != "" {
				result.Path = ptr(relativePath)
			}
			if result.EntryID == nil {
				result.EntryID = result.DirectoryID
			}
		}
	}

	if result.EndpointID != nil {
		var name string
		var sourcePath string
		var storageNodeID string
		if err := tx.QueryRow(ctx, `SELECT name, source_path, storage_node_id FROM mounts WHERE id = $1`, *result.EndpointID).Scan(&name, &sourcePath, &storageNodeID); err == nil {
			result.EndpointLabel = ptr(name)
			result.SourceLabel = ptr(name)
			if result.Path == nil && strings.TrimSpace(sourcePath) != "" {
				result.Path = ptr(sourcePath)
			}
			if strings.TrimSpace(storageNodeID) != "" {
				result.StorageNodeIDs[storageNodeID] = struct{}{}
			}
		}
	}

	if len(result.StorageNodeIDs) > 0 {
		for storageNodeID := range result.StorageNodeIDs {
			var name string
			if err := tx.QueryRow(ctx, `SELECT name FROM storage_nodes WHERE id = $1`, storageNodeID).Scan(&name); err == nil && result.SourceLabel == nil {
				result.SourceLabel = ptr(name)
			}
		}
	}

	if result.EndpointLabel != nil {
		result.RouteLabel = result.EndpointLabel
	}

	return result, nil
}

func (s *Service) loadExistingIssuesByDedupe(ctx context.Context, tx pgx.Tx, dedupeKeys []string) (map[string]*issueRow, error) {
	result := make(map[string]*issueRow)
	if len(dedupeKeys) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT
			id, code, library_id, issue_category, issue_type, nature, source_domain, severity, status, dedupe_key,
			title, summary, object_label, asset_label, suggested_action, suggested_action_label, suggestion, detail,
			source_snapshot, impact_snapshot, first_detected_at, last_detected_at, last_status_changed_at,
			resolved_at, archived_at, latest_event_at, latest_error_code, latest_error_message, created_at, updated_at
		FROM issues
		WHERE dedupe_key = ANY($1)
		ORDER BY updated_at DESC
	`, dedupeKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		row, err := scanIssueRow(rows)
		if err != nil {
			return nil, err
		}
		if row.DedupeKey == nil {
			continue
		}
		if _, ok := result[*row.DedupeKey]; !ok {
			current := row
			result[*row.DedupeKey] = &current
		}
	}
	return result, rows.Err()
}

func (s *Service) insertIssue(ctx context.Context, tx pgx.Tx, candidate issueCandidate) error {
	now := s.now().UTC()
	issueID := buildCode("issue-id", now)
	issueCode := buildCode("issue", now)
	sourceSnapshot, err := json.Marshal(candidate.SourceSnapshot)
	if err != nil {
		return err
	}
	impactSnapshot, err := json.Marshal(candidate.ImpactSnapshot)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO issues (
			id, code, library_id, issue_category, issue_type, nature, source_domain, severity, status, dedupe_key,
			title, summary, object_label, asset_label, suggested_action, suggested_action_label, suggestion, detail,
			source_snapshot, impact_snapshot, first_detected_at, last_detected_at, last_status_changed_at,
			latest_event_at, latest_error_code, latest_error_message, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, 'OPEN', $9,
			$10, $11, $12, $13, $14, $15, $16, $17,
			$18, $19, $20, $20, $20,
			$20, $21, $22, $20, $20
		)
	`, issueID, issueCode, candidate.LibraryID, candidate.IssueCategory, candidate.IssueType, candidate.Nature, candidate.SourceDomain, candidate.Severity, candidate.DedupeKey,
		candidate.Title, candidate.Summary, candidate.ObjectLabel, candidate.AssetLabel, candidate.SuggestedAction, candidate.SuggestedActionLabel, candidate.Suggestion, candidate.Detail,
		sourceSnapshot, impactSnapshot, now, candidate.LatestErrorCode, candidate.LatestErrorMessage)
	if err != nil {
		return err
	}

	if err := s.insertIssueEvent(ctx, tx, issueID, issueEventInput{
		EventType:     EventDetected,
		ActorType:     "SYSTEM",
		OperatorLabel: ptr("系统"),
		Message:       ptr("已基于真实任务失败创建异常。"),
		CreatedAt:     now,
	}); err != nil {
		return err
	}

	for _, link := range candidate.Links {
		if err := s.insertIssueLink(ctx, tx, issueID, link, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertExistingIssue(ctx context.Context, tx pgx.Tx, existing issueRow, candidate issueCandidate, job jobSnapshot) error {
	now := s.now().UTC()
	sourceSnapshot, err := json.Marshal(candidate.SourceSnapshot)
	if err != nil {
		return err
	}
	impactSnapshot, err := json.Marshal(candidate.ImpactSnapshot)
	if err != nil {
		return err
	}

	nextStatus := existing.Status
	eventType := EventUpdated
	eventMessage := "已刷新异常上下文。"
	statusChanged := false

	switch existing.Status {
	case StatusResolved, StatusArchived:
		nextStatus = StatusOpen
		eventType = EventAutoReopened
		eventMessage = "异常再次出现，已自动重新打开。"
		statusChanged = true
	case StatusInProgress:
		if job.Status == "FAILED" || job.Status == "PARTIAL_SUCCESS" {
			nextStatus = StatusOpen
			eventType = EventUpdated
			eventMessage = "重试后仍存在失败，异常已回到待处理状态。"
			statusChanged = true
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE issues
		SET library_id = $2,
		    issue_category = $3,
		    issue_type = $4,
		    nature = $5,
		    source_domain = $6,
		    severity = $7,
		    status = $8,
		    title = $9,
		    summary = $10,
		    object_label = $11,
		    asset_label = $12,
		    suggested_action = $13,
		    suggested_action_label = $14,
		    suggestion = $15,
		    detail = $16,
		    source_snapshot = $17,
		    impact_snapshot = $18,
		    last_detected_at = $19,
		    last_status_changed_at = CASE WHEN $20 THEN $19 ELSE last_status_changed_at END,
		    resolved_at = CASE WHEN $8 = 'RESOLVED' THEN COALESCE(resolved_at, $19) ELSE NULL END,
		    archived_at = CASE WHEN $8 = 'ARCHIVED' THEN COALESCE(archived_at, $19) ELSE NULL END,
		    latest_event_at = $19,
		    latest_error_code = $21,
		    latest_error_message = $22,
		    updated_at = $19
		WHERE id = $1
	`, existing.ID, candidate.LibraryID, candidate.IssueCategory, candidate.IssueType, candidate.Nature, candidate.SourceDomain, candidate.Severity, nextStatus,
		candidate.Title, candidate.Summary, candidate.ObjectLabel, candidate.AssetLabel, candidate.SuggestedAction, candidate.SuggestedActionLabel, candidate.Suggestion, candidate.Detail,
		sourceSnapshot, impactSnapshot, now, statusChanged, candidate.LatestErrorCode, candidate.LatestErrorMessage)
	if err != nil {
		return err
	}

	if err := s.insertIssueEvent(ctx, tx, existing.ID, issueEventInput{
		EventType:     eventType,
		FromStatus:    statusMaybePtr(existing.Status),
		ToStatus:      statusMaybePtr(nextStatus),
		ActorType:     "SYSTEM",
		OperatorLabel: ptr("系统"),
		Message:       ptr(eventMessage),
		CreatedAt:     now,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) resolveNoLongerDetectedIssues(ctx context.Context, tx pgx.Tx, job jobSnapshot, desired map[string]struct{}) error {
	rows, err := tx.Query(ctx, `
		SELECT
			id, code, library_id, issue_category, issue_type, nature, source_domain, severity, status, dedupe_key,
			title, summary, object_label, asset_label, suggested_action, suggested_action_label, suggestion, detail,
			source_snapshot, impact_snapshot, first_detected_at, last_detected_at, last_status_changed_at,
			resolved_at, archived_at, latest_event_at, latest_error_code, latest_error_message, created_at, updated_at
		FROM issues
		WHERE COALESCE(source_snapshot ->> 'taskId', '') = $1
		FOR UPDATE
	`, job.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := s.now().UTC()
	for rows.Next() {
		row, err := scanIssueRow(rows)
		if err != nil {
			return err
		}
		if row.DedupeKey != nil {
			if _, ok := desired[*row.DedupeKey]; ok {
				continue
			}
		}
		if row.Status == StatusIgnored || row.Status == StatusResolved || row.Status == StatusArchived {
			continue
		}
		if row.Status == StatusInProgress && (job.Status == "WAITING_RETRY" || job.Status == "QUEUED" || job.Status == "RUNNING" || job.Status == "PENDING") {
			continue
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
			return err
		}
		if err := s.insertIssueEvent(ctx, tx, row.ID, issueEventInput{
			EventType:     EventResolved,
			FromStatus:    ptr(row.Status),
			ToStatus:      ptr(StatusResolved),
			ActorType:     "SYSTEM",
			OperatorLabel: ptr("系统"),
			Message:       ptr("关联问题已消失，异常已自动解决。"),
			CreatedAt:     now,
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) refreshJobIssueCounters(ctx context.Context, jobID string) error {
	activeStatuses := []string{StatusOpen, StatusAwaitingConfirmation, StatusInProgress}

	var issueCount int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM issues
		WHERE COALESCE(source_snapshot ->> 'taskId', '') = $1
		  AND status = ANY($2)
	`, jobID, activeStatuses).Scan(&issueCount); err != nil {
		return err
	}

	var latestErrorCode *string
	var latestErrorMessage *string
	_ = s.pool.QueryRow(ctx, `
		SELECT latest_error_code, latest_error_message
		FROM issues
		WHERE COALESCE(source_snapshot ->> 'taskId', '') = $1
		  AND status = ANY($2)
		ORDER BY latest_event_at DESC NULLS LAST, updated_at DESC
		LIMIT 1
	`, jobID, activeStatuses).Scan(&latestErrorCode, &latestErrorMessage)

	if _, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET issue_count = $2,
		    latest_error_code = $3,
		    latest_error_message = $4,
		    updated_at = $5
		WHERE id = $1
	`, jobID, issueCount, latestErrorCode, latestErrorMessage, s.now().UTC()); err != nil {
		return err
	}

	rows, err := s.pool.Query(ctx, `SELECT id FROM job_items WHERE job_id = $1`, jobID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return err
		}
		var itemIssueCount int
		if err := s.pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM issues
			WHERE COALESCE(source_snapshot ->> 'taskItemId', '') = $1
			  AND status = ANY($2)
		`, itemID, activeStatuses).Scan(&itemIssueCount); err != nil {
			return err
		}

		var itemErrorCode *string
		var itemErrorMessage *string
		_ = s.pool.QueryRow(ctx, `
			SELECT latest_error_code, latest_error_message
			FROM issues
			WHERE COALESCE(source_snapshot ->> 'taskItemId', '') = $1
			  AND status = ANY($2)
			ORDER BY latest_event_at DESC NULLS LAST, updated_at DESC
			LIMIT 1
		`, itemID, activeStatuses).Scan(&itemErrorCode, &itemErrorMessage)

		if _, err := s.pool.Exec(ctx, `
			UPDATE job_items
			SET issue_count = $2,
			    latest_error_code = $3,
			    latest_error_message = $4,
			    updated_at = $5
			WHERE id = $1
		`, itemID, itemIssueCount, itemErrorCode, itemErrorMessage, s.now().UTC()); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) loadJobSnapshot(ctx context.Context, tx pgx.Tx, jobID string) (jobSnapshot, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, library_id, job_family, job_intent, source_domain, title, latest_error_code, latest_error_message, status
		FROM jobs
		WHERE id = $1
		FOR UPDATE
	`, jobID)
	var item jobSnapshot
	if err := row.Scan(&item.ID, &item.LibraryID, &item.JobFamily, &item.JobIntent, &item.SourceDomain, &item.Title, &item.LatestErrorCode, &item.LatestErrorMessage, &item.Status); err != nil {
		return jobSnapshot{}, err
	}
	return item, nil
}

func (s *Service) loadJobItems(ctx context.Context, tx pgx.Tx, jobID string) ([]jobItemSnapshot, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, title, status, source_path, target_path, latest_error_code, latest_error_message
		FROM job_items
		WHERE job_id = $1
		ORDER BY created_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]jobItemSnapshot, 0)
	for rows.Next() {
		var item jobItemSnapshot
		if err := rows.Scan(&item.ID, &item.Title, &item.Status, &item.SourcePath, &item.TargetPath, &item.LatestErrorCode, &item.LatestErrorMessage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadLinks(ctx context.Context, tx pgx.Tx, condition string, arg string) ([]issueLinkRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT link_role, object_type, asset_id, asset_replica_id, directory_id, mount_id, storage_node_id
		FROM job_object_links
		WHERE `+condition+`
	`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]issueLinkRow, 0)
	for rows.Next() {
		var item issueLinkRow
		if err := rows.Scan(&item.LinkRole, &item.ObjectType, &item.AssetID, &item.AssetReplicaID, &item.DirectoryID, &item.MountID, &item.StorageNodeID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanIssueRow(row pgx.Row) (issueRow, error) {
	var item issueRow
	err := row.Scan(
		&item.ID, &item.Code, &item.LibraryID, &item.IssueCategory, &item.IssueType, &item.Nature, &item.SourceDomain, &item.Severity, &item.Status, &item.DedupeKey,
		&item.Title, &item.Summary, &item.ObjectLabel, &item.AssetLabel, &item.SuggestedAction, &item.SuggestedActionLabel, &item.Suggestion, &item.Detail,
		&item.SourceSnapshot, &item.ImpactSnapshot, &item.FirstDetectedAt, &item.LastDetectedAt, &item.LastStatusChangedAt,
		&item.ResolvedAt, &item.ArchivedAt, &item.LatestEventAt, &item.LatestErrorCode, &item.LatestErrorMessage, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return issueRow{}, apperrors.NotFound("异常不存在")
	}
	return item, err
}
