package dto

// TokenRequest 申请 token。
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

// LoginRequest 登录请求，Account 支持学号或绑定邮箱；StudentID 保留给旧客户端兼容。
type LoginRequest struct {
	Account   string `json:"account"`
	StudentID string `json:"student_id"`
	Password  string `json:"password" binding:"required"`
}

type PasswordResetCodeRequest struct {
	StudentID string `json:"student_id" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	StudentID string `json:"student_id" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Code      string `json:"code" binding:"required,len=6"`
	Password  string `json:"password" binding:"required,min=6"`
}

// LoginResponse 登录成功返回基本信息，JWT 仅写入 HttpOnly Cookie。
type LoginResponse struct {
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	ClassID   string `json:"class_id"`
}

type AvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}

type UserProfileResponse struct {
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	ClassID   string `json:"class_id"`
}

type UpdateProfileRequest struct {
	Name string `json:"name"`
}

type UpdateEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

type UserPreferenceResponse struct {
	Nickname          string `json:"nickname"`
	AnswerStyle       string `json:"answer_style"`
	OutputFormat      string `json:"output_format"`
	Language          string `json:"language"`
	CustomInstruction string `json:"custom_instruction"`
}

type UpdateUserPreferenceRequest struct {
	Nickname          string `json:"nickname"`
	AnswerStyle       string `json:"answer_style" binding:"required"`
	OutputFormat      string `json:"output_format" binding:"required"`
	Language          string `json:"language" binding:"required"`
	CustomInstruction string `json:"custom_instruction"`
}
