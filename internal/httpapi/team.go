package httpapi

import (
	"yupao-go/internal/httpapi/middleware"
	"yupao-go/internal/module/team"
	teamhttp "yupao-go/internal/module/team/http"
	"yupao-go/internal/module/user"

	"github.com/gin-gonic/gin"
)

// registerTeam 注册队伍相关路由。
func registerTeam(api *gin.RouterGroup, teamSvc *team.Service, userSvc *user.Service) {
	h := teamhttp.NewHandler(teamSvc, userSvc)

	t := api.Group("/team")
	{
		// 列表可匿名（用于浏览公开队伍）；hasJoin 在已登录时填充
		t.GET("/list", h.List)
		t.GET("/list/page", h.ListPage)
		t.GET("/get", h.Get)

		auth := t.Group("", middleware.AuthRequired())
		{
			auth.POST("/add", h.Add)
			auth.POST("/update", h.Update)
			auth.POST("/join", h.Join)
			auth.POST("/quit", h.Quit)
			auth.POST("/delete", h.Delete)
			auth.GET("/list/my/create", h.ListMyCreate)
			auth.GET("/list/my/join", h.ListMyJoin)
		}
	}
}
