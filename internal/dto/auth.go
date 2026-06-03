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

// SendCodeRequest 申请邮箱验证码。
type SendCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RegisterRequest 注册请求，Code 为邮箱验证码。
type RegisterRequest struct {
	StudentID string `json:"student_id" binding:"required"`
	Name      string `json:"name"`
	Email     string `json:"email" binding:"required,email"`
	ClassID   string `json:"class_id"`
	Password  string `json:"password" binding:"required,min=6"`
	Code      string `json:"code" binding:"required,len=6"`
}

// LoginRequest 登录请求，按学号加密码校验。
type LoginRequest struct {
	StudentID string `json:"student_id" binding:"required"`
	Password  string `json:"password" binding:"required"`
}

// LoginResponse 登录成功返回 JWT 与基本信息。
type LoginResponse struct {
	Token     string `json:"token"`
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
}
