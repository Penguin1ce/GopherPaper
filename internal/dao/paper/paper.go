// Package paper 是论文及其元信息、章节、标签的数据访问层，复用 dao.DB。
// 论文软删，解析状态由 worker 逐步流转。
package paper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// Create 新建论文记录，UUID 主键与默认状态由模型钩子生成。
func Create(ctx context.Context, p *model.Paper) error {
	if err := dao.DB.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("dao/paper: 创建论文失败: %w", err)
	}
	return nil
}

// Get 按 id 取论文，不存在返回 errs.ErrPaperNotFound。
func Get(ctx context.Context, id string) (*model.Paper, error) {
	var p model.Paper
	err := dao.DB.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrPaperNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询论文失败: %w", err)
	}
	return &p, nil
}

// List 按创建时间倒序列出某用户的全部论文。
func List(ctx context.Context, ownerID string) ([]model.Paper, error) {
	var papers []model.Paper
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("created_at desc").
		Find(&papers).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询论文列表失败: %w", err)
	}
	if err := attachKeywords(ctx, papers); err != nil {
		return nil, err
	}
	return papers, nil
}

// ListByStatus 列出某状态的全部论文,跨用户,供知识图谱回填等离线任务用。
func ListByStatus(ctx context.Context, status constant.PaperStatus) ([]model.Paper, error) {
	var papers []model.Paper
	err := dao.DB.WithContext(ctx).Where("status = ?", status).Find(&papers).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 按状态查询论文失败: %w", err)
	}
	return papers, nil
}

// ListAll 列出全部未删除论文,供离线维护任务按本地归档重建索引。
func ListAll(ctx context.Context) ([]model.Paper, error) {
	var papers []model.Paper
	err := dao.DB.WithContext(ctx).Order("created_at desc").Find(&papers).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询全部论文失败: %w", err)
	}
	return papers, nil
}

// attachKeywords 批量回填论文关键词。关键词在 paper_meta 表，列表不联表故单查回填，供前端做关键词筛选。
func attachKeywords(ctx context.Context, papers []model.Paper) error {
	if len(papers) == 0 {
		return nil
	}
	ids := make([]string, 0, len(papers))
	for i := range papers {
		ids = append(ids, papers[i].ID)
	}
	var metas []model.PaperMeta
	err := dao.DB.WithContext(ctx).
		Select("paper_id", "keywords").
		Where("paper_id in ?", ids).
		Find(&metas).Error
	if err != nil {
		return fmt.Errorf("dao/paper: 查询关键词失败: %w", err)
	}
	kw := make(map[string]model.JSONStrings, len(metas))
	for _, m := range metas {
		kw[m.PaperID] = m.Keywords
	}
	for i := range papers {
		papers[i].Keywords = kw[papers[i].ID]
	}
	return nil
}

// Search 在某用户论文里按标题、文件名、作者或关键词模糊检索，做历史文献检索。
func Search(ctx context.Context, ownerID, keyword string) ([]model.Paper, error) {
	var papers []model.Paper
	like := "%" + keyword + "%"
	err := dao.DB.WithContext(ctx).
		Model(&model.Paper{}).
		Select("papers.*").
		Joins("left join paper_metas on paper_metas.paper_id = papers.id").
		Where("papers.owner_id = ? and (papers.title like ? or papers.file_name like ? or paper_metas.authors like ? or paper_metas.keywords like ?)", ownerID, like, like, like, like).
		Order("papers.created_at desc").
		Find(&papers).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 检索论文失败: %w", err)
	}
	if err := attachKeywords(ctx, papers); err != nil {
		return nil, err
	}
	return papers, nil
}

// UpdateStatus 流转解析状态，failReason 仅失败态有意义。
func UpdateStatus(ctx context.Context, id string, status constant.PaperStatus, failReason string) error {
	fields := map[string]any{"status": status, "fail_reason": failReason}
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("dao/paper: 更新状态失败: %w", err)
	}
	return nil
}

// UpdateInfo 抽取完成后回填标题与页数。
func UpdateInfo(ctx context.Context, id, title string, pageCount int) error {
	fields := map[string]any{}
	if title != "" {
		fields["title"] = title
	}
	if pageCount > 0 {
		fields["page_count"] = pageCount
	}
	if len(fields) == 0 {
		return nil
	}
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("dao/paper: 回填论文信息失败: %w", err)
	}
	return nil
}

// UpdateParseProgress records MinerU parse progress 0-100.
func UpdateParseProgress(ctx context.Context, id string, progress, parsedPages, totalPages int) error {
	fields := map[string]any{
		"parse_progress": progress,
		"parsed_pages":   parsedPages,
		"total_pages":    totalPages,
	}
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("dao/paper: update parse progress failed: %w", err)
	}
	return nil
}

// UpdateProgress 记录阅读进度 0-100 与最近阅读页。
func UpdateProgress(ctx context.Context, id string, progress, lastReadPage int) error {
	fields := map[string]any{"progress": progress, "last_read_page": lastReadPage}
	if err := dao.DB.WithContext(ctx).Model(&model.Paper{}).Where("id = ?", id).
		Updates(fields).Error; err != nil {
		return fmt.Errorf("dao/paper: 更新阅读进度失败: %w", err)
	}
	return nil
}

// Delete 软删论文并清理其元信息、章节、标签关联。
func Delete(ctx context.Context, id string) error {
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperMeta{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperSection{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperReport{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperFlowCache{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperAnnotation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", id).Delete(&model.MindMap{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", id).Delete(&model.PaperTag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("source_paper_id = ? or target_paper_id = ?", id, id).Delete(&model.PaperSemanticRelation{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.Paper{}).Error
	})
}

// SaveMeta 写入或覆盖论文结构化元信息。
func SaveMeta(ctx context.Context, meta *model.PaperMeta) error {
	meta.UpdatedAt = time.Now()
	err := dao.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "paper_id"}},
		UpdateAll: true,
	}).Create(meta).Error
	if err != nil {
		return fmt.Errorf("dao/paper: 写入元信息失败: %w", err)
	}
	return nil
}

// GetMeta 取论文元信息，不存在返回 errs.ErrPaperNotFound。
func GetMeta(ctx context.Context, paperID string) (*model.PaperMeta, error) {
	var m model.PaperMeta
	err := dao.DB.WithContext(ctx).Where("paper_id = ?", paperID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrPaperNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询元信息失败: %w", err)
	}
	return &m, nil
}

// GetReport 取某篇论文某类研读报告缓存，不存在返回 errs.ErrPaperNotFound。
func GetReport(ctx context.Context, paperID string, t constant.ReportType) (*model.PaperReport, error) {
	var r model.PaperReport
	err := dao.DB.WithContext(ctx).Where("paper_id = ? and report_type = ?", paperID, t).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrReportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询研读报告失败: %w", err)
	}
	return &r, nil
}

// ListReportTypes 列出某篇论文已落库的研读报告类型，供前端进页面时回填就绪态。
func ListReportTypes(ctx context.Context, paperID string) ([]constant.ReportType, error) {
	var types []constant.ReportType
	err := dao.DB.WithContext(ctx).
		Model(&model.PaperReport{}).
		Where("paper_id = ?", paperID).
		Pluck("report_type", &types).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 列出研读报告类型失败: %w", err)
	}
	return types, nil
}

// DeleteReports 删除某篇论文已生成的研读报告缓存记录。
func DeleteReports(ctx context.Context, paperID string) error {
	if err := dao.DB.WithContext(ctx).Where("paper_id = ?", paperID).Delete(&model.PaperReport{}).Error; err != nil {
		return fmt.Errorf("dao/paper: 删除研读报告失败: %w", err)
	}
	return nil
}

// SaveReport 写入或覆盖某篇论文某类研读报告缓存，按 (paper_id, report_type) 幂等。
func SaveReport(ctx context.Context, r *model.PaperReport) error {
	err := dao.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "paper_id"}, {Name: "report_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "meta", "updated_at"}),
	}).Create(r).Error
	if err != nil {
		return fmt.Errorf("dao/paper: 写入研读报告失败: %w", err)
	}
	return nil
}

// GetPaperFlow 取某篇论文的小云雀同款思路图缓存。
func GetPaperFlow(ctx context.Context, ownerID, paperID string) (*model.PaperFlowCache, error) {
	var r model.PaperFlowCache
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ? and paper_id = ?", ownerID, paperID).
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrPaperFlowNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询论文思路图失败: %w", err)
	}
	return &r, nil
}

// HasPaperFlow 判断某篇论文是否已有思路图缓存,不读取大 JSON。
func HasPaperFlow(ctx context.Context, ownerID, paperID string) (bool, error) {
	var id uint64
	err := dao.DB.WithContext(ctx).
		Model(&model.PaperFlowCache{}).
		Select("id").
		Where("owner_id = ? and paper_id = ?", ownerID, paperID).
		Limit(1).
		Scan(&id).Error
	if err != nil {
		return false, fmt.Errorf("dao/paper: 查询论文思路图状态失败: %w", err)
	}
	return id > 0, nil
}

// SavePaperFlow 写入或覆盖某篇论文的小云雀同款思路图缓存。
func SavePaperFlow(ctx context.Context, r *model.PaperFlowCache) error {
	err := dao.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_id"}, {Name: "paper_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"flow_json", "updated_at"}),
	}).Create(r).Error
	if err != nil {
		return fmt.Errorf("dao/paper: 写入论文思路图失败: %w", err)
	}
	return nil
}

// SaveSections 覆盖论文章节，先删后插保证幂等。
// SaveCompareReport 写入一份多论文对比报告历史记录。
func SaveCompareReport(ctx context.Context, r *model.PaperCompareReport) error {
	if err := dao.DB.WithContext(ctx).Create(r).Error; err != nil {
		return fmt.Errorf("dao/paper: 写入多论文对比报告失败: %w", err)
	}
	return nil
}

// ListCompareReports 按更新时间倒序列出用户的多论文对比报告。
func ListCompareReports(ctx context.Context, ownerID string) ([]model.PaperCompareReport, error) {
	var reports []model.PaperCompareReport
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("updated_at desc, id desc").
		Find(&reports).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询多论文对比报告失败: %w", err)
	}
	return reports, nil
}

// GetCompareReport 按 ID 获取多论文对比报告。
func GetCompareReport(ctx context.Context, id uint64) (*model.PaperCompareReport, error) {
	var report model.PaperCompareReport
	err := dao.DB.WithContext(ctx).Where("id = ?", id).First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrReportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询多论文对比报告失败: %w", err)
	}
	return &report, nil
}

// DeleteCompareReport 删除一份多论文对比报告。
func DeleteCompareReport(ctx context.Context, id uint64) error {
	if err := dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&model.PaperCompareReport{}).Error; err != nil {
		return fmt.Errorf("dao/paper: 删除多论文对比报告失败: %w", err)
	}
	return nil
}

func SaveSections(ctx context.Context, paperID string, sections []model.PaperSection) error {
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("paper_id = ?", paperID).Delete(&model.PaperSection{}).Error; err != nil {
			return err
		}
		if len(sections) == 0 {
			return nil
		}
		return tx.Create(&sections).Error
	})
}

// ListSections 按文档顺序列出论文章节。
func ListSections(ctx context.Context, paperID string) ([]model.PaperSection, error) {
	var sections []model.PaperSection
	err := dao.DB.WithContext(ctx).
		Where("paper_id = ?", paperID).
		Order("order_idx asc").
		Find(&sections).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询章节失败: %w", err)
	}
	return sections, nil
}

// ListAnnotations 按页码与更新时间列出某篇论文的全部精读批注。
func ListAnnotations(ctx context.Context, paperID string) ([]model.PaperAnnotation, error) {
	var annotations []model.PaperAnnotation
	err := dao.DB.WithContext(ctx).
		Where("paper_id = ?", paperID).
		Order("page_no asc, updated_at desc, id desc").
		Find(&annotations).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询批注失败: %w", err)
	}
	return annotations, nil
}

// CreateAnnotation 新建一条精读批注。
func CreateAnnotation(ctx context.Context, annotation *model.PaperAnnotation) error {
	if err := dao.DB.WithContext(ctx).Create(annotation).Error; err != nil {
		return fmt.Errorf("dao/paper: 创建批注失败: %w", err)
	}
	return nil
}

// GetAnnotation 按 id 取一条批注，不存在返回 errs.ErrAnnotationNotFound。
func GetAnnotation(ctx context.Context, id uint64) (*model.PaperAnnotation, error) {
	var annotation model.PaperAnnotation
	err := dao.DB.WithContext(ctx).Where("id = ?", id).First(&annotation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrAnnotationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询批注失败: %w", err)
	}
	return &annotation, nil
}

// UpdateAnnotation 更新批注的可编辑字段。
func UpdateAnnotation(ctx context.Context, id uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	if err := dao.DB.WithContext(ctx).Model(&model.PaperAnnotation{}).
		Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("dao/paper: 更新批注失败: %w", err)
	}
	return nil
}

// DeleteAnnotation 删除一条批注。
func DeleteAnnotation(ctx context.Context, id uint64) error {
	if err := dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&model.PaperAnnotation{}).Error; err != nil {
		return fmt.Errorf("dao/paper: 删除批注失败: %w", err)
	}
	return nil
}

func GetMindMap(ctx context.Context, id uint64) (*model.MindMap, error) {
	var mindMap model.MindMap
	err := dao.DB.WithContext(ctx).Where("id = ?", id).First(&mindMap).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrMindMapNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: get mind map failed: %w", err)
	}
	return &mindMap, nil
}

func GetMindMapByPaper(ctx context.Context, ownerID, paperID string) (*model.MindMap, error) {
	var mindMap model.MindMap
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ? and paper_id = ?", ownerID, paperID).
		First(&mindMap).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrMindMapNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/paper: get paper mind map failed: %w", err)
	}
	return &mindMap, nil
}

func SaveMindMap(ctx context.Context, mindMap *model.MindMap) error {
	var existing model.MindMap
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ? and paper_id = ?", mindMap.OwnerID, mindMap.PaperID).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := dao.DB.WithContext(ctx).Create(mindMap).Error; err != nil {
			return fmt.Errorf("dao/paper: create mind map failed: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("dao/paper: save mind map failed: %w", err)
	}
	mindMap.ID = existing.ID
	return UpdateMindMapGraph(ctx, existing.ID, mindMap.GraphJSON)
}

func UpdateMindMapGraph(ctx context.Context, id uint64, graph model.MindMapGraph) error {
	if err := dao.DB.WithContext(ctx).Model(&model.MindMap{}).
		Where("id = ?", id).
		Updates(map[string]any{"graph_json": graph, "updated_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("dao/paper: update mind map failed: %w", err)
	}
	return nil
}
