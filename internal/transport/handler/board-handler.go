package handler

import (
	"draw/internal/dto"
	"draw/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type BoardHandler struct {
	boardService service.BoardService
}

func NewBoardHandler(boardService service.BoardService) *BoardHandler {
	return &BoardHandler{
		boardService: boardService,
	}
}

func (h *BoardHandler) CreateBoard(c *gin.Context) {
	var req dto.CreateBoardRequest
	req.UserID = c.MustGet("userId").(string)

	board, err := h.boardService.CreateBoard(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Message: "Failed to create board",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "Board created",
		Data:    board,
	})
}