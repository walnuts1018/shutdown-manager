package config

import (
	"log/slog"
	"reflect"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Port     string     `env:"PORT" envDefault:"8080"`
	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"info"`
	LogType  LogType    `env:"LOG_TYPE" envDefault:"json"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.ParseWithOptions(cfg, env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(slog.Level(0)): returnAny(ParseLogLevel),
			reflect.TypeOf(LogType("")):   returnAny(ParseLogType),
		},
	}); err != nil {
		return nil, err
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
