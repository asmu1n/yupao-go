package router

import (
	"fmt"
	"os"

	"yupao-go/ent"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "yupao-go/docs"
)

func New(client *ent.Client) *gin.Engine {
	r := gin.Default()

	// 定义 cookie 中心存储，并传入加密密钥
	// store 直接读取 request 获取 `name` 对应的 cookie 信息解析成 session map结构数据，也是直接写入 response 来指定 cookie 信息
	// store := cookie.NewStore([]byte("yupao-secret-key"))

	store := newRedisStore()
	// 使用 session 中间件，指定 cookie 名称，以及所使用的存储中心
	// 中间件闭包函数本身接收了 请求上下文，赋予 store 读取 request.cookie 和 操作 ResponseWriter 的能力
	r.Use(sessions.Sessions("session", store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	registerUser(api, client)

	return r
}

func newRedisStore() redis.Store {
	_ = godotenv.Load()

	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")
	password := getEnv("REDIS_PASSWORD", "")

	addr := fmt.Sprintf("%s:%s", host, port)

	store, err := redis.NewStore(10, "tcp", addr, "", password, []byte("yupao-secret-key"))
	if err != nil {
		panic(fmt.Sprintf("connect redis: %v", err))
	}
	return store
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
