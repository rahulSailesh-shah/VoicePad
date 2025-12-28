package llm

import (
	"context"
	"fmt"
	"time"

	"draw/pkg/config"
)

type LLMResponse struct {
	Response  string    `json:"response"`
	Timestamp time.Time `json:"timestamp"`
}

type LLMClient interface {
	GenerateResponse(ctx context.Context, text string, boardState string) (*LLMResponse, error)
	Close() error
}

type LLMProvider string

const (
	LLMProviderOllama LLMProvider = "ollama"
	LLMProviderNvidia LLMProvider = "nvidia"
)

func NewLLMClient(cfg *config.LLMConfig) (LLMClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("llm config is required")
	}

	switch LLMProvider(cfg.Provider) {
	case LLMProviderOllama:fmt.Println("Creating Ollama LLM client")
		return NewOllamaLLMClient(cfg.Host, cfg.Model)
	case LLMProviderNvidia:
		return NewNvidiaLLMClient(cfg.Host, cfg.Model, cfg.APIKey)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}
