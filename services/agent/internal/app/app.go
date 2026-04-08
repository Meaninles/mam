package app

import (
	"context"
	"time"

	agentdto "mare/shared/contracts/dto/agent"
)

type CenterClient interface {
	Register(ctx context.Context, payload agentdto.RegisterRequest) error
	Heartbeat(ctx context.Context, payload agentdto.HeartbeatRequest) error
}

type Options struct {
	AgentID           string
	Version           string
	Hostname          string
	Platform          string
	Mode              string
	ProcessID         int64
	HeartbeatInterval time.Duration
	RetryDelay        time.Duration
}

type Runner struct {
	client  CenterClient
	options Options
}

func NewRunner(client CenterClient, options Options) *Runner {
	return &Runner{
		client:  client,
		options: options,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	registerPayload := agentdto.RegisterRequest{
		AgentID:   r.options.AgentID,
		Version:   r.options.Version,
		Hostname:  r.options.Hostname,
		Platform:  r.options.Platform,
		Mode:      r.options.Mode,
		ProcessID: r.options.ProcessID,
	}
	heartbeatPayload := agentdto.HeartbeatRequest{
		AgentID:   r.options.AgentID,
		Version:   r.options.Version,
		Hostname:  r.options.Hostname,
		Platform:  r.options.Platform,
		Mode:      r.options.Mode,
		ProcessID: r.options.ProcessID,
	}

	for {
		if err := r.client.Register(ctx, registerPayload); err != nil {
			if !sleepWithContext(ctx, r.options.RetryDelay) {
				return nil
			}
			continue
		}
		break
	}

	ticker := time.NewTicker(r.options.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.client.Heartbeat(ctx, heartbeatPayload); err != nil {
				if !sleepWithContext(ctx, r.options.RetryDelay) {
					return nil
				}
			}
		}
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
