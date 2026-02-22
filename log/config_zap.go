package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

// this config is used to configure the zap logger by yaml file
// additional to the zap config, it contains a default level and a map of named loggers
// with theire respective levels. These level have precedence over the default level.
type Config struct {
	DefaultLevel string            `yaml:"defaultLevel"`
	Loggers      map[string]string `yaml:"loggers"`
	Zap          zap.Config        `yaml:"zap"`
	Filters      []string          `yaml:"filters"`
}

func DefaultDevConfig() *Config {
	z := zap.NewDevelopmentConfig()
	z.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	return &Config{
		DefaultLevel: "info",
		Zap:          z,
	}
}

func DefaultProdConfig() *Config {
	return &Config{
		DefaultLevel: "info",
		Zap:          zap.NewProductionConfig(),
	}
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := Config{
		Zap: zap.NewProductionConfig(),
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
