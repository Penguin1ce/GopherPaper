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

// ── 后台数据分析看板 ─────────────────────────────────────────

// AdminTrendPoint 是某一天的运营指标聚合,构成时间序列。
type AdminTrendPoint struct {
	Date         string  `json:"date"` // YYYY-MM-DD
	Papers       int64   `json:"papers"`
	Calls        int64   `json:"calls"`
	Success      int64   `json:"success"`
	Failed       int64   `json:"failed"`
	Users        int64   `json:"users"`
	Sessions     int64   `json:"sessions"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
}

// AdminStatusSlice 是论文按状态的分布切片。
type AdminStatusSlice struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// AdminServiceStat 是单个服务类型的调用统计。
type AdminServiceStat struct {
	ServiceType string  `json:"service_type"`
	Total       int64   `json:"total"`
	Success     int64   `json:"success"`
	Failed      int64   `json:"failed"`
	AvgMS       float64 `json:"avg_ms"`
	MaxMS       int64   `json:"max_ms"`
}

// AdminAnalytics 汇总看板所需的时间序列、分布与总量。
type AdminAnalytics struct {
	Days          int                `json:"days"`
	Trend         []AdminTrendPoint  `json:"trend"`
	StatusDist    []AdminStatusSlice `json:"status_distribution"`
	Services      []AdminServiceStat `json:"services"`
	TotalPapers   int64              `json:"total_papers"`
	TotalUsers    int64              `json:"total_users"`
	TotalCalls    int64              `json:"total_calls"`
	TotalSessions int64              `json:"total_sessions"`
}

// ── 后台系统健康监控 ─────────────────────────────────────────

// AdminHealthItem 是单个依赖组件的探活结果。
type AdminHealthItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // up | down
	LatencyMS int64  `json:"latency_ms"`
	Detail    string `json:"detail,omitempty"`
}

// AdminHealth 是全部依赖的健康快照。
type AdminHealth struct {
	Items     []AdminHealthItem `json:"items"`
	CheckedAt time.Time         `json:"checked_at"`
	Healthy   int               `json:"healthy"`
	Total     int               `json:"total"`
}

// ── 后台实时活动流 ───────────────────────────────────────────

// AdminActivityItem 是活动流里的一条事件。
type AdminActivityItem struct {
	Type      string    `json:"type"` // paper | user | call
	Title     string    `json:"title"`
	Subtitle  string    `json:"subtitle,omitempty"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminActivity 是最近事件时间线。
type AdminActivity struct {
	Items []AdminActivityItem `json:"items"`
}

// AdminSeedResult 是演示数据种子写入/清除的计数。
type AdminSeedResult struct {
	Users    int `json:"users"`
	Papers   int `json:"papers"`
	Calls    int `json:"calls"`
	Sessions int `json:"sessions"`
}
