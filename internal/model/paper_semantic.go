package model

import "time"

type PaperSemanticRelation struct {
	ID                         uint64      `gorm:"primaryKey" json:"id"`
	OwnerID                    string      `gorm:"size:64;not null;index;uniqueIndex:idx_owner_semantic_pair" json:"owner_id"`
	SourcePaperID              string      `gorm:"size:36;not null;index;uniqueIndex:idx_owner_semantic_pair" json:"source_paper_id"`
	TargetPaperID              string      `gorm:"size:36;not null;index;uniqueIndex:idx_owner_semantic_pair" json:"target_paper_id"`
	RelationType               string      `gorm:"size:64;not null;default:SEMANTIC_SIMILAR" json:"relation_type"`
	Score                      float64     `json:"score"`
	MatchedFields              JSONStrings `gorm:"type:text" json:"matched_fields"`
	Summary                    string      `gorm:"type:text" json:"summary"`
	KeywordSimilarity          string      `gorm:"type:text" json:"keyword_similarity"`
	ResearchQuestionSimilarity string      `gorm:"type:text" json:"research_question_similarity"`
	MethodSimilarity           string      `gorm:"type:text" json:"method_similarity"`
	ExperimentSimilarity       string      `gorm:"type:text" json:"experiment_similarity"`
	InnovationSimilarity       string      `gorm:"type:text" json:"innovation_similarity"`
	Model                      string      `gorm:"size:128" json:"model"`
	CreatedAt                  time.Time   `json:"created_at"`
	UpdatedAt                  time.Time   `json:"updated_at"`
}

func (PaperSemanticRelation) TableName() string { return "paper_semantic_relations" }
