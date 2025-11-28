# Token 结构融合迁移指南

## 概述

本文档说明如何将现有的 `Token` 结构体与新设计的 `t_api_keys` 表结构融合。

## 两个设计的对比

### 现有 Token 结构（MySQL/SQLite 风格）
- ✅ 存储明文 key（存在安全风险）
- ✅ 配额管理：`RemainQuota`, `UsedQuota`, `UnlimitedQuota`
- ✅ 模型限制：`ModelLimitsEnabled`, `ModelLimits`
- ✅ IP 白名单：`AllowIps`
- ✅ 分组管理：`Group`
- ✅ 状态：数字状态码（1=启用, 2=禁用, 3=过期, 4=耗尽）
- ✅ 时间：Unix 时间戳（int64）

### 新 t_api_keys 表（PostgreSQL 风格）
- ✅ 安全存储：`key_hash`（SHA256），`key_prefix`，`display_key`
- ✅ 速率限制：`rate_limit_per_minute/hour/day`
- ✅ 消费限制：`daily_spending_limit_cents`, `monthly_spending_limit_cents`
- ✅ 统计信息：`total_requests`, `total_tokens`, `total_cost_cents`
- ✅ 时间戳：PostgreSQL `timestamptz`
- ✅ 状态：字符串状态（active, disabled, expired, exhausted）
- ✅ 描述字段：`description`

## 融合方案

### 方案一：渐进式迁移（推荐）

保持现有 `Token` 结构不变，逐步添加新字段到现有表。

**优点：**
- 无需修改现有代码
- 向后兼容
- 风险低

**步骤：**

1. **添加新字段到现有 Token 表**
   ```sql
   -- 如果使用 PostgreSQL
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS key_hash VARCHAR(255);
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS key_prefix VARCHAR(100);
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS display_key VARCHAR(32);
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS description TEXT;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS rate_limit_per_minute INT DEFAULT 60;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS rate_limit_per_hour INT DEFAULT 1000;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS rate_limit_per_day INT DEFAULT 10000;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS daily_spending_limit_cents INT DEFAULT 1000;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS monthly_spending_limit_cents INT DEFAULT 10000;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS total_requests INT DEFAULT 0;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS total_tokens INT DEFAULT 0;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS total_cost_cents INT DEFAULT 0;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS status_str VARCHAR(20) DEFAULT 'active';
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
   ALTER TABLE tokens ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
   ```

2. **更新 Token 结构体**
   在 `model/token.go` 中添加新字段（可选字段，使用指针类型）

3. **数据迁移脚本**
   - 为现有 key 生成 `key_hash` 和 `key_prefix`
   - 将数字状态转换为字符串状态
   - 将 Unix 时间戳转换为 PostgreSQL 时间戳

### 方案二：使用 TokenEnhanced（完全迁移）

使用新的 `TokenEnhanced` 结构体，完全迁移到新表结构。

**优点：**
- 安全性更高（使用 key_hash）
- 功能更完整
- 符合新设计规范

**缺点：**
- 需要大量代码修改
- 需要数据迁移
- 风险较高

**步骤：**

1. **创建新表**
   ```sql
   CREATE TABLE "t_api_keys" (
     "id" VARCHAR(32) PRIMARY KEY,
     "user_id" VARCHAR(32) NOT NULL,
     "key_hash" VARCHAR(255) NOT NULL UNIQUE,
     "key_prefix" VARCHAR(100) NOT NULL,
     "display_key" VARCHAR(32),
     "name" VARCHAR(100) NOT NULL,
     "description" TEXT,
     "status" INT DEFAULT 1,
     "status_str" VARCHAR(20) DEFAULT 'active',
     "rate_limit_per_minute" INT DEFAULT 60,
     "rate_limit_per_hour" INT DEFAULT 1000,
     "rate_limit_per_day" INT DEFAULT 10000,
     "daily_spending_limit_cents" INT DEFAULT 1000,
     "monthly_spending_limit_cents" INT DEFAULT 10000,
     "total_requests" INT DEFAULT 0,
     "total_tokens" INT DEFAULT 0,
     "total_cost_cents" INT DEFAULT 0,
     "remain_quota" INT DEFAULT 0,
     "unlimited_quota" BOOLEAN DEFAULT FALSE,
     "used_quota" INT DEFAULT 0,
     "model_limits_enabled" BOOLEAN DEFAULT FALSE,
     "model_limits" VARCHAR(1024) DEFAULT '',
     "allow_ips" TEXT DEFAULT '',
     "group" VARCHAR(100) DEFAULT '',
     "created_time" BIGINT,
     "accessed_time" BIGINT,
     "expired_time" BIGINT DEFAULT -1,
     "created_at" TIMESTAMPTZ DEFAULT NOW(),
     "updated_at" TIMESTAMPTZ DEFAULT NOW(),
     "last_used_at" TIMESTAMPTZ,
     "expires_at" TIMESTAMPTZ,
     "deleted_at" TIMESTAMPTZ
   );
   ```

2. **数据迁移**
   - 从旧表迁移数据到新表
   - 生成 key_hash（SHA256）
   - 转换时间格式
   - 转换状态格式

3. **代码迁移**
   - 将所有使用 `Token` 的地方改为 `TokenEnhanced`
   - 更新查询逻辑（使用 key_hash 而不是 key）
   - 更新验证逻辑

## 推荐方案：渐进式迁移

### 第一步：扩展现有 Token 结构

在 `model/token.go` 中添加新字段：

```go
type Token struct {
    // ... 现有字段 ...
    
    // 新增字段（可选，使用指针类型以兼容旧数据）
    KeyHash                  *string    `json:"key_hash" gorm:"type:varchar(255);uniqueIndex"`
    KeyPrefix                *string    `json:"key_prefix" gorm:"type:varchar(100)"`
    DisplayKey               *string    `json:"display_key" gorm:"type:varchar(32)"`
    Description              *string    `json:"description" gorm:"type:text"`
    RateLimitPerMinute       *int       `json:"rate_limit_per_minute" gorm:"default:60"`
    RateLimitPerHour         *int       `json:"rate_limit_per_hour" gorm:"default:1000"`
    RateLimitPerDay          *int       `json:"rate_limit_per_day" gorm:"default:10000"`
    DailySpendingLimitCents  *int       `json:"daily_spending_limit_cents" gorm:"default:1000"`
    MonthlySpendingLimitCents *int       `json:"monthly_spending_limit_cents" gorm:"default:10000"`
    TotalRequests            *int       `json:"total_requests" gorm:"default:0"`
    TotalTokens              *int       `json:"total_tokens" gorm:"default:0"`
    TotalCostCents           *int       `json:"total_cost_cents" gorm:"default:0"`
    StatusStr                *string    `json:"status_str" gorm:"type:varchar(20);default:'active'"`
    LastUsedAt               *time.Time `json:"last_used_at" gorm:"type:timestamptz(6)"`
    ExpiresAt                *time.Time `json:"expires_at" gorm:"type:timestamptz(6)"`
}
```

### 第二步：添加辅助方法

```go
// GetKeyHash 获取 key 的哈希值，如果不存在则生成
func (token *Token) GetKeyHash() string {
    if token.KeyHash != nil && *token.KeyHash != "" {
        return *token.KeyHash
    }
    // 生成 SHA256 哈希
    hash := common.GenerateHMAC(token.Key)
    token.KeyHash = &hash
    return hash
}

// GetKeyPrefix 获取 key 前缀
func (token *Token) GetKeyPrefix() string {
    if token.KeyPrefix != nil {
        return *token.KeyPrefix
    }
    // 从 key 提取前缀
    if len(token.Key) > 8 {
        prefix := token.Key[:8]
        token.KeyPrefix = &prefix
        return prefix
    }
    return ""
}
```

### 第三步：逐步迁移数据

1. 为新创建的 token 自动生成 key_hash 和 key_prefix
2. 为现有 token 批量生成 key_hash 和 key_prefix
3. 逐步将查询逻辑从 key 改为 key_hash

## 安全建议

1. **立即实施：**
   - 为新创建的 token 生成并存储 key_hash
   - 不再在日志中输出完整 key

2. **短期（1-2周）：**
   - 为所有现有 token 生成 key_hash
   - 更新查询逻辑，优先使用 key_hash

3. **长期（1-3个月）：**
   - 完全迁移到 key_hash 查询
   - 考虑移除明文 key 存储（或加密存储）

## 兼容性考虑

- 保持现有 API 接口不变
- 保持现有查询逻辑可用
- 新功能作为可选功能逐步启用
- 提供配置开关控制新功能

## 测试建议

1. 单元测试：测试新旧字段的兼容性
2. 集成测试：测试数据迁移脚本
3. 性能测试：测试 key_hash 索引性能
4. 安全测试：验证 key_hash 的安全性

