package router

import (
	"yupao-go/internal/domain/user"

	"github.com/gin-gonic/gin"

	_ "yupao-go/api/swagger"
)

func RegisterRouter(r *gin.Engine, userSvc *user.Service) {
	api := r.Group("/api")
	registerUser(api, userSvc)
}
