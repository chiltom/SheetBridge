package utils

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config represents the application's configuration
type Config struct {
	App struct {
		Env  string // "dev", "prod", "test"
		Port string
		// BaseURL string // Useful for constructing full URLs if needed
	}
	DB struct {
		Host         string
		Name         string
		User         string
		Password     string
		Port         string
		DSN          string
		MaxOpenConns int
		MaxIdleConns int
		MaxIdleTime  time.Duration
	}
	// Add other configs when needed
}

// LoadConfig loads the application config from a .env file
func LoadConfig(path ...string) *Config {
	// Attempt to load .env file. If path is provided, use it.
	// Otherwise, try to load from the current directory.
	// godotenv.Load() is fine for development, but in prod, env vars are usually set directly.
	envPath := ".env"
	if len(path) > 0 {
		envPath = path[0]
	}
	err := godotenv.Load(envPath)
	if err != nil {
		log.Printf("Info: No .env file found at %s, relying on environment variables. Error: %v", envPath, err)
	}

	var cfg Config

	cfg.App.Env = strings.ToLower(os.Getenv("SERVER_ENV"))
	if cfg.App.Env == "" {
		cfg.App.Env = "dev" // Default to development
	}

	cfg.App.Port = os.Getenv("SERVER_PORT")
	if cfg.App.Port == "" {
		cfg.App.Port = "8000"
	}

	// PostgreSQL DSN Example: "postgres://user:password@localhost:5432/dbname?sslmode=disable"

	cfg.DB.Host = os.Getenv("DB_HOST")
	if cfg.DB.Host == "" {
		cfg.DB.Host = "localhost"
	}

	cfg.DB.Name = os.Getenv("DB_NAME")
	if cfg.DB.Name == "" {
		cfg.DB.Name = "spreadsheet_db"
	}

	cfg.DB.User = os.Getenv("DB_USER")
	if cfg.DB.User == "" {
		cfg.DB.User = "spreadsheet_user"
	}

	cfg.DB.Password = os.Getenv("DB_PASSWORD")
	if cfg.DB.Password == "" {
		cfg.DB.Password = "spreadsheet_password"
	}

	cfg.DB.Port = os.Getenv("DB_PORT")
	if cfg.DB.Port == "" {
		cfg.DB.Port = "5432"
	}

	cfg.DB.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name)

	cfg.DB.MaxOpenConns, err = strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS"))
	if err != nil || cfg.DB.MaxOpenConns == 0 {
		cfg.DB.MaxOpenConns = 25
	}

	cfg.DB.MaxIdleConns, err = strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS"))
	if err != nil || cfg.DB.MaxIdleConns == 0 {
		cfg.DB.MaxIdleConns = 15
	}

	maxIdleTimeStr := os.Getenv("DB_MAX_IDLE_TIME")
	if maxIdleTimeStr == "" {
		maxIdleTimeStr = "15m"
	}
	cfg.DB.MaxIdleTime, err = time.ParseDuration(maxIdleTimeStr)
	if err != nil {
		cfg.DB.MaxIdleTime = 15 * time.Minute
	}

	return &cfg
}

// IsDevelopment returns whether the application is in development configuration or not
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "dev"
}
