package importing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	importdto "mare/shared/contracts/dto/importing"
)

type HTTPAgentBridge struct {
	client *http.Client
}

func NewHTTPAgentBridge(timeout time.Duration) *HTTPAgentBridge {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPAgentBridge{
		client: &http.Client{Timeout: timeout},
	}
}

func (b *HTTPAgentBridge) DiscoverSources(ctx context.Context, callbackBaseURL string) ([]importdto.SourceDescriptor, error) {
	var response importdto.DiscoverSourcesResponse
	if err := b.post(ctx, callbackBaseURL, "/api/import/sources/discover", nil, &response); err != nil {
		return nil, err
	}
	return response.Sources, nil
}

func (b *HTTPAgentBridge) BrowseSource(ctx context.Context, callbackBaseURL string, request importdto.BrowseRequest) (importdto.BrowseResponse, error) {
	var response importdto.BrowseResponse
	if err := b.post(ctx, callbackBaseURL, "/api/import/sources/browse", request, &response); err != nil {
		return importdto.BrowseResponse{}, err
	}
	return response, nil
}

func (b *HTTPAgentBridge) ExecuteImport(ctx context.Context, callbackBaseURL string, request importdto.ExecuteImportRequest) (importdto.ExecuteImportResponse, error) {
	var response importdto.ExecuteImportResponse
	if err := b.post(ctx, callbackBaseURL, "/api/import/execute", request, &response); err != nil {
		return importdto.ExecuteImportResponse{}, err
	}
	return response, nil
}

func (b *HTTPAgentBridge) post(ctx context.Context, callbackBaseURL string, path string, payload any, target any) error {
	endpoint := strings.TrimRight(strings.TrimSpace(callbackBaseURL), "/") + path
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return err
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := b.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var body map[string]map[string]string
		if err := json.NewDecoder(response.Body).Decode(&body); err == nil {
			if message := body["error"]["message"]; strings.TrimSpace(message) != "" {
				return fmt.Errorf("%s", message)
			}
		}
		return fmt.Errorf("agent http %d", response.StatusCode)
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Data) == 0 || target == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, target)
}
