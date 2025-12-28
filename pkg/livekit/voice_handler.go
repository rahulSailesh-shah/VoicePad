package livekit

import (
	"context"
	"fmt"
	"sync"

	"draw/pkg/llm"
	"draw/pkg/speech"

	"github.com/livekit/media-sdk"
	"github.com/livekit/protocol/logger"
)

type LLMResponseCallback func(response *llm.LLMResponse, err error)

type GetBoardStateFunc func() (string, error)

type VoiceHandler struct {
	sessionID             string
	boardID               string
	userID                string
	speechClient          *speech.Client
	llmClient             llm.LLMClient
	session               *speech.TranscribeSession
	ctx                   context.Context
	cancel                context.CancelFunc
	mu                    sync.Mutex
	isMuted               bool
	onTranscribe          TranscriptionCallback
	onLLMResponse         LLMResponseCallback
	getBoardState         GetBoardStateFunc
	transcriptionCallback speech.TranscriptionCallback
}

type VoiceHandlerConfig struct {
	SessionID     string
	BoardID       string
	UserID        string
	SpeechClient  *speech.Client
	LLMClient     llm.LLMClient
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
	
	
	
	handler := &VoiceHandler{
		sessionID:     cfg.SessionID,
		boardID:       cfg.BoardID,
		userID:        cfg.UserID,
		speechClient:  cfg.SpeechClient,
		llmClient:     cfg.LLMClient,
		ctx:           ctx,
		cancel:        cancel,
		isMuted:       true, 
		onTranscribe:  cfg.OnTranscribe,
		onLLMResponse: cfg.OnLLMResponse,
		getBoardState: cfg.GetBoardState,
	}

	transcriptionCallback := func(transcription string, err error) {
		if err != nil {
			if handler.onTranscribe != nil {
				handler.onTranscribe(handler.sessionID, "", err)
			}
			return
		}
		if handler.llmClient != nil {
			go handler.handleLLMResponse(transcription)
		}
		if handler.onTranscribe != nil {
			handler.onTranscribe(handler.sessionID, transcription, nil)
		}
	}

	handler.transcriptionCallback = transcriptionCallback

	return handler, nil
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

func (h *VoiceHandler) OnMute() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.isMuted = true

	if h.session == nil {
		return nil
	}

	if err := h.session.Finalize(); err != nil {
		logger.Errorw("Failed to finalize transcription session", err, "sessionID", h.sessionID)
	}

	h.session = nil
	logger.Infow("Transcription session finalized", "sessionID", h.sessionID)

	return nil
}

func (h *VoiceHandler) OnUnmute() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		_ = h.session.Close()
		h.session = nil
	}

	session, err := h.speechClient.NewTranscribeSession(h.ctx, h.sessionID, h.transcriptionCallback)
	if err != nil {
		logger.Errorw("Failed to create transcription session", err, "sessionID", h.sessionID)
		return err
	}

	h.session = session
	h.isMuted = false

	logger.Infow("Started transcription session with VAD", "sessionID", h.sessionID)

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

func (h *VoiceHandler) handleLLMResponse(transcription string) {
	var boardStateJSON string = "[]"
	if h.getBoardState != nil && h.boardID != "" {
		boardState, err := h.getBoardState()
		if err != nil {
			fmt.Println("Failed to get board state for LLM", err, "boardID", h.boardID)
		} else {
			boardStateJSON = boardState
		}
	}

	fmt.Println("Board state", boardStateJSON)

	response, err := h.llmClient.GenerateResponse(context.Background(), transcription, boardStateJSON)
	if err != nil {
		if h.onLLMResponse != nil {
			h.onLLMResponse(nil, err)
		}
		return
	}

	if h.onLLMResponse != nil {
		h.onLLMResponse(response, nil)
	}
}


func pcm16ToBytes(sample media.PCM16Sample) []byte {
	bytes := make([]byte, len(sample)*2)
	for i, s := range sample {
		bytes[i*2] = byte(s)
		bytes[i*2+1] = byte(s >> 8)
	}
	return bytes
}

