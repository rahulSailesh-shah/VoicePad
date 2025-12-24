package http

import (
	"net/http"

	"draw/internal/app"
	"draw/internal/transport/handler"
	"draw/internal/transport/http/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

func RegisterRoutes(r *gin.Engine, authKeys jwk.Set, app *app.App) {
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:9000", "http://127.0.0.1:9000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Middlewares
	protected := r.Group("")
	protected.Use(middleware.AuthMiddleware(authKeys))

	// Inngest Endpoint
	// r.Any("/api/inngest", app.Inngest.Handler())

	userHandler := handler.NewUserHandler(app.Service.UserService)
	protected.GET("/users/:id", userHandler.GetUserByID)

	boardHandler := handler.NewBoardHandler(app.Service.BoardService)
	protected.POST("/boards", boardHandler.CreateBoard)
}
