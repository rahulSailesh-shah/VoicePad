package livekit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"draw/pkg/llm"
	"draw/pkg/speech"

	"github.com/livekit/media-sdk"
	"github.com/livekit/protocol/logger"
)

type LLMResponseCallback func(sessionID string, response *llm.CanvasResponse, err error)

type GetBoardStateFunc func(boardID string) (json.RawMessage, error)

type VoiceHandler struct {
	sessionID     string
	boardID       string
	userID        string
	speechClient  *speech.Client
	llmClient     *llm.Client
	session       *speech.TranscribeSession
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	isMuted       bool
	onTranscribe  TranscriptionCallback
	onLLMResponse LLMResponseCallback
	getBoardState GetBoardStateFunc
}

type VoiceHandlerConfig struct {
	SessionID     string
	BoardID       string
	UserID        string
	SpeechClient  *speech.Client
	LLMClient     *llm.Client
	OnTranscribe  TranscriptionCallback
	OnLLMResponse LLMResponseCallback
	GetBoardState GetBoardStateFunc
}

func NewVoiceHandler(cfg VoiceHandlerConfig) (*VoiceHandler, error) {
	if cfg.SpeechClient == nil {
		return nil, fmt.Errorf("speech client is required")
	}
	if cfg.SessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	// session, err := cfg.SpeechClient.NewTranscribeSession(ctx, cfg.SessionID)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create speech session: %w", err)
	// }
	return &VoiceHandler{
		sessionID:     cfg.SessionID,
		boardID:       cfg.BoardID,
		userID:        cfg.UserID,
		speechClient:  cfg.SpeechClient,
		// session:       session,
		ctx:           ctx,
		cancel:        cancel,
		isMuted:       true, // Start muted until OnUnmute is called
		onTranscribe:  cfg.OnTranscribe,
		onLLMResponse: cfg.OnLLMResponse,
		getBoardState: cfg.GetBoardState,
	}, nil
}

func (h *VoiceHandler) SendAudioChunk(sample media.PCM16Sample) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isMuted || h.session == nil {
		return nil
	}

	audioBytes := pcm16ToBytes(sample)

	if err := h.session.SendAudio(audioBytes); err != nil {
		logger.Errorw("Failed to send audio chunk", err, "sessionID", h.sessionID)
		return err
	}

	return nil
}

func (h *VoiceHandler) OnMute() (string, error) {
	h.mu.Lock()
	h.isMuted = true

	if h.session == nil {
		h.mu.Unlock()
		return "", nil
	}

	transcription, err := h.session.Finalize()
	if err != nil {
		logger.Errorw("Failed to finalize transcription", err, "sessionID", h.sessionID)
		h.session = nil
		h.mu.Unlock()
		return "", err
	}

	h.session = nil
	h.mu.Unlock()

	logger.Infow("Transcription complete", "sessionID", h.sessionID, "transcription", transcription)

	if h.onTranscribe != nil {
		go h.onTranscribe(h.sessionID, transcription, nil)
	}

	if transcription == "" || h.llmClient == nil {
		return transcription, nil
	}

	// go h.processWithLLM(transcription)

	return transcription, nil
}



func (h *VoiceHandler) OnUnmute() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		_ = h.session.Close()
		h.session = nil
	}

	session, err := h.speechClient.NewTranscribeSession(h.ctx, h.sessionID)
	if err != nil {
		logger.Errorw("Failed to create transcription session", err, "sessionID", h.sessionID)
		return err
	}

	h.session = session
	h.isMuted = false

	logger.Infow("Started transcription session", "sessionID", h.sessionID)

	return nil
}

func (h *VoiceHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cancel()

	if h.session != nil {
		_ = h.session.Close()
		h.session = nil
	}

	cleanupCtx := context.Background()
	if err := h.speechClient.CleanupSession(cleanupCtx, h.sessionID); err != nil {
		logger.Warnw("Failed to cleanup speech session", err, "sessionID", h.sessionID)
	}

	logger.Infow("Voice handler closed", "sessionID", h.sessionID)

	return nil
}



func pcm16ToBytes(sample media.PCM16Sample) []byte {
	bytes := make([]byte, len(sample)*2)
	for i, s := range sample {
		bytes[i*2] = byte(s)
		bytes[i*2+1] = byte(s >> 8)
	}
	return bytes
}

