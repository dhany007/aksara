package config

import (
	"strings"
	"testing"
	"time"
)

func TestFromEnvLoadsTranslatorDefaults(t *testing.T) {
	cfg, err := FromEnv(mapEnv(map[string]string{
		"DEEPSEEK_API_KEY": "sk-test",
	}))
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}

	if cfg.DeepSeekAPIKey != "sk-test" {
		t.Fatalf("DeepSeekAPIKey = %q", cfg.DeepSeekAPIKey)
	}
	if cfg.DeepSeekModel != "deepseek-v4-pro" {
		t.Fatalf("DeepSeekModel = %q", cfg.DeepSeekModel)
	}
	if cfg.BooksDir != "./books" {
		t.Fatalf("BooksDir = %q", cfg.BooksDir)
	}
	if cfg.ResultsDir != "./results" {
		t.Fatalf("ResultsDir = %q", cfg.ResultsDir)
	}
	if cfg.TranslationConcurrency != 1 {
		t.Fatalf("TranslationConcurrency = %d", cfg.TranslationConcurrency)
	}
	if cfg.TranslationRetries != 2 {
		t.Fatalf("TranslationRetries = %d", cfg.TranslationRetries)
	}
	if cfg.TranslationTimeout != 120*time.Second {
		t.Fatalf("TranslationTimeout = %s", cfg.TranslationTimeout)
	}
	if cfg.Overwrite {
		t.Fatal("Overwrite should default false")
	}
}

func TestFromEnvValidatesMissingAPIKey(t *testing.T) {
	_, err := FromEnv(mapEnv(nil))
	if err == nil {
		t.Fatal("expected missing API key error")
	}
	if !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Fatalf("error = %v", err)
	}
}

func TestFromEnvParsesRuntimeOptions(t *testing.T) {
	cfg, err := FromEnv(mapEnv(map[string]string{
		"DEEPSEEK_API_KEY":        "sk-test",
		"DEEPSEEK_MODEL":          "deepseek-v4-flash",
		"BOOKS_DIR":               "/books",
		"RESULTS_DIR":             "/results",
		"TRANSLATION_CONCURRENCY": "3",
		"TRANSLATION_RETRIES":     "4",
		"TRANSLATION_TIMEOUT":     "45s",
		"MAX_PAGES":               "3",
		"OVERWRITE":               "true",
		"PYTHON_BIN":              "python",
		"PARSER_SCRIPT":           "/app/parser/extract.py",
	}))
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}

	if cfg.DeepSeekModel != "deepseek-v4-flash" ||
		cfg.BooksDir != "/books" ||
		cfg.ResultsDir != "/results" ||
		cfg.TranslationConcurrency != 3 ||
		cfg.TranslationRetries != 4 ||
		cfg.TranslationTimeout != 45*time.Second ||
		cfg.MaxPages != 3 ||
		!cfg.Overwrite ||
		cfg.PythonBin != "python" ||
		cfg.ParserScript != "/app/parser/extract.py" {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
}

func TestFromEnvDefaultsToNoPageLimit(t *testing.T) {
	cfg, err := FromEnv(mapEnv(map[string]string{
		"DEEPSEEK_API_KEY": "sk-test",
	}))
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}
	if cfg.MaxPages != 0 {
		t.Fatalf("MaxPages = %d", cfg.MaxPages)
	}
}

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
