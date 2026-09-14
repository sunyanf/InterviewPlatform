package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// 清除可能影响测试的环境变量
	os.Unsetenv("APP_ENV")
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("DB_HOST")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Server.Port)
	}
	if cfg.App.Env != "dev" {
		t.Errorf("expected default env dev, got %s", cfg.App.Env)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("expected default db host localhost, got %s", cfg.Database.Host)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DB_NAME", "test_db")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DB_NAME")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Server.Port)
	}
	if cfg.Database.DBName != "test_db" {
		t.Errorf("expected db name test_db, got %s", cfg.Database.DBName)
	}
}

func TestLoad_ProductionRequiresJWTSecret(t *testing.T) {
	os.Setenv("APP_ENV", "prod")
	os.Setenv("JWT_SECRET", "change-me-in-production")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("JWT_SECRET")
	}()

	_, err := Load()
	if err == nil {
		t.Error("expected error when JWT_SECRET is default in production")
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "secret",
		DBName:   "ai_interview",
		SSLMode:  "disable",
	}

	dsn := cfg.DSN()
	expected := "postgres://postgres:secret@localhost:5432/ai_interview?sslmode=disable"
	if dsn != expected {
		t.Errorf("expected DSN %s, got %s", expected, dsn)
	}
}
