package service

import (
	"context"
	"fmt"

	"draw/internal/db/repo"
	"draw/internal/dto"
	"draw/pkg/config"
	"draw/pkg/livekit"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BoardService interface {
	CreateBoard(ctx context.Context, request dto.CreateBoardRequest) (*dto.CreateBoardResponse, error)
}

type boardService struct {
	queries *repo.Queries
	db      *pgxpool.Pool
	config  *config.AppConfig
}

func NewBoardService(
	db *pgxpool.Pool,
	queries *repo.Queries,
	config *config.AppConfig,
) BoardService {
	return &boardService{
		db:      db,
		queries: queries,
		config: config,
	}
}


func (s *boardService) CreateBoard(ctx context.Context, request dto.CreateBoardRequest) (*dto.CreateBoardResponse, error) {
	userDetails, err := s.queries.GetUserByID(ctx, request.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	session := livekit.NewLiveKitSession(
		&userDetails,
		s.config,
		livekit.SessionCallbacks{},
	)

	// if err := session.Start(); err != nil {
	// 	return nil, fmt.Errorf("failed to start session: %w", err)
	// }
	
	token, err := session.GenerateUserToken()
	if err != nil {
		session.Stop()
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &dto.CreateBoardResponse{
		ID:    uuid.New().String(),
		Token: token,
	}, nil
}
