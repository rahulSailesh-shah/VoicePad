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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Message: "Invalid request",
			Error:   err.Error(),
		})
		return
	}
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

func (h *BoardHandler) GetBoard(c *gin.Context) {
	boardId := c.Param("id")
	userId := c.MustGet("userId").(string)
	board, err := h.boardService.GetBoard(c.Request.Context(), dto.GetBoardRequest{
		BoardID: boardId,
		UserID:  userId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Message: "Failed to get board",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "Board fetched",
		Data:    board,
	})
}

func (h *BoardHandler) GetBoardsByUserID(c *gin.Context) {
	userId := c.MustGet("userId").(string)
	boards, err := h.boardService.GetBoardsByUserID(c.Request.Context(), dto.GetBoardsByUserIDRequest{
		UserID: userId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Message: "Failed to get boards",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "Boards fetched",
		Data:    boards,
	})
}

func (h *BoardHandler) UpdateBoard(c *gin.Context) {
	var req dto.UpdateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Message: "Invalid request",
			Error:   err.Error(),
		})
		return
	}
	req.BoardID = c.Param("id")
	req.UserID = c.MustGet("userId").(string)
	resp, err := h.boardService.UpdateBoard(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Message: "Failed to update board",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "Board updated",
		Data:    resp,
	})
}

func (h *BoardHandler) DeleteBoard(c *gin.Context) {
	
	boardId := c.Param("id")
	userId := c.MustGet("userId").(string)
	err := h.boardService.DeleteBoard(c.Request.Context(), dto.DeleteBoardRequest{
		BoardID: boardId,
		UserID:  userId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Message: "Failed to delete board",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "Board deleted",
	})
}