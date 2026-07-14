package router

import (
	"yupao-go/ent"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "yupao-go/api/swagger"
)

func New(client *ent.Client, store sessions.Store) *gin.Engine {
	r := gin.Default()

	// 使用 session 中间件，指定 cookie 名称，以及所使用的存储中心
	// 中间件闭包函数本身接收了 请求上下文，赋予 store 读取 request.cookie 和 操作 ResponseWriter 的能力
	r.Use(sessions.Sessions("session", store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	registerUser(api, client)

	return r
}
