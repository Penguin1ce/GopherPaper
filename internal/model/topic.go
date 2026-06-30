package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Topic 是一组语义相近会话的主题归类，归属某个用户，主键为 UUID。
// Centroid 存该主题成员会话(归一化)向量之和(JSON 编码的 []float64)，质心方向即 normalize(Centroid)；
// 用和而非均值便于加入/扣减/合并都做精确可逆的加减(余弦比较与长度无关，无需显式归一化)。
// MemberCount 记成员会话数，与 Centroid 配合在重排时从旧主题精确扣减某会话贡献。
type Topic struct {
	ID        string `gorm:"size:36;primaryKey" json:"id"`
	StudentID string `gorm:"size:64;index;not null" json:"student_id"`
	// AgentType 标记主题所属 agent，当前仅 pioneer，为将来扩展留口。
	AgentType   string         `gorm:"size:16;index" json:"agent_type,omitempty"`
	Name        string         `gorm:"size:64" json:"name"`
	Centroid    string         `gorm:"type:longtext" json:"-"`
	MemberCount int            `gorm:"not null;default:0" json:"member_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Topic) TableName() string { return "topics" }

// BeforeCreate 在入库前自动生成 UUID 主键。
func (t *Topic) BeforeCreate(*gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}
