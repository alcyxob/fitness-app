package config

import (
	"strings"
	"time" // Ensure time package is imported

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
// The values are read by Viper from a config file or environment variables.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	S3       S3Config       `mapstructure:"s3"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type ServerConfig struct {
	Address string `mapstructure:"address"`
}

type DatabaseConfig struct {
	URI  string `mapstructure:"uri"`
	Name string `mapstructure:"name"`
}

type S3Config struct {
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	BucketName      string `mapstructure:"bucket_name"`
	UseSSL          bool   `mapstructure:"use_ssl"`
}

// JWTConfig defines JWT specific configuration
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
	// --- THIS IS THE KEY CHANGE ---
	// Field MUST be of type time.Duration
	// Mapstructure tag MUST match the key in config.yaml ("expiration")
	Expiration time.Duration `mapstructure:"expiration"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	// Set the path to look for the config file in
	viper.AddConfigPath(path)
	// Set the name of the config file (without extension)
	viper.SetConfigName("config")
	// Set the type of the config file
	viper.SetConfigType("yaml") // or json, toml, etc.

	// --- Environment Variable Handling ---
	viper.AutomaticEnv()
	// Use replacer for nested keys e.g., server.address -> SERVER_ADDRESS
	// jwt.expiration -> JWT_EXPIRATION
	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`))

	// --- Set default values (optional but recommended) ---
	// Use duration string format for the default value as well
	viper.SetDefault("server.address", ":8080")
	viper.SetDefault("database.uri", "mongodb://localhost:27017")
	viper.SetDefault("database.name", "fitness_app_default")
	viper.SetDefault("s3.use_ssl", true)     // Default to true for cloud providers
	viper.SetDefault("jwt.expiration", "1h") // Default JWT expiry to 1 hour

	// --- Read Config File ---
	err = viper.ReadInConfig()
	// If config file not found, continue (might rely solely on env vars),
	// but log it or handle differently if the file is mandatory.
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		// Config file not found; ignore error if desired
		// log.Println("Config file not found, using defaults/env vars.")
		err = nil // Reset err to nil if we want to proceed without a file
	} else if err != nil {
		// Some other error occurred reading the config file
		return // Return the error
	}

	// --- Unmarshal Config ---
	// Viper will now attempt to parse the duration string ("60m", "1h", etc.)
	// directly into the time.Duration field (config.JWT.Expiration).
	err = viper.Unmarshal(&config)
	if err != nil {
		// Return the unmarshalling error (this is where the "missing unit" error originates)
		return
	}

	// --- REMOVED ---
	// NO manual conversion of duration is needed here anymore.
	// The viper.Unmarshal step handles it directly because:
	// 1. The YAML value is a duration string (e.g., "60m").
	// 2. The Go struct field is time.Duration.

	return config, nil // Return the populated config struct and nil error if successful
}
