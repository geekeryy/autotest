package config

import "testing"

func TestNormalizeEnvironment(t *testing.T) {
	t.Parallel()

	cases := map[string]Environment{
		"development": EnvDevelopment,
		"dev":         EnvDevelopment,
		"test":        EnvTest,
		"staging":     EnvStaging,
		"production":  EnvProduction,
		"prod":        EnvProduction,
	}
	for in, want := range cases {
		if got := normalizeEnvironment(in); got != want {
			t.Fatalf("normalizeEnvironment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateProductionRejectsWeakSecrets(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Env:         EnvProduction,
		DatabaseURL: BuildDatabaseURL(defaultDatabase()),
		JWTSecret:   defaultDevJWTSecret,
	}
	if err := cfg.validateProduction(); err == nil {
		t.Fatal("expected production validation error")
	}
}

func TestValidateProductionAcceptsStrongSecrets(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Env:         EnvProduction,
		DatabaseURL: "postgres://user:pass@db:5432/autotest?sslmode=require",
		JWTSecret:   "long-random-production-secret",
	}
	if err := cfg.validateProduction(); err != nil {
		t.Fatalf("validateProduction: %v", err)
	}
}

func TestParseBoolEnv(t *testing.T) {
	t.Setenv("MCP_HTTP_ENABLED", "true")
	if !parseBoolEnv("MCP_HTTP_ENABLED") {
		t.Fatal("expected true")
	}
	t.Setenv("MCP_HTTP_ENABLED", "off")
	if parseBoolEnv("MCP_HTTP_ENABLED") {
		t.Fatal("expected false")
	}
}

func TestEnableDevCORS(t *testing.T) {
	t.Parallel()

	dev := Config{Env: EnvDevelopment}
	if !dev.EnableDevCORS() {
		t.Fatal("development should enable dev CORS")
	}
	if !dev.EnableCORS() {
		t.Fatal("development should enable CORS")
	}
	prod := Config{Env: EnvProduction, CORSAllowedOrigins: []string{"https://app.example.com"}}
	if prod.EnableDevCORS() {
		t.Fatal("production should not enable dev CORS")
	}
	if !prod.EnableCORS() {
		t.Fatal("production with allowed origins should enable CORS")
	}
}

func TestBuildDatabaseURL(t *testing.T) {
	t.Parallel()

	got := BuildDatabaseURL(Database{
		User:     "autotest",
		Password: "p@ss:w/rd",
		Host:     "localhost",
		Port:     "5432",
		DB:       "autotest",
		SSLMode:  "disable",
	})
	want := "postgres://autotest:p%40ss%3Aw%2Frd@localhost:5432/autotest?sslmode=disable"
	if got != want {
		t.Fatalf("BuildDatabaseURL() = %q, want %q", got, want)
	}
}

func TestLoadCORSAllowedOrigins(t *testing.T) {
	t.Parallel()

	origins := loadCORSAllowedOrigins(EnvProduction)
	if len(origins) != 0 {
		t.Fatalf("production without env should have no static origins, got %#v", origins)
	}

	devOrigins := loadCORSAllowedOrigins(EnvDevelopment)
	if len(devOrigins) != 0 {
		t.Fatalf("development should not auto-configure origins, got %#v", devOrigins)
	}
}

func TestEnableCORSAlwaysOn(t *testing.T) {
	t.Parallel()

	prod := Config{Env: EnvProduction}
	if !prod.EnableCORS() {
		t.Fatal("CORS middleware should always be enabled for DB-backed origins")
	}
}