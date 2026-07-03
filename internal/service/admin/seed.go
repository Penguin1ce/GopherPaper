package admin

import (
	"context"
	"fmt"
	mrand "math/rand"
	"time"

	"golang.org/x/crypto/bcrypt"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

// 演示数据统一以 demo- 前缀标记(学号 / 论文 ID / 会话 ID / 日志 actor),
// 便于 ClearDemo 精确清除,绝不误删真实数据。
const demoPrefix = "demo-"

var demoNames = []string{
	"周深研", "李向量", "王图谱", "陈解析", "赵知识",
	"孙检索", "钱嵌入", "吴报告", "郑问答", "冯精读",
}

var demoClasses = []string{"CS2101", "CS2102", "AI2103", "DS2104"}

var demoTitles = []string{
	"Attention Is All You Need",
	"Retrieval-Augmented Generation for Knowledge-Intensive NLP",
	"Deep Residual Learning for Image Recognition",
	"BERT: Pre-training of Deep Bidirectional Transformers",
	"Denoising Diffusion Probabilistic Models",
	"GPT-4 Technical Report",
	"Chain-of-Thought Prompting Elicits Reasoning",
	"LoRA: Low-Rank Adaptation of Large Language Models",
	"Segment Anything",
	"A Survey of Large Language Models",
	"Mamba: Linear-Time Sequence Modeling",
	"FlashAttention: Fast and Memory-Efficient Exact Attention",
}

var demoStatusPool = []constant.PaperStatus{
	constant.PaperReady, constant.PaperReady, constant.PaperReady, constant.PaperReady, constant.PaperReady,
	constant.PaperIndexed, constant.PaperIndexed,
	constant.PaperExtracted,
	constant.PaperParsing,
	constant.PaperUploaded,
	constant.PaperFailed, constant.PaperFailed,
}

var demoServiceTypes = []string{"chat", "report", "parse"}

var demoAgents = []string{"owl", "lark", "gopher"}

// SeedDemo 灌入一批跨最近 30 天的仿真运营数据(用户/论文/调用日志/会话),
// 让看板图表与活动流有数据可展示。执行前先清空旧的演示数据,可重复运行。
func SeedDemo(ctx context.Context) (*dto.AdminSeedResult, error) {
	if dao.DB == nil {
		return nil, fmt.Errorf("admin: 数据库未初始化")
	}
	if _, err := ClearDemo(ctx); err != nil {
		return nil, err
	}
	db := dao.DB.WithContext(ctx)
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	now := time.Now()
	const window = 30
	result := &dto.AdminSeedResult{}

	hash, err := bcrypt.GenerateFromPassword([]byte("demo123456"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("admin: 生成演示密码失败: %w", err)
	}

	users := make([]model.User, 0, len(demoNames))
	for i, name := range demoNames {
		created := randPast(rng, now, window)
		u := model.User{
			StudentID:    fmt.Sprintf("%s%04d", demoPrefix, i+1),
			Name:         name,
			Email:        fmt.Sprintf("%s%04d@demo.local", demoPrefix, i+1),
			ClassID:      demoClasses[rng.Intn(len(demoClasses))],
			PasswordHash: string(hash),
			CreatedAt:    created,
			UpdatedAt:    created,
		}
		if err := db.Create(&u).Error; err != nil {
			return nil, fmt.Errorf("admin: 写入演示用户失败: %w", err)
		}
		users = append(users, u)
	}
	result.Users = len(users)

	papers := make([]model.Paper, 0, 45)
	for i := 0; i < 45; i++ {
		owner := users[rng.Intn(len(users))]
		created := randPast(rng, now, window)
		status := demoStatusPool[rng.Intn(len(demoStatusPool))]
		p := model.Paper{
			ID:        fmt.Sprintf("%s%d-%06d", demoPrefix, i+1, rng.Intn(1_000_000)),
			OwnerID:   owner.StudentID,
			Title:     demoTitles[rng.Intn(len(demoTitles))],
			FileName:  fmt.Sprintf("demo_paper_%02d.pdf", i+1),
			FileURI:   fmt.Sprintf("demo://papers/%02d.pdf", i+1),
			Size:      int64(500_000 + rng.Intn(8_000_000)),
			Status:    status,
			PageCount: 4 + rng.Intn(24),
			Category:  "DEMO",
			CreatedAt: created,
			UpdatedAt: created,
		}
		if status == constant.PaperFailed {
			p.FailReason = "demo: MinerU 解析超时,已触发降级"
		}
		if err := db.Create(&p).Error; err != nil {
			return nil, fmt.Errorf("admin: 写入演示论文失败: %w", err)
		}
		papers = append(papers, p)
	}
	result.Papers = len(papers)

	const callCount = 320
	for i := 0; i < callCount; i++ {
		owner := users[rng.Intn(len(users))]
		created := randPast(rng, now, window)
		svc := demoServiceTypes[rng.Intn(len(demoServiceTypes))]
		success := rng.Float64() < 0.86
		var dur int64
		if success {
			dur = int64(200 + rng.Intn(2800))
		} else {
			dur = int64(1500 + rng.Intn(6500))
		}
		log := model.ServiceCallLog{
			ServiceType: svc,
			ActorID:     owner.StudentID,
			Success:     success,
			DurationMS:  dur,
			CreatedAt:   created,
		}
		if len(papers) > 0 {
			log.PaperID = papers[rng.Intn(len(papers))].ID
		}
		if !success {
			log.ErrorMessage = "demo: upstream model timeout"
		}
		if err := db.Create(&log).Error; err != nil {
			return nil, fmt.Errorf("admin: 写入演示调用日志失败: %w", err)
		}
	}
	result.Calls = callCount

	const sessionCount = 36
	for i := 0; i < sessionCount; i++ {
		owner := users[rng.Intn(len(users))]
		created := randPast(rng, now, window)
		s := model.Session{
			ID:        fmt.Sprintf("%ssess-%d-%06d", demoPrefix, i+1, rng.Intn(1_000_000)),
			StudentID: owner.StudentID,
			AgentType: demoAgents[rng.Intn(len(demoAgents))],
			Title:     demoTitles[rng.Intn(len(demoTitles))],
			CreatedAt: created,
			UpdatedAt: created,
		}
		if len(papers) > 0 {
			s.PaperID = papers[rng.Intn(len(papers))].ID
		}
		if err := db.Create(&s).Error; err != nil {
			return nil, fmt.Errorf("admin: 写入演示会话失败: %w", err)
		}
	}
	result.Sessions = sessionCount

	_, _ = SeedDemoFeedbacks(ctx)

	return result, nil
}

// ClearDemo 清除全部以 demo- 前缀标记的演示数据(硬删除),真实数据不受影响。
func ClearDemo(ctx context.Context) (*dto.AdminSeedResult, error) {
	if dao.DB == nil {
		return nil, fmt.Errorf("admin: 数据库未初始化")
	}
	db := dao.DB.WithContext(ctx)
	res := &dto.AdminSeedResult{}
	like := demoPrefix + "%"

	r := db.Unscoped().Where("actor_id LIKE ?", like).Delete(&model.ServiceCallLog{})
	res.Calls = int(r.RowsAffected)
	r = db.Unscoped().Where("id LIKE ? OR student_id LIKE ?", like, like).Delete(&model.Session{})
	res.Sessions = int(r.RowsAffected)
	r = db.Unscoped().Where("id LIKE ? OR owner_id LIKE ?", like, like).Delete(&model.Paper{})
	res.Papers = int(r.RowsAffected)
	r = db.Unscoped().Where("student_id LIKE ?", like).Delete(&model.User{})
	res.Users = int(r.RowsAffected)
	db.Unscoped().Where("student_id LIKE ?", like).Delete(&model.Feedback{})

	return res, nil
}

// randPast 返回最近 days 天内的一个随机时刻,用于把演示数据铺满时间轴。
func randPast(rng *mrand.Rand, now time.Time, days int) time.Time {
	if days < 1 {
		days = 1
	}
	offsetSec := rng.Intn(days * 24 * 3600)
	return now.Add(-time.Duration(offsetSec) * time.Second)
}
