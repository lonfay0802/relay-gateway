package model

import (
	"fmt"
	"time"

	"relay-gateway/common"
	"relay-gateway/constant"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
)

// 本地缓存实例（1h过期）
var userLocalCache = common.NewLocalCache(60 * 60 * time.Second)

// UserBase struct remains the same as it represents the cached data structure
type UserBase struct {
	Id       string `json:"id"`
	Group    string `json:"group"`
	Email    string `json:"email"`
	Quota    int    `json:"quota"`
	Status   int    `json:"status"`
	Username string `json:"username"`
}

func (user *UserBase) WriteContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	common.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
}

// getUserCacheKey returns the key for user cache
func getUserCacheKey(userId string) string {
	return fmt.Sprintf("user:%s", userId)
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId string) error {
	// 同步删除本地缓存
	LocalCacheDeleteUser(userId)

	// 删除 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisDelKey(getUserCacheKey(userId))
}

// updateUserCache updates all user cache fields using JSON
func updateUserCache(user UserEnhance) error {
	// 创建缓存对象
	userBase := user.ToBaseUser()

	// 同步更新本地缓存
	LocalCacheSetUser(user.Id, userBase)

	// 更新 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}

	return common.RedisSetJSON(
		getUserCacheKey(user.Id),
		userBase,
		time.Duration(common.RedisKeyCacheSeconds())*time.Second,
	)
}

// GetUserCache gets complete user cache from hash
func GetUserCache(userId string) (userCache *UserBase, err error) {
	var user *UserEnhance
	var fromDB bool
	cacheKey := getUserCacheKey(userId)
	defer func() {
		// 异步写入本地缓存和Redis缓存（仅在从数据库读取成功时）
		if fromDB && err == nil && user != nil {
			gopool.Go(func() {
				// 创建缓存对象
				cacheObj := &UserBase{
					Id:       user.Id,
					Group:    user.Group,
					Quota:    user.Quota,
					Status:   user.Status,
					Username: user.Username,
					Email:    user.Email,
				}
				// 异步写入本地缓存
				userLocalCache.Set(cacheKey, cacheObj)
				common.SysLog(fmt.Sprintf("[UserCache] Local Cache WRITE SUCCESS (async) for userId=%s", userId))
				// 异步写入Redis缓存
				if common.RedisEnabled {
					if err := updateUserCache(*user); err != nil {
						common.SysLog(fmt.Sprintf("[UserCache] Redis Cache WRITE FAILED (async) for userId=%s, error=%v", userId, err))
					} else {
						common.SysLog(fmt.Sprintf("[UserCache] Redis Cache WRITE SUCCESS (async) for userId=%s", userId))
					}
				}
			})
		}
	}()

	// 1. 先查本地缓存
	if cachedValue, found := userLocalCache.Get(cacheKey); found {
		// 本地缓存命中
		if cachedUser, ok := cachedValue.(*UserBase); ok {
			common.SysLog(fmt.Sprintf("[UserCache] Local Cache HIT for userId=%s, group=%s, quota=%d", userId, cachedUser.Group, cachedUser.Quota))
			return cachedUser, nil
		}
	}
	common.SysLog(fmt.Sprintf("[UserCache] Local Cache MISS for userId=%s", userId))

	// 2. 查Redis缓存
	userCache, err = cacheGetUserBase(userId)
	if err == nil {
		// Redis缓存命中，同步写入本地缓存（需要立即可用）
		userLocalCache.Set(cacheKey, userCache)
		common.SysLog(fmt.Sprintf("[UserCache] Redis Cache HIT for userId=%s, group=%s, quota=%d", userId, userCache.Group, userCache.Quota))
		return userCache, nil
	}
	common.SysLog(fmt.Sprintf("[UserCache] Redis Cache MISS for userId=%s, error=%v", userId, err))

	// 3. 从数据库读取
	fromDB = true
	user, err = GetUserByIdEnhance(userId, false)
	if err != nil {
		return nil, err // Return nil and error if DB lookup fails
	}

	// Create cache object from user data
	userCache = &UserBase{
		Id:       user.Id,
		Group:    user.Group,
		Quota:    user.Quota,
		Status:   user.Status,
		Username: user.Username,
		Email:    user.Email,
	}

	common.SysLog(fmt.Sprintf("[UserCache] DB query SUCCESS for userId=%s, group=%s, quota=%d", userId, userCache.Group, userCache.Quota))

	return userCache, nil
}

func cacheGetUserBase(userId string) (*UserBase, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var userCache UserBase
	// Try getting from Redis using JSON
	err := common.RedisGetJSON(getUserCacheKey(userId), &userCache)
	if err != nil {
		return nil, err
	}
	return &userCache, nil
}

// Add atomic quota operations using hash fields
func cacheIncrUserQuota(userId string, delta int64) error {
	// 同步更新本地缓存
	LocalCacheIncrUserQuota(userId, int(delta))

	// 更新 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHIncrBy(getUserCacheKey(userId), "Quota", delta)
}

func cacheDecrUserQuota(userId string, delta int64) error {
	return cacheIncrUserQuota(userId, -delta)
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId string) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId string) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserStatusCache(userId string) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Status, nil
}

func getUserNameCache(userId string) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

// New functions for individual field updates
func updateUserStatusCache(userId string, status bool) error {
	statusInt := common.UserStatusEnabled
	if !status {
		statusInt = common.UserStatusDisabled
	}

	// 同步更新本地缓存
	LocalCacheUpdateUserStatus(userId, statusInt)

	// 更新 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Status", fmt.Sprintf("%d", statusInt))
}

func updateUserQuotaCache(userId string, quota int) error {
	// 同步更新本地缓存
	LocalCacheUpdateUserQuota(userId, quota)

	// 更新 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Quota", fmt.Sprintf("%d", quota))
}

func updateUserGroupCache(userId string, group string) error {
	// 同步更新本地缓存
	LocalCacheUpdateUserGroup(userId, group)

	// 更新 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Group", group)
}

func updateUserNameCache(userId string, username string) error {
	// 同步更新本地缓存
	LocalCacheUpdateUserName(userId, username)

	// 更新 Redis 缓存
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Username", username)
}

func updateUserSettingCache(userId string, setting string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Setting", setting)
}

// ========== 本地缓存辅助函数 ==========

// LocalCacheSetUser 将 UserBase 写入本地缓存
func LocalCacheSetUser(userId string, user *UserBase) {
	if user == nil {
		return
	}
	cacheKey := getUserCacheKey(userId)
	userLocalCache.Set(cacheKey, user)
	common.SysLog(fmt.Sprintf("[UserCache] Local Cache SET for userId=%s, group=%s, quota=%d", userId, user.Group, user.Quota))
}

// LocalCacheDeleteUser 从本地缓存删除 User
func LocalCacheDeleteUser(userId string) {
	cacheKey := getUserCacheKey(userId)
	userLocalCache.Delete(cacheKey)
	common.SysLog(fmt.Sprintf("[UserCache] Local Cache DELETE for userId=%s", userId))
}

// LocalCacheUpdateUserQuota 更新本地缓存中的 User 配额
func LocalCacheUpdateUserQuota(userId string, quota int) {
	cacheKey := getUserCacheKey(userId)

	// 尝试从本地缓存获取
	if cachedValue, found := userLocalCache.Get(cacheKey); found {
		if cachedUser, ok := cachedValue.(*UserBase); ok {
			// 更新配额字段
			cachedUser.Quota = quota

			// 重新写入缓存
			userLocalCache.Set(cacheKey, cachedUser)
			common.SysLog(fmt.Sprintf("[UserCache] Local Cache UPDATE QUOTA for userId=%s, quota=%d", userId, quota))
		}
	}
}

// LocalCacheIncrUserQuota 增加本地缓存中的 User 配额
func LocalCacheIncrUserQuota(userId string, delta int) {
	cacheKey := getUserCacheKey(userId)

	// 尝试从本地缓存获取
	if cachedValue, found := userLocalCache.Get(cacheKey); found {
		if cachedUser, ok := cachedValue.(*UserBase); ok {
			// 增加配额
			cachedUser.Quota += delta

			// 重新写入缓存
			userLocalCache.Set(cacheKey, cachedUser)
			common.SysLog(fmt.Sprintf("[UserCache] Local Cache INCR QUOTA for userId=%s, delta=%d, new_quota=%d", userId, delta, cachedUser.Quota))
		}
	}
}

// LocalCacheUpdateUserStatus 更新本地缓存中的 User 状态
func LocalCacheUpdateUserStatus(userId string, status int) {
	cacheKey := getUserCacheKey(userId)

	// 尝试从本地缓存获取
	if cachedValue, found := userLocalCache.Get(cacheKey); found {
		if cachedUser, ok := cachedValue.(*UserBase); ok {
			// 更新状态字段
			cachedUser.Status = status

			// 重新写入缓存
			userLocalCache.Set(cacheKey, cachedUser)
			common.SysLog(fmt.Sprintf("[UserCache] Local Cache UPDATE STATUS for userId=%s, status=%d", userId, status))
		}
	}
}

// LocalCacheUpdateUserGroup 更新本地缓存中的 User 分组
func LocalCacheUpdateUserGroup(userId string, group string) {
	cacheKey := getUserCacheKey(userId)

	// 尝试从本地缓存获取
	if cachedValue, found := userLocalCache.Get(cacheKey); found {
		if cachedUser, ok := cachedValue.(*UserBase); ok {
			// 更新分组字段
			cachedUser.Group = group

			// 重新写入缓存
			userLocalCache.Set(cacheKey, cachedUser)
			common.SysLog(fmt.Sprintf("[UserCache] Local Cache UPDATE GROUP for userId=%s, group=%s", userId, group))
		}
	}
}

// LocalCacheUpdateUserName 更新本地缓存中的 User 用户名
func LocalCacheUpdateUserName(userId string, username string) {
	cacheKey := getUserCacheKey(userId)

	// 尝试从本地缓存获取
	if cachedValue, found := userLocalCache.Get(cacheKey); found {
		if cachedUser, ok := cachedValue.(*UserBase); ok {
			// 更新用户名字段
			cachedUser.Username = username

			// 重新写入缓存
			userLocalCache.Set(cacheKey, cachedUser)
			common.SysLog(fmt.Sprintf("[UserCache] Local Cache UPDATE USERNAME for userId=%s, username=%s", userId, username))
		}
	}
}
