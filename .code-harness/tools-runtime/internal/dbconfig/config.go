package dbconfig

import (
	"errors"
	"fmt"
	"os"

	"codea-harness-tools/internal/schema"
	"gopkg.in/yaml.v3"
)

const (
	EnvironmentTest  = "TEST"
	EnvironmentLocal = "LOCAL"
	DialectMySQL     = "mysql"
)

type Config struct {
	Version     int        `yaml:"version"`
	Enabled     bool       `yaml:"enabled"`
	Environment string     `yaml:"environment"`
	Dialect     string     `yaml:"dialect"`
	Connection  Connection `yaml:"connection"`
	Safety      Safety     `yaml:"safety"`
}

type Connection struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Charset  string `yaml:"charset"`
}

type Safety struct {
	AllowedSchemas         []string `yaml:"allowedSchemas"`
	MaxRows                int      `yaml:"maxRows"`
	TimeoutSeconds         int      `yaml:"timeoutSeconds"`
	MaxQueriesPerDiagnosis int      `yaml:"maxQueriesPerDiagnosis"`
	AllowSchemaDiscovery   bool     `yaml:"allowSchemaDiscovery"`
	AllowReadonlySQL       bool     `yaml:"allowReadonlySql"`
}

func Load(path, schemaPath string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Enabled: false}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read database config: %w", err)
	}
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return Config{}, fmt.Errorf("read database config schema: %w", err)
	}
	if err := schema.ValidateYAML(schemaBytes, data); err != nil {
		return Config{}, errors.New("database config validation failed")
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, errors.New("database config decoding failed")
	}
	if err := validateSafety(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateSafety(cfg Config) error {
	if cfg.Version != 1 {
		return errors.New("database config version must be 1")
	}
	if cfg.Environment != EnvironmentTest && cfg.Environment != EnvironmentLocal {
		return errors.New("database environment must be TEST or LOCAL")
	}
	if cfg.Dialect != DialectMySQL {
		return errors.New("database dialect must be mysql")
	}
	if len(cfg.Safety.AllowedSchemas) == 0 {
		return errors.New("database allowedSchemas must not be empty")
	}
	if cfg.Safety.MaxRows < 1 || cfg.Safety.MaxRows > 1000 {
		return errors.New("database maxRows must be between 1 and 1000")
	}
	if cfg.Safety.TimeoutSeconds < 1 || cfg.Safety.TimeoutSeconds > 30 {
		return errors.New("database timeoutSeconds must be between 1 and 30")
	}
	if cfg.Safety.MaxQueriesPerDiagnosis < 1 || cfg.Safety.MaxQueriesPerDiagnosis > 20 {
		return errors.New("database maxQueriesPerDiagnosis must be between 1 and 20")
	}
	return nil
}
