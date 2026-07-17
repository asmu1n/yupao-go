package main

import (
	"context"
	"log"

	"yupao-go/internal/infra/database"
	"yupao-go/internal/infra/session"
	"yupao-go/internal/router"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "yupao-go/api/swagger"
)

// @title           Yupao Go API
// @version         1.0
// @description     伙伴匹配系统后端接口文档
// @host            localhost:8080
// @BasePath        /api
// @securityDefinitions.apikey SessionAuth
// @in header
// @name Cookie
// @description Session cookie authentication. Example: session=your-session-id

func main() {

	r := gin.Default()

	db, err := database.New()
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store, err := session.NewRedisStore()

	if err != nil {
		log.Fatalf("failed load redis: %v", err)
	}

	// 使用 session 中间件，指定 cookie 名称，以及所使用的存储中心
	// 中间件闭包函数本身接收了 请求上下文，赋予 store 读取 request.cookie 和 操作 ResponseWriter 的能力
	r.Use(sessions.Sessions("session", store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.RegisterRouter(r, db.Client)

	log.Fatal(r.Run(":8080"))
}
