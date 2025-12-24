package livekit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"draw/pkg/config"
	"draw/pkg/llm"
	"draw/pkg/speech"

	"draw/internal/db/repo"

	"github.com/livekit/media-sdk"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/logger"

	lksdk "github.com/livekit/server-sdk-go/v2"
	lkmedia "github.com/livekit/server-sdk-go/v2/pkg/media"
	"github.com/pion/webrtc/v4"
)

type SessionCallbacks struct {
	OnMeetingEnd  func(meetingID string, recordingURL string, transcriptURL string, err error)
	OnLLMResponse func(boardID string, response *llm.CanvasResponse, err error)
	GetBoardState func(boardID string) (json.RawMessage, error)
}

type StreamTextData struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type LiveKitSession struct {
	userDetails     *repo.User
	boardID         string
	room            *lksdk.Room
	handler         LivekitHandler
	speechClient    *speech.Client
	llmClient       *llm.Client
	egressInfo      *livekit.EgressInfo
	lkConfig        *config.LiveKitConfig
	speechConfig    *config.SpeechConfig
	llmConfig       *config.LLMConfig
	awsConfig       *config.AWSConfig
	ctx             context.Context
	cancel          context.CancelFunc
	callbacks       SessionCallbacks
	stopOnce        sync.Once
	textStreamQueue chan StreamTextData
	recordingURL    string
	transcriptURL   string
}

func NewLiveKitSession(
	userDetails *repo.User,
	boardID string,
	cfg *config.AppConfig,
	callbacks SessionCallbacks,
) (*LiveKitSession, error) {
	ctx, cancel := context.WithCancel(context.Background())

	speechClient, err := speech.NewClient(cfg.Speech.Host)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create speech client: %w", err)
	}

	llmClient, err := llm.NewClient(&cfg.LLM)
	if err != nil {
		speechClient.Close()
		cancel()
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &LiveKitSession{
		userDetails:     userDetails,
		boardID:         boardID,
		lkConfig:        &cfg.LiveKit,
		speechConfig:    &cfg.Speech,
		llmConfig:       &cfg.LLM,
		speechClient:    speechClient,
		llmClient:       llmClient,
		awsConfig:       &cfg.AWS,
		ctx:             ctx,
		cancel:          cancel,
		callbacks:       callbacks,
		stopOnce:        sync.Once{},
		textStreamQueue: make(chan StreamTextData, 100),
	}, nil
}

func (s *LiveKitSession) Start() error {
	if err := s.connectBot(); err != nil {
		return fmt.Errorf("failed to connect bot: %w", err)
	}
	return nil
}

func (s *LiveKitSession) Stop() error {
	var stopErr error
	s.stopOnce.Do(func() {
		s.cancel()
		if s.egressInfo != nil {
			if err := s.stopRecording(s.egressInfo.EgressId); err != nil {
				stopErr = fmt.Errorf("failed to stop recording: %w", err)
			}
		}
		if s.textStreamQueue != nil {
			close(s.textStreamQueue)
		}
		if s.room != nil {
			s.room.Disconnect()
		}
		if s.handler != nil {
			s.handler.Close()
		}
		if s.speechClient != nil {
			s.speechClient.Close()
		}
		if s.llmClient != nil {
			s.llmClient.Close()
		}
	})
	return stopErr
}

func (s *LiveKitSession) GenerateUserToken() (string, error) {
	at := auth.NewAccessToken(s.lkConfig.APIKey, s.lkConfig.APISecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     s.boardID,
	}
	at.SetVideoGrant(grant).
		SetIdentity(s.userDetails.Name).
		SetValidFor(time.Hour)
	token, err := at.ToJWT()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *LiveKitSession) HandleMute() (string, error) {
	if s.handler == nil {
		return "", nil
	}
	return s.handler.OnMute()
}

func (s *LiveKitSession) HandleUnmute() error {
	if s.handler == nil {
		return nil
	}
	return s.handler.OnUnmute()
}

func (s *LiveKitSession) connectBot() error {
	audioWriterChan := make(chan media.PCM16Sample, 500)

	sessionID := fmt.Sprintf("%s:%s", s.boardID, s.userDetails.ID)

	handler, err := NewVoiceHandler(VoiceHandlerConfig{
		SessionID:    sessionID,
		BoardID:      s.boardID,
		UserID:       s.userDetails.ID,
		SpeechClient: s.speechClient,
		LLMClient:    s.llmClient,
		OnTranscribe: func(sessID string, transcription string, err error) {
			if err != nil {
				logger.Errorw("Transcription error", err, "sessionID", sessID)
				return
			}
			fmt.Println("Transcription received", "sessionID", sessID, "transcription", transcription)
		},
		OnLLMResponse: func(sessID string, response *llm.CanvasResponse, err error) {
			if err != nil {
				logger.Errorw("LLM error", err, "sessionID", sessID)
				if s.callbacks.OnLLMResponse != nil {
					s.callbacks.OnLLMResponse(s.boardID, nil, err)
				}
				return
			}
			logger.Infow("LLM response", "sessionID", sessID, "explanation", response.Explanation)

			// Stream response to client via text stream
			s.textStreamQueue <- StreamTextData{
				Type: "canvas_update",
				Data: response,
			}

			// Call the callback
			if s.callbacks.OnLLMResponse != nil {
				s.callbacks.OnLLMResponse(s.boardID, response, nil)
			}
		},
		GetBoardState: s.callbacks.GetBoardState,
	})
	if err != nil {
		close(audioWriterChan)
		return fmt.Errorf("failed to create voice handler: %w", err)
	}
	s.handler = handler

	if err := s.connectToRoom(); err != nil {
		s.handler.Close()
		close(audioWriterChan)
		return fmt.Errorf("failed to connect to room: %w", err)
	}

	go s.handlePublish(audioWriterChan)
	go s.handleTextStreamQueue()

	// egressInfo, err := s.startRecording()
	// if err != nil {
	// 	logger.Errorw("Failed to start recording", err, "meetingID", s.meetingDetails.ID.String())
	// } else {
	// 	s.egressInfo = egressInfo
	// }
	return nil
}

func (s *LiveKitSession) connectToRoom() error {
	room, err := lksdk.ConnectToRoom(s.lkConfig.Host, lksdk.ConnectInfo{
		APIKey:              s.lkConfig.APIKey,
		APISecret:           s.lkConfig.APISecret,
		RoomName:            s.boardID,
		ParticipantIdentity: "bot",
	}, s.callbacksForRoom())
	if err != nil {
		return err
	}
	s.room = room
	return nil
}

func (s *LiveKitSession) callbacksForRoom() *lksdk.RoomCallback {
	var pcmRemoteTrack *lkmedia.PCMRemoteTrack

	return &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if publication.Kind() != lksdk.TrackKindAudio {
					return
				}
				if pcmRemoteTrack != nil {
					return
				}
				pcmRemoteTrack, _ = s.handleSubscribe(track)
			},
			OnTrackMuted: func(pub lksdk.TrackPublication, p lksdk.Participant) {
				// Handle track mute - trigger transcription finalization
				if pub.Kind() == lksdk.TrackKindAudio {
					logger.Infow("Audio track muted", "participant", p.Identity())
					go func() {
						if _, err := s.HandleMute(); err != nil {
							logger.Errorw("HandleMute failed", err)
						}
					}()
				}
			},
			OnTrackUnmuted: func(pub lksdk.TrackPublication, p lksdk.Participant) {
				// Handle track unmute - start new transcription session
				if pub.Kind() == lksdk.TrackKindAudio {
					logger.Infow("Audio track unmuted", "participant", p.Identity())
					go func() {
						if err := s.HandleUnmute(); err != nil {
							logger.Errorw("HandleUnmute failed", err)
						}
					}()
				}
			},
		},
		OnParticipantDisconnected: func(participant *lksdk.RemoteParticipant) {
			s.Stop()
		},
		OnDisconnected: func() {
			if pcmRemoteTrack != nil {
				pcmRemoteTrack.Close()
				pcmRemoteTrack = nil
			}
		},
		OnDisconnectedWithReason: func(reason lksdk.DisconnectionReason) {
			if pcmRemoteTrack != nil {
				pcmRemoteTrack.Close()
				pcmRemoteTrack = nil
			}
		},
	}
}


func (s *LiveKitSession) handlePublish(audioWriterChan chan media.PCM16Sample) {
	publishTrack, err := lkmedia.NewPCMLocalTrack(24000, 1, logger.GetLogger())
	if err != nil {
		return
	}
	defer func() {
		publishTrack.ClearQueue()
		publishTrack.Close()
		close(audioWriterChan)
	}()

	// !FIXME
	if _, err = s.room.LocalParticipant.PublishTrack(publishTrack, &lksdk.TrackPublicationOptions{
		Name: "",
	}); err != nil {
		return
	}

	for {
		select {
		case sample, ok := <-audioWriterChan:
			if !ok {
				return
			}
			if err := publishTrack.WriteSample(sample); err != nil {
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *LiveKitSession) handleTextStreamQueue() {
	for {
		select {
		case data, ok := <-s.textStreamQueue:
			if !ok {
				// Channel closed, exit worker
				return
			}
			marshalData, err := json.Marshal(data)
			if err != nil {
				continue
			}
			s.room.LocalParticipant.SendText(string(marshalData), lksdk.StreamTextOptions{
				Topic: "board",
			})
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *LiveKitSession) handleSubscribe(track *webrtc.TrackRemote) (*lkmedia.PCMRemoteTrack, error) {
	// Only process audio tracks
	if track.Kind() != webrtc.RTPCodecTypeAudio {
		return nil, fmt.Errorf("expected audio track, got %v", track.Kind())
	}

	if track.Codec().MimeType != webrtc.MimeTypeOpus {
		logger.Warnw("Received non-opus track", nil, "track", track.Codec().MimeType)
	}

	writer := NewRemoteTrackWriter(s.handler)
	trackWriter, err := lkmedia.NewPCMRemoteTrack(track, writer, lkmedia.WithTargetSampleRate(16000))
	if err != nil {
		return nil, err
	}

	return trackWriter, nil
}

func (s *LiveKitSession) startRecording() (*livekit.EgressInfo, error) {
	req := &livekit.RoomCompositeEgressRequest{
		RoomName:  ""	,
		Layout:    "grid",
		AudioOnly: false,
	}
	outputPath := fmt.Sprintf("%s/%s/recording.mp4", s.userDetails.ID, s.userDetails.Name)
	req.FileOutputs = []*livekit.EncodedFileOutput{
		{
			Filepath: outputPath,
			Output: &livekit.EncodedFileOutput_S3{
				S3: &livekit.S3Upload{
					AccessKey:      s.awsConfig.AccessKey,
					Secret:         s.awsConfig.SecretKey,
					Region:         s.awsConfig.Region,
					Bucket:         s.awsConfig.Bucket,
					ForcePathStyle: false,
				},
			},
		},
	}

	egressClient := lksdk.NewEgressClient(
		s.lkConfig.Host,
		s.lkConfig.APIKey,
		s.lkConfig.APISecret,
	)
	res, err := egressClient.StartRoomCompositeEgress(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *LiveKitSession) stopRecording(egressID string) error {
	egressClient := lksdk.NewEgressClient(
		s.lkConfig.Host,
		s.lkConfig.APIKey,
		s.lkConfig.APISecret,
	)

	_, err := egressClient.StopEgress(context.Background(), &livekit.StopEgressRequest{
		EgressId: egressID,
	})
	if err != nil {
		return err
	}
	return nil
}
