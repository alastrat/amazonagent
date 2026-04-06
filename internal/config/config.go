package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	Env         string
	DatabaseURL string

	SupabaseURL            string
	SupabaseAnonKey        string
	SupabaseServiceRoleKey string
	SupabaseJWTSecret      string

	InngestEventKey   string
	InngestSigningKey string
	InngestDev        bool

	OpenFangAPIURL string
	OpenFangAPIKey string

	PostHogAPIKey string
	PostHogHost   string

	SPAPIClientID     string
	SPAPIClientSecret string
	SPAPIRefreshToken string
	SPAPIMarketplace  string
	SPAPISellerID     string

	ExaAPIKey       string
	FirecrawlAPIKey string
	OpenAIAPIKey    string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	inngestDev, _ := strconv.ParseBool(getEnv("INNGEST_DEV", "true"))

	cfg := &Config{
		Port:        port,
		Env:         getEnv("ENV", "development"),
		DatabaseURL: mustEnv("DATABASE_URL"),

		SupabaseURL:            getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:        getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceRoleKey: getEnv("SUPABASE_SERVICE_ROLE_KEY", ""),
		SupabaseJWTSecret:      getEnv("SUPABASE_JWT_SECRET", ""),

		InngestEventKey:   getEnv("INNGEST_EVENT_KEY", "test"),
		InngestSigningKey: getEnv("INNGEST_SIGNING_KEY", "test"),
		InngestDev:        inngestDev,

		OpenFangAPIURL: getEnv("OPENFANG_API_URL", ""),
		OpenFangAPIKey: getEnv("OPENFANG_API_KEY", ""),

		PostHogAPIKey: getEnv("POSTHOG_API_KEY", ""),
		PostHogHost:   getEnv("POSTHOG_HOST", "https://app.posthog.com"),

		SPAPIClientID:     getEnvAny("SP_API_CLIENT_ID", "SP_API_LWA_APP_ID"),
		SPAPIClientSecret: getEnvAny("SP_API_CLIENT_SECRET", "SP_API_LWA_CLIENT_SECRET"),
		SPAPIRefreshToken: getEnvAny("SP_API_REFRESH_TOKEN"),
		SPAPIMarketplace:  getEnv("SP_API_MARKETPLACE_ID", "ATVPDKIKX0DER"),
		SPAPISellerID:     getEnvAny("AMAZON_MERCHANT_TOKEN", "AMAZON_SELLER_ID", "SP_API_SELLER_ID"),

		ExaAPIKey:       getEnv("EXA_API_KEY", ""),
		FirecrawlAPIKey: getEnv("FIRECRAWL_API_KEY", ""),
		OpenAIAPIKey:    getEnv("OPENAI_API_KEY", ""),
	}

	return cfg, nil
}

func (c *Config) IsDev() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvAny returns the first non-empty value from multiple env var names.
func getEnvAny(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %s is not set", key))
	}
	return v
}
