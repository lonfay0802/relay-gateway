package model

import "time"

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

type Model struct {
	Id                       string     `json:"id" gorm:"type:varchar(32);primaryKey"`
	ModelName                string     `json:"model_name" gorm:"size:128;not null;"`
	Description              string     `json:"description" gorm:"type:text"`
	DisplayName              string     `json:"display_name" gorm:"type:varchar(255)"`
	ModelGroup               string     `json:"model_group" gorm:"type:varchar(255)"`
	Category                 string     `json:"category" gorm:"type:varchar(50)"`
	Capabilities             string     `json:"capabilities" gorm:"type:varchar(255)"`
	MaxTokens                string     `json:"max_tokens" gorm:"type:int4"`
	SupportsStreaming        bool       `json:"supports_streaming" gorm:"type:bool"`
	SupportsFunctionCalling  bool       `json:"supports_function_calling" gorm:"type:bool"`
	Status                   string     `json:"status" gorm:"type:varchar(128)"`
	CreatedAt                *time.Time `json:"created_at" gorm:"type:timestamptz(6);default:now()"` // 创建时间
	UpdatedAt                *time.Time `json:"updated_at" gorm:"type:timestamptz(6);default:now()"` // 修改时间
	ModelIcon                string     `json:"model_icon" gorm:"type:varchar(1024)"`                // 模型icon
	SupportsCache            bool       `json:"supports_cache" gorm:"type:bool"`
	SupportsPrefixCompletion bool       `json:"supports_prefix_completion" gorm:"type:bool"`
	SupportsBatchInference   bool       `json:"supports_batch_inference" gorm:"type:bool"`
	InputModality            string     `json:"input_modality" gorm:"type:varchar(50)"`  // 输入模态
	OutputModality           string     `json:"output_modality" gorm:"type:varchar(50)"` // 输出模态
	SupportsWebSearch        bool       `json:"supports_web_search" gorm:"type:bool"`
	TpmLimit                 int        `json:"tpm_limit" gorm:"type:int4"`
	RpmLimit                 int        `json:"rpm_limit" gorm:"type:int4"`
	ModelProvider            string     `json:"model_provider" gorm:"type:varchar(32)"`
	VendorID                 string     `json:"vendor_id" gorm:"type:varchar(32)"`
	Endpoints                string     `json:"endpoints,omitempty" gorm:"type:text"`
}

// TableName 指定表名
func (Model) TableName() string {
	return "t_models"
}
