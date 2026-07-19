package httpapi

import (
	"yupao-go/internal/httpapi/middleware"
	"yupao-go/internal/module/user"
	userhttp "yupao-go/internal/module/user/http"

	"github.com/gin-gonic/gin"
)

// registerUser 注册用户相关路由。
func registerUser(api *gin.RouterGroup, userSvc *user.Service) {
	h := userhttp.NewHandler(userSvc)

	u := api.Group("/user")
	{
		u.POST("/register", h.Register)
		u.POST("/login", h.Login)

		auth := u.Group("", middleware.AuthRequired())
		{
			auth.POST("/logout", h.Logout)
			auth.GET("/current", h.CurrentUser)
			auth.GET("/search/tags", h.SearchByTags)
			auth.POST("/update", h.Update)
			auth.GET("/match", h.MatchUsers)
		}
	}
}
