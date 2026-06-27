package dto

import "time"

type AdminRegisterRequest struct {
	Username         string `json:"username" binding:"required"`
	Email            string `json:"email" binding:"required,email"`
	Name             string `json:"name"`
	Password         string `json:"password" binding:"required,min=6"`
	Code             string `json:"code" binding:"required,len=6"`
	RegistrationCode string `json:"registration_code" binding:"required"`
}

type AdminLoginRequest struct {
	Email    string `json:"email" binding:"omitempty,email"`
	Account  string `json:"account"`
	Password string `json:"password" binding:"required"`
}

type AdminProfile struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

type AdminLoginResponse struct {
	Token string       `json:"token"`
	Admin AdminProfile `json:"admin"`
}

type AdminServiceBreakdown struct {
	ServiceType string `json:"service_type"`
	Total       int64  `json:"total"`
	Success     int64  `json:"success"`
	Failed      int64  `json:"failed"`
}

type AdminOverview struct {
	PaperCount               int64                   `json:"paper_count"`
	ServiceCallCount         int64                   `json:"service_call_count"`
	ServiceSuccessRate       *float64                `json:"service_success_rate"`
	ParseSuccessRate         *float64                `json:"parse_success_rate"`
	VectorCount              *int64                  `json:"vector_count"`
	VectorError              string                  `json:"vector_error,omitempty"`
	VectorReadyPapers        int64                   `json:"vector_ready_papers"`
	VectorIndexedPapers      *int64                  `json:"vector_indexed_papers"`
	VectorAvgChunksPerPaper  *float64                `json:"vector_avg_chunks_per_paper"`
	VectorCoverageRate       *float64                `json:"vector_coverage_rate"`
	VectorMissingReadyPapers *int64                  `json:"vector_missing_ready_papers"`
	ServiceSuccess           int64                   `json:"service_success"`
	ServiceFailed            int64                   `json:"service_failed"`
	ParseReady               int64                   `json:"parse_ready"`
	ParseFailed              int64                   `json:"parse_failed"`
	Breakdown                []AdminServiceBreakdown `json:"breakdown"`
}

type AdminPaperItem struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"owner_id"`
	OwnerName  string    `json:"owner_name,omitempty"`
	OwnerEmail string    `json:"owner_email,omitempty"`
	OwnerClass string    `json:"owner_class,omitempty"`
	Title      string    `json:"title"`
	FileName   string    `json:"file_name"`
	Size       int64     `json:"size"`
	Status     string    `json:"status"`
	FailReason string    `json:"fail_reason,omitempty"`
	PageCount  int       `json:"page_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AdminPaperListResponse struct {
	Items    []AdminPaperItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
