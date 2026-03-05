package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// loadDotEnv reads .env from the current directory and sets any unset env vars.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return // no .env file, that's fine
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

type Config struct {
	Port          string
	PostsDir      string
	StaticDir     string
	TemplatesDir  string
	DBPath        string
	AdminUser     string
	AdminHash     string
	SessionSecret string

	IntervalsKey  string
	IntervalsID   string
	IntervalsURL  string
	CacheHours    int

	Dev bool
}

func Load() *Config {
	loadDotEnv()
	return &Config{
		Port:          env("PORT", "5002"),
		PostsDir:      env("POSTS_DIR", "content/posts"),
		StaticDir:     env("STATIC_DIR", "static"),
		TemplatesDir:  env("TEMPLATES_DIR", "templates"),
		DBPath:        env("DB_PATH", "traffic.db"),
		AdminUser:     env("ADMIN_USER", "admin"),
		AdminHash:     env("ADMIN_HASH", "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"),
		SessionSecret: env("SESSION_SECRET", "change-me-in-production"),
		IntervalsKey:  env("INTERVALS_API_KEY", ""),
		IntervalsID:   env("INTERVALS_ATHLETE_ID", ""),
		IntervalsURL:  env("INTERVALS_BASE_URL", "https://intervals.icu/api/v1/"),
		CacheHours:    envInt("CACHE_HOURS", 12),
		Dev:           env("ENV", "production") == "development",
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
