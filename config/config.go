package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppName string
	AppEnv  string
	AppPort int

	LogLevel string

	// Postgres (optional; enabled only when DBHost + DBName are set).
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     int
	DBName     string

	// Redis (optional; enabled only when RedisHost is set).
	RedisUser     string
	RedisPassword string
	RedisHost     string
	RedisPort     int
	RedisScheme   string
}

func NewViper() *viper.Viper {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("APP_NAME", "peasydeal-product-miner")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("LOG_LEVEL", "info")

	v.SetDefault("DB_PORT", 5432)
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_SCHEME", "redis")

	return v
}

func NewConfig(v *viper.Viper) (Config, error) {
	cfg := Config{
		AppName: v.GetString("APP_NAME"),
		AppEnv:  v.GetString("APP_ENV"),
		AppPort: v.GetInt("APP_PORT"),

		LogLevel: v.GetString("LOG_LEVEL"),

		DBUser:     v.GetString("DB_USER"),
		DBPassword: v.GetString("DB_PASSWORD"),
		DBHost:     v.GetString("DB_HOST"),
		DBPort:     v.GetInt("DB_PORT"),
		DBName:     v.GetString("DB_NAME"),

		RedisUser:     v.GetString("REDIS_USER"),
		RedisPassword: v.GetString("REDIS_PASSWORD"),
		RedisHost:     v.GetString("REDIS_HOST"),
		RedisPort:     v.GetInt("REDIS_PORT"),
		RedisScheme:   v.GetString("REDIS_SCHEME"),
	}

	if cfg.AppPort <= 0 || cfg.AppPort > 65535 {
		return Config{}, fmt.Errorf("invalid APP_PORT %d", cfg.AppPort)
	}
	if cfg.DBPort <= 0 || cfg.DBPort > 65535 {
		return Config{}, fmt.Errorf("invalid DB_PORT %d", cfg.DBPort)
	}
	if cfg.RedisPort <= 0 || cfg.RedisPort > 65535 {
		return Config{}, fmt.Errorf("invalid REDIS_PORT %d", cfg.RedisPort)
	}

	return cfg, nil
}
