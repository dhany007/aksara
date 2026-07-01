package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DeepSeekAPIKey         string
	DeepSeekModel          string
	BooksDir               string
	ResultsDir             string
	Overwrite              bool
	TranslationConcurrency int
	TranslationRetries     int
	TranslationTimeout     time.Duration
	PythonBin              string
	ParserScript           string
	MaxChunkChars          int
	MaxPages               int
}

func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}
	return FromEnv(os.Getenv)
}

func FromEnv(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		DeepSeekAPIKey:         getenv("DEEPSEEK_API_KEY"),
		DeepSeekModel:          envOr(getenv, "DEEPSEEK_MODEL", "deepseek-v4-pro"),
		BooksDir:               envOr(getenv, "BOOKS_DIR", "./books"),
		ResultsDir:             envOr(getenv, "RESULTS_DIR", "./results"),
		PythonBin:              envOr(getenv, "PYTHON_BIN", "python3"),
		ParserScript:           envOr(getenv, "PARSER_SCRIPT", "parser/extract.py"),
		TranslationConcurrency: 1,
		TranslationRetries:     2,
		TranslationTimeout:     120 * time.Second,
		MaxChunkChars:          7000,
	}

	var err error
	if cfg.Overwrite, err = parseBool(envOr(getenv, "OVERWRITE", "false")); err != nil {
		return nil, fmt.Errorf("OVERWRITE: %w", err)
	}
	if cfg.TranslationConcurrency, err = parsePositiveInt(envOr(getenv, "TRANSLATION_CONCURRENCY", "1")); err != nil {
		return nil, fmt.Errorf("TRANSLATION_CONCURRENCY: %w", err)
	}
	if cfg.TranslationRetries, err = parseNonNegativeInt(envOr(getenv, "TRANSLATION_RETRIES", "2")); err != nil {
		return nil, fmt.Errorf("TRANSLATION_RETRIES: %w", err)
	}
	if cfg.TranslationTimeout, err = time.ParseDuration(envOr(getenv, "TRANSLATION_TIMEOUT", "120s")); err != nil {
		return nil, fmt.Errorf("TRANSLATION_TIMEOUT: %w", err)
	}
	if cfg.MaxChunkChars, err = parsePositiveInt(envOr(getenv, "MAX_CHUNK_CHARS", "7000")); err != nil {
		return nil, fmt.Errorf("MAX_CHUNK_CHARS: %w", err)
	}
	if cfg.MaxPages, err = parseNonNegativeInt(envOr(getenv, "MAX_PAGES", "0")); err != nil {
		return nil, fmt.Errorf("MAX_PAGES: %w", err)
	}

	if cfg.DeepSeekAPIKey == "" {
		return nil, fmt.Errorf("missing required env var: DEEPSEEK_API_KEY")
	}
	if cfg.TranslationTimeout <= 0 {
		return nil, fmt.Errorf("TRANSLATION_TIMEOUT must be positive")
	}
	if !filepath.IsAbs(cfg.ParserScript) {
		if abs, err := filepath.Abs(cfg.ParserScript); err == nil {
			cfg.ParserScript = abs
		}
	}

	return cfg, nil
}

func EnsureDirs(cfg *Config) error {
	if err := os.MkdirAll(cfg.BooksDir, 0755); err != nil {
		return fmt.Errorf("create books dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ResultsDir, 0755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}
	return nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open .env: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set env %s: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .env: %w", err)
	}
	return nil
}

func envOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", value)
	}
}

func parsePositiveInt(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return n, nil
}

func parseNonNegativeInt(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	return n, nil
}
