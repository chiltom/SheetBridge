package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config represents the overall configuration of the application
type Config struct {
	DB     DBConfig
	Server ServerConfig
}

// DBConfig represents the db configuration for the application
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// ServerConfig represents the db configuration for the application
type ServerConfig struct {
	Port string
}

// LoadConfig loads all of the environment values into a Config
func LoadConfig() (Config, error) {
	if err := godotenv.Load(); err != nil {
		return Config{}, fmt.Errorf("failed to load .env: %w", err)
	}

	return Config{
		DB: DBConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     os.Getenv("DB_PORT"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
		},
		Server: ServerConfig{
			Port: os.Getenv("SERVER_PORT"),
		},
	}, nil
}
