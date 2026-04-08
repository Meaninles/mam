package centerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	agentdto "mare/shared/contracts/dto/agent"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Register(ctx context.Context, payload agentdto.RegisterRequest) error {
	return c.post(ctx, "/agent/register", payload)
}

func (c *Client) Heartbeat(ctx context.Context, payload agentdto.HeartbeatRequest) error {
	return c.post(ctx, "/agent/heartbeat", payload)
}

func (c *Client) post(ctx context.Context, path string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		var envelope errorEnvelope
		if err := json.NewDecoder(response.Body).Decode(&envelope); err == nil && strings.TrimSpace(envelope.Error.Message) != "" {
			return fmt.Errorf("%s", envelope.Error.Message)
		}
		return fmt.Errorf("center service returned status %d", response.StatusCode)
	}

	return nil
}
