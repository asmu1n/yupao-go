package main

import (
	"context"
	"log"

	"yupao-go/internal/domain/user"
	urepo "yupao-go/internal/domain/user/repository"
	"yupao-go/internal/infra/cache"
	"yupao-go/internal/infra/database"
	"yupao-go/internal/infra/lock"
	"yupao-go/internal/infra/redis"
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

	store, err := redis.NewSessionStore()

	if err != nil {
		log.Fatalf("failed load redis: %v", err)
	}

	redisClient, err := redis.NewClient()
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	locker := lock.New(redisClient)

	cacheClient := cache.New(redisClient)

	userSvc := user.NewService(urepo.New(db.Client), cacheClient, locker)

	// 使用 session 中间件，指定 cookie 名称，以及所使用的存储中心
	// 中间件闭包函数本身接收了 请求上下文，赋予 store 读取 request.cookie 和 操作 ResponseWriter 的能力
	r.Use(sessions.Sessions("session", store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.RegisterRouter(r, userSvc)

	log.Fatal(r.Run(":8080"))
}
