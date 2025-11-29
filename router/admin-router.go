package router

import (
	"relay-gateway/controller"
	"relay-gateway/middleware"

	"github.com/gin-gonic/gin"
)

// SetAdminRouter 设置管理 API 路由
// 这些路由用于内部管理，如缓存管理等
func SetAdminRouter(router *gin.Engine) {
	router.Use(middleware.CORS())

	// 管理 API 路由组
	adminRouter := router.Group("/api/admin")
	{
		// Token 缓存管理
		tokenCacheRouter := adminRouter.Group("/token/cache")
		{
			// 单个删除
			tokenCacheRouter.POST("/delete", controller.DeleteTokenCache)
			// 批量删除
			tokenCacheRouter.POST("/batch-delete", controller.BatchDeleteTokenCache)
		}

		// User 缓存管理
		userCacheRouter := adminRouter.Group("/user/cache")
		{
			// 单个删除
			userCacheRouter.POST("/delete", controller.DeleteUserCache)
			// 批量删除
			userCacheRouter.POST("/batch-delete", controller.BatchDeleteUserCache)
		}
	}
}
