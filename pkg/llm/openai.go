package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"draw/pkg/llm/prompts"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAILLMClient struct {
	client      openai.Client
	model       string
	requestChan chan llmRequest
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
}

func NewOpenAILLMClient(baseURL, model, apiKey string) (*OpenAILLMClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("openai api key is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("openai model is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &OpenAILLMClient{
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		),
		model:       model,
		requestChan: make(chan llmRequest, 10),
		ctx:         ctx,
		cancel:      cancel,
	}

	go c.worker()

	return c, nil
}

func (c *OpenAILLMClient) worker() {
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

func (c *OpenAILLMClient) GenerateResponse(ctx context.Context, prompt string, boardState string) (*LLMResponse, error) {
	fmt.Println("Generating response for prompt", prompt, "and board state", boardState)
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

func (c *OpenAILLMClient) generateResponseSync(prompt string, systemPrompt string) (*LLMResponse, error) {
	reqCtx, cancel := context.WithTimeout(c.ctx, 20*time.Second)
	defer cancel()

	messages := []openai.ChatCompletionMessageParamUnion{}
	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}
	messages = append(messages, openai.UserMessage(prompt))

	resp, err := c.client.Chat.Completions.New(reqCtx, openai.ChatCompletionNewParams{
		Model:       openai.ChatModel(c.model),
		Messages:    messages,
		MaxTokens:   openai.Int(1024),
		Temperature: openai.Float(0.2),
		TopP:        openai.Float(0.9),
	})
	if err != nil {
		return nil, fmt.Errorf("openai api request error: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("openai api returned empty response")
	}

	return &LLMResponse{
		Response:  strings.TrimSpace(resp.Choices[0].Message.Content),
		Timestamp: time.Now(),
	}, nil
}

func (c *OpenAILLMClient) Close() error {
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.requestChan)
	})
	return nil
}
