package database

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"yupao-go/ent"

	"github.com/joho/godotenv"
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

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	port, err := strconv.Atoi(getEnv("POSTGRES_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT: %w", err)
	}

	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     port,
		Username: getEnv("POSTGRES_USER", "root"),
		Password: getEnv("POSTGRES_PASSWORD", "root"),
		DBName:   getEnv("POSTGRES_DB", "yupao_db"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func New(config *Config) (*DB, error) {
	client, err := ent.Open("postgres", config.DSN())
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
