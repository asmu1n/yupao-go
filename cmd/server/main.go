package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yupao-go/internal/httpapi"
	"yupao-go/internal/infra/cache"
	"yupao-go/internal/infra/database"
	"yupao-go/internal/infra/lock"
	"yupao-go/internal/infra/redis"
	"yupao-go/internal/infra/scheduler"
	"yupao-go/internal/module/team"
	teamrepo "yupao-go/internal/module/team/repo"
	"yupao-go/internal/module/user"
	userrepo "yupao-go/internal/module/user/repo"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "yupao-go/docs/api/swagger"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := gin.Default()

	db, err := database.New()
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
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
	userSvc := user.NewService(userrepo.New(db.Client), cacheClient, locker)
	teamSvc := team.NewService(teamrepo.New(db.Client), userSvc, locker)

	// 初始化定时调度器
	sched := scheduler.New()
	// 每天 03:00 预热；候选池与在线 MatchUsers 一致（近 matchActiveWindow 活跃用户）
	if _, err := sched.Schedule("0 0 3 * * *", func() {
		taskCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		userSvc.WarmUpMatchUsers(taskCtx)
	}); err != nil {
		log.Fatalf("schedule warmup: %v", err)
	}
	sched.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := sched.Stop(shutdownCtx); err != nil {
			log.Printf("scheduler stop: %v", err)
		}
	}()

	// 使用 session 中间件，指定 cookie 名称，以及所使用的存储中心
	// 中间件闭包函数本身接收了 请求上下文，赋予 store 读取 request.cookie 和 操作 ResponseWriter 的能力
	r.Use(sessions.Sessions("session", store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	httpapi.RegisterRouter(r, userSvc, teamSvc)

	log.Fatal(r.Run(":8080"))
}
