package session

import (
	"fmt"

	"yupao-go/internal/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
)

type Config struct {
	Host     string
	Port     string
	Password string
}

const defaultSecret = "yupao-secret-key"

func loadConfig() *Config {
	config.LoadEnv()

	return &Config{
		Host:     config.GetEnv("REDIS_HOST", "localhost"),
		Port:     config.GetEnv("REDIS_PORT", "6379"),
		Password: config.GetEnv("REDIS_PASSWORD", ""),
	}
}

func NewRedisStore() (sessions.Store, error) {
	cfg := loadConfig()

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	store, err := redis.NewStore(10, "tcp", addr, "", cfg.Password, []byte(defaultSecret))
	if err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	return store, nil
}
