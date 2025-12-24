package dto

type CreateBoardRequest struct {
	UserID string `json:"user_id"`
}

type CreateBoardResponse struct {
	ID string `json:"id"`
	Token string `json:"token"`
}