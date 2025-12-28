package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"draw/pkg/llm/prompts"
)

// NvidiaLLMClient calls Nvidia's Chat Completions API to generate whiteboard updates.
type NvidiaLLMClient struct {
	httpClient  *http.Client
	baseURL     string
	model       string
	apiKey      string
	requestChan chan llmRequest
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
}

func NewNvidiaLLMClient(baseURL, model, apiKey string) (*NvidiaLLMClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("nvidia api key is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("nvidia model is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://integrate.api.nvidia.com/v1/chat/completions"
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &NvidiaLLMClient{
		httpClient:  &http.Client{Timeout: 25 * time.Second},
		baseURL:     baseURL,
		model:       model,
		apiKey:      apiKey,
		requestChan: make(chan llmRequest, 10),
		ctx:         ctx,
		cancel:      cancel,
	}

	go client.worker()

	return client, nil
}

func (c *NvidiaLLMClient) worker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case req := <-c.requestChan:
			result, err := c.generateResponseSync(req.prompt, req.systemPrompt)
			if err != nil {
				req.errCh <- err
			} else {
				req.resultCh <- result
			}
		}
	}
}

func (c *NvidiaLLMClient) GenerateResponse(ctx context.Context, prompt string, boardState string) (*LLMResponse, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty text provided")
	}

	boardStateJSON := boardState
	if boardState == "" {
		boardStateJSON = "[]"
	} else {
		var js json.RawMessage
		if err := json.Unmarshal([]byte(boardState), &js); err != nil {
			boardStateJSON = "[]"
		}
	}

	userPrompt := prompts.BuildWhiteboardPrompt(prompt, boardStateJSON)
	systemPrompt := prompts.WhiteboardSystemPrompt

	resultCh := make(chan *LLMResponse, 1)
	errCh := make(chan error, 1)

	select {
	case c.requestChan <- llmRequest{
		prompt:       userPrompt,
		systemPrompt: systemPrompt,
		resultCh:     resultCh,
		errCh:        errCh,
	}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *NvidiaLLMClient) generateResponseSync(prompt string, systemPrompt string) (*LLMResponse, error) {
	messages := []nvidiaChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}
	if systemPrompt != "" {
		messages = append([]nvidiaChatMessage{{
			Role:    "system",
			Content: systemPrompt,
		}}, messages...)
	}

	payload := nvidiaChatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   1024,
		Temperature: 0.2,
		TopP:        0.9,
		Stream:      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nvidia request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(c.ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create nvidia request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nvidia api request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("nvidia api error: status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var chatResp nvidiaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode nvidia response: %w", err)
	}

	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("nvidia api returned empty response")
	}

	responseText := strings.TrimSpace(chatResp.Choices[0].Message.Content)

	return &LLMResponse{
		Response:  responseText,
		Timestamp: time.Now(),
	}, nil
}

func (c *NvidiaLLMClient) Close() error {
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.requestChan)
	})
	return nil
}

type nvidiaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type nvidiaChatRequest struct {
	Model       string              `json:"model"`
	Messages    []nvidiaChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature"`
	TopP        float64             `json:"top_p"`
	Stream      bool                `json:"stream"`
}

type nvidiaChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
