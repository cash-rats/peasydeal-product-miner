package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppName  string
	Env      string
	Port     int
	LogLevel string

	DB    DBConfig
	Redis RedisConfig
}

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Name     string
}

type RedisConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Scheme   string
}

func NewViper() *viper.Viper {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	v.SetDefault("APP_NAME", "go-vps-service")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("LOG_LEVEL", "info")

	v.SetDefault("DB_USER", "")
	v.SetDefault("DB_PASSWORD", "")
	v.SetDefault("DB_HOST", "")
	v.SetDefault("DB_PORT", 5432)
	v.SetDefault("DB_NAME", "")

	v.SetDefault("REDIS_USER", "")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_HOST", "")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_SCHEME", "redis")

	return v
}

func NewConfig(v *viper.Viper) (Config, error) {
	return Config{
		AppName:  v.GetString("APP_NAME"),
		Env:      v.GetString("APP_ENV"),
		Port:     v.GetInt("APP_PORT"),
		LogLevel: v.GetString("LOG_LEVEL"),
		DB: DBConfig{
			User:     v.GetString("DB_USER"),
			Password: v.GetString("DB_PASSWORD"),
			Host:     v.GetString("DB_HOST"),
			Port:     v.GetInt("DB_PORT"),
			Name:     v.GetString("DB_NAME"),
		},
		Redis: RedisConfig{
			User:     v.GetString("REDIS_USER"),
			Password: v.GetString("REDIS_PASSWORD"),
			Host:     v.GetString("REDIS_HOST"),
			Port:     v.GetInt("REDIS_PORT"),
			Scheme:   v.GetString("REDIS_SCHEME"),
		},
	}, nil
}
