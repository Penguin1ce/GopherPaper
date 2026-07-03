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

type AdminModelConfigItem struct {
	Role              string     `json:"role"`
	Label             string     `json:"label"`
	Description       string     `json:"description"`
	Kind              string     `json:"kind"`
	Provider          string     `json:"provider"`
	BaseURL           string     `json:"base_url"`
	Model             string     `json:"model"`
	APIKeyMask        string     `json:"api_key_mask"`
	HasAPIKey         bool       `json:"has_api_key"`
	Dim               int        `json:"dim"`
	MaxTokens         int        `json:"max_tokens"`
	ReasoningEffort   string     `json:"reasoning_effort"`
	Thinking          string     `json:"thinking"`
	Enabled           bool       `json:"enabled"`
	Timeout           int        `json:"timeout"`
	Source            string     `json:"source"`
	Active            bool       `json:"active"`
	RestartRequired   bool       `json:"restart_required"`
	LastTestStatus    string     `json:"last_test_status,omitempty"`
	LastTestError     string     `json:"last_test_error,omitempty"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	LastTestLatencyMS int64      `json:"last_test_latency_ms,omitempty"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
	Warnings          []string   `json:"warnings,omitempty"`
}

type AdminModelConfigsResponse struct {
	Items []AdminModelConfigItem `json:"items"`
}

type AdminModelConfigUpdateRequest struct {
	Provider        string  `json:"provider"`
	BaseURL         string  `json:"base_url"`
	Model           string  `json:"model"`
	APIKey          *string `json:"api_key"`
	Dim             int     `json:"dim"`
	MaxTokens       int     `json:"max_tokens"`
	ReasoningEffort string  `json:"reasoning_effort"`
	Thinking        string  `json:"thinking"`
	Enabled         bool    `json:"enabled"`
	Timeout         int     `json:"timeout"`
}

type AdminModelConfigTestResponse struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

type AdminModelConfigApplyResponse struct {
	AppliedRoles []string `json:"applied_roles"`
	Warnings     []string `json:"warnings,omitempty"`
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

// ── 后台用户管理 ─────────────────────────────────────────────

// AdminUserItem 是用户列表里的一行,附带该用户的关键运营计数。
type AdminUserItem struct {
	ID           uint       `json:"id"`
	StudentID    string     `json:"student_id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	ClassID      string     `json:"class_id"`
	AvatarURL    string     `json:"avatar_url,omitempty"`
	PaperCount   int64      `json:"paper_count"`
	SessionCount int64      `json:"session_count"`
	CallCount    int64      `json:"call_count"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
}

// AdminUserListResponse 是分页的用户列表。
type AdminUserListResponse struct {
	Items    []AdminUserItem `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// AdminUserStatusCount 是某用户论文按状态的计数切片。
type AdminUserStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// AdminUserActivityPoint 是某用户某一天的活动计数。
type AdminUserActivityPoint struct {
	Date     string `json:"date"`
	Papers   int64  `json:"papers"`
	Sessions int64  `json:"sessions"`
	Calls    int64  `json:"calls"`
}

// AdminUserDetail 是单个用户的画像:基础信息 + 论文状态分布 + 活动趋势 + 最近论文。
type AdminUserDetail struct {
	User            AdminUserItem          `json:"user"`
	PaperStatusDist []AdminUserStatusCount `json:"paper_status_distribution"`
	Activity        []AdminUserActivityPoint `json:"activity"`
	RecentPapers    []AdminPaperItem       `json:"recent_papers"`
}

// AdminClassStat 是按班级聚合的用户与论文统计。
type AdminClassStat struct {
	ClassID    string `json:"class_id"`
	UserCount  int64  `json:"user_count"`
	PaperCount int64  `json:"paper_count"`
}

// AdminClassStatsResponse 是班级维度的聚合列表。
type AdminClassStatsResponse struct {
	Items []AdminClassStat `json:"items"`
}

// ── 后台服务调用日志浏览器 ───────────────────────────────────

// AdminLogItem 是一条服务调用日志。
type AdminLogItem struct {
	ID           uint64    `json:"id"`
	ServiceType  string    `json:"service_type"`
	ActorID      string    `json:"actor_id,omitempty"`
	PaperID      string    `json:"paper_id,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	Success      bool      `json:"success"`
	DurationMS   int64     `json:"duration_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AdminLogListResponse 是分页的服务调用日志列表。
type AdminLogListResponse struct {
	Items    []AdminLogItem `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// AdminLogStats 是日志筛选面板需要的辅助统计与可选项。
type AdminLogStats struct {
	ServiceTypes []string `json:"service_types"`
	Total        int64    `json:"total"`
	Success      int64    `json:"success"`
	Failed       int64    `json:"failed"`
	AvgMS        float64  `json:"avg_ms"`
	MaxMS        int64    `json:"max_ms"`
}

// ── 后台高级分析 ─────────────────────────────────────────────

// AdminLatencyBucket 是延迟直方图的一个区间。
type AdminLatencyBucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// AdminHourPoint 是按一天 24 小时聚合的调用分布点。
type AdminHourPoint struct {
	Hour    int   `json:"hour"`
	Count   int64 `json:"count"`
	Success int64 `json:"success"`
}

// AdminTopActor 是调用量最高的用户。
type AdminTopActor struct {
	ActorID string `json:"actor_id"`
	Calls   int64  `json:"calls"`
}

// AdminPipelineStage 是论文解析流水线某一阶段的滞留数量。
type AdminPipelineStage struct {
	Stage string `json:"stage"`
	Count int64  `json:"count"`
}

// AdminAdvancedAnalytics 汇总高级分析页所需的多维统计。
type AdminAdvancedAnalytics struct {
	LatencyHistogram   []AdminLatencyBucket `json:"latency_histogram"`
	HourlyDistribution []AdminHourPoint     `json:"hourly_distribution"`
	TopActors          []AdminTopActor      `json:"top_actors"`
	Pipeline           []AdminPipelineStage `json:"pipeline"`
}

// ── 后台操作审计 ─────────────────────────────────────────────

// AdminAuditItem 是一条管理员操作审计记录。
type AdminAuditItem struct {
	ID        uint64    `json:"id"`
	AdminID   uint      `json:"admin_id"`
	AdminName string    `json:"admin_name"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminAuditListResponse 是分页的审计记录列表。
type AdminAuditListResponse struct {
	Items    []AdminAuditItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
