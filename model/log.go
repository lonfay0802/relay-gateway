package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"relay-gateway/common"
	"relay-gateway/logger"
	"relay-gateway/types"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1"`
	UserId           string `json:"user_id" gorm:"index"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        string `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          string `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	Other            string `json:"other"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func formatUserLogs(logs []*Log) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// delete admin
			delete(otherMap, "admin_info")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = logs[i].Id % 1024
	}
}

func GetLogByKey(key string) (logs []*Log, err error) {
	if os.Getenv("LOG_SQL_DSN") != "" {
		var tk Token
		if err = DB.Model(&Token{}).Where(logKeyCol+"=?", strings.TrimPrefix(key, "sk-")).First(&tk).Error; err != nil {
			return nil, err
		}
		err = LOG_DB.Model(&Log{}).Where("token_id=?", tk.Id).Find(&logs).Error
	} else {
		err = LOG_DB.Joins("left join tokens on tokens.id = logs.token_id").Where("tokens.key = ?", strings.TrimPrefix(key, "sk-")).Find(&logs).Error
	}
	formatUserLogs(logs)
	return logs, err
}

func RecordErrorLog(c *gin.Context, userId string, channelId string, modelName string, tokenName string, content string, tokenId string, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	//if settingMap, err := GetUserSetting(userId, false); err == nil {
	//	if settingMap.RecordIpLog {
	//		needRecordIp = true
	//	}
	//}
	log := &Log{
		Id:               getNextLogId(),
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		Other: otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        string                 `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          string                 `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
	// 新增字段用于新表记录
	ModelID      string           `json:"model_id,omitempty"`      // 模型ID
	StatusCode   int              `json:"status_code,omitempty"`   // HTTP状态码
	Success      bool             `json:"success,omitempty"`       // 是否成功
	ErrorMessage string           `json:"error_message,omitempty"` // 错误信息
	PriceData    *types.PriceData `json:"price_data,omitempty"`    // 价格数据，用于计算cost_cents
}

// ApiKeyUsageLog 对应新的调用记录表
type ApiKeyUsageLog struct {
	ID        string `json:"id" gorm:"column:id;type:varchar(32);primaryKey"`
	ApiKeyID  string `json:"api_key_id" gorm:"column:api_key_id;type:varchar(32);not null"`
	UserID    string `json:"user_id" gorm:"column:user_id;type:varchar(32);not null"`
	ModelID   string `json:"model_id" gorm:"column:model_id;type:varchar(32);not null"`
	ChannelID string `json:"channel_id" gorm:"column:channel_id;type:varchar(32);not null"`

	TotalCostCents  int             `json:"total_cost_cents" gorm:"column:total_cost_cents;type:int4;default:0"`
	ResponseTimeMs  *int            `json:"response_time_ms" gorm:"column:response_time_ms;type:int4"`
	StatusCode      *int            `json:"status_code" gorm:"column:status_code;type:int4"`
	Success         bool            `json:"success" gorm:"column:success;type:bool;default:true"`
	ErrorMessage    string          `json:"error_message" gorm:"column:error_message;type:text"`
	IPAddress       *net.IP         `json:"ip_address" gorm:"column:ip_address;type:inet"`
	UserAgent       string          `json:"user_agent" gorm:"column:user_agent;type:text"`
	RequestMetadata json.RawMessage `json:"request_metadata" gorm:"column:request_metadata;type:jsonb"`
	CreatedAt       time.Time       `json:"created_at" gorm:"column:created_at;type:timestamptz(6);default:now()"`
	ResourceUsage   json.RawMessage `json:"resource_usage" gorm:"column:resource_usage;type:json"`
	BillingDetails  json.RawMessage `json:"billing_details" gorm:"column:billing_details;type:json"`
	IsStream        bool            `json:"is_stream" gorm:"column:is_stream;type:bool;default:false"`
}

// TableName 指定表名
func (ApiKeyUsageLog) TableName() string {
	return "t_api_key_usage_logs"
}

// calculateCostCents 计算总成本（单位：分）
// quota: 总配额消耗
// priceData: 价格数据，如果为nil则使用quota计算
func calculateCostCents(quota int, priceData *types.PriceData) int {
	// QuotaPerUnit = 500,000 表示 $1 = 500,000 单位
	// 所以 quota 转换为美元：quota / QuotaPerUnit
	// 转换为分（cents）：(quota / QuotaPerUnit) * 100
	return int((float64(quota) / common.QuotaPerUnit) * 100)
}

// getNextLogId 获取下一个日志 id（自增）
func getNextLogId() int {
	var maxId int
	err := LOG_DB.Table("t_logs").Select("COALESCE(MAX(id), 0)").Scan(&maxId).Error
	if err != nil {
		common.SysLog("failed to get next log id: " + err.Error())
		return 1
	}
	return maxId + 1
}

func RecordConsumeLog(c *gin.Context, userId string, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}

	// 生成ID
	id := common.GetUUID()

	// 转换ID为字符串
	apiKeyID := params.TokenId
	userIDStr := userId
	channelIDStr := params.ChannelId

	// 获取model_id，如果没有则使用model_name
	modelID := params.ModelID
	if modelID == "" {
		modelID = params.ModelName // 如果没有model_id，使用model_name作为fallback
	}

	// 响应时间（毫秒）
	var responseTimeMs *int
	if params.UseTimeSeconds > 0 {
		ms := params.UseTimeSeconds * 1000
		responseTimeMs = &ms
	}

	// 状态码
	var statusCode *int
	if params.StatusCode > 0 {
		statusCode = &params.StatusCode
	} else {
		// 尝试从context获取状态码
		if c != nil && c.Writer != nil {
			code := c.Writer.Status()
			if code > 0 {
				statusCode = &code
			}
		}
	}

	// IP地址
	var ipAddress *net.IP
	if c != nil {
		ipStr := c.ClientIP()
		if ipStr != "" {
			ip := net.ParseIP(ipStr)
			if ip != nil {
				ipAddress = &ip
			}
		}
	}

	// User-Agent
	userAgent := ""
	if c != nil && c.Request != nil {
		userAgent = c.Request.Header.Get("User-Agent")
	}

	// 请求元数据
	var requestMetadata json.RawMessage
	if params.Other != nil {
		if metadataBytes, err := json.Marshal(params.Other); err == nil {
			requestMetadata = metadataBytes
		}
	}

	// 资源使用量
	resourceUsage := json.RawMessage(fmt.Sprintf(`{"input_tokens":%d,"output_tokens":%d,"total_tokens":%d}`,
		params.PromptTokens, params.CompletionTokens, params.PromptTokens+params.CompletionTokens))

	// 计费详情：直接存储 other 字段
	var billingDetails json.RawMessage
	if params.Other != nil {
		if otherBytes, err := json.Marshal(params.Other); err == nil {
			billingDetails = otherBytes
		}
	}

	log := &ApiKeyUsageLog{
		ID:              id,
		ApiKeyID:        apiKeyID,
		UserID:          userIDStr,
		ModelID:         modelID,
		ChannelID:       channelIDStr,
		TotalCostCents:  params.Quota,
		ResponseTimeMs:  responseTimeMs,
		StatusCode:      statusCode,
		Success:         params.Success,
		ErrorMessage:    params.ErrorMessage,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
		RequestMetadata: requestMetadata,
		CreatedAt:       time.Now(),
		ResourceUsage:   resourceUsage,
		BillingDetails:  billingDetails,
		IsStream:        params.IsStream,
	}

	// 如果Success未设置，根据状态码判断
	if !params.Success && params.StatusCode == 0 && statusCode != nil {
		log.Success = *statusCode >= 200 && *statusCode < 300
	} else if params.Success {
		log.Success = true
	}

	err := DB.Table("t_api_key_usage_logs").Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record api key usage log: "+err.Error())
		return
	}

	// 如果扣费成功且成本大于0，创建钱包交易记录
	// 如果需要同时扣费和记录交易，应该使用 CreateWalletTransactionWithBalanceUpdate
	if params.Quota > 0 && log.Success {
		// 获取当前余额（扣费后）
		wallet, err := GetUserWalletByUserId(userId)
		if err == nil {
			// 计算扣费前的余额
			balanceBeforeCents := wallet.BalanceCents + params.Quota
			description := fmt.Sprintf("API调用扣费 - 模型: %s, 成本: $%.2f", params.ModelName, params.Quota)
			relatedID := &id

			// 创建交易记录（不更新余额，因为余额已经在其他地方更新了）
			// amount_cents 使用负数表示扣款
			_, err = CreateWalletTransaction(DB, userId, TransactionTypeDeduction, -params.Quota, balanceBeforeCents, description, relatedID, RelatedTypeAPIUsage)
			if err != nil {
				logger.LogError(c, "failed to record wallet transaction: "+err.Error())
			}
		}
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[string]()
	for _, log := range logs {
		if log.ChannelId != "" {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
			return logs, total, err
		}
		channelMap := make(map[string]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	formatUserLogs(logs)
	return logs, total, err
}

func SearchAllLogs(keyword string) (logs []*Log, err error) {
	err = LOG_DB.Where("type = ? or content LIKE ?", keyword, keyword+"%").Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	return logs, err
}

func SearchUserLogs(userId int, keyword string) (logs []*Log, err error) {
	err = LOG_DB.Where("user_id = ? and type = ?", userId, keyword).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs)
	return logs, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		tx = tx.Where("username = ?", username)
		rpmTpmQuery = rpmTpmQuery.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name like ?", modelName)
		rpmTpmQuery = rpmTpmQuery.Where("model_name like ?", modelName)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	tx.Scan(&stat)
	rpmTpmQuery.Scan(&stat)

	return stat
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}

// RecordLog 记录日志
// quota: 可选的配额值，如果提供且不为0，会创建钱包交易记录
//
//	正数表示扣费，负数表示退款
func RecordLog(userId string, logType int, content string, quota ...int) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}

	// 生成ID
	id := common.GetUUID()

	// 计算成本（美分）
	var totalCostCents int
	if len(quota) > 0 && quota[0] != 0 {
		totalCostCents = calculateCostCents(quota[0], nil)
	}

	// 构建请求元数据，包含日志类型和内容
	requestMetadataMap := map[string]interface{}{
		"log_type": logType,
		"content":  content,
	}
	requestMetadataBytes, _ := json.Marshal(requestMetadataMap)
	requestMetadata := json.RawMessage(requestMetadataBytes)

	// 构建计费详情
	var billingDetails json.RawMessage
	if len(quota) > 0 && quota[0] != 0 {
		billingDetailsMap := map[string]interface{}{
			"quota":      quota[0],
			"cost_cents": totalCostCents,
		}
		billingDetailsBytes, _ := json.Marshal(billingDetailsMap)
		billingDetails = json.RawMessage(billingDetailsBytes)
	}

	// 记录到 api_key_usage_logs 表
	// 对于系统日志，使用 "system" 作为 ApiKeyID、ModelID、ChannelID 的默认值
	log := &ApiKeyUsageLog{
		ID:              id,
		ApiKeyID:        "system", // 系统日志使用 "system" 标识
		UserID:          userId,
		ModelID:         "system", // 系统日志使用 "system" 标识
		ChannelID:       "system", // 系统日志使用 "system" 标识
		TotalCostCents:  quota[0],
		ResponseTimeMs:  nil,
		StatusCode:      nil,
		Success:         true, // 系统日志默认成功
		ErrorMessage:    "",
		IPAddress:       nil,
		UserAgent:       "",
		RequestMetadata: requestMetadata,
		CreatedAt:       time.Now(),
		ResourceUsage:   json.RawMessage(`{"input_tokens":0,"output_tokens":0,"total_tokens":0}`),
		BillingDetails:  billingDetails,
		IsStream:        false,
	}

	err := DB.Table("t_api_key_usage_logs").Create(log).Error
	if err != nil {
		common.SysLog("failed to record log to api_key_usage_logs: " + err.Error())
		return
	}

	// 如果提供了配额且不为0，创建钱包交易记录
	if len(quota) > 0 && quota[0] != 0 {
		quotaValue := quota[0]
		// 将 quota 转换为美分
		amountCents := calculateCostCents(quotaValue, nil)
		if amountCents != 0 {
			wallet, err := GetUserWalletByUserId(userId)
			if err == nil {
				// 根据金额正负判断是扣费还是退款
				var transactionType string
				var balanceBeforeCents int
				var description string

				if amountCents > 0 {
					// 扣费：计算扣费前的余额
					transactionType = TransactionTypeDeduction
					balanceBeforeCents = wallet.BalanceCents + amountCents
					description = fmt.Sprintf("系统扣费 - %s", content)
				} else {
					// 退款：计算退款前的余额
					transactionType = TransactionTypeRefund
					balanceBeforeCents = wallet.BalanceCents - amountCents // amountCents是负数，所以用减法
					description = fmt.Sprintf("系统退款 - %s", content)
				}

				// 创建交易记录（不更新余额，因为余额已经在其他地方更新了）
				_, err = CreateWalletTransaction(DB, userId, transactionType, amountCents, balanceBeforeCents, description, nil, RelatedTypeSystem)
				if err != nil {
					common.SysLog("failed to record wallet transaction in RecordLog: " + err.Error())
				}
			}
		}
	}
}
