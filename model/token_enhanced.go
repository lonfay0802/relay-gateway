package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"relay-gateway/common"
)

// TokenEnhanced 融合后的增强版 Token 结构体
// 融合了现有 Token 和新 t_api_keys 表的设计
type TokenEnhanced struct {
	// ========== 基础字段 ==========
	Id     string `json:"id" gorm:"type:varchar(32);primaryKey"`          // 改为 varchar(32) 以兼容新设计
	UserId string `json:"user_id" gorm:"type:varchar(32);index;not null"` // 改为 varchar(32)

	// ========== Key 相关字段（安全性增强）==========
	// 保留明文 key 用于向后兼容，但建议逐步迁移到 key_hash
	Key string `json:"key" gorm:"type:char(48);uniqueIndex"` // 保留用于兼容

	// 新增：安全存储方式
	KeyHash    string `json:"key_hash" gorm:"type:varchar(255);uniqueIndex;not null"` // SHA256 哈希值
	KeyPrefix  string `json:"key_prefix" gorm:"type:varchar(100);not null"`           // 前缀，如：ak_123456
	DisplayKey string `json:"display_key" gorm:"type:varchar(32)"`                    // 展示用，如：sk_ah_v1-prefix01

	// ========== 基本信息 ==========
	Name        string  `json:"name" gorm:"type:varchar(100);index;not null"`
	Description *string `json:"description" gorm:"type:text"` // 新增描述字段

	// ========== 状态管理 ==========
	// 使用 int 类型状态码：1=启用, 2=禁用, 3=过期, 4=耗尽
	Status  int `json:"status" gorm:"default:1"` // 状态码（1=启用, 2=禁用, 3=过期, 4=耗尽）
	Deleted int `json:"deleted" gorm:"default:0"`

	// ========== 时间字段 ==========
	CreatedAt  *time.Time `json:"created_at" gorm:"type:timestamptz(6);default:now()"` // 创建时间
	UpdatedAt  *time.Time `json:"updated_at" gorm:"type:timestamptz(6);default:now()"` // 更新时间
	LastUsedAt *time.Time `json:"last_used_at" gorm:"type:timestamptz(6)"`             // 最后使用时间
	ExpiresAt  *time.Time `json:"expires_at" gorm:"type:timestamptz(6)"`               // 过期时间（nil 表示永不过期）

	// ========== 配额管理（现有功能保留）==========
	RemainQuota    int  `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota bool `json:"unlimited_quota"`
	UsedQuota      int  `json:"used_quota" gorm:"default:0"`

	// ========== 速率限制（新功能）==========
	RateLimitPerMinute int `json:"rate_limit_per_minute" gorm:"default:60"` // 每分钟请求限制
	RateLimitPerHour   int `json:"rate_limit_per_hour" gorm:"default:1000"` // 每小时请求限制
	RateLimitPerDay    int `json:"rate_limit_per_day" gorm:"default:10000"` // 每日请求限制

	// ========== 消费限制（新功能）==========
	DailySpendingLimitCents   int `json:"daily_spending_limit_cents" gorm:"default:1000"`    // 每日消费限制（美分）
	MonthlySpendingLimitCents int `json:"monthly_spending_limit_cents" gorm:"default:10000"` // 每月消费限制（美分）

	// ========== 统计信息（新功能）==========
	TotalRequests  int `json:"total_requests" gorm:"default:0"`   // 总请求数
	TotalTokens    int `json:"total_tokens" gorm:"default:0"`     // 总 token 数
	TotalCostCents int `json:"total_cost_cents" gorm:"default:0"` // 总消费（美分）

	// ========== 访问控制（现有功能保留）==========
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	ModelLimits        string  `json:"model_limits" gorm:"type:varchar(1024);default:''"`
	AllowIps           *string `json:"allow_ips" gorm:"default:''"`

	// ========== 分组管理（现有功能保留）==========
	Group string `json:"group" gorm:"type:varchar(100);default:''"`
}

// TableName 指定表名
func (TokenEnhanced) TableName() string {
	return "t_api_keys"
}

// Clean 清理敏感信息（用于返回给客户端时）
func (token *TokenEnhanced) Clean() {
	token.Key = ""
	token.KeyHash = ""
	// 保留 DisplayKey 和 KeyPrefix 用于展示
}

// GetStatusString 将数字状态转换为字符串（用于 API 返回或日志）
func (token *TokenEnhanced) GetStatusString() string {
	switch token.Status {
	case common.TokenStatusEnabled:
		return "active"
	case common.TokenStatusDisabled:
		return "disabled"
	case common.TokenStatusExpired:
		return "expired"
	case common.TokenStatusExhausted:
		return "exhausted"
	default:
		return "unknown"
	}
}

// SetStatusFromString 从字符串设置状态（用于 API 接收）
func (token *TokenEnhanced) SetStatusFromString(status string) {
	switch status {
	case "active":
		token.Status = common.TokenStatusEnabled
	case "disabled":
		token.Status = common.TokenStatusDisabled
	case "expired":
		token.Status = common.TokenStatusExpired
	case "exhausted":
		token.Status = common.TokenStatusExhausted
	default:
		token.Status = 0 // 未知状态
	}
}

// IsEnabled 检查是否启用
func (token *TokenEnhanced) IsEnabled() bool {
	return token.Status == common.TokenStatusEnabled
}

// IsDisabled 检查是否禁用
func (token *TokenEnhanced) IsDisabled() bool {
	return token.Status == common.TokenStatusDisabled
}

// IsExhausted 检查是否耗尽
func (token *TokenEnhanced) IsExhausted() bool {
	return token.Status == common.TokenStatusExhausted
}

// IsValid 检查 token 是否有效（启用且未过期且未耗尽）
func (token *TokenEnhanced) IsValid() bool {
	if !token.IsEnabled() {
		return false
	}
	if token.IsExpired() {
		return false
	}
	if token.IsExhausted() {
		return false
	}
	return token.HasQuotaRemaining()
}

// IsExpired 检查是否过期
func (token *TokenEnhanced) IsExpired() bool {
	// 首先检查状态码
	if token.Status == common.TokenStatusExpired {
		return true
	}
	// 检查过期时间（nil 表示永不过期）
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return true
	}
	return false
}

// HasQuotaRemaining 检查是否有剩余配额
func (token *TokenEnhanced) HasQuotaRemaining() bool {
	if token.UnlimitedQuota {
		return true
	}
	return token.RemainQuota > 0
}

// CheckRateLimit 检查速率限制（需要配合 Redis 或其他缓存实现）
// 这里只是结构定义，实际实现需要根据业务逻辑
func (token *TokenEnhanced) CheckRateLimit() bool {
	// TODO: 实现速率限制检查逻辑
	// 需要配合 Redis 记录每分钟/小时/天的请求数
	return true
}

// CheckSpendingLimit 检查消费限制
func (token *TokenEnhanced) CheckSpendingLimit(dailySpent, monthlySpent int) bool {
	if token.DailySpendingLimitCents > 0 && dailySpent >= token.DailySpendingLimitCents {
		return false
	}
	if token.MonthlySpendingLimitCents > 0 && monthlySpent >= token.MonthlySpendingLimitCents {
		return false
	}
	return true
}

// SelectUpdate 更新 token 状态和最后使用时间
func (token *TokenEnhanced) SelectUpdate() (err error) {
	// This can update zero values
	now := time.Now()
	token.LastUsedAt = &now
	return DB.Model(token).Select("last_used_at", "status").Updates(token).Error
}

// GetTokenEnhancedByKey 根据 key 获取 TokenEnhanced
func GetTokenEnhancedByKey(key string, fromDB bool) (token *TokenEnhanced, err error) {
	// TODO: 可以添加 Redis 缓存支持，类似 GetTokenByKey
	token = &TokenEnhanced{}
	err = DB.Where("key = ? AND deleted = ? AND status = ?", key, 0, 1).First(token).Error
	return token, err
}

// ValidateUserTokenEnhanced 验证用户 token（TokenEnhanced 版本）
func ValidateUserTokenEnhanced(key string) (token *TokenEnhanced, err error) {
	if key == "" {
		return nil, errors.New("未提供令牌")
	}
	token, err = GetTokenEnhancedByKey(key, false)
	if err == nil {
		if token.Status == common.TokenStatusExhausted {
			keyPrefix := key[:3]
			keySuffix := key[len(key)-3:]
			return token, errors.New("该令牌额度已用尽 TokenStatusExhausted[sk-" + keyPrefix + "***" + keySuffix + "]")
		} else if token.Status == common.TokenStatusExpired {
			return token, errors.New("该令牌已过期")
		}
		if token.Status != common.TokenStatusEnabled {
			return token, errors.New("该令牌状态不可用")
		}
		// 检查过期时间（nil 表示永不过期）
		if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
			if !common.RedisEnabled {
				token.Status = common.TokenStatusExpired
				err := token.SelectUpdate()
				if err != nil {
					common.SysLog("failed to update token status" + err.Error())
				}
			}
			return token, errors.New("该令牌已过期")
		}
		if !token.UnlimitedQuota && token.RemainQuota <= 0 {
			if !common.RedisEnabled {
				// in this case, we can make sure the token is exhausted
				token.Status = common.TokenStatusExhausted
				err := token.SelectUpdate()
				if err != nil {
					common.SysLog("failed to update token status" + err.Error())
				}
			}
			keyPrefix := key[:3]
			keySuffix := key[len(key)-3:]
			return token, errors.New(fmt.Sprintf("[sk-%s***%s] 该令牌额度已用尽 !token.UnlimitedQuota && token.RemainQuota = %d", keyPrefix, keySuffix, token.RemainQuota))
		}
		return token, nil
	}
	return nil, errors.New("无效的令牌")
}

// GetIpLimitsMapEnhanced 获取 IP 限制映射（TokenEnhanced 版本）
func (token *TokenEnhanced) GetIpLimitsMapEnhanced() map[string]any {
	// delete empty spaces
	//split with \n
	ipLimitsMap := make(map[string]any)
	if token.AllowIps == nil {
		return ipLimitsMap
	}
	cleanIps := strings.ReplaceAll(*token.AllowIps, " ", "")
	if cleanIps == "" {
		return ipLimitsMap
	}
	ips := strings.Split(cleanIps, "\n")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		ip = strings.ReplaceAll(ip, ",", "")
		if common.IsIP(ip) {
			ipLimitsMap[ip] = true
		}
	}
	return ipLimitsMap
}

// GetModelLimits 获取模型限制列表（TokenEnhanced 版本）
func (token *TokenEnhanced) GetModelLimits() []string {
	if token.ModelLimits == "" {
		return []string{}
	}
	return strings.Split(token.ModelLimits, ",")
}

// GetModelLimitsMap 获取模型限制映射（TokenEnhanced 版本）
func (token *TokenEnhanced) GetModelLimitsMap() map[string]bool {
	limits := token.GetModelLimits()
	limitsMap := make(map[string]bool)
	for _, limit := range limits {
		limitsMap[limit] = true
	}
	return limitsMap
}
