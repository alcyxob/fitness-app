package config

import (
	"strings"
	"time"

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

type JWTConfig struct {
	Secret            string        `mapstructure:"secret"`
	ExpirationMinutes time.Duration `mapstructure:"expiration_minutes"`
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
	// Automatically override values from config file with values from env vars
	viper.AutomaticEnv()
	// Useful for nested structs like server.address -> SERVER_ADDRESS
	// You might need custom replacer for keys like s3.access_key_id -> S3_ACCESS_KEY_ID
	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`)) // Replace dots with underscores

	// Set default values (optional but recommended)
	viper.SetDefault("server.address", ":8080")
	viper.SetDefault("database.uri", "mongodb://localhost:27017")
	viper.SetDefault("database.name", "fitness_app_default")
	viper.SetDefault("s3.use_ssl", true) // Default to true for cloud providers
	viper.SetDefault("jwt.expiration_minutes", 60)

	// Attempt to read the config file
	err = viper.ReadInConfig()
	// If config file not found, continue (might rely solely on env vars),
	// but log it or handle differently if the file is mandatory.
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		// Config file not found; ignore error if desired
		// log.Println("Config file not found, using defaults/env vars.")
		// Reset err to nil if we want to proceed without a file
		err = nil
	} else if err != nil {
		// Some other error occurred reading the config file
		return
	}

	// Unmarshal the config into the Config struct
	err = viper.Unmarshal(&config)

	// Convert minutes to time.Duration for JWT Expiration
	// Viper doesn't directly unmarshal into time.Duration from integer minutes easily
	// So we read as int and convert. Or use string like "60m" in yaml/env.
	// Let's adjust to expect a duration string like "60m" or "1h"
	// Update JWTConfig and config.yaml if using duration strings.
	// For now, let's do the conversion manually after unmarshal:
	rawMinutes := viper.GetInt64("jwt.expiration_minutes") // Read as Int64
	config.JWT.ExpirationMinutes = time.Duration(rawMinutes) * time.Minute

	return
}