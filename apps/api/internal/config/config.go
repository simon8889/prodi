package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr        string
	DatabaseURL string
	CORSOrigins []string
	WSPath      string
	DeviceToken string
}

func Load() Config {
	return Config{
		Addr:        env("PRODI_ADDR", ":8080"),
		DatabaseURL: env("PRODI_DB", defaultDatabasePath()),
		CORSOrigins: split(env("PRODI_CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173,http://127.0.0.1:4173")),
		WSPath:      env("PRODI_WS_PATH", "/ws"),
		DeviceToken: env("PRODI_DEVICE_TOKEN", ""),
	}
}

func (c Config) ResolveDatabasePath() string {
	if c.DatabaseURL == ":memory:" {
		return c.DatabaseURL
	}
	cleaned := filepath.Clean(c.DatabaseURL)
	if dir := filepath.Dir(cleaned); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	return cleaned
}

func defaultDatabasePath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join("data", "prodi.db")
	}
	if root := findModuleRoot(filepath.Dir(exe)); root != "" {
		return filepath.Join(root, "data", "prodi.db")
	}
	return filepath.Join("data", "prodi.db")
}

func findModuleRoot(start string) string {
	dir := start
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func split(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
