package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                string
	PgConnectionString string
	Port               string
}

func Load() Config {
	if os.Getenv("ENV") == "" || os.Getenv("ENV") == "local" {
		err := godotenv.Load(".env")
		if err != nil {
			panic("Error loading .env file")
		}
	}

	return Config{
		Env:                get("ENV", "local"),
		PgConnectionString: get("PG_CONNECTION_STRING", "postgres://postgres:password@localhost:5432/appt_scheduler?sslmode=disable"),
		Port:               get("PORT", "8080"),
	}
}

// get ensures that if an environment variable is not set, a default value is returned instead.
// This is useful for local development and testing, where you may not want to set all environment variables.
func get(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
