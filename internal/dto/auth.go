package dto

// TokenRequest 申请 token，骨架阶段直接按 student_id 签发。
type TokenRequest struct {
	StudentID string `json:"student_id" binding:"required"`
	ClassID   string `json:"class_id"`
}

// TokenResponse 返回签发的 JWT。
type TokenResponse struct {
	Token string `json:"token"`
}
