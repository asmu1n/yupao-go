package router

import (
	"yupao-go/ent"
	"yupao-go/internal/domain/user"
	userHandler "yupao-go/internal/domain/user/handler"
	userRepo "yupao-go/internal/domain/user/repo"
	"yupao-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

func registerUser(api *gin.RouterGroup, client *ent.Client) {
	repo := userRepo.New(client)
	svc := user.NewService(repo)
	h := userHandler.NewUserHandler(svc)

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
