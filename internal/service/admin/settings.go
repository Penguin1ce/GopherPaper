package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// defaultSettings 是首次访问时自动落库的默认系统设置。
var defaultSettings = []model.SystemSetting{
	{Key: "site.registration_open", Value: "true", Group: "站点", Label: "开放用户注册", Type: "bool"},
	{Key: "site.max_upload_mb", Value: "80", Group: "站点", Label: "单文件上传上限(MB)", Type: "number"},
	{Key: "site.banner_text", Value: "", Group: "站点", Label: "顶部横幅文案", Type: "string"},
	{Key: "agent.owl_enabled", Value: "true", Group: "智能体", Label: "启用小文鸮(论文精读)", Type: "bool"},
	{Key: "agent.lark_enabled", Value: "true", Group: "智能体", Label: "启用小云雀(先锋工具)", Type: "bool"},
	{Key: "agent.gopher_enabled", Value: "true", Group: "智能体", Label: "启用小囊鼠(知识管理)", Type: "bool"},
	{Key: "report.concurrency", Value: "6", Group: "研读报告", Label: "报告生成并发度", Type: "number"},
	{Key: "report.auto_generate", Value: "true", Group: "研读报告", Label: "上传后自动预生成报告", Type: "bool"},
	{Key: "rag.top_k", Value: "8", Group: "检索", Label: "RAG 召回条数 top_k", Type: "number"},
	{Key: "rag.rerank_enabled", Value: "true", Group: "检索", Label: "启用精排 rerank", Type: "bool"},
}

// ListSettings 确保默认设置已落库,返回全部设置(按分组排序)。
func ListSettings(ctx context.Context) (*dto.AdminSettingsResponse, error) {
	db := dao.DB.WithContext(ctx)
	for _, s := range defaultSettings {
		s.UpdatedAt = time.Now()
		if err := db.Where("`key` = ?", s.Key).FirstOrCreate(&s).Error; err != nil {
			return nil, fmt.Errorf("admin: 初始化设置失败: %w", err)
		}
	}

	var rows []model.SystemSetting
	if err := db.Order("`group`, `key`").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询设置失败: %w", err)
	}
	out := &dto.AdminSettingsResponse{}
	for _, r := range rows {
		out.Items = append(out.Items, dto.AdminSettingItem{
			Key:   r.Key,
			Value: r.Value,
			Group: r.Group,
			Label: r.Label,
			Type:  r.Type,
		})
	}
	return out, nil
}

// UpdateSettings 批量更新设置值,只改已存在的键。
func UpdateSettings(ctx context.Context, req dto.AdminSettingsUpdateRequest) error {
	db := dao.DB.WithContext(ctx)
	for _, item := range req.Items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		if err := db.Model(&model.SystemSetting{}).
			Where("`key` = ?", key).
			Updates(map[string]any{"value": item.Value, "updated_at": time.Now()}).Error; err != nil {
			return fmt.Errorf("admin: 更新设置失败: %w", err)
		}
	}
	return nil
}
