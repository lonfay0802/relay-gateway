package router

import (
	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine) {
	SetRelayRouter(router)
	SetVideoRouter(router)
	SetAdminRouter(router)
}
