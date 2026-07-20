package main

import (
	"context"
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
	"yupao-go/internal/pkg/logger"

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
	logger.Init()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := gin.Default()

	db, err := database.New()
	if err != nil {
		logger.Fatal("connect db failed", logger.FieldErr, err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		logger.Fatal("migrate failed", logger.FieldErr, err)
	}

	store, err := redis.NewSessionStore()
	if err != nil {
		logger.Fatal("load redis session store failed", logger.FieldErr, err)
	}

	redisClient, err := redis.NewClient()
	if err != nil {
		logger.Fatal("connect redis failed", logger.FieldErr, err)
	}
	defer redisClient.Close()

	locker := lock.New(redisClient)
	cacheClient := cache.New(redisClient)
	userSvc := user.NewService(userrepo.New(db.Client), cacheClient, locker)
	teamSvc := team.NewService(teamrepo.New(db.Client), userSvc, locker)

	sched := scheduler.New()
	// 每天 03:00 预热；候选池与在线 MatchUsers 一致
	if _, err := sched.Schedule("0 0 3 * * *", func() {
		taskCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		userSvc.WarmUpMatchUsers(taskCtx)
	}); err != nil {
		logger.Fatal("schedule warmup failed", logger.FieldErr, err)
	}
	sched.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := sched.Stop(shutdownCtx); err != nil {
			logger.Warn("scheduler stop",
				logger.FieldPurpose, logger.PurposeJob,
				logger.FieldModule, "scheduler",
				logger.FieldEvent, "cron.stop_error",
				logger.FieldErr, err,
			)
		}
	}()

	r.Use(sessions.Sessions("session", store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	httpapi.RegisterRouter(r, userSvc, teamSvc)

	logger.Info("http server starting",
		logger.FieldPurpose, logger.PurposeInfra,
		logger.FieldEvent, "http.listen",
		"addr", ":8080",
	)
	if err := r.Run(":8080"); err != nil {
		logger.Fatal("http server stopped", logger.FieldErr, err)
	}
}
