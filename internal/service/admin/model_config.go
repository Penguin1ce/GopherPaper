package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/internal/secret"
	"GopherPaper/pkg/constant"
)

const (
	modelConfigSourceConfig   = "config"
	modelConfigSourceDatabase = "database"
	modelConfigStatusOK       = "ok"
	modelConfigStatusFailed   = "failed"
)

var (
	runtimeConfigMu sync.RWMutex
	runtimeConfig   *config.Config
	baseConfigMu    sync.RWMutex
	baseConfig      *config.Config

	ErrModelRoleInvalid          = errors.New("admin model config: invalid role")
	ErrModelConfigInvalid        = errors.New("admin model config: invalid")
	ErrModelRuntimeNotReady      = errors.New("admin model config: runtime config is not initialized")
	ErrModelConfigConnectionFail = errors.New("admin model config: connection test failed")
)

type modelRoleSpec struct {
	Role        constant.ModelRole
	Label       string
	Description string
	Kind        string
}

var modelRoleSpecs = []modelRoleSpec{
	{Role: constant.ModelRoleChat, Label: "主问答模型", Description: "负责论文问答、结构化抽取和报告生成，是系统主力模型。", Kind: "chat"},
	{Role: constant.ModelRolePioneer, Label: "小云雀模型", Description: "负责开放式学术检索与工具调用，建议选择响应快、工具调用稳定的模型。", Kind: "chat"},
	{Role: constant.ModelRoleMaodie, Label: "小耄耋模型", Description: "负责精读页局部问答，配置为空时回退主问答模型。", Kind: "chat"},
	{Role: constant.ModelRoleIntent, Label: "意图分类模型", Description: "负责判断用户问题类型，建议使用低延迟小模型。", Kind: "chat"},
	{Role: constant.ModelRoleVLM, Label: "视觉理解模型", Description: "负责图片描述生成和带图问答，必须选择支持图像输入的模型。", Kind: "chat"},
	{Role: constant.ModelRoleTranslate, Label: "翻译模型", Description: "负责精读页逐段翻译，建议使用快速低成本模型。", Kind: "chat"},
	{Role: constant.ModelRoleEmbedding, Label: "Embedding 模型", Description: "负责论文向量化入库，维度变化需要重启并重建向量索引。", Kind: "embedding"},
	{Role: constant.ModelRoleRerank, Label: "Rerank 模型", Description: "负责 RAG 检索结果重排，可关闭，失败时系统退化为纯向量召回。", Kind: "rerank"},
}

func ApplyStoredModelConfigs(ctx context.Context, cfg *config.Config) error {
	setBaseConfig(cfg)
	aimodel.SetUserConfigResolver(ConfigForUser)
	rows, err := loadModelConfigRows(ctx)
	if err != nil {
		return err
	}
	if _, _, err := overlayModelConfigRows(cfg, rows, true); err != nil {
		return err
	}
	setRuntimeConfig(cfg)
	return nil
}

func ListModelConfigs(ctx context.Context) (*dto.AdminModelConfigsResponse, error) {
	activeCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	baseCfg, err := currentBaseConfig()
	if err != nil {
		return nil, err
	}
	rows, err := loadModelConfigRows(ctx)
	if err != nil {
		return nil, err
	}
	byRole := rowsByRole(rows)
	items := make([]dto.AdminModelConfigItem, 0, len(modelRoleSpecs))
	for _, spec := range modelRoleSpecs {
		if row, ok := byRole[string(spec.Role)]; ok {
			items = append(items, itemFromRow(row, spec, activeCfg))
			continue
		}
		items = append(items, itemFromConfig(baseCfg, spec, activeCfg))
	}
	return &dto.AdminModelConfigsResponse{Items: items}, nil
}

func UpdateModelConfig(ctx context.Context, role string, req dto.AdminModelConfigUpdateRequest, adminID uint) (*dto.AdminModelConfigItem, error) {
	spec, ok := findModelRoleSpec(role)
	if !ok {
		return nil, ErrModelRoleInvalid
	}
	cfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	row, exists, err := getModelConfigRow(ctx, role)
	if err != nil {
		return nil, err
	}
	if !exists {
		row = rowFromConfig(cfg, spec)
	}
	applyModelConfigRequest(&row, req, spec)
	if req.APIKey != nil {
		row.APIKey = strings.TrimSpace(*req.APIKey)
	}
	row.Role = role
	row.UpdatedBy = adminID
	if isFallbackRole(spec) && isBlankModelConfigRow(row) {
		if err := dao.DB.WithContext(ctx).Where("role = ?", role).Delete(&model.SystemModelConfig{}).Error; err != nil {
			return nil, fmt.Errorf("admin model config: restore fallback failed: %w", err)
		}
		return ptr(itemFromConfig(cfg, spec, cfg)), nil
	}
	if err := validateModelConfigRow(&row, spec); err != nil {
		return nil, err
	}
	saveRow := row
	if err := protectModelConfigAPIKey(ctx, &saveRow); err != nil {
		return nil, err
	}
	if err := dao.DB.WithContext(ctx).Save(&saveRow).Error; err != nil {
		return nil, fmt.Errorf("admin model config: save failed: %w", err)
	}
	return ptr(itemFromRow(row, spec, cfg)), nil
}

func RestoreModelConfig(ctx context.Context, role string) (*dto.AdminModelConfigItem, error) {
	spec, ok := findModelRoleSpec(role)
	if !ok {
		return nil, ErrModelRoleInvalid
	}
	if err := dao.DB.WithContext(ctx).Where("role = ?", role).Delete(&model.SystemModelConfig{}).Error; err != nil {
		return nil, fmt.Errorf("admin model config: restore failed: %w", err)
	}
	activeCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	baseCfg, err := currentBaseConfig()
	if err != nil {
		return nil, err
	}
	return ptr(itemFromConfig(baseCfg, spec, activeCfg)), nil
}

func TestModelConfig(ctx context.Context, role string) (*dto.AdminModelConfigTestResponse, error) {
	spec, ok := findModelRoleSpec(role)
	if !ok {
		return nil, ErrModelRoleInvalid
	}
	cfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	row, exists, err := getModelConfigRow(ctx, role)
	if err != nil {
		return nil, err
	}
	if !exists {
		row = rowFromConfig(cfg, spec)
	}
	if isFallbackRole(spec) && isBlankModelConfigRow(row) {
		message := fmt.Sprintf("%s 未配置独立模型，当前会回退主问答模型，不影响使用。", spec.Label)
		if exists {
			now := time.Now()
			_ = dao.DB.WithContext(ctx).Model(&row).Updates(map[string]any{
				"last_test_status":     modelConfigStatusOK,
				"last_test_error":      "",
				"last_test_at":         &now,
				"last_test_latency_ms": 0,
			}).Error
		}
		return &dto.AdminModelConfigTestResponse{OK: true, Message: message, LatencyMS: 0}, nil
	}
	if err := validateModelConfigRow(&row, spec); err != nil {
		return nil, err
	}
	started := time.Now()
	testErr := testModelConfigConnection(ctx, row, spec)
	latencyMS := time.Since(started).Milliseconds()
	status := modelConfigStatusOK
	message := "连接测试通过"
	if testErr != nil {
		status = modelConfigStatusFailed
		message = testErr.Error()
	}
	if exists {
		now := time.Now()
		updates := map[string]any{
			"last_test_status":     status,
			"last_test_error":      "",
			"last_test_at":         &now,
			"last_test_latency_ms": latencyMS,
		}
		if testErr != nil {
			updates["last_test_error"] = truncateText(message, 1024)
		}
		_ = dao.DB.WithContext(ctx).Model(&row).Updates(updates).Error
	}
	return &dto.AdminModelConfigTestResponse{OK: testErr == nil, Message: message, LatencyMS: latencyMS}, nil
}

func ApplyModelConfigs(ctx context.Context) (*dto.AdminModelConfigApplyResponse, error) {
	rows, err := loadModelConfigRows(ctx)
	if err != nil {
		return nil, err
	}
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()
	if runtimeConfig == nil {
		return nil, ErrModelRuntimeNotReady
	}
	baseCfg, err := currentBaseConfigLocked()
	if err != nil {
		return nil, err
	}
	next := *baseCfg
	next.Embedding = runtimeConfig.Embedding
	applied, warnings, err := overlayModelConfigRows(&next, rows, false)
	if err != nil {
		return nil, err
	}
	runtimeConfig = &next
	aimodel.Init(runtimeConfig)
	if err := ai.ReloadModels(ctx, aimodel.NewReranker(runtimeConfig.Rerank), runtimeConfig.Redis, runtimeConfig.Models.Chat); err != nil {
		return nil, err
	}
	return &dto.AdminModelConfigApplyResponse{AppliedRoles: applied, Warnings: warnings}, nil
}

func setRuntimeConfig(cfg *config.Config) {
	ensureBaseConfig(cfg)
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()
	runtimeConfig = cfg
}

func setBaseConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	cp := *cfg
	baseConfigMu.Lock()
	defer baseConfigMu.Unlock()
	baseConfig = &cp
}

func ensureBaseConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	baseConfigMu.Lock()
	defer baseConfigMu.Unlock()
	if baseConfig != nil {
		return
	}
	cp := *cfg
	baseConfig = &cp
}

func currentRuntimeConfig() (*config.Config, error) {
	runtimeConfigMu.RLock()
	defer runtimeConfigMu.RUnlock()
	if runtimeConfig == nil {
		return nil, ErrModelRuntimeNotReady
	}
	cp := *runtimeConfig
	return &cp, nil
}

func currentBaseConfig() (*config.Config, error) {
	baseConfigMu.RLock()
	defer baseConfigMu.RUnlock()
	if baseConfig == nil {
		return nil, ErrModelRuntimeNotReady
	}
	cp := *baseConfig
	return &cp, nil
}

func currentBaseConfigLocked() (*config.Config, error) {
	baseConfigMu.RLock()
	defer baseConfigMu.RUnlock()
	if baseConfig == nil {
		return nil, ErrModelRuntimeNotReady
	}
	cp := *baseConfig
	return &cp, nil
}

func loadModelConfigRows(ctx context.Context) ([]model.SystemModelConfig, error) {
	var rows []model.SystemModelConfig
	if err := dao.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin model config: query failed: %w", err)
	}
	for i := range rows {
		row, err := prepareModelConfigRow(ctx, rows[i])
		if err != nil {
			return nil, err
		}
		rows[i] = row
	}
	return rows, nil
}

func getModelConfigRow(ctx context.Context, role string) (model.SystemModelConfig, bool, error) {
	var row model.SystemModelConfig
	err := dao.DB.WithContext(ctx).Where("role = ?", role).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	if err != nil {
		return row, false, fmt.Errorf("admin model config: query role failed: %w", err)
	}
	row, err = prepareModelConfigRow(ctx, row)
	if err != nil {
		return row, false, err
	}
	return row, true, nil
}

func rowsByRole(rows []model.SystemModelConfig) map[string]model.SystemModelConfig {
	out := make(map[string]model.SystemModelConfig, len(rows))
	for _, row := range rows {
		out[row.Role] = row
	}
	return out
}

func prepareModelConfigRow(ctx context.Context, row model.SystemModelConfig) (model.SystemModelConfig, error) {
	if strings.TrimSpace(row.APIKey) != "" {
		plain := strings.TrimSpace(row.APIKey)
		saveRow := row
		saveRow.APIKey = plain
		if err := protectModelConfigAPIKey(ctx, &saveRow); err != nil {
			return row, err
		}
		now := time.Now()
		updates := map[string]any{
			"api_key":             "",
			"api_key_ciphertext":  saveRow.APIKeyCiphertext,
			"api_key_nonce":       saveRow.APIKeyNonce,
			"api_key_key_id":      saveRow.APIKeyKeyID,
			"api_key_key_version": saveRow.APIKeyKeyVersion,
			"api_key_algorithm":   saveRow.APIKeyAlgorithm,
			"api_key_mask":        saveRow.APIKeyMask,
			"api_key_fingerprint": saveRow.APIKeyFingerprint,
			"api_key_migrated_at": &now,
		}
		if err := dao.DB.WithContext(ctx).Model(&model.SystemModelConfig{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return row, fmt.Errorf("admin model config: migrate api key failed: %w", err)
		}
		saveRow.APIKey = plain
		saveRow.APIKeyMigratedAt = &now
		return saveRow, nil
	}

	if strings.TrimSpace(row.APIKeyCiphertext) == "" {
		return row, nil
	}
	m, err := secret.Default()
	if err != nil {
		return row, fmt.Errorf("admin model config: secret manager unavailable: %w", err)
	}
	plain, err := m.Decrypt(ctx, modelConfigSecretPurpose(row.Role), modelConfigCiphertext(row))
	if err != nil {
		return row, fmt.Errorf("admin model config: decrypt api key failed for role %s: %w", row.Role, err)
	}
	row.APIKey = plain
	if row.APIKeyMask == "" {
		row.APIKeyMask = m.Mask(plain)
	}
	if row.APIKeyFingerprint == "" {
		row.APIKeyFingerprint = m.Fingerprint(plain)
	}
	return row, nil
}

func protectModelConfigAPIKey(ctx context.Context, row *model.SystemModelConfig) error {
	plain := strings.TrimSpace(row.APIKey)
	if plain == "" {
		row.APIKey = ""
		row.APIKeyCiphertext = ""
		row.APIKeyNonce = ""
		row.APIKeyKeyID = ""
		row.APIKeyKeyVersion = ""
		row.APIKeyAlgorithm = ""
		row.APIKeyMask = ""
		row.APIKeyFingerprint = ""
		row.APIKeyMigratedAt = nil
		return nil
	}
	m, err := secret.Default()
	if err != nil {
		return fmt.Errorf("admin model config: secret manager unavailable: %w", err)
	}
	ciphertext, err := m.Encrypt(ctx, modelConfigSecretPurpose(row.Role), plain)
	if err != nil {
		return fmt.Errorf("admin model config: encrypt api key failed: %w", err)
	}
	row.APIKey = ""
	row.APIKeyCiphertext = ciphertext.Value
	row.APIKeyNonce = ciphertext.Nonce
	row.APIKeyKeyID = ciphertext.KeyID
	row.APIKeyKeyVersion = ciphertext.KeyVersion
	row.APIKeyAlgorithm = ciphertext.Algorithm
	row.APIKeyMask = m.Mask(plain)
	row.APIKeyFingerprint = m.Fingerprint(plain)
	now := time.Now()
	row.APIKeyMigratedAt = &now
	return nil
}

func modelConfigCiphertext(row model.SystemModelConfig) secret.Ciphertext {
	return secret.Ciphertext{
		Value:      row.APIKeyCiphertext,
		Nonce:      row.APIKeyNonce,
		KeyID:      row.APIKeyKeyID,
		KeyVersion: row.APIKeyKeyVersion,
		Algorithm:  row.APIKeyAlgorithm,
	}
}

func modelConfigSecretPurpose(role string) string {
	return "admin_model_config:" + strings.TrimSpace(role)
}

func overlayModelConfigRows(cfg *config.Config, rows []model.SystemModelConfig, includeEmbedding bool) ([]string, []string, error) {
	applied := make([]string, 0, len(rows))
	warnings := make([]string, 0)
	for _, row := range rows {
		spec, ok := findModelRoleSpec(row.Role)
		if !ok {
			return nil, nil, fmt.Errorf("%w: %s", ErrModelRoleInvalid, row.Role)
		}
		if err := validateModelConfigRow(&row, spec); err != nil {
			return nil, nil, err
		}
		if spec.Kind == "embedding" && !includeEmbedding {
			warnings = append(warnings, "Embedding 配置已保存，但需要重启后端后生效；如果维度变化，还需要重建向量索引。")
			continue
		}
		applyModelConfigRow(cfg, row, spec)
		applied = append(applied, row.Role)
	}
	return applied, warnings, nil
}

func findModelRoleSpec(role string) (modelRoleSpec, bool) {
	role = strings.TrimSpace(role)
	for _, spec := range modelRoleSpecs {
		if string(spec.Role) == role {
			return spec, true
		}
	}
	return modelRoleSpec{}, false
}

func rowFromConfig(cfg *config.Config, spec modelRoleSpec) model.SystemModelConfig {
	row := model.SystemModelConfig{Role: string(spec.Role)}
	switch spec.Role {
	case constant.ModelRoleIntent:
		fillRowFromModelConfig(&row, cfg.Models.Intent)
	case constant.ModelRoleChat:
		fillRowFromModelConfig(&row, cfg.Models.Chat)
	case constant.ModelRoleVLM:
		fillRowFromModelConfig(&row, cfg.Models.Vlm)
	case constant.ModelRoleTranslate:
		fillRowFromModelConfig(&row, cfg.Models.Translate)
	case constant.ModelRolePioneer:
		fillRowFromModelConfig(&row, cfg.Models.Pioneer)
	case constant.ModelRoleMaodie:
		fillRowFromModelConfig(&row, cfg.Models.Maodie)
	case constant.ModelRoleEmbedding:
		fillRowFromModelConfig(&row, cfg.Embedding)
	case constant.ModelRoleRerank:
		row.Provider = string(constant.ProviderOpenAI)
		row.BaseURL = cfg.Rerank.BaseURL
		row.APIKey = cfg.Rerank.APIKey
		row.Model = cfg.Rerank.Model
		row.Enabled = cfg.Rerank.Enabled
		row.Timeout = cfg.Rerank.Timeout
	}
	return row
}

func fillRowFromModelConfig(row *model.SystemModelConfig, mc config.ModelConfig) {
	row.Provider = string(mc.Provider)
	row.BaseURL = mc.BaseURL
	row.APIKey = mc.APIKey
	row.Model = mc.Model
	row.Dim = mc.Dim
	row.MaxTokens = mc.MaxTokens
	row.ReasoningEffort = mc.ReasoningEffort
	row.Thinking = mc.Thinking
}

func applyModelConfigRow(cfg *config.Config, row model.SystemModelConfig, spec modelRoleSpec) {
	switch spec.Role {
	case constant.ModelRoleIntent:
		cfg.Models.Intent = rowToModelConfig(row)
	case constant.ModelRoleChat:
		cfg.Models.Chat = rowToModelConfig(row)
	case constant.ModelRoleVLM:
		cfg.Models.Vlm = rowToModelConfig(row)
	case constant.ModelRoleTranslate:
		cfg.Models.Translate = rowToModelConfig(row)
	case constant.ModelRolePioneer:
		cfg.Models.Pioneer = rowToModelConfig(row)
	case constant.ModelRoleMaodie:
		cfg.Models.Maodie = rowToModelConfig(row)
	case constant.ModelRoleEmbedding:
		cfg.Embedding = rowToModelConfig(row)
	case constant.ModelRoleRerank:
		cfg.Rerank = config.RerankConfig{
			Enabled: row.Enabled,
			BaseURL: row.BaseURL,
			APIKey:  row.APIKey,
			Model:   row.Model,
			Timeout: row.Timeout,
		}
	}
}

func rowToModelConfig(row model.SystemModelConfig) config.ModelConfig {
	return config.ModelConfig{
		Provider:        constant.Provider(row.Provider),
		BaseURL:         row.BaseURL,
		APIKey:          row.APIKey,
		Model:           row.Model,
		Dim:             row.Dim,
		MaxTokens:       row.MaxTokens,
		ReasoningEffort: row.ReasoningEffort,
		Thinking:        row.Thinking,
	}
}

func applyModelConfigRequest(row *model.SystemModelConfig, req dto.AdminModelConfigUpdateRequest, spec modelRoleSpec) {
	row.Provider = strings.TrimSpace(req.Provider)
	row.BaseURL = strings.TrimSpace(req.BaseURL)
	row.Model = strings.TrimSpace(req.Model)
	row.Dim = req.Dim
	row.MaxTokens = req.MaxTokens
	row.ReasoningEffort = strings.TrimSpace(req.ReasoningEffort)
	row.Thinking = strings.TrimSpace(req.Thinking)
	if spec.Kind == "rerank" {
		row.Provider = string(constant.ProviderOpenAI)
		row.Enabled = req.Enabled
		row.Timeout = req.Timeout
	}
}

func isFallbackRole(spec modelRoleSpec) bool {
	return spec.Role == constant.ModelRolePioneer || spec.Role == constant.ModelRoleMaodie
}

func isBlankModelConfigRow(row model.SystemModelConfig) bool {
	return strings.TrimSpace(row.BaseURL) == "" &&
		strings.TrimSpace(row.Model) == ""
}

func validateModelConfigRow(row *model.SystemModelConfig, spec modelRoleSpec) error {
	switch spec.Kind {
	case "chat", "embedding":
		if row.Provider == "" {
			row.Provider = string(constant.ProviderOpenAI)
		}
		if row.Provider != string(constant.ProviderOpenAI) && row.Provider != string(constant.ProviderOllama) {
			return fmt.Errorf("%w: provider must be openai or ollama", ErrModelConfigInvalid)
		}
		if strings.TrimSpace(row.BaseURL) == "" {
			return fmt.Errorf("%w: base_url is required", ErrModelConfigInvalid)
		}
		if strings.TrimSpace(row.Model) == "" {
			return fmt.Errorf("%w: model is required", ErrModelConfigInvalid)
		}
		if row.MaxTokens < 0 {
			return fmt.Errorf("%w: max_tokens cannot be negative", ErrModelConfigInvalid)
		}
		if spec.Kind == "embedding" && row.Dim <= 0 {
			return fmt.Errorf("%w: embedding dim must be greater than 0", ErrModelConfigInvalid)
		}
	case "rerank":
		row.Provider = string(constant.ProviderOpenAI)
		if !row.Enabled {
			return nil
		}
		if strings.TrimSpace(row.BaseURL) == "" {
			return fmt.Errorf("%w: rerank base_url is required", ErrModelConfigInvalid)
		}
		if strings.TrimSpace(row.Model) == "" {
			return fmt.Errorf("%w: rerank model is required", ErrModelConfigInvalid)
		}
		if row.Timeout <= 0 {
			return fmt.Errorf("%w: rerank timeout must be greater than 0", ErrModelConfigInvalid)
		}
	}
	if row.BaseURL != "" {
		if err := validateHTTPURL(row.BaseURL); err != nil {
			return fmt.Errorf("%w: %v", ErrModelConfigInvalid, err)
		}
	}
	if row.Thinking != "" && row.Thinking != "enabled" && row.Thinking != "disabled" {
		return fmt.Errorf("%w: thinking must be enabled or disabled", ErrModelConfigInvalid)
	}
	if row.ReasoningEffort != "" &&
		row.ReasoningEffort != "low" &&
		row.ReasoningEffort != "medium" &&
		row.ReasoningEffort != "high" {
		return fmt.Errorf("%w: reasoning_effort must be low, medium or high", ErrModelConfigInvalid)
	}
	return nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base_url must start with http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("base_url host is required")
	}
	return nil
}

func itemFromConfig(cfg *config.Config, spec modelRoleSpec, activeCfg *config.Config) dto.AdminModelConfigItem {
	row := rowFromConfig(cfg, spec)
	item := itemFromRow(row, spec, activeCfg)
	item.Source = modelConfigSourceConfig
	item.Active = modelConfigRowActive(row, spec, activeCfg)
	item.RestartRequired = spec.Kind == "embedding" && !item.Active
	item.UpdatedAt = nil
	item.LastTestStatus = ""
	item.LastTestError = ""
	item.LastTestAt = nil
	return item
}

func itemFromRow(row model.SystemModelConfig, spec modelRoleSpec, activeCfg *config.Config) dto.AdminModelConfigItem {
	active := modelConfigRowActive(row, spec, activeCfg)
	updatedAt := row.UpdatedAt
	item := dto.AdminModelConfigItem{
		Role:              row.Role,
		Label:             spec.Label,
		Description:       spec.Description,
		Kind:              spec.Kind,
		Provider:          row.Provider,
		BaseURL:           row.BaseURL,
		Model:             row.Model,
		APIKeyMask:        modelConfigAPIKeyMask(row),
		HasAPIKey:         modelConfigHasAPIKey(row),
		Dim:               row.Dim,
		MaxTokens:         row.MaxTokens,
		ReasoningEffort:   row.ReasoningEffort,
		Thinking:          row.Thinking,
		Enabled:           row.Enabled,
		Timeout:           row.Timeout,
		Source:            modelConfigSourceDatabase,
		Active:            active,
		RestartRequired:   spec.Kind == "embedding" && !active,
		LastTestStatus:    row.LastTestStatus,
		LastTestError:     row.LastTestError,
		LastTestAt:        row.LastTestAt,
		LastTestLatencyMS: row.LastTestLatencyMS,
		UpdatedAt:         &updatedAt,
	}
	if spec.Kind == "embedding" {
		item.Warnings = append(item.Warnings, "Embedding 会影响论文入库和检索；维度变化后需要重启后端并重建向量索引。")
	}
	if spec.Kind == "rerank" && !row.Enabled {
		item.Warnings = append(item.Warnings, "Rerank 关闭后，问答会退化为纯向量召回。")
	}
	if spec.Role == constant.ModelRoleMaodie && strings.TrimSpace(row.Model) == "" {
		item.Warnings = append(item.Warnings, "小耄耋模型为空时会回退主问答模型。")
	}
	return item
}

func modelConfigRowActive(row model.SystemModelConfig, spec modelRoleSpec, cfg *config.Config) bool {
	active := rowFromConfig(cfg, spec)
	if spec.Kind == "rerank" {
		return active.Enabled == row.Enabled &&
			active.BaseURL == row.BaseURL &&
			active.APIKey == row.APIKey &&
			active.Model == row.Model &&
			active.Timeout == row.Timeout
	}
	return active.Provider == row.Provider &&
		active.BaseURL == row.BaseURL &&
		active.APIKey == row.APIKey &&
		active.Model == row.Model &&
		active.Dim == row.Dim &&
		active.MaxTokens == row.MaxTokens &&
		active.ReasoningEffort == row.ReasoningEffort &&
		active.Thinking == row.Thinking
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func modelConfigAPIKeyMask(row model.SystemModelConfig) string {
	if strings.TrimSpace(row.APIKey) != "" {
		return maskAPIKey(row.APIKey)
	}
	return strings.TrimSpace(row.APIKeyMask)
}

func modelConfigHasAPIKey(row model.SystemModelConfig) bool {
	return strings.TrimSpace(row.APIKey) != "" ||
		strings.TrimSpace(row.APIKeyCiphertext) != "" ||
		strings.TrimSpace(row.APIKeyMask) != ""
}

func testModelConfigConnection(ctx context.Context, row model.SystemModelConfig, spec modelRoleSpec) error {
	if spec.Kind == "rerank" {
		if !row.Enabled {
			return nil
		}
		return testRerankEndpoint(ctx, row)
	}
	return testOpenAICompatibleModelsEndpoint(ctx, row)
}

func testOpenAICompatibleModelsEndpoint(ctx context.Context, row model.SystemModelConfig) error {
	endpoint := strings.TrimRight(row.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return newModelConfigUserError("测试地址格式不正确，请检查 Base URL 是否是完整的 http/https 地址。", err)
	}
	if row.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+row.APIKey)
	}
	return doModelConfigTestRequest(req, modelConfigTestOptions{
		Provider: row.Provider,
		Model:    row.Model,
	})
}

func testRerankEndpoint(ctx context.Context, row model.SystemModelConfig) error {
	payload := map[string]any{
		"model":     row.Model,
		"query":     "ping",
		"documents": []string{"ping"},
		"top_n":     1,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, row.BaseURL, bytes.NewReader(body))
	if err != nil {
		return newModelConfigUserError("Rerank 测试地址格式不正确，请检查 Base URL 是否是完整的 http/https 地址。", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if row.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+row.APIKey)
	}
	return doModelConfigTestRequest(req, modelConfigTestOptions{
		Model:    row.Model,
		IsRerank: true,
	})
}

type modelConfigTestOptions struct {
	Provider string
	Model    string
	IsRerank bool
}

type modelConfigUserError struct {
	message string
	cause   error
}

func (e modelConfigUserError) Error() string {
	return e.message
}

func (e modelConfigUserError) Unwrap() error {
	return e.cause
}

func newModelConfigUserError(message string, cause error) error {
	return modelConfigUserError{message: message, cause: cause}
}

func doModelConfigTestRequest(req *http.Request, opts modelConfigTestOptions) error {
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return newModelConfigUserError(humanizeModelConfigRequestError(req.URL, err), err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		if opts.Provider == string(constant.ProviderOllama) && strings.TrimSpace(opts.Model) != "" {
			raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
			return validateOllamaModelList(raw, opts.Model)
		}
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	msg := strings.TrimSpace(string(raw))
	if msg == "" {
		msg = res.Status
	}
	return newModelConfigUserError(humanizeModelConfigStatusError(res.StatusCode, msg, req.URL, opts), nil)
}

func humanizeModelConfigRequestError(endpoint *url.URL, err error) string {
	raw := strings.ToLower(err.Error())
	host := ""
	port := ""
	target := ""
	if endpoint != nil {
		host = endpoint.Hostname()
		port = endpoint.Port()
		if port == "" {
			port = defaultPort(endpoint.Scheme)
		}
		target = endpoint.String()
	}

	if strings.Contains(raw, "actively refused") || strings.Contains(raw, "connection refused") || strings.Contains(raw, "connectex") {
		if isLocalHost(host) && port == "11434" {
			return "本地 Ollama 服务没有启动，当前电脑没有程序监听 11434 端口。请先运行 `D:\\Ollama\\ollama.exe serve`，再重新测试。"
		}
		if isLocalHost(host) {
			return fmt.Sprintf("本机 %s 端口没有服务在监听。请确认对应模型服务已经启动，或检查 Base URL 是否填错。当前测试地址：%s", port, target)
		}
		return fmt.Sprintf("目标模型服务拒绝连接。请确认供应商地址、端口和中转站服务是否在线。当前测试地址：%s", target)
	}
	if strings.Contains(raw, "no such host") {
		return fmt.Sprintf("模型服务域名解析失败。请检查 Base URL 的域名是否写错，或确认网络/DNS 是否正常。当前测试地址：%s", target)
	}
	if strings.Contains(raw, "timeout") || strings.Contains(raw, "deadline exceeded") {
		return fmt.Sprintf("连接模型服务超时。常见原因是网络不通、供应商服务慢、中转站不可达，或本地模型正在加载。当前测试地址：%s", target)
	}
	if strings.Contains(raw, "tls") || strings.Contains(raw, "x509") || strings.Contains(raw, "certificate") {
		return fmt.Sprintf("HTTPS 证书或 TLS 握手失败。请检查 Base URL 是否应该使用 http/https，或中转站证书是否正常。当前测试地址：%s", target)
	}
	if strings.Contains(raw, "proxyconnect") {
		return "代理连接失败。请检查系统代理、梯子或中转站代理配置是否可用。"
	}
	if strings.Contains(raw, "connection reset") || strings.Contains(raw, "forcibly closed") {
		return fmt.Sprintf("模型服务中途断开连接。通常是服务崩溃、网关限流或模型进程异常。当前测试地址：%s", target)
	}
	return fmt.Sprintf("模型服务连接失败。当前测试地址：%s。底层错误：%v", target, err)
}

func humanizeModelConfigStatusError(status int, body string, endpoint *url.URL, opts modelConfigTestOptions) string {
	target := ""
	if endpoint != nil {
		target = endpoint.String()
	}
	detail := compactResponseBody(body)
	var message string
	switch status {
	case http.StatusBadRequest:
		if opts.IsRerank {
			message = "服务已连上，但 Rerank 测试请求被拒绝。请检查 Rerank 地址是否填的是完整接口，以及模型名是否支持 rerank。"
		} else {
			message = "服务已连上，但模型接口拒绝了测试请求。请检查模型名、Base URL 和供应商类型是否匹配。"
		}
	case http.StatusUnauthorized:
		message = "服务已连上，但认证失败。请检查 API Key 是否填写正确，或者密钥是否已经过期。"
	case http.StatusForbidden:
		message = "服务已连上，但供应商拒绝访问。常见原因是账户余额不足、API Key 权限不够、模型未开通，或中转站禁止当前模型。"
	case http.StatusNotFound:
		if opts.IsRerank {
			message = "服务已连上，但 Rerank 接口不存在。Rerank 通常需要填写完整地址，例如 `https://api.example.com/v1/rerank`。"
		} else {
			message = "服务已连上，但 `/models` 接口不存在。OpenAI 兼容地址通常要以 `/v1` 结尾；Ollama 应使用 `http://127.0.0.1:11434/v1`。"
		}
	case http.StatusTooManyRequests:
		message = "服务已连上，但请求被限流。请稍后再试，或检查供应商配额、并发限制和余额。"
	default:
		if status >= 500 {
			message = "服务已连上，但模型供应商或中转站返回了服务器错误。请稍后重试，或检查中转站日志。"
		} else {
			message = fmt.Sprintf("服务已连上，但返回了异常状态码 %d。请检查供应商配置和模型权限。", status)
		}
	}
	if target != "" {
		message += " 当前测试地址：" + target
	}
	if detail != "" {
		message += "\n原始响应：" + truncateText(detail, 320)
	}
	return message
}

func validateOllamaModelList(raw []byte, expectedModel string) error {
	type modelItem struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Model string `json:"model"`
	}
	var payload struct {
		Data   []modelItem `json:"data"`
		Models []modelItem `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return newModelConfigUserError("Ollama 已响应，但返回的不是 OpenAI 兼容模型列表。请确认 Base URL 是否为 `http://127.0.0.1:11434/v1`。", err)
	}
	expectedModel = strings.TrimSpace(expectedModel)
	for _, item := range append(payload.Data, payload.Models...) {
		if item.ID == expectedModel || item.Name == expectedModel || item.Model == expectedModel {
			return nil
		}
	}
	return newModelConfigUserError(
		fmt.Sprintf("Ollama 服务已连接，但本机没有找到模型 `%s`。请运行 `D:\\Ollama\\ollama.exe pull %s`，或者把模型名改成 `ollama list` 里已有的名称。", expectedModel, expectedModel),
		nil,
	)
}

func compactResponseBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	return strings.Join(strings.Fields(body), " ")
}

func isLocalHost(host string) bool {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func defaultPort(scheme string) string {
	switch strings.ToLower(scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func ptr[T any](v T) *T {
	return &v
}
