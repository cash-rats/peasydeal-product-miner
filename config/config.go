package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type ENV string

// Vercel envs
var (
	Production ENV = "production"
	Preview    ENV = "preview"
	Dev        ENV = "development"
	Test       ENV = "test"
)

func NewViper() *viper.Viper {
	vp := viper.New()

	vp.SetDefault("env", Dev)
	vp.SetDefault("app.port", "8080")
	vp.SetDefault("app.addr", "0.0.0.0")

	vp.SetDefault("db.host", "")
	vp.SetDefault("db.port", 5432)
	vp.SetDefault("db.user", "")
	vp.SetDefault("db.password", "")
	vp.SetDefault("db.name", "")
	vp.SetDefault("db.timezone", "")

	vp.SetDefault("redis.host", "")
	vp.SetDefault("redis.user", "default")
	vp.SetDefault("redis.port", 6379)
	vp.SetDefault("redis.password", "")
	vp.SetDefault("redis.db", 0)

	vp.SetDefault("inngest.dev", "")
	vp.SetDefault("inngest.app_id", "")
	vp.SetDefault("inngest.signing_key", "")
	vp.SetDefault("inngest.serve_host", "")
	vp.SetDefault("inngest.serve_path", "")

	replacer := strings.NewReplacer(".", "_")
	vp.SetEnvKeyReplacer(replacer)
	vp.AutomaticEnv()

	return vp
}

type Config struct {
	ENV ENV `mapstructure:"env"`

	App struct {
		Port string `mapstructure:"port"`
		Addr string `mapstructure:"addr"`
	} `mapstructure:"app"`

	DB struct {
		Host     string `mapstructure:"host"`
		Port     uint   `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Name     string `mapstructure:"name"`
		TimeZone string `mapstructure:"timezone"`
	} `mapstructure:"db"`

	Redis struct {
		User     string `mapstructure:"user"`
		Host     string `mapstructure:"host"`
		Port     uint   `mapstructure:"port"`
		Password string `mapstructure:"password"`
		DB       uint   `mapstructure:"db"`
	} `mapstructure:"redis"`
}

func NewConfig(vp *viper.Viper) (*Config, error) {
	cfg := &Config{}
	if err := vp.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}
