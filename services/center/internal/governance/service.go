package governance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/jobs"
	jobdto "mare/shared/contracts/dto/job"
)

const defaultScheduledScanInterval = time.Hour

type JobService interface {
	CreateJob(ctx context.Context, input jobs.CreateJobInput) (jobdto.CreateResponse, error)
}

type Options struct {
	JobService            JobService
	TickInterval          time.Duration
	ScheduledScanInterval time.Duration
	MissingReplicaSyncer  MissingReplicaSyncer
}

type MissingReplicaSyncer interface {
	SyncMissingReplicaIssues(ctx context.Context) error
}

type Service struct {
	pool                  *pgxpool.Pool
	jobService            JobService
	tickInterval          time.Duration
	scheduledScanInterval time.Duration
	missingReplicaSyncer  MissingReplicaSyncer
	now                   func() time.Time
	startOnce             sync.Once
}

type dueMountScan struct {
	MountID          string
	MountName        string
	LibraryID        string
	LibraryName      string
	RelativeRootPath string
	SourcePath       string
	ScanPolicy       string
	NextScanAt       *time.Time
}

type dueMountHeartbeat struct {
	MountID          string
	MountName        string
	LibraryID        string
	LibraryName      string
	RelativeRootPath string
	SourcePath       string
	HeartbeatPolicy  string
}

type AuditRecord struct {
	ID         string
	ActionKind string
	SubjectType string
	SubjectID  string
	Reason     string
	CreatedAt  time.Time
}

func NewService(pool *pgxpool.Pool, options Options) *Service {
	interval := options.TickInterval
	if interval <= 0 {
		interval = time.Second
	}
	scanInterval := options.ScheduledScanInterval
	if scanInterval <= 0 {
		scanInterval = defaultScheduledScanInterval
	}
	return &Service{
		pool:                  pool,
		jobService:            options.JobService,
		tickInterval:          interval,
		scheduledScanInterval: scanInterval,
		missingReplicaSyncer:  options.MissingReplicaSyncer,
		now:                   time.Now,
	}
}

func (s *Service) SetNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *Service) SetMissingReplicaSyncer(syncer MissingReplicaSyncer) {
	s.missingReplicaSyncer = syncer
}

func (s *Service) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		go s.run(ctx)
	})
}

func (s *Service) run(ctx context.Context) {
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.Tick(ctx)
		}
	}
}

func (s *Service) Tick(ctx context.Context) error {
	if s.jobService != nil {
		for {
			mount, err := s.claimDueMountScan(ctx)
			if err != nil {
				return err
			}
		if mount == nil {
			break
		}
		_ = s.appendAudit(ctx, "SCHEDULE_SCAN", "MOUNT", mount.MountID, "挂载已到定时扫描时间，准备创建扫描作业", map[string]any{
			"mountName": mount.MountName,
		})
		if err := s.createScheduledMountScanJob(ctx, *mount); err != nil {
			return err
		}
		}
		for {
			mount, err := s.claimDueMountHeartbeat(ctx)
			if err != nil {
				return err
			}
			if mount == nil {
				break
			}
			_ = s.appendAudit(ctx, "SCHEDULE_HEARTBEAT", "MOUNT", mount.MountID, "挂载已到心跳巡检时间，准备创建连接测试作业", map[string]any{
				"mountName": mount.MountName,
			})
			if err := s.createScheduledMountHeartbeatJob(ctx, *mount); err != nil {
				return err
			}
		}
	}
	if s.missingReplicaSyncer != nil {
		if err := s.missingReplicaSyncer.SyncMissingReplicaIssues(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) claimDueMountScan(ctx context.Context) (*dueMountScan, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT
			m.id,
			m.name,
			m.library_id,
			m.library_name,
			m.relative_root_path,
			m.source_path,
			m.scan_policy,
			mr.next_scan_at
		FROM mounts m
		INNER JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.deleted_at IS NULL
		  AND m.enabled = TRUE
		  AND (
			(m.scan_policy = 'SCHEDULED' AND mr.next_scan_at IS NOT NULL AND mr.next_scan_at <= $1)
			OR
			(m.scan_policy = 'ON_START' AND mr.last_scan_at IS NULL)
		  )
		ORDER BY COALESCE(mr.next_scan_at, mr.created_at) ASC
		FOR UPDATE OF mr SKIP LOCKED
		LIMIT 1
	`, now)

	var mount dueMountScan
	if err := row.Scan(
		&mount.MountID,
		&mount.MountName,
		&mount.LibraryID,
		&mount.LibraryName,
		&mount.RelativeRootPath,
		&mount.SourcePath,
		&mount.ScanPolicy,
		&mount.NextScanAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	active, err := hasActiveMountScanJob(ctx, tx, mount.MountID)
	if err != nil {
		return nil, err
	}

	nextScanAt := computeNextScanAt(now, mount.ScanPolicy, s.scheduledScanInterval)
	if active {
		if _, err := tx.Exec(ctx, `
			UPDATE mount_runtime
			SET next_scan_at = $2,
			    updated_at = $3
			WHERE mount_id = $1
		`, mount.MountID, nextScanAt, now); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE mount_runtime
		SET next_scan_at = $2,
		    updated_at = $3
		WHERE mount_id = $1
	`, mount.MountID, nextScanAt, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &mount, nil
}

func hasActiveMountScanJob(ctx context.Context, tx pgx.Tx, mountID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM jobs j
			INNER JOIN job_object_links jol ON jol.job_id = j.id
			WHERE j.job_intent = 'SCAN_DIRECTORY'
			  AND j.source_domain = 'SCHEDULED'
			  AND j.status IN ('PENDING', 'QUEUED', 'RUNNING', 'WAITING_RETRY', 'PAUSED')
			  AND jol.object_type = 'MOUNT'
			  AND jol.link_role = 'TARGET_MOUNT'
			  AND jol.mount_id = $1
		)
	`, mountID).Scan(&exists)
	return exists, err
}

func (s *Service) claimDueMountHeartbeat(ctx context.Context) (*dueMountHeartbeat, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT
			m.id,
			m.name,
			m.library_id,
			m.library_name,
			m.relative_root_path,
			m.source_path,
			m.heartbeat_policy
		FROM mounts m
		INNER JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.deleted_at IS NULL
		  AND m.enabled = TRUE
		  AND m.heartbeat_policy <> 'NEVER'
		  AND mr.next_heartbeat_at IS NOT NULL
		  AND mr.next_heartbeat_at <= $1
		ORDER BY mr.next_heartbeat_at ASC
		FOR UPDATE OF mr SKIP LOCKED
		LIMIT 1
	`, now)

	var mount dueMountHeartbeat
	if err := row.Scan(
		&mount.MountID,
		&mount.MountName,
		&mount.LibraryID,
		&mount.LibraryName,
		&mount.RelativeRootPath,
		&mount.SourcePath,
		&mount.HeartbeatPolicy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	active, err := hasActiveMountHeartbeatJob(ctx, tx, mount.MountID)
	if err != nil {
		return nil, err
	}

	nextHeartbeatAt := computeNextHeartbeat(now, mount.HeartbeatPolicy)
	if _, err := tx.Exec(ctx, `
		UPDATE mount_runtime
		SET next_heartbeat_at = $2,
		    updated_at = $3
		WHERE mount_id = $1
	`, mount.MountID, nextHeartbeatAt, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if active {
		return nil, nil
	}
	return &mount, nil
}

func hasActiveMountHeartbeatJob(ctx context.Context, tx pgx.Tx, mountID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM jobs j
			INNER JOIN job_object_links jol ON jol.job_id = j.id
			WHERE j.job_intent = 'CONNECTION_TEST'
			  AND j.source_domain = 'SCHEDULED'
			  AND j.status IN ('PENDING', 'QUEUED', 'RUNNING', 'WAITING_RETRY', 'PAUSED')
			  AND jol.object_type = 'MOUNT'
			  AND jol.link_role = 'TARGET_MOUNT'
			  AND jol.mount_id = $1
		)
	`, mountID).Scan(&exists)
	return exists, err
}

func (s *Service) createScheduledMountScanJob(ctx context.Context, mount dueMountScan) error {
	libraryID := mount.LibraryID
	mountID := mount.MountID
	input := jobs.CreateJobInput{
		LibraryID:     &libraryID,
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         fmt.Sprintf("定时扫描：%s", mount.MountName),
		Summary:       "后台治理调度触发挂载扫描",
		SourceDomain:  jobs.SourceDomainScheduled,
		SourceSnapshot: map[string]any{
			"mountId":      mount.MountID,
			"mountName":    mount.MountName,
			"libraryName":  mount.LibraryName,
			"relativePath": normalizedRelativePath(mount.RelativeRootPath),
			"sourcePath":   mount.SourcePath,
		},
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedBySystem,
		Items: []jobs.CreateItemInput{
			{
				ItemKey:    "mount:" + mount.MountID,
				ItemType:   jobs.ItemTypeDirectoryScan,
				Title:      fmt.Sprintf("扫描挂载：%s", mount.MountName),
				Summary:    "后台调度扫描挂载目录",
				TargetPath: &mount.SourcePath,
				Links: []jobs.CreateObjectLinkInput{
					{
						LinkRole:   jobs.LinkRoleTargetMount,
						ObjectType: jobs.ObjectTypeMount,
						MountID:    &mountID,
					},
				},
			},
		},
		Links: []jobs.CreateObjectLinkInput{
			{
				LinkRole:   jobs.LinkRoleTargetMount,
				ObjectType: jobs.ObjectTypeMount,
				MountID:    &mountID,
			},
		},
	}
	_, err := s.jobService.CreateJob(ctx, input)
	return err
}

func (s *Service) createScheduledMountHeartbeatJob(ctx context.Context, mount dueMountHeartbeat) error {
	libraryID := mount.LibraryID
	mountID := mount.MountID
	input := jobs.CreateJobInput{
		LibraryID:     &libraryID,
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentConnectionTest,
		Title:         fmt.Sprintf("心跳巡检：%s", mount.MountName),
		Summary:       "后台治理调度触发挂载健康检查",
		SourceDomain:  jobs.SourceDomainScheduled,
		SourceSnapshot: map[string]any{
			"mountId":      mount.MountID,
			"mountName":    mount.MountName,
			"libraryName":  mount.LibraryName,
			"relativePath": normalizedRelativePath(mount.RelativeRootPath),
			"sourcePath":   mount.SourcePath,
			"sourceLabel":  mount.MountName,
		},
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedBySystem,
		Items: []jobs.CreateItemInput{
			{
				ItemKey: "mount:" + mount.MountID + ":heartbeat",
				ItemType: jobs.ItemTypeConnectivityCheck,
				Title:   fmt.Sprintf("巡检挂载：%s", mount.MountName),
				Summary: "后台调度执行挂载连接测试",
				Links: []jobs.CreateObjectLinkInput{
					{
						LinkRole:   jobs.LinkRoleTargetMount,
						ObjectType: jobs.ObjectTypeMount,
						MountID:    &mountID,
					},
				},
			},
		},
		Links: []jobs.CreateObjectLinkInput{
			{
				LinkRole:   jobs.LinkRoleTargetMount,
				ObjectType: jobs.ObjectTypeMount,
				MountID:    &mountID,
			},
		},
	}
	_, err := s.jobService.CreateJob(ctx, input)
	return err
}

func computeNextHeartbeat(now time.Time, policy string) *time.Time {
	switch strings.TrimSpace(policy) {
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

func computeNextScanAt(now time.Time, policy string, interval time.Duration) *time.Time {
	switch strings.TrimSpace(policy) {
	case "SCHEDULED":
		next := now.Add(interval)
		return &next
	default:
		return nil
	}
}

func normalizedRelativePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if value == "/" {
		return value
	}
	return "/" + strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
}

func (s *Service) appendAudit(ctx context.Context, actionKind string, subjectType string, subjectID string, reason string, payload map[string]any) error {
	if s.pool == nil {
		return nil
	}
	var payloadJSON any
	if len(payload) > 0 {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadJSON = raw
	}
	now := s.now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO governance_audit_logs (
			id, action_kind, subject_type, subject_id, reason, payload_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, buildCode("governance-audit", now), actionKind, subjectType, subjectID, reason, payloadJSON, now)
	return err
}

func (s *Service) ListAuditLogs(ctx context.Context) ([]AuditRecord, error) {
	if s.pool == nil {
		return []AuditRecord{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, action_kind, subject_type, subject_id, reason, created_at
		FROM governance_audit_logs
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditRecord, 0)
	for rows.Next() {
		var item AuditRecord
		if err := rows.Scan(&item.ID, &item.ActionKind, &item.SubjectType, &item.SubjectID, &item.Reason, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func buildCode(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%d", strings.ToLower(prefix), now.UnixNano())
}
