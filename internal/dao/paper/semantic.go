package paper

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
)

func ReplaceSemanticRelations(ctx context.Context, ownerID, sourcePaperID string, relations []model.PaperSemanticRelation) error {
	now := time.Now()
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_id = ? and source_paper_id = ?", ownerID, sourcePaperID).
			Delete(&model.PaperSemanticRelation{}).Error; err != nil {
			return fmt.Errorf("dao/paper: delete semantic relations failed: %w", err)
		}
		if len(relations) == 0 {
			return nil
		}
		for i := range relations {
			relations[i].OwnerID = ownerID
			relations[i].SourcePaperID = sourcePaperID
			if relations[i].RelationType == "" {
				relations[i].RelationType = "SEMANTIC_SIMILAR"
			}
			relations[i].UpdatedAt = now
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "owner_id"},
				{Name: "source_paper_id"},
				{Name: "target_paper_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"relation_type",
				"score",
				"matched_fields",
				"summary",
				"keyword_similarity",
				"research_question_similarity",
				"method_similarity",
				"experiment_similarity",
				"innovation_similarity",
				"model",
				"updated_at",
			}),
		}).Create(&relations).Error; err != nil {
			return fmt.Errorf("dao/paper: save semantic relations failed: %w", err)
		}
		return nil
	})
}
