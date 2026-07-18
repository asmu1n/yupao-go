package router

import (
	"yupao-go/internal/domain/user"
	"yupao-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

func registerUser(api *gin.RouterGroup, userSvc *user.Service) {
	h := user.NewUserHandler(userSvc)

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
