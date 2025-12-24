package livekit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"draw/pkg/config"

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
	OnMeetingEnd func(meetingID string, recordingURL string, transcriptURL string, err error)
}

type StreamTextData struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type LiveKitSession struct {
	userDetails     *repo.User
	room            *lksdk.Room
	handler         LivekitHandler
	egressInfo      *livekit.EgressInfo
	lkConfig        *config.LiveKitConfig
	geminiConfig    *config.GeminiConfig
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
	config *config.AppConfig,
	callbacks SessionCallbacks,
) *LiveKitSession {
	ctx, cancel := context.WithCancel(context.Background())

	return &LiveKitSession{
		userDetails:     userDetails,
		lkConfig:        &config.LiveKit,
		geminiConfig:    &config.Gemini,
		awsConfig:       &config.AWS,
		ctx:             ctx,
		cancel:          cancel,
		callbacks:       callbacks,
		stopOnce:        sync.Once{},
		textStreamQueue: make(chan StreamTextData, 100),
	}
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
			// if err := s.stopRecording(s.egressInfo.EgressId); err != nil {
			// 	stopErr = fmt.Errorf("failed to stop recording: %w", err)
			// }
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
		// invoke post process callback
	})
	return stopErr
}

func (s *LiveKitSession) GenerateUserToken() (string, error) {
	at := auth.NewAccessToken(s.lkConfig.APIKey, s.lkConfig.APISecret)
	// !FIXME
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     "board",
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

func (s *LiveKitSession) connectBot() error {
	audioWriterChan := make(chan media.PCM16Sample, 500)

	// !FIXME
	s.handler = nil

	if err := s.connectToRoom(); err != nil {
		s.handler.Close()
		close(audioWriterChan)
		return fmt.Errorf("failed to connect to room: %w", err)
	}

	go s.handlePublish(audioWriterChan)
	// go s.handleTextStreamQueue()

	// egressInfo, err := s.startRecording()
	// if err != nil {
	// 	logger.Errorw("Failed to start recording", err, "meetingID", s.meetingDetails.ID.String())
	// } else {
	// 	s.egressInfo = egressInfo
	// }
	return nil
}

func (s *LiveKitSession) connectToRoom() error {
	// !FIXME
	room, err := lksdk.ConnectToRoom(s.lkConfig.Host, lksdk.ConnectInfo{
		APIKey:              s.lkConfig.APIKey,
		APISecret:           s.lkConfig.APISecret,
		RoomName:            "board",
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
				if pcmRemoteTrack != nil {
					return
				}
				pcmRemoteTrack, _ = s.handleSubscribe(track)
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
