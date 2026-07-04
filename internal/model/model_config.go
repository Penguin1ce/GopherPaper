package model

import "time"

type SystemModelConfig struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	Role              string     `gorm:"size:32;uniqueIndex;not null" json:"role"`
	Provider          string     `gorm:"size:32" json:"provider"`
	BaseURL           string     `gorm:"size:512" json:"base_url"`
	APIKey            string     `gorm:"size:1024" json:"-"` // legacy plaintext column; new writes keep it empty.
	APIKeyCiphertext  string     `gorm:"size:4096" json:"-"`
	APIKeyNonce       string     `gorm:"size:128" json:"-"`
	APIKeyKeyID       string     `gorm:"size:160" json:"-"`
	APIKeyKeyVersion  string     `gorm:"size:32" json:"-"`
	APIKeyAlgorithm   string     `gorm:"size:32" json:"-"`
	APIKeyMask        string     `gorm:"size:64" json:"-"`
	APIKeyFingerprint string     `gorm:"size:64;index" json:"-"`
	APIKeyMigratedAt  *time.Time `json:"api_key_migrated_at,omitempty"`
	Model             string     `gorm:"size:160" json:"model"`
	Dim               int        `json:"dim"`
	MaxTokens         int        `json:"max_tokens"`
	ReasoningEffort   string     `gorm:"size:16" json:"reasoning_effort"`
	Thinking          string     `gorm:"size:16" json:"thinking"`
	Enabled           bool       `json:"enabled"`
	Timeout           int        `json:"timeout"`
	LastTestStatus    string     `gorm:"size:16" json:"last_test_status"`
	LastTestError     string     `gorm:"size:1024" json:"last_test_error"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	LastTestLatencyMS int64      `json:"last_test_latency_ms"`
	UpdatedBy         uint       `gorm:"index" json:"updated_by"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (SystemModelConfig) TableName() string { return "system_model_configs" }

type UserModelConfig struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	StudentID         string     `gorm:"size:64;uniqueIndex:idx_user_model_config;not null" json:"student_id"`
	Role              string     `gorm:"size:32;uniqueIndex:idx_user_model_config;not null" json:"role"`
	Provider          string     `gorm:"size:32" json:"provider"`
	BaseURL           string     `gorm:"size:512" json:"base_url"`
	APIKey            string     `gorm:"size:1024" json:"-"` // legacy/plaintext staging column; new writes keep it empty.
	APIKeyCiphertext  string     `gorm:"size:4096" json:"-"`
	APIKeyNonce       string     `gorm:"size:128" json:"-"`
	APIKeyKeyID       string     `gorm:"size:160" json:"-"`
	APIKeyKeyVersion  string     `gorm:"size:32" json:"-"`
	APIKeyAlgorithm   string     `gorm:"size:32" json:"-"`
	APIKeyMask        string     `gorm:"size:64" json:"-"`
	APIKeyFingerprint string     `gorm:"size:64;index" json:"-"`
	APIKeyMigratedAt  *time.Time `json:"api_key_migrated_at,omitempty"`
	Model             string     `gorm:"size:160" json:"model"`
	Dim               int        `json:"dim"`
	MaxTokens         int        `json:"max_tokens"`
	ReasoningEffort   string     `gorm:"size:16" json:"reasoning_effort"`
	Thinking          string     `gorm:"size:16" json:"thinking"`
	Enabled           bool       `json:"enabled"`
	Timeout           int        `json:"timeout"`
	LastTestStatus    string     `gorm:"size:16" json:"last_test_status"`
	LastTestError     string     `gorm:"size:1024" json:"last_test_error"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	LastTestLatencyMS int64      `json:"last_test_latency_ms"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (UserModelConfig) TableName() string { return "user_model_configs" }
