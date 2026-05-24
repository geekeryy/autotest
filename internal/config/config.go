package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultDevJWTSecret = "autotest-dev-secret-change-me"

	defaultPostgresUser     = "autotest"
	defaultPostgresPassword = "autotest"
	defaultPostgresHost     = "localhost"
	defaultPostgresPort     = "5432"
	defaultPostgresDB       = "autotest"
	defaultPostgresSSLMode  = "disable"
)

// Database holds PostgreSQL connection settings from POSTGRES_* env vars.
type Database struct {
	User     string
	Password string
	Host     string
	Port     string
	DB       string
	SSLMode  string
}

// Environment identifies the application runtime environment.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvTest        Environment = "test"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// Config is the process-wide application configuration loaded once at startup.
type Config struct {
	Env                Environment
	Addr               string
	DatabaseURL        string
	JWTSecret          string
	CORSAllowedOrigins []string
	LogLevel           string
	LogFormat          string
}

// Load reads configuration from the environment (and .env in non-production).
func Load() (Config, error) {
	loadDotEnvIfAllowed()

	env := resolveEnvironment()
	db, err := loadDatabase()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Env:                env,
		Addr:               envOr("ADDR", ":8080"),
		DatabaseURL:        BuildDatabaseURL(db),
		JWTSecret:          envOr("JWT_SECRET", defaultDevJWTSecret),
		CORSAllowedOrigins: loadCORSAllowedOrigins(env),
		LogLevel:           defaultLogLevel(env),
		LogFormat:          defaultLogFormat(env),
	}

	if v := strings.TrimSpace(os.Getenv("LOG_LEVEL")); v != "" {
		cfg.LogLevel = v
	}
	if v := strings.TrimSpace(os.Getenv("LOG_FORMAT")); v != "" {
		cfg.LogFormat = v
	}

	if cfg.Env == EnvProduction {
		if err := cfg.validateProduction(); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func (c Config) IsDevelopment() bool { return c.Env == EnvDevelopment }
func (c Config) IsTest() bool        { return c.Env == EnvTest }
func (c Config) IsProduction() bool  { return c.Env == EnvProduction }
func (c Config) EnableDevCORS() bool { return c.Env == EnvDevelopment }
func (c Config) EnableCORS() bool    { return true }

func (c Config) validateProduction() error {
	var errs []error
	if strings.TrimSpace(c.DatabaseURL) == "" {
		errs = append(errs, errors.New("POSTGRES_* database settings are required in production"))
	}
	if c.DatabaseURL == BuildDatabaseURL(defaultDatabase()) {
		errs = append(errs, errors.New("POSTGRES_PASSWORD must not use the default development password in production"))
	}
	if c.JWTSecret == "" || c.JWTSecret == defaultDevJWTSecret {
		errs = append(errs, errors.New("JWT_SECRET must be set to a strong value in production"))
	}
	return errors.Join(errs...)
}

func loadDotEnvIfAllowed() {
	if isProductionEnv() {
		return
	}
	_ = godotenv.Load()
}

func isProductionEnv() bool {
	return resolveEnvironment() == EnvProduction
}

func resolveEnvironment() Environment {
	if v := strings.TrimSpace(os.Getenv("APP_ENV")); v != "" {
		return normalizeEnvironment(v)
	}
	return EnvDevelopment
}

func normalizeEnvironment(raw string) Environment {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dev", "development", "local":
		return EnvDevelopment
	case "test", "testing":
		return EnvTest
	case "stage", "staging":
		return EnvStaging
	case "prod", "production":
		return EnvProduction
	default:
		return Environment(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func loadCORSAllowedOrigins(env Environment) []string {
	if origins := splitCSVEnv("CORS_ALLOWED_ORIGINS"); len(origins) > 0 {
		return origins
	}
	if env == EnvDevelopment {
		return nil
	}
	return nil
}

func splitCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimRight(strings.TrimSpace(part), "/"); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func defaultLogLevel(env Environment) string {
	switch env {
	case EnvDevelopment:
		return "debug"
	case EnvTest:
		return "warn"
	default:
		return "info"
	}
}

func defaultLogFormat(env Environment) string {
	switch env {
	case EnvStaging, EnvProduction:
		return "json"
	default:
		return "text"
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func defaultDatabase() Database {
	return Database{
		User:     defaultPostgresUser,
		Password: defaultPostgresPassword,
		Host:     defaultPostgresHost,
		Port:     defaultPostgresPort,
		DB:       defaultPostgresDB,
		SSLMode:  defaultPostgresSSLMode,
	}
}

func loadDatabase() (Database, error) {
	db := Database{
		User:     envOr("POSTGRES_USER", defaultPostgresUser),
		Password: envOr("POSTGRES_PASSWORD", defaultPostgresPassword),
		Host:     envOr("POSTGRES_HOST", defaultPostgresHost),
		Port:     envOr("POSTGRES_PORT", defaultPostgresPort),
		DB:       envOr("POSTGRES_DB", defaultPostgresDB),
		SSLMode:  envOr("POSTGRES_SSLMODE", defaultPostgresSSLMode),
	}
	if db.User == "" {
		return Database{}, errors.New("POSTGRES_USER is required")
	}
	if db.DB == "" {
		return Database{}, errors.New("POSTGRES_DB is required")
	}
	return db, nil
}

// BuildDatabaseURL assembles a PostgreSQL connection URL from structured settings.
func BuildDatabaseURL(db Database) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(db.User, db.Password),
		Host:   net.JoinHostPort(db.Host, db.Port),
		Path:   "/" + db.DB,
	}
	if db.SSLMode != "" {
		u.RawQuery = "sslmode=" + url.QueryEscape(db.SSLMode)
	}
	return u.String()
}

// DatabaseURLFromEnv builds a PostgreSQL URL from POSTGRES_* environment variables.
func DatabaseURLFromEnv() (string, error) {
	db, err := loadDatabase()
	if err != nil {
		return "", err
	}
	return BuildDatabaseURL(db), nil
}

// AuthSettings maps config fields used by internal/auth.
type AuthSettings struct {
	JWTSecret       string
	IsDevelopment   bool
}

func (c Config) AuthSettings() AuthSettings {
	return AuthSettings{
		JWTSecret:     c.JWTSecret,
		IsDevelopment: c.IsDevelopment(),
	}
}

func (c Config) String() string {
	return fmt.Sprintf("env=%s addr=%s log_level=%s log_format=%s cors_env_origins=%d",
		c.Env, c.Addr, c.LogLevel, c.LogFormat, len(c.CORSAllowedOrigins))
}
