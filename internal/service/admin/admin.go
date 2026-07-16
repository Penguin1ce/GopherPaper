package admin

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	paperservice "GopherPaper/internal/service/paper"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
	"GopherPaper/pkg/utils"
)

const adminStatusActive = "active"

var (
	registrationCode string
	mqURL            string // RabbitMQ 连接串,供健康监控探活

	ErrRegistrationCodeNotConfigured = errors.New("admin: registration code is not configured")
	ErrRegistrationCodeInvalid       = errors.New("admin: registration code is invalid")
	ErrAdminExists                   = errors.New("admin: username or email already exists")
	ErrAdminNotFound                 = errors.New("admin: admin not found")
	ErrAdminInactive                 = errors.New("admin: admin is disabled")
	ErrWrongPassword                 = errors.New("admin: password is incorrect")
)

func Init(cfg *config.Config) {
	if cfg == nil {
		return
	}
	registrationCode = strings.TrimSpace(cfg.Admin.RegistrationCode)
	mqURL = strings.TrimSpace(cfg.MQ.URL)
}

func SendVerifyCode(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	code := genCode()
	if err := dao.SetTTL(ctx, adminCodeKey(email), code, constant.VerifyCodeTTL); err != nil {
		return fmt.Errorf("admin: store verification code failed: %w", err)
	}
	return utils.SendMail(email, code)
}

func Register(ctx context.Context, req dto.AdminRegisterRequest) (*model.Admin, error) {
	if registrationCode == "" {
		return nil, ErrRegistrationCodeNotConfigured
	}
	if strings.TrimSpace(req.RegistrationCode) != registrationCode {
		return nil, ErrRegistrationCodeInvalid
	}
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	if username == "" || email == "" {
		return nil, ErrAdminNotFound
	}
	if err := verifyEmailCode(ctx, email, strings.TrimSpace(req.Code)); err != nil {
		return nil, err
	}
	var count int64
	if err := dao.DB.WithContext(ctx).Model(&model.Admin{}).
		Where("username = ? OR email = ?", username, email).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("admin: query admin failed: %w", err)
	}
	if count > 0 {
		return nil, ErrAdminExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("admin: hash password failed: %w", err)
	}
	admin := &model.Admin{
		Username:     username,
		Email:        email,
		Name:         strings.TrimSpace(req.Name),
		PasswordHash: string(hash),
		Status:       adminStatusActive,
	}
	if err := dao.DB.WithContext(ctx).Create(admin).Error; err != nil {
		return nil, fmt.Errorf("admin: create admin failed: %w", err)
	}
	_, _ = dao.Del(ctx, adminCodeKey(email))
	return admin, nil
}

func Login(ctx context.Context, email, password string) (string, *model.Admin, error) {
	email = strings.TrimSpace(email)
	var admin model.Admin
	err := dao.DB.WithContext(ctx).
		Where("email = ?", email).
		First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, ErrAdminNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("admin: query admin failed: %w", err)
	}
	if admin.Status != adminStatusActive {
		return "", nil, ErrAdminInactive
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return "", nil, ErrWrongPassword
	}
	token, err := auth.IssueAdmin(ctx, admin.ID, admin.Username)
	if err != nil {
		return "", nil, fmt.Errorf("admin: sign token failed: %w", err)
	}
	now := time.Now()
	_ = dao.DB.WithContext(ctx).Model(&admin).Update("last_login_at", now).Error
	admin.LastLoginAt = &now
	return token, &admin, nil
}

func Logout(ctx context.Context, adminID uint) error {
	if err := auth.RevokeAdmin(ctx, adminID); err != nil {
		return fmt.Errorf("admin: revoke login session: %w", err)
	}
	return nil
}

func Get(ctx context.Context, id uint) (*model.Admin, error) {
	var admin model.Admin
	err := dao.DB.WithContext(ctx).First(&admin, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAdminNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("admin: query admin failed: %w", err)
	}
	return &admin, nil
}

func Overview(ctx context.Context) (*dto.AdminOverview, error) {
	out := &dto.AdminOverview{}
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Count(&out.PaperCount).Error; err != nil {
		return nil, err
	}
	if err := dao.DB.WithContext(ctx).Model(&model.ServiceCallLog{}).Count(&out.ServiceCallCount).Error; err != nil {
		return nil, err
	}
	if err := dao.DB.WithContext(ctx).Model(&model.ServiceCallLog{}).Where("success = ?", true).Count(&out.ServiceSuccess).Error; err != nil {
		return nil, err
	}
	if err := dao.DB.WithContext(ctx).Model(&model.ServiceCallLog{}).Where("success = ?", false).Count(&out.ServiceFailed).Error; err != nil {
		return nil, err
	}
	if out.ServiceCallCount > 0 {
		rate := float64(out.ServiceSuccess) / float64(out.ServiceCallCount)
		out.ServiceSuccessRate = &rate
	}
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Where("status = ?", constant.PaperReady).Count(&out.ParseReady).Error; err != nil {
		return nil, err
	}
	out.VectorReadyPapers = out.ParseReady
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Where("status = ?", constant.PaperFailed).Count(&out.ParseFailed).Error; err != nil {
		return nil, err
	}
	if denom := out.ParseReady + out.ParseFailed; denom > 0 {
		rate := float64(out.ParseReady) / float64(denom)
		out.ParseSuccessRate = &rate
	}
	type row struct {
		ServiceType string
		Total       int64
		Success     int64
	}
	var rows []row
	if err := dao.DB.WithContext(ctx).
		Model(&model.ServiceCallLog{}).
		Select("service_type, count(*) as total, sum(case when success then 1 else 0 end) as success").
		Group("service_type").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out.Breakdown = make([]dto.AdminServiceBreakdown, 0, len(rows))
	for _, r := range rows {
		out.Breakdown = append(out.Breakdown, dto.AdminServiceBreakdown{
			ServiceType: r.ServiceType,
			Total:       r.Total,
			Success:     r.Success,
			Failed:      r.Total - r.Success,
		})
	}
	if count, err := knowledge.VectorCount(ctx); err != nil {
		zlog.Warn("admin vector count failed", "err", err)
		out.VectorError = err.Error()
	} else {
		out.VectorCount = &count
		if err := fillVectorIndexStats(ctx, out); err != nil {
			zlog.Warn("admin vector index stats failed", "err", err)
			out.VectorError = err.Error()
		}
	}
	return out, nil
}

type readyPaperRef struct {
	ID      string
	OwnerID string
}

func fillVectorIndexStats(ctx context.Context, out *dto.AdminOverview) error {
	var papers []readyPaperRef
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).
		Select("id, owner_id").
		Where("status = ?", constant.PaperReady).
		Scan(&papers).Error; err != nil {
		return err
	}
	ready := int64(len(papers))
	out.VectorReadyPapers = ready
	if ready == 0 {
		zeroInt := int64(0)
		zeroRate := float64(0)
		out.VectorIndexedPapers = &zeroInt
		out.VectorAvgChunksPerPaper = &zeroRate
		out.VectorCoverageRate = &zeroRate
		out.VectorMissingReadyPapers = &zeroInt
		return nil
	}
	var indexed, readyChunks int64
	for _, p := range papers {
		n, err := knowledge.CountPaperChunks(ctx, p.OwnerID, p.ID)
		if err != nil {
			return err
		}
		readyChunks += n
		if n > 0 {
			indexed++
		}
	}
	avg := float64(readyChunks) / float64(ready)
	coverage := float64(indexed) / float64(ready)
	missing := ready - indexed
	out.VectorIndexedPapers = &indexed
	out.VectorAvgChunksPerPaper = &avg
	out.VectorCoverageRate = &coverage
	out.VectorMissingReadyPapers = &missing
	return nil
}

func ListPapers(ctx context.Context, query, status, dateRange string, page, pageSize int) (*dto.AdminPaperListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	db := dao.DB.WithContext(ctx).Model(&model.Paper{}).
		Joins("left join users on users.student_id = papers.owner_id")
	query = strings.TrimSpace(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("papers.id = ? OR papers.title LIKE ? OR papers.file_name LIKE ? OR papers.owner_id LIKE ? OR users.name LIKE ? OR users.email LIKE ?",
			query, like, like, like, like, like)
	}
	if status = strings.TrimSpace(status); status != "" {
		db = db.Where("papers.status = ?", status)
	}
	if start, ok := paperDateRangeStart(dateRange, time.Now()); ok {
		db = db.Where("papers.created_at >= ?", start)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	items := make([]dto.AdminPaperItem, 0)
	if err := db.Select(`papers.id, papers.owner_id, users.name as owner_name, users.email as owner_email, users.class_id as owner_class,
		papers.title, papers.file_name, papers.size, papers.status, papers.fail_reason, papers.page_count, papers.created_at, papers.updated_at`).
		Order("papers.created_at desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&items).Error; err != nil {
		return nil, err
	}
	return &dto.AdminPaperListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func paperDateRangeStart(dateRange string, now time.Time) (time.Time, bool) {
	switch strings.TrimSpace(dateRange) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), true
	case "7d", "week":
		return now.AddDate(0, 0, -7), true
	case "month", "30d":
		return now.AddDate(0, -1, 0), true
	default:
		return time.Time{}, false
	}
}

func DeletePaper(ctx context.Context, paperID string) error {
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		return err
	}
	return paperservice.Delete(ctx, p.OwnerID, paperID)
}

func Profile(a *model.Admin) dto.AdminProfile {
	if a == nil {
		return dto.AdminProfile{}
	}
	return dto.AdminProfile{ID: a.ID, Username: a.Username, Email: a.Email, Name: a.Name}
}

func verifyEmailCode(ctx context.Context, email, code string) error {
	stored, err := dao.Get(ctx, adminCodeKey(email))
	if errors.Is(err, dao.ErrCacheMiss) {
		return errs.ErrCodeExpired
	}
	if err != nil {
		return fmt.Errorf("admin: read verification code failed: %w", err)
	}
	if stored != code {
		return errs.ErrCodeMismatch
	}
	return nil
}

func adminCodeKey(email string) string {
	return constant.RedisKeyAdminVerifyCode + email
}

func genCode() string {
	const digits = "0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = digits[int(b[i])%len(digits)]
	}
	return string(b)
}
