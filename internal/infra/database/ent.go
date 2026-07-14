package database

import (
	"context"
	"fmt"
	"strconv"

	"yupao-go/ent"
	"yupao-go/internal/config"

	_ "github.com/lib/pq"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	DBName   string
}

type DB struct {
	Client *ent.Client
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.Username, c.Password, c.DBName)
}

func loadConfig() (*Config, error) {
	config.LoadEnv()

	port, err := strconv.Atoi(config.GetEnv("POSTGRES_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT: %w", err)
	}

	return &Config{
		Host:     config.GetEnv("DB_HOST", "localhost"),
		Port:     port,
		Username: config.GetEnv("POSTGRES_USER", "root"),
		Password: config.GetEnv("POSTGRES_PASSWORD", "root"),
		DBName:   config.GetEnv("POSTGRES_DB", "yupao_db"),
	}, nil
}

func New() (*DB, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	client, err := ent.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &DB{Client: client}, nil
}

func (db *DB) Migrate(ctx context.Context) error {
	return db.Client.Schema.Create(ctx)
}

func (db *DB) Close() error {
	if db.Client != nil {
		return db.Client.Close()
	}
	return nil
}
