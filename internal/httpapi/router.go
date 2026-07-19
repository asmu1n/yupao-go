package httpapi

import (
	"yupao-go/internal/module/user"

	"github.com/gin-gonic/gin"

	_ "yupao-go/api/swagger"
)

// RegisterRouter 注册全部 HTTP 路由。
func RegisterRouter(r *gin.Engine, userSvc *user.Service) {
	api := r.Group("/api")
	registerUser(api, userSvc)
}
