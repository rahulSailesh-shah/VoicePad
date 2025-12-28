package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"draw/pkg/llm/prompts"

	"github.com/ollama/ollama/api"
)

type llmRequest struct {
	prompt     string
	systemPrompt string
	resultCh chan *LLMResponse
	errCh    chan error
}

type OllamaLLMClient struct {
	client       *api.Client
	model        string
	requestChan chan llmRequest
	ctx          context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
}



func NewOllamaLLMClient(ollamaHost string, model string) (*OllamaLLMClient, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	llmClient := &OllamaLLMClient{
		client:       client,
		model:        model,
		requestChan: make(chan llmRequest, 10),
		ctx:          ctx,
		cancel:       cancel,
	}

	go llmClient.worker()

	return llmClient, nil
}

func (c *OllamaLLMClient) worker() {
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

func (c *OllamaLLMClient) GenerateResponse(ctx context.Context, prompt string, boardState string) (*LLMResponse, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("empty text provided")
	}

	// Format board state as JSON string if it's not empty
	boardStateJSON := boardState
	if boardState == "" {
		boardStateJSON = "[]"
	} else {
		// Validate it's valid JSON
		var js json.RawMessage
		if err := json.Unmarshal([]byte(boardState), &js); err != nil {
			// If not valid JSON, wrap it in an array or use empty array
			boardStateJSON = "[]"
		}
	}

	// Build the user prompt with board state
	userPrompt := prompts.BuildWhiteboardPrompt(prompt, boardStateJSON)
	systemPrompt := prompts.WhiteboardSystemPrompt

	resultCh := make(chan *LLMResponse, 1)
	errCh := make(chan error, 1)

	select {
	case c.requestChan <- llmRequest{
		prompt:       userPrompt,
		systemPrompt: systemPrompt,
		resultCh:    resultCh,
		errCh:       errCh,
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

func (c *OllamaLLMClient) generateResponseSync(prompt string, systemPrompt string) (*LLMResponse, error) {
	req := &api.GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: new(bool),
		Options: map[string]any{
			"temperature": 0.1,
			"num_predict": 2000,
		},
	}

	// Set system prompt if provided
	if systemPrompt != "" {
		req.System = systemPrompt
	}

	// daata, _ := json.MarshalIndent(req, "", "  ")
	// fmt.Println("Request", string(daata))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var fullResponse strings.Builder
	err := c.client.Generate(ctx, req, func(resp api.GenerateResponse) error {
		fullResponse.WriteString(resp.Response)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ollama generate error: %w", err)
	}

	responseText := strings.TrimSpace(fullResponse.String())

	return &LLMResponse{
		Response:  responseText,
		Timestamp: time.Now(),
	}, nil
}

func (c *OllamaLLMClient) Close() error {
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.requestChan)
	})
	return nil
}
