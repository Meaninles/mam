package app

import (
	"context"
	"sync"
	"testing"
	"time"

	agentdto "mare/shared/contracts/dto/agent"
)

type fakeCenterClient struct {
	mu             sync.Mutex
	registerCalls  int
	heartbeatCalls int
	registerErr    error
	heartbeatErr   error
}

func (f *fakeCenterClient) Register(_ context.Context, _ agentdto.RegisterRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerCalls++
	return f.registerErr
}

func (f *fakeCenterClient) Heartbeat(_ context.Context, _ agentdto.HeartbeatRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heartbeatCalls++
	return f.heartbeatErr
}

func TestRunnerRegistersAndHeartbeats(t *testing.T) {
	client := &fakeCenterClient{}
	runner := NewRunner(client, Options{
		AgentID:           "agent-dev-1",
		Version:           "dev",
		Hostname:          "工作站-A",
		Platform:          "windows/amd64",
		Mode:              "attached",
		ProcessID:         1024,
		HeartbeatInterval: 20 * time.Millisecond,
		RetryDelay:        10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Millisecond)
	defer cancel()

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("run runner: %v", err)
	}

	if client.registerCalls < 1 {
		t.Fatalf("expected register call, got %d", client.registerCalls)
	}
	if client.heartbeatCalls < 1 {
		t.Fatalf("expected heartbeat call, got %d", client.heartbeatCalls)
	}
}

func TestRunnerRetriesRegisterWithoutCrashing(t *testing.T) {
	client := &fakeCenterClient{registerErr: context.DeadlineExceeded}
	runner := NewRunner(client, Options{
		AgentID:           "agent-dev-1",
		Version:           "dev",
		Hostname:          "工作站-A",
		Platform:          "windows/amd64",
		Mode:              "attached",
		ProcessID:         1024,
		HeartbeatInterval: 50 * time.Millisecond,
		RetryDelay:        10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("run runner: %v", err)
	}

	if client.registerCalls < 2 {
		t.Fatalf("expected retry register calls, got %d", client.registerCalls)
	}
	if client.heartbeatCalls != 0 {
		t.Fatalf("expected no heartbeat before successful register, got %d", client.heartbeatCalls)
	}
}
