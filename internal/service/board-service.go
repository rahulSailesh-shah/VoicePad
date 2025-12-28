package service

import (
	"context"
	"encoding/json"
	"fmt"

	"draw/internal/db/repo"
	"draw/internal/dto"
	"draw/pkg/config"
	"draw/pkg/livekit"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BoardService interface {
	CreateBoard(ctx context.Context, req dto.CreateBoardRequest) (*dto.CreateBoardResponse, error)
	GetBoard(ctx context.Context, req dto.GetBoardRequest) (*dto.GetBoardResponse, error)
	GetBoardsByUserID(ctx context.Context, req dto.GetBoardsByUserIDRequest) (*dto.GetBoardsByUserIDResponse, error)
	UpdateBoard(ctx context.Context, req dto.UpdateBoardRequest) (*dto.GetBoardResponse, error)
	DeleteBoard(ctx context.Context, req dto.DeleteBoardRequest) error
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


func (s *boardService) CreateBoard(ctx context.Context, req dto.CreateBoardRequest) (*dto.CreateBoardResponse, error) {
	board, err := s.queries.CreateBoard(ctx, repo.CreateBoardParams{
		Name: req.Name,
		OwnerID: req.UserID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create board: %w", err)
	}

	return &dto.CreateBoardResponse{
		BoardID: board.ID,

	}, nil
}

func (s *boardService) GetBoard(ctx context.Context, req dto.GetBoardRequest) (*dto.GetBoardResponse, error) {
	board, err := s.queries.GetBoardByID(ctx, repo.GetBoardByIDParams{
		ID:     uuid.MustParse(req.BoardID),
		OwnerID: req.UserID,
	})	
	if err != nil {
		return nil, fmt.Errorf("failed to get board: %w", err)	
	}

	userDetails, err := s.queries.GetUserByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	session, err := livekit.NewLiveKitSession(
		&userDetails,
		board.ID.String(),
		s.config,
		livekit.SessionCallbacks{
			GetBoardState: func(boardID string, userID string) (json.RawMessage, error) {
				board, err := s.queries.GetBoardByID(context.Background(), repo.GetBoardByIDParams{
					ID: uuid.MustParse(boardID),
					OwnerID: userID,
				})
				if err != nil || board.Elements == nil {
					return nil, fmt.Errorf("failed to get board: %w", err)
				}
				return board.Elements, nil
			},
		},
	)

	if err := session.Start(); err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}
	
	token, err := session.GenerateUserToken()
	if err != nil {
		session.Stop()
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &dto.GetBoardResponse{
		Board: toBoardResponse(board),
		Token: token,
	}, nil
}

func (s *boardService) GetBoardsByUserID(ctx context.Context, req dto.GetBoardsByUserIDRequest) (*dto.GetBoardsByUserIDResponse, error) {
	boards, err := s.queries.GetBoardsByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get boards: %w", err)
	}
	boardsResponse := make([]dto.Board, 0, len(boards))
	for _, board := range boards {
		boardsResponse = append(boardsResponse, toBoardResponse(board))
	}
	return &dto.GetBoardsByUserIDResponse{
		Boards: boardsResponse,
	}, nil
}

func (s *boardService) UpdateBoard(ctx context.Context, req dto.UpdateBoardRequest) (*dto.GetBoardResponse, error) {
	currentBoard, err := s.queries.GetBoardByID(ctx, repo.GetBoardByIDParams{
		ID: uuid.MustParse(req.BoardID),
		OwnerID: req.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get board: %w", err)
	}

	if req.Name != "" {
		currentBoard.Name = req.Name
	}
	if req.Elements != nil {
		currentBoard.Elements = req.Elements
	}

	board, err := s.queries.UpdateBoard(ctx, repo.UpdateBoardParams{
		ID: currentBoard.ID,
		Name: currentBoard.Name,
		Elements: currentBoard.Elements,
		OwnerID: req.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update board: %w", err)
	}

	return &dto.GetBoardResponse{
		Board: toBoardResponse(board),
	}, nil
}

func (s *boardService) DeleteBoard(ctx context.Context, req dto.DeleteBoardRequest) error {
	err := s.queries.DeleteBoard(ctx, repo.DeleteBoardParams{
		ID: uuid.MustParse(req.BoardID),
		OwnerID: req.UserID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete board: %w", err)
	}
	return nil
}	

func toBoardResponse(board repo.Board) dto.Board {
	return dto.Board{
		ID: board.ID,
		Name: board.Name,
		OwnerID: board.OwnerID,
		Elements: board.Elements,
	}
}