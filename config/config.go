package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application settings loaded from environment variables.
// Access it anywhere via config.C after calling config.Load().
type Config struct {
	AppName          string
	Port             int
	Debug            bool
	Rate             float64
	LogLevel         string
	LogFormat        string
	LogFile          string
	QdrantURL        string
	QdrantAPIKey     string
	OpenRouterAPIKey string
	OllamaURL        string
	AllowedOrigin    string
	RedisURL         string
	RateLimitMax     int
	RateLimitWindow  int
}

// C is the global configuration instance.
var C Config

// Load reads .env (if present), validates required variables, and populates C.
// The application must call this at startup; it returns an error when a required
// variable is missing or has an invalid type.
func Load() error {
	if err := godotenv.Load(); err != nil {
		// .env is optional when variables are provided by the host environment.
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("load .env file: %w", err)
		}
	}

	appName, err := getString("APP_NAME")
	if err != nil {
		return err
	}

	port, err := getInt("PORT")
	if err != nil {
		return err
	}

	debug, err := getBool("DEBUG")
	if err != nil {
		return err
	}

	rate, err := getFloat("RATE")
	if err != nil {
		return err
	}

	logLevel := "info"
	if value, ok := getOptionalString("LOG_LEVEL"); ok {
		logLevel = value
	} else if debug {
		logLevel = "debug"
	}

	logFormat := "text"
	if value, ok := getOptionalString("LOG_FORMAT"); ok {
		logFormat = value
	}

	logFile := ""
	if value, ok := getOptionalString("LOG_FILE"); ok {
		logFile = value
	}

	qdrantURL, err := getString("QDRANT_URL")
	if err != nil {
		return err
	}

	qdrantAPIKey := ""
	if value, ok := getOptionalString("QDRANT_API_KEY"); ok {
		qdrantAPIKey = value
	}

	openRouterAPIKey, err := getString("OPENROUTER_API_KEY")
	if err != nil {
		return err
	}

	ollamaURL, err := getString("OLLAMA_URL")
	if err != nil {
		return err
	}

	allowedOrigin, err := getString("ALLOWED_ORIGIN")
	if err != nil {
		return err
	}

	redisURL, err := getString("REDIS_URL")
	if err != nil {
		return err
	}

	rateLimitMax := 60
	if value, ok := getOptionalString("RATE_LIMIT_MAX"); ok {
		rateLimitMax, err = strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("environment variable %q must be an integer, got %q", "RATE_LIMIT_MAX", value)
		}
		if rateLimitMax < 1 {
			return fmt.Errorf("environment variable %q must be at least 1, got %d", "RATE_LIMIT_MAX", rateLimitMax)
		}
	}

	rateLimitWindow := 60
	if value, ok := getOptionalString("RATE_LIMIT_WINDOW"); ok {
		rateLimitWindow, err = strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("environment variable %q must be an integer, got %q", "RATE_LIMIT_WINDOW", value)
		}
		if rateLimitWindow < 1 {
			return fmt.Errorf("environment variable %q must be at least 1, got %d", "RATE_LIMIT_WINDOW", rateLimitWindow)
		}
	}

	C = Config{
		AppName:          appName,
		Port:             port,
		Debug:            debug,
		Rate:             rate,
		LogLevel:         logLevel,
		LogFormat:        logFormat,
		LogFile:          logFile,
		QdrantURL:        qdrantURL,
		QdrantAPIKey:     qdrantAPIKey,
		OpenRouterAPIKey: openRouterAPIKey,
		OllamaURL:        ollamaURL,
		AllowedOrigin:    allowedOrigin,
		RedisURL:         redisURL,
		RateLimitMax:     rateLimitMax,
		RateLimitWindow:  rateLimitWindow,
	}

	return nil
}
