package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"relay-gateway/common"
)

// PlatformPricing 模型平台定价表
type PlatformPricing struct {
	ID                       string          `json:"id" gorm:"column:id;type:varchar(32);primaryKey"`
	ModelID                  string          `json:"model_id" gorm:"column:model_id;type:varchar(32);not null;index"`
	BillingMode              string          `json:"billing_mode" gorm:"column:billing_mode;type:varchar(50);not null"`
	InputPricePer1kTokens    int             `json:"input_price_per_1k_tokens" gorm:"column:input_price_per_1k_tokens;type:int4;default:0"`
	OutputPricePer1kTokens   int             `json:"output_price_per_1k_tokens" gorm:"column:output_price_per_1k_tokens;type:int4;default:0"`
	PricePerCallCents        float32         `json:"price_per_call_cents" gorm:"column:price_per_call_cents;type:float4;default:0"`
	VideoPricePerSecondCents *int            `json:"video_price_per_second_cents" gorm:"column:video_price_per_second_cents;type:int4"`
	VideoPricePerMinuteCents int             `json:"video_price_per_minute_cents" gorm:"column:video_price_per_minute_cents;type:int4;default:0"`
	EffectiveFrom            *time.Time      `json:"effective_from" gorm:"column:effective_from;type:timestamptz(6);default:now()"`
	EffectiveTo              *time.Time      `json:"effective_to" gorm:"column:effective_to;type:timestamptz(6)"`
	CreatedAt                time.Time       `json:"created_at" gorm:"column:created_at;type:timestamptz(6);not null;default:now()"`
	UpdatedAt                time.Time       `json:"updated_at" gorm:"column:updated_at;type:timestamptz(6);not null;default:now()"`
	AudioPricePerSecondCents int             `json:"audio_price_per_second_cents" gorm:"column:audio_price_per_second_cents;type:int4;default:0"`
	PriceConfig              PriceConfigJSON `json:"price_config" gorm:"column:price_config;type:json"`
	VideoPricePerDuration    int             `json:"video_price_per_duration" gorm:"column:video_price_per_duration;type:int4;default:0"`
	AudioPricePerDuration    int             `json:"audio_price_per_duration" gorm:"column:audio_price_per_duration;type:int4;default:0"`
	DurationPrice            int             `json:"duration_price" gorm:"column:duration_price;type:int4;default:0"`
	DurationLength           int             `json:"duration_length" gorm:"column:duration_length;type:int4;default:0"`
	DurationUnit             string          `json:"duration_unit" gorm:"column:duration_unit;type:varchar(8)"`
	BillingModalities        string          `json:"billing_modalities" gorm:"column:billing_modalities;type:varchar(50)"`
	ModelRatio               float32         `json:"model_ratio" gorm:"column:model_ratio;type:float4;default:1.0"`
	CompletionRatio          float32         `json:"completion_ratio" gorm:"column:completion_ratio;type:float4;default:1.0"`
	CacheRatio               float32         `json:"cache_ratio" gorm:"column:cache_ratio;type:float4;default:1.0"`
	CacheCreationRatio       float32         `json:"cache_creation_ratio" gorm:"column:cache_creation_ratio;type:float4;default:1.0"`
	ImageRatio               float32         `json:"image_ratio" gorm:"column:image_ratio;type:float4;default:1.0"`
	AudioRatio               float32         `json:"audio_ratio" gorm:"column:audio_ratio;type:float4;default:1.0"`
	AudioCompletionRatio     float32         `json:"audio_completion_ratio" gorm:"column:audio_completion_ratio;type:float4;default:1.0"`
	DefaultRatioGroup        string          `json:"default_ratio_group" gorm:"column:default_ratio_group;type:varchar(50)"`
}

// TableName 指定表名
func (PlatformPricing) TableName() string {
	return "t_model_platform_pricing"
}

// PriceConfigJSON 用于存储 price_config JSON 字段
type PriceConfigJSON map[string]interface{}

// Value 实现 driver.Valuer 接口
func (p PriceConfigJSON) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口
func (p *PriceConfigJSON) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}
	bytesValue, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytesValue, p)
}

// GetActivePricingByModelID 根据 model_id 获取当前有效的定价配置
func GetActivePricingByModelID(modelID string) (*PlatformPricing, error) {
	var pricing PlatformPricing
	now := time.Now()

	err := DB.Where("model_id = ?", modelID).
		Where("(effective_from IS NULL OR effective_from <= ?)", now).
		Where("(effective_to IS NULL OR effective_to >= ?)", now).
		Order("effective_from DESC").
		First(&pricing).Error

	if err != nil {
		return nil, err
	}
	return &pricing, nil
}

// PricingWithModelName 包含定价配置和模型名称的结构体
type PricingWithModelName struct {
	PlatformPricing
	ModelName string `json:"model_name" gorm:"column:model_name"`
}

// GetAllActivePricing 获取所有当前有效的定价配置（连表查询模型名称）
// 对于每个 model_id，只返回最新的有效记录（按 effective_from DESC 排序，取第一条）
func GetAllActivePricing() ([]PlatformPricing, error) {
	var allPricings []PricingWithModelName
	now := time.Now()

	// 连表查询：获取定价配置和模型名称
	err := DB.Table("t_model_platform_pricing").
		Select("t_model_platform_pricing.*, t_models.model_name").
		Joins("LEFT JOIN t_models ON t_model_platform_pricing.model_id = t_models.id").
		Where("(t_model_platform_pricing.effective_from IS NULL OR t_model_platform_pricing.effective_from <= ?)", now).
		Where("(t_model_platform_pricing.effective_to IS NULL OR t_model_platform_pricing.effective_to >= ?)", now).
		Order("t_model_platform_pricing.model_id, t_model_platform_pricing.effective_from DESC, t_model_platform_pricing.created_at DESC").
		Find(&allPricings).Error

	if err != nil {
		return nil, err
	}

	// 去重：对于每个 model_id，只保留第一条（最新的）记录
	pricingMap := make(map[string]PlatformPricing)
	modelNameMap := make(map[string]string) // model_id -> model_name 映射

	for _, pricingWithName := range allPricings {
		modelID := pricingWithName.ModelID
		if _, exists := pricingMap[modelID]; !exists {
			pricingMap[modelID] = pricingWithName.PlatformPricing
			if pricingWithName.ModelName != "" {
				modelNameMap[modelID] = pricingWithName.ModelName
			}
		}
	}

	// 转换为切片
	result := make([]PlatformPricing, 0, len(pricingMap))
	for _, pricing := range pricingMap {
		result = append(result, pricing)
	}

	return result, nil
}

// GetAllActivePricingWithModelName 获取所有当前有效的定价配置（包含模型名称）
// 返回 map[model_name]PlatformPricing
func GetAllActivePricingWithModelName() (map[string]PlatformPricing, error) {
	var allPricings []PricingWithModelName
	now := time.Now()

	// 连表查询：获取定价配置和模型名称
	err := DB.Table("t_model_platform_pricing").
		Select("t_model_platform_pricing.*, t_models.model_name").
		Joins("LEFT JOIN t_models ON t_model_platform_pricing.model_id = t_models.id").
		Where("(t_model_platform_pricing.effective_from IS NULL OR t_model_platform_pricing.effective_from <= ?)", now).
		Where("(t_model_platform_pricing.effective_to IS NULL OR t_model_platform_pricing.effective_to >= ?)", now).
		Order("t_model_platform_pricing.model_id, t_model_platform_pricing.effective_from DESC, t_model_platform_pricing.created_at DESC").
		Find(&allPricings).Error

	if err != nil {
		return nil, err
	}

	// 去重：对于每个 model_name，只保留第一条（最新的）记录
	result := make(map[string]PlatformPricing)
	seenModelIDs := make(map[string]bool) // 用于去重 model_id

	for _, pricingWithName := range allPricings {
		modelID := pricingWithName.ModelID
		modelName := pricingWithName.ModelName

		// 如果 model_name 为空，跳过
		if modelName == "" {
			continue
		}

		// 对于同一个 model_id，只保留第一条记录
		if !seenModelIDs[modelID] {
			seenModelIDs[modelID] = true
			// 如果同一个 model_name 有多条记录，后面的会覆盖前面的（已按 effective_from DESC 排序）
			result[modelName] = pricingWithName.PlatformPricing
		}
	}

	return result, nil
}

// GetLatestPricingByModelID 获取指定模型的最新定价配置（不考虑时间范围）
func GetLatestPricingByModelID(modelID string) (*PlatformPricing, error) {
	var pricing PlatformPricing

	err := DB.Where("model_id = ?", modelID).
		Order("effective_from DESC, created_at DESC").
		First(&pricing).Error

	if err != nil {
		return nil, err
	}
	return &pricing, nil
}

// 内存缓存：模型名称 -> 默认分组倍率
var (
	defaultGroupRatioCache     = make(map[string]float64) // key: modelName, value: ratio
	defaultGroupRatioCacheLock sync.RWMutex
)

// LoadDefaultGroupRatiosToCache 从数据库加载所有模型的默认分组倍率到内存缓存（使用 model_name 作为 key）
func LoadDefaultGroupRatiosToCache() error {
	pricings, err := GetAllActivePricingWithModelName()

	if err != nil {
		return err
	}

	defaultGroupRatioCacheLock.Lock()
	defer defaultGroupRatioCacheLock.Unlock()

	// 清空旧缓存
	defaultGroupRatioCache = make(map[string]float64)

	// 遍历所有定价配置，加载 DefaultRatioGroup 到缓存（使用 model_name 作为 key）
	for modelName, pricing := range pricings {
		if pricing.DefaultRatioGroup != "" {
			ratio, ok := GetOption(pricing.DefaultRatioGroup)
			if ok {
				ratio, err := strconv.ParseFloat(ratio, 64)
				if err == nil {
					// 使用 model_name 作为 key
					defaultGroupRatioCache[modelName] = ratio
				}
			}
		}
	}

	// 打印缓存中的所有值
	common.SysLog("=== DefaultGroupRatioCache loaded ===")
	if len(defaultGroupRatioCache) == 0 {
		common.SysLog("defaultGroupRatioCache is empty")
	} else {
		for modelName, ratio := range defaultGroupRatioCache {
			common.SysLog(fmt.Sprintf("  model_name: %s, ratio: %.4f", modelName, ratio))
		}
		common.SysLog(fmt.Sprintf("Total cached models: %d", len(defaultGroupRatioCache)))
	}
	common.SysLog("=====================================")

	return nil
}

// GetDefaultGroupRatioByModelName 根据模型名称获取默认分组倍率
// 优先从内存缓存读取，如果缓存中没有则从数据库查询
// DefaultRatioGroup 字段直接存储倍率值（如 "0.7"），返回解析后的 float64 和是否找到的标识
func GetDefaultGroupRatioByModelName(modelName string) (float64, bool) {
	if modelName == "" {
		return 0, false
	}

	// 1. 优先从内存缓存读取
	defaultGroupRatioCacheLock.RLock()
	if ratio, ok := defaultGroupRatioCache[modelName]; ok {
		defaultGroupRatioCacheLock.RUnlock()
		return ratio, true
	}
	defaultGroupRatioCacheLock.RUnlock()

	// 2. 缓存未命中，从数据库查询（降级方案）
	// 先通过 model_name 查询 model_id，再查询定价配置
	var model Model
	err := DB.Table("t_models").Where("model_name = ?", modelName).First(&model).Error
	if err != nil {
		return 0, false
	}

	pricing, err := GetActivePricingByModelID(model.Id)
	if err != nil {
		return 0, false
	}

	// 3. 如果没有配置 DefaultRatioGroup，返回 false
	if pricing.DefaultRatioGroup == "" {
		return 0, false
	}

	// 4. 解析 DefaultRatioGroup 字段为倍率值
	ratio, err := strconv.ParseFloat(pricing.DefaultRatioGroup, 64)
	if err != nil {
		return 0, false
	}

	// 5. 更新缓存（异步更新，避免阻塞）
	go func() {
		defaultGroupRatioCacheLock.Lock()
		defaultGroupRatioCache[modelName] = ratio
		defaultGroupRatioCacheLock.Unlock()
	}()

	return ratio, true
}

// GetDefaultGroupRatioByModelID 根据模型ID获取默认分组倍率（保留向后兼容）
// 优先从内存缓存读取，如果缓存中没有则从数据库查询
func GetDefaultGroupRatioByModelID(modelID string) (float64, bool) {
	// 先通过 model_id 查询 model_name
	var model Model
	err := DB.Table("t_models").Where("id = ?", modelID).First(&model).Error
	if err != nil {
		return 0, false
	}

	// 使用 model_name 查询
	return GetDefaultGroupRatioByModelName(model.ModelName)
}
