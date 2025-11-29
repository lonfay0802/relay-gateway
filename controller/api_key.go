package controller

import (
	"net/http"

	"relay-gateway/common"
	"relay-gateway/model"

	"github.com/gin-gonic/gin"
)

// ========== Token 缓存管理 ==========

// DeleteTokenCacheRequest 删除 Token 缓存请求结构
type DeleteTokenCacheRequest struct {
	Key string `json:"key" binding:"required"` // API Key
}

// DeleteTokenCacheResponse 删除 Token 缓存响应结构
type DeleteTokenCacheResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DeleteTokenCache 删除 Token 的 Redis 缓存和本地缓存
// 用于外部系统（如 Python 项目）删除数据库中的 API Key 后，同步删除缓存
// POST /api/admin/token/cache/delete
func DeleteTokenCache(c *gin.Context) {
	var req DeleteTokenCacheRequest

	// 解析请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, DeleteTokenCacheResponse{
			Success: false,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 验证 key 是否为空
	if req.Key == "" {
		c.JSON(http.StatusBadRequest, DeleteTokenCacheResponse{
			Success: false,
			Message: "key 不能为空",
		})
		return
	}

	// 调用删除缓存函数（会同时删除 Redis 和本地缓存）
	err := model.CacheDeleteTokenEnhanced(req.Key)
	if err != nil {
		common.SysLog("Failed to delete token cache for key: " + req.Key + ", error: " + err.Error())
		c.JSON(http.StatusInternalServerError, DeleteTokenCacheResponse{
			Success: false,
			Message: "删除缓存失败: " + err.Error(),
		})
		return
	}

	// 隐藏完整的 key，只显示前后各3个字符
	keyPrefix := req.Key
	if len(req.Key) > 10 {
		keyPrefix = req.Key[:3] + "***" + req.Key[len(req.Key)-3:]
	}

	common.SysLog("Successfully deleted token cache for key: " + keyPrefix)

	c.JSON(http.StatusOK, DeleteTokenCacheResponse{
		Success: true,
		Message: "缓存删除成功",
	})
}

// BatchDeleteTokenCacheRequest 批量删除 Token 缓存请求结构
type BatchDeleteTokenCacheRequest struct {
	Keys []string `json:"keys" binding:"required"` // API Key 列表
}

// BatchDeleteTokenCacheResponse 批量删除 Token 缓存响应结构
type BatchDeleteTokenCacheResponse struct {
	Success      bool              `json:"success"`
	Message      string            `json:"message"`
	TotalCount   int               `json:"total_count"`   // 总数
	SuccessCount int               `json:"success_count"` // 成功数量
	FailedCount  int               `json:"failed_count"`  // 失败数量
	FailedKeys   map[string]string `json:"failed_keys"`   // 失败的 keys 及原因
}

// BatchDeleteTokenCache 批量删除 Token 缓存
// 用于批量删除多个 API Key 的缓存
// POST /api/admin/token/cache/batch-delete
func BatchDeleteTokenCache(c *gin.Context) {
	var req BatchDeleteTokenCacheRequest

	// 解析请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BatchDeleteTokenCacheResponse{
			Success: false,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 验证 keys 是否为空
	if len(req.Keys) == 0 {
		c.JSON(http.StatusBadRequest, BatchDeleteTokenCacheResponse{
			Success: false,
			Message: "keys 列表不能为空",
		})
		return
	}

	totalCount := len(req.Keys)
	successCount := 0
	failedKeys := make(map[string]string)

	// 逐个删除缓存
	for _, key := range req.Keys {
		if key == "" {
			failedKeys[key] = "key 为空"
			continue
		}

		err := model.CacheDeleteTokenEnhanced(key)
		if err != nil {
			keyPrefix := key
			if len(key) > 10 {
				keyPrefix = key[:3] + "***" + key[len(key)-3:]
			}
			failedKeys[keyPrefix] = err.Error()
			common.SysLog("Failed to delete token cache for key: " + keyPrefix + ", error: " + err.Error())
		} else {
			successCount++
			keyPrefix := key
			if len(key) > 10 {
				keyPrefix = key[:3] + "***" + key[len(key)-3:]
			}
			common.SysLog("Successfully deleted token cache for key: " + keyPrefix)
		}
	}

	failedCount := totalCount - successCount

	// 构建响应消息
	message := "批量删除完成"
	if failedCount > 0 {
		message = "部分删除失败"
	}

	c.JSON(http.StatusOK, BatchDeleteTokenCacheResponse{
		Success:      failedCount == 0,
		Message:      message,
		TotalCount:   totalCount,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		FailedKeys:   failedKeys,
	})
}

// ========== User 缓存管理 ==========

// DeleteUserCacheRequest 删除 User 缓存请求结构
type DeleteUserCacheRequest struct {
	UserId string `json:"user_id" binding:"required"` // User ID
}

// DeleteUserCacheResponse 删除 User 缓存响应结构
type DeleteUserCacheResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DeleteUserCache 删除 User 的 Redis 缓存和本地缓存
// 用于外部系统（如 Python 项目）删除或更新数据库中的 User 后，同步删除缓存
// POST /api/admin/user/cache/delete
func DeleteUserCache(c *gin.Context) {
	var req DeleteUserCacheRequest

	// 解析请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, DeleteUserCacheResponse{
			Success: false,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 验证 user_id 是否为空
	if req.UserId == "" {
		c.JSON(http.StatusBadRequest, DeleteUserCacheResponse{
			Success: false,
			Message: "user_id 不能为空",
		})
		return
	}

	// 调用删除缓存函数（会同时删除 Redis 和本地缓存）
	// 使用 model 包中未导出的函数，需要通过导出的函数访问
	// 这里我们直接删除
	model.LocalCacheDeleteUser(req.UserId)

	// 同时删除 Redis 缓存
	cacheKey := "user:" + req.UserId
	err := common.RedisDelKey(cacheKey)
	if err != nil && common.RedisEnabled {
		common.SysLog("Failed to delete user Redis cache for userId: " + req.UserId + ", error: " + err.Error())
		c.JSON(http.StatusInternalServerError, DeleteUserCacheResponse{
			Success: false,
			Message: "删除 Redis 缓存失败: " + err.Error(),
		})
		return
	}

	common.SysLog("Successfully deleted user cache for userId: " + req.UserId)

	c.JSON(http.StatusOK, DeleteUserCacheResponse{
		Success: true,
		Message: "缓存删除成功",
	})
}

// BatchDeleteUserCacheRequest 批量删除 User 缓存请求结构
type BatchDeleteUserCacheRequest struct {
	UserIds []string `json:"user_ids" binding:"required"` // User ID 列表
}

// BatchDeleteUserCacheResponse 批量删除 User 缓存响应结构
type BatchDeleteUserCacheResponse struct {
	Success      bool              `json:"success"`
	Message      string            `json:"message"`
	TotalCount   int               `json:"total_count"`   // 总数
	SuccessCount int               `json:"success_count"` // 成功数量
	FailedCount  int               `json:"failed_count"`  // 失败数量
	FailedIds    map[string]string `json:"failed_ids"`    // 失败的 IDs 及原因
}

// BatchDeleteUserCache 批量删除 User 缓存
// 用于批量删除多个 User 的缓存
// POST /api/admin/user/cache/batch-delete
func BatchDeleteUserCache(c *gin.Context) {
	var req BatchDeleteUserCacheRequest

	// 解析请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BatchDeleteUserCacheResponse{
			Success: false,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 验证 user_ids 是否为空
	if len(req.UserIds) == 0 {
		c.JSON(http.StatusBadRequest, BatchDeleteUserCacheResponse{
			Success: false,
			Message: "user_ids 列表不能为空",
		})
		return
	}

	totalCount := len(req.UserIds)
	successCount := 0
	failedIds := make(map[string]string)

	// 逐个删除缓存
	for _, userId := range req.UserIds {
		if userId == "" {
			failedIds[userId] = "user_id 为空"
			continue
		}

		// 删除本地缓存
		model.LocalCacheDeleteUser(userId)

		// 删除 Redis 缓存
		cacheKey := "user:" + userId
		err := common.RedisDelKey(cacheKey)
		if err != nil && common.RedisEnabled {
			failedIds[userId] = err.Error()
			common.SysLog("Failed to delete user cache for userId: " + userId + ", error: " + err.Error())
		} else {
			successCount++
			common.SysLog("Successfully deleted user cache for userId: " + userId)
		}
	}

	failedCount := totalCount - successCount

	// 构建响应消息
	message := "批量删除完成"
	if failedCount > 0 {
		message = "部分删除失败"
	}

	c.JSON(http.StatusOK, BatchDeleteUserCacheResponse{
		Success:      failedCount == 0,
		Message:      message,
		TotalCount:   totalCount,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		FailedIds:    failedIds,
	})
}
