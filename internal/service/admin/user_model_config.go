package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/internal/secret"
)

const (
	modelConfigSourceUser   = "user"
	modelConfigSourceSystem = "system"
)

func ListUserModelConfigs(ctx context.Context, studentID string) (*dto.AdminModelConfigsResponse, error) {
	studentID, err := normalizeModelConfigStudentID(studentID)
	if err != nil {
		return nil, err
	}
	systemCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	rows, err := loadUserModelConfigRows(ctx, studentID)
	if err != nil {
		return nil, err
	}
	effectiveCfg := *systemCfg
	if _, _, err := overlayUserModelConfigRows(&effectiveCfg, rows); err != nil {
		return nil, err
	}
	byRole := userRowsByRole(rows)
	specs := userModelRoleSpecs()
	items := make([]dto.AdminModelConfigItem, 0, len(specs))
	for _, spec := range specs {
		if row, ok := byRole[string(spec.Role)]; ok {
			items = append(items, itemFromUserRow(row, spec, &effectiveCfg))
			continue
		}
		items = append(items, itemFromSystemDefault(systemCfg, spec, &effectiveCfg))
	}
	return &dto.AdminModelConfigsResponse{Items: items}, nil
}

func UpdateUserModelConfig(ctx context.Context, studentID, role string, req dto.AdminModelConfigUpdateRequest) (*dto.AdminModelConfigItem, error) {
	studentID, err := normalizeModelConfigStudentID(studentID)
	if err != nil {
		return nil, err
	}
	spec, ok := findUserModelRoleSpec(role)
	if !ok {
		return nil, ErrModelRoleInvalid
	}
	systemCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	row, exists, err := getUserModelConfigRow(ctx, studentID, role)
	if err != nil {
		return nil, err
	}
	if !exists {
		row = userRowFromConfig(studentID, systemCfg, spec)
	}
	applyUserModelConfigRequest(&row, req, spec)
	if req.APIKey != nil {
		row.APIKey = strings.TrimSpace(*req.APIKey)
	}
	row.StudentID = studentID
	row.Role = role
	if isFallbackRole(spec) && isBlankUserModelConfigRow(row) {
		if err := deleteUserModelConfigRow(ctx, studentID, role); err != nil {
			return nil, err
		}
		ai.EvictUser(studentID)
		return ptr(itemFromSystemDefault(systemCfg, spec, systemCfg)), nil
	}
	if err := validateUserModelConfigRow(&row, spec); err != nil {
		return nil, err
	}
	saveRow := row
	if err := protectUserModelConfigAPIKey(ctx, &saveRow); err != nil {
		return nil, err
	}
	if err := dao.DB.WithContext(ctx).Save(&saveRow).Error; err != nil {
		return nil, fmt.Errorf("user model config: save failed: %w", err)
	}
	row.ID = saveRow.ID
	row.CreatedAt = saveRow.CreatedAt
	row.UpdatedAt = saveRow.UpdatedAt
	row.APIKeyCiphertext = saveRow.APIKeyCiphertext
	row.APIKeyNonce = saveRow.APIKeyNonce
	row.APIKeyKeyID = saveRow.APIKeyKeyID
	row.APIKeyKeyVersion = saveRow.APIKeyKeyVersion
	row.APIKeyAlgorithm = saveRow.APIKeyAlgorithm
	row.APIKeyMask = saveRow.APIKeyMask
	row.APIKeyFingerprint = saveRow.APIKeyFingerprint
	row.APIKeyMigratedAt = saveRow.APIKeyMigratedAt
	ai.EvictUser(studentID)
	effectiveCfg := *systemCfg
	if _, _, err := overlayUserModelConfigRows(&effectiveCfg, []model.UserModelConfig{row}); err != nil {
		return nil, err
	}
	return ptr(itemFromUserRow(row, spec, &effectiveCfg)), nil
}

func RestoreUserModelConfig(ctx context.Context, studentID, role string) (*dto.AdminModelConfigItem, error) {
	studentID, err := normalizeModelConfigStudentID(studentID)
	if err != nil {
		return nil, err
	}
	spec, ok := findUserModelRoleSpec(role)
	if !ok {
		return nil, ErrModelRoleInvalid
	}
	if err := deleteUserModelConfigRow(ctx, studentID, role); err != nil {
		return nil, err
	}
	ai.EvictUser(studentID)
	systemCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	return ptr(itemFromSystemDefault(systemCfg, spec, systemCfg)), nil
}

func TestUserModelConfig(ctx context.Context, studentID, role string) (*dto.AdminModelConfigTestResponse, error) {
	studentID, err := normalizeModelConfigStudentID(studentID)
	if err != nil {
		return nil, err
	}
	spec, ok := findUserModelRoleSpec(role)
	if !ok {
		return nil, ErrModelRoleInvalid
	}
	systemCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	row, exists, err := getUserModelConfigRow(ctx, studentID, role)
	if err != nil {
		return nil, err
	}
	testRow := systemRowFromUser(row)
	if !exists {
		testRow = rowFromConfig(systemCfg, spec)
	}
	if isFallbackRole(spec) && isBlankModelConfigRow(testRow) {
		message := fmt.Sprintf("%s 未配置独立模型，当前会回退主问答模型，不影响使用。", spec.Label)
		if exists {
			_ = updateUserModelConfigTestStatus(ctx, row.ID, modelConfigStatusOK, "", 0)
		}
		return &dto.AdminModelConfigTestResponse{OK: true, Message: message, LatencyMS: 0}, nil
	}
	if err := validateModelConfigRow(&testRow, spec); err != nil {
		return nil, err
	}
	started := time.Now()
	testErr := testModelConfigConnection(ctx, testRow, spec)
	latencyMS := time.Since(started).Milliseconds()
	status := modelConfigStatusOK
	message := "连接测试通过"
	if testErr != nil {
		status = modelConfigStatusFailed
		message = testErr.Error()
	}
	if exists {
		_ = updateUserModelConfigTestStatus(ctx, row.ID, status, message, latencyMS)
	}
	return &dto.AdminModelConfigTestResponse{OK: testErr == nil, Message: message, LatencyMS: latencyMS}, nil
}

func ApplyUserModelConfigs(ctx context.Context, studentID string) (*dto.AdminModelConfigApplyResponse, error) {
	studentID, err := normalizeModelConfigStudentID(studentID)
	if err != nil {
		return nil, err
	}
	systemCfg, err := currentRuntimeConfig()
	if err != nil {
		return nil, err
	}
	rows, err := loadUserModelConfigRows(ctx, studentID)
	if err != nil {
		return nil, err
	}
	effectiveCfg := *systemCfg
	applied, warnings, err := overlayUserModelConfigRows(&effectiveCfg, rows)
	if err != nil {
		return nil, err
	}
	ai.EvictUser(studentID)
	return &dto.AdminModelConfigApplyResponse{AppliedRoles: applied, Warnings: warnings}, nil
}

func ConfigForUser(ctx context.Context, studentID string, base *config.Config) (*config.Config, error) {
	studentID, err := normalizeModelConfigStudentID(studentID)
	if err != nil {
		return nil, err
	}
	var next config.Config
	if base != nil {
		next = *base
	} else {
		systemCfg, err := currentRuntimeConfig()
		if err != nil {
			return nil, err
		}
		next = *systemCfg
	}
	rows, err := loadUserModelConfigRows(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if _, _, err := overlayUserModelConfigRows(&next, rows); err != nil {
		return nil, err
	}
	return &next, nil
}

func normalizeModelConfigStudentID(studentID string) (string, error) {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return "", fmt.Errorf("%w: student_id is required", ErrModelConfigInvalid)
	}
	return studentID, nil
}

func userModelRoleSpecs() []modelRoleSpec {
	specs := make([]modelRoleSpec, 0, len(modelRoleSpecs))
	for _, spec := range modelRoleSpecs {
		if spec.Kind == "chat" {
			specs = append(specs, spec)
		}
	}
	return specs
}

func findUserModelRoleSpec(role string) (modelRoleSpec, bool) {
	spec, ok := findModelRoleSpec(role)
	return spec, ok && spec.Kind == "chat"
}

func loadUserModelConfigRows(ctx context.Context, studentID string) ([]model.UserModelConfig, error) {
	var rows []model.UserModelConfig
	if err := dao.DB.WithContext(ctx).Where("student_id = ?", studentID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("user model config: query failed: %w", err)
	}
	out := rows[:0]
	for i := range rows {
		if _, ok := findUserModelRoleSpec(rows[i].Role); !ok {
			continue
		}
		row, err := prepareUserModelConfigRow(ctx, rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func getUserModelConfigRow(ctx context.Context, studentID, role string) (model.UserModelConfig, bool, error) {
	var row model.UserModelConfig
	err := dao.DB.WithContext(ctx).Where("student_id = ? AND role = ?", studentID, role).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	if err != nil {
		return row, false, fmt.Errorf("user model config: query role failed: %w", err)
	}
	row, err = prepareUserModelConfigRow(ctx, row)
	if err != nil {
		return row, false, err
	}
	return row, true, nil
}

func userRowsByRole(rows []model.UserModelConfig) map[string]model.UserModelConfig {
	out := make(map[string]model.UserModelConfig, len(rows))
	for _, row := range rows {
		out[row.Role] = row
	}
	return out
}

func prepareUserModelConfigRow(ctx context.Context, row model.UserModelConfig) (model.UserModelConfig, error) {
	if strings.TrimSpace(row.APIKey) != "" {
		plain := strings.TrimSpace(row.APIKey)
		saveRow := row
		saveRow.APIKey = plain
		if err := protectUserModelConfigAPIKey(ctx, &saveRow); err != nil {
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
		if err := dao.DB.WithContext(ctx).Model(&model.UserModelConfig{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return row, fmt.Errorf("user model config: migrate api key failed: %w", err)
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
		return row, fmt.Errorf("user model config: secret manager unavailable: %w", err)
	}
	plain, err := m.Decrypt(ctx, userModelConfigSecretPurpose(row.StudentID, row.Role), userModelConfigCiphertext(row))
	if err != nil {
		return row, fmt.Errorf("user model config: decrypt api key failed for student %s role %s: %w", row.StudentID, row.Role, err)
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

func protectUserModelConfigAPIKey(ctx context.Context, row *model.UserModelConfig) error {
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
		return fmt.Errorf("user model config: secret manager unavailable: %w", err)
	}
	ciphertext, err := m.Encrypt(ctx, userModelConfigSecretPurpose(row.StudentID, row.Role), plain)
	if err != nil {
		return fmt.Errorf("user model config: encrypt api key failed: %w", err)
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

func userModelConfigCiphertext(row model.UserModelConfig) secret.Ciphertext {
	return secret.Ciphertext{
		Value:      row.APIKeyCiphertext,
		Nonce:      row.APIKeyNonce,
		KeyID:      row.APIKeyKeyID,
		KeyVersion: row.APIKeyKeyVersion,
		Algorithm:  row.APIKeyAlgorithm,
	}
}

func userModelConfigSecretPurpose(studentID, role string) string {
	return "user_model_config:" + strings.TrimSpace(studentID) + ":" + strings.TrimSpace(role)
}

func overlayUserModelConfigRows(cfg *config.Config, rows []model.UserModelConfig) ([]string, []string, error) {
	applied := make([]string, 0, len(rows))
	warnings := make([]string, 0)
	for _, row := range rows {
		spec, ok := findUserModelRoleSpec(row.Role)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("ignored unsupported role %s", row.Role))
			continue
		}
		if isFallbackRole(spec) && isBlankUserModelConfigRow(row) {
			continue
		}
		sysRow := systemRowFromUser(row)
		if err := validateModelConfigRow(&sysRow, spec); err != nil {
			return nil, nil, err
		}
		applyModelConfigRow(cfg, sysRow, spec)
		applied = append(applied, row.Role)
	}
	return applied, warnings, nil
}

func userRowFromConfig(studentID string, cfg *config.Config, spec modelRoleSpec) model.UserModelConfig {
	row := rowFromConfig(cfg, spec)
	row.APIKey = ""
	return userRowFromSystem(studentID, row)
}

func systemRowFromUser(row model.UserModelConfig) model.SystemModelConfig {
	return model.SystemModelConfig{
		ID:                row.ID,
		Role:              row.Role,
		Provider:          row.Provider,
		BaseURL:           row.BaseURL,
		APIKey:            row.APIKey,
		APIKeyCiphertext:  row.APIKeyCiphertext,
		APIKeyNonce:       row.APIKeyNonce,
		APIKeyKeyID:       row.APIKeyKeyID,
		APIKeyKeyVersion:  row.APIKeyKeyVersion,
		APIKeyAlgorithm:   row.APIKeyAlgorithm,
		APIKeyMask:        row.APIKeyMask,
		APIKeyFingerprint: row.APIKeyFingerprint,
		APIKeyMigratedAt:  row.APIKeyMigratedAt,
		Model:             row.Model,
		Dim:               row.Dim,
		MaxTokens:         row.MaxTokens,
		ReasoningEffort:   row.ReasoningEffort,
		Thinking:          row.Thinking,
		Enabled:           row.Enabled,
		Timeout:           row.Timeout,
		LastTestStatus:    row.LastTestStatus,
		LastTestError:     row.LastTestError,
		LastTestAt:        row.LastTestAt,
		LastTestLatencyMS: row.LastTestLatencyMS,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func userRowFromSystem(studentID string, row model.SystemModelConfig) model.UserModelConfig {
	return model.UserModelConfig{
		ID:                row.ID,
		StudentID:         studentID,
		Role:              row.Role,
		Provider:          row.Provider,
		BaseURL:           row.BaseURL,
		APIKey:            row.APIKey,
		APIKeyCiphertext:  row.APIKeyCiphertext,
		APIKeyNonce:       row.APIKeyNonce,
		APIKeyKeyID:       row.APIKeyKeyID,
		APIKeyKeyVersion:  row.APIKeyKeyVersion,
		APIKeyAlgorithm:   row.APIKeyAlgorithm,
		APIKeyMask:        row.APIKeyMask,
		APIKeyFingerprint: row.APIKeyFingerprint,
		APIKeyMigratedAt:  row.APIKeyMigratedAt,
		Model:             row.Model,
		Dim:               row.Dim,
		MaxTokens:         row.MaxTokens,
		ReasoningEffort:   row.ReasoningEffort,
		Thinking:          row.Thinking,
		Enabled:           row.Enabled,
		Timeout:           row.Timeout,
		LastTestStatus:    row.LastTestStatus,
		LastTestError:     row.LastTestError,
		LastTestAt:        row.LastTestAt,
		LastTestLatencyMS: row.LastTestLatencyMS,
	}
}

func applyUserModelConfigRequest(row *model.UserModelConfig, req dto.AdminModelConfigUpdateRequest, spec modelRoleSpec) {
	id := row.ID
	studentID := row.StudentID
	createdAt := row.CreatedAt
	sysRow := systemRowFromUser(*row)
	applyModelConfigRequest(&sysRow, req, spec)
	next := userRowFromSystem(studentID, sysRow)
	next.ID = id
	next.CreatedAt = createdAt
	*row = next
}

func validateUserModelConfigRow(row *model.UserModelConfig, spec modelRoleSpec) error {
	id := row.ID
	studentID := row.StudentID
	createdAt := row.CreatedAt
	sysRow := systemRowFromUser(*row)
	if err := validateModelConfigRow(&sysRow, spec); err != nil {
		return err
	}
	next := userRowFromSystem(studentID, sysRow)
	next.ID = id
	next.CreatedAt = createdAt
	*row = next
	return nil
}

func isBlankUserModelConfigRow(row model.UserModelConfig) bool {
	return strings.TrimSpace(row.BaseURL) == "" && strings.TrimSpace(row.Model) == ""
}

func itemFromUserRow(row model.UserModelConfig, spec modelRoleSpec, activeCfg *config.Config) dto.AdminModelConfigItem {
	item := itemFromRow(systemRowFromUser(row), spec, activeCfg)
	item.Source = modelConfigSourceUser
	item.RestartRequired = false
	return item
}

func itemFromSystemDefault(cfg *config.Config, spec modelRoleSpec, activeCfg *config.Config) dto.AdminModelConfigItem {
	item := itemFromConfig(cfg, spec, activeCfg)
	item.Source = modelConfigSourceSystem
	item.APIKeyMask = ""
	item.HasAPIKey = false
	item.RestartRequired = false
	return item
}

func deleteUserModelConfigRow(ctx context.Context, studentID, role string) error {
	if err := dao.DB.WithContext(ctx).
		Where("student_id = ? AND role = ?", studentID, role).
		Delete(&model.UserModelConfig{}).Error; err != nil {
		return fmt.Errorf("user model config: restore failed: %w", err)
	}
	return nil
}

func updateUserModelConfigTestStatus(ctx context.Context, id uint, status, message string, latencyMS int64) error {
	now := time.Now()
	updates := map[string]any{
		"last_test_status":     status,
		"last_test_error":      "",
		"last_test_at":         &now,
		"last_test_latency_ms": latencyMS,
	}
	if status == modelConfigStatusFailed {
		updates["last_test_error"] = truncateText(message, 1024)
	}
	return dao.DB.WithContext(ctx).Model(&model.UserModelConfig{}).Where("id = ?", id).Updates(updates).Error
}
