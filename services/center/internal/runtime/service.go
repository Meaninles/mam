package runtime

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/agentregistry"
	"mare/services/center/internal/db"
)

type Service struct {
	serviceName      string
	serviceVersion   string
	startedAt        time.Time
	heartbeatTimeout time.Duration
	pool             *pgxpool.Pool
	migrator         db.Migrator
	agents           *agentregistry.Service
	now              func() time.Time
}

type HealthPayload struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

type ComponentStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ReadinessPayload struct {
	Status    string            `json:"status"`
	Service   ComponentStatus   `json:"service"`
	Database  ComponentStatus   `json:"database"`
	Migration db.MigrationState `json:"migration"`
	Version   string            `json:"version"`
	Timestamp string            `json:"timestamp"`
}

type ServiceRuntimeStatus struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
}

type AgentRuntimeStatus struct {
	AgentID         string `json:"agentId"`
	Version         string `json:"version"`
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	Mode            string `json:"mode"`
	ProcessID       int64  `json:"processId"`
	Status          string `json:"status"`
	RegisteredAt    string `json:"registeredAt"`
	LastHeartbeatAt string `json:"lastHeartbeatAt"`
}

type RuntimeStatusPayload struct {
	Status    string               `json:"status"`
	Service   ServiceRuntimeStatus `json:"service"`
	Database  ComponentStatus      `json:"database"`
	Migration db.MigrationState    `json:"migration"`
	Agents    []AgentRuntimeStatus `json:"agents"`
	Timestamp string               `json:"timestamp"`
}

func NewService(
	serviceName string,
	serviceVersion string,
	startedAt time.Time,
	heartbeatTimeout time.Duration,
	pool *pgxpool.Pool,
	migrator db.Migrator,
	agents *agentregistry.Service,
) *Service {
	return &Service{
		serviceName:      serviceName,
		serviceVersion:   serviceVersion,
		startedAt:        startedAt.UTC(),
		heartbeatTimeout: heartbeatTimeout,
		pool:             pool,
		migrator:         migrator,
		agents:           agents,
		now:              time.Now,
	}
}

func (s *Service) Health() HealthPayload {
	return HealthPayload{
		Status:    "up",
		Service:   s.serviceName,
		Version:   s.serviceVersion,
		Timestamp: s.now().UTC().Format(time.RFC3339),
	}
}

func (s *Service) Ready(ctx context.Context) (ReadinessPayload, error) {
	databaseStatus := ComponentStatus{
		Status:  "up",
		Message: "数据库连接正常",
	}
	if err := s.pool.Ping(ctx); err != nil {
		databaseStatus = ComponentStatus{
			Status:  "down",
			Message: "数据库连接失败",
		}
	}

	migrationState, err := s.migrator.State(ctx, s.pool)
	if err != nil {
		migrationState = db.MigrationState{
			Status: "error",
		}
	}

	status := "ready"
	if databaseStatus.Status != "up" || migrationState.Status != "ready" {
		status = "not_ready"
	}

	return ReadinessPayload{
		Status: status,
		Service: ComponentStatus{
			Status:  "up",
			Message: "中心服务已启动",
		},
		Database:  databaseStatus,
		Migration: migrationState,
		Version:   s.serviceVersion,
		Timestamp: s.now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) Status(ctx context.Context) (RuntimeStatusPayload, error) {
	ready, _ := s.Ready(ctx)
	agents, err := s.agents.Snapshot(ctx)
	if err != nil {
		return RuntimeStatusPayload{}, err
	}

	now := s.now().UTC()
	agentStatuses := make([]AgentRuntimeStatus, 0, len(agents))
	for _, agent := range agents {
		status := "online"
		if now.Sub(agent.LastHeartbeatAt.UTC()) > s.heartbeatTimeout {
			status = "heartbeat_timeout"
		}

		agentStatuses = append(agentStatuses, AgentRuntimeStatus{
			AgentID:         agent.AgentID,
			Version:         agent.Version,
			Hostname:        agent.Hostname,
			Platform:        agent.Platform,
			Mode:            agent.Mode,
			ProcessID:       agent.ProcessID,
			Status:          status,
			RegisteredAt:    agent.RegisteredAt.UTC().Format(time.RFC3339),
			LastHeartbeatAt: agent.LastHeartbeatAt.UTC().Format(time.RFC3339),
		})
	}

	return RuntimeStatusPayload{
		Status: ready.Status,
		Service: ServiceRuntimeStatus{
			Name:      s.serviceName,
			Version:   s.serviceVersion,
			Status:    "up",
			StartedAt: s.startedAt.Format(time.RFC3339),
		},
		Database:  ready.Database,
		Migration: ready.Migration,
		Agents:    agentStatuses,
		Timestamp: now.Format(time.RFC3339),
	}, nil
}
