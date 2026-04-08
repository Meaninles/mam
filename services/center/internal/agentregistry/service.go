package agentregistry

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool: pool,
		now:  time.Now,
	}
}

func (s *Service) Register(ctx context.Context, registration Registration) (Agent, error) {
	if err := validateAgentIdentity(registration.AgentID, registration.Hostname, registration.Platform, registration.Mode); err != nil {
		return Agent{}, err
	}

	now := s.now().UTC()
	row := s.pool.QueryRow(ctx, `
		INSERT INTO agents (
			agent_id, version, hostname, platform, mode, process_id, registered_at, last_heartbeat_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (agent_id) DO UPDATE SET
			version = EXCLUDED.version,
			hostname = EXCLUDED.hostname,
			platform = EXCLUDED.platform,
			mode = EXCLUDED.mode,
			process_id = EXCLUDED.process_id,
			registered_at = EXCLUDED.registered_at,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at
		RETURNING agent_id, version, hostname, platform, mode, process_id, registered_at, last_heartbeat_at
	`,
		registration.AgentID,
		registration.Version,
		registration.Hostname,
		registration.Platform,
		registration.Mode,
		registration.ProcessID,
		now,
	)

	return scanAgent(row)
}

func (s *Service) Heartbeat(ctx context.Context, heartbeat Heartbeat) (Agent, error) {
	if err := validateAgentIdentity(heartbeat.AgentID, heartbeat.Hostname, heartbeat.Platform, heartbeat.Mode); err != nil {
		return Agent{}, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE agents
		SET version = $2,
		    hostname = $3,
		    platform = $4,
		    mode = $5,
		    process_id = $6,
		    last_heartbeat_at = $7
		WHERE agent_id = $1
		RETURNING agent_id, version, hostname, platform, mode, process_id, registered_at, last_heartbeat_at
	`,
		heartbeat.AgentID,
		heartbeat.Version,
		heartbeat.Hostname,
		heartbeat.Platform,
		heartbeat.Mode,
		heartbeat.ProcessID,
		s.now().UTC(),
	)

	agent, err := scanAgent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return Agent{}, apperrors.NotFound("执行器尚未注册")
		}
		return Agent{}, err
	}

	return agent, nil
}

func (s *Service) Snapshot(ctx context.Context) ([]Agent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT agent_id, version, hostname, platform, mode, process_id, registered_at, last_heartbeat_at
		FROM agents
		ORDER BY agent_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return agents, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAgent(row scanner) (Agent, error) {
	var agent Agent
	if err := row.Scan(
		&agent.AgentID,
		&agent.Version,
		&agent.Hostname,
		&agent.Platform,
		&agent.Mode,
		&agent.ProcessID,
		&agent.RegisteredAt,
		&agent.LastHeartbeatAt,
	); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func validateAgentIdentity(agentID string, hostname string, platform string, mode string) error {
	if strings.TrimSpace(agentID) == "" {
		return apperrors.BadRequest("agentId 不能为空")
	}
	if strings.TrimSpace(hostname) == "" {
		return apperrors.BadRequest("hostname 不能为空")
	}
	if strings.TrimSpace(platform) == "" {
		return apperrors.BadRequest("platform 不能为空")
	}
	if strings.TrimSpace(mode) == "" {
		return apperrors.BadRequest("mode 不能为空")
	}
	return nil
}
