package service

import (
	"draw/internal/db/repo"
	"draw/pkg/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	UserService  UserService
	BoardService BoardService
}

func NewService(db *pgxpool.Pool, queries *repo.Queries, cfg *config.AppConfig) *Service {
	return &Service{
		UserService:  NewUserService(db, queries),
		BoardService: NewBoardService(db, queries, cfg),
	}

}
