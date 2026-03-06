package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"draw/internal/db/repo"
	"draw/internal/service"
	"draw/pkg/config"
	"draw/pkg/database"
	"draw/pkg/logger"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type App struct {
	Config  *config.AppConfig
	DB      database.DB
	Service *service.Service
	Log     *logger.Logger
}

func NewApp(ctx context.Context, cfg *config.AppConfig) (*App, error) {
	db := database.NewPostgresDB(ctx, &cfg.DB)
	if err := db.Connect(); err != nil {
		fmt.Println("Error connecting to database:", err)
		return nil, err
	}

	dbInstance := db.GetDB()
	if dbInstance == nil {
		fmt.Println("Database instance is nil")
		return nil, fmt.Errorf("database not initialize")
	}

	queries := repo.New(dbInstance)

	services := service.NewService(dbInstance, queries, cfg)

	traceIDFn := func(ctx context.Context) string {
		return uuid.New().String()
	}
	prettyLog := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	logConfig := logger.Config{
		MinLevel:  logger.LevelDebug,
		Service:   "draw",
		Handlers:  []io.WriteCloser{prettyLog},
		TraceIDFn: traceIDFn,
	}
	log := logger.NewLogger(logConfig)

	return &App{
		Config:  cfg,
		DB:      db,
		Service: services,
		Log:     log,
	}, nil
}
