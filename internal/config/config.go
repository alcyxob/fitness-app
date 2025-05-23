package config

import (
	"log"
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
	PublicEndpoint  string `mapstructure:"public_endpoint"`
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
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path) // Path for config file (e.g., ".")

	// --- Explicit Environment Variable Binding ---
	// This tells Viper to specifically look for these ENV VARS
	// and map them to these config keys. This often has higher precedence
	// or works more reliably with Unmarshal when a config file is also present.
	viper.BindEnv("jwt.secret", "JWT_SECRET")
	viper.BindEnv("jwt.expiration", "JWT_EXPIRATION") // Assuming this key from earlier fix
	viper.BindEnv("database.uri", "DATABASE_URI")
	viper.BindEnv("database.name", "DATABASE_NAME")
	viper.BindEnv("server.address", "SERVER_ADDRESS") // Or PORT if App Runner sets that
	viper.BindEnv("s3.endpoint", "S3_ENDPOINT")
	viper.BindEnv("s3.public_endpoint", "S3_PUBLIC_ENDPOINT")
	viper.BindEnv("s3.region", "S3_REGION")
	viper.BindEnv("s3.access_key_id", "S3_ACCESS_KEY_ID") // If using access keys for S3 directly from app
	viper.BindEnv("s3.secret_access_key", "S3_SECRET_ACCESS_KEY") // If using access keys for S3
	viper.BindEnv("s3.bucket_name", "S3_BUCKET_NAME")
	viper.BindEnv("s3.use_ssl", "S3_USE_SSL")
	// Add any other critical env vars here

	// AutomaticEnv can still be used for other variables or as a fallback
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`)) // Still useful with AutomaticEnv

	log.Println("Viper: BindEnv, AutomaticEnv and KeyReplacer set.")
	// ... (your Viper debug logs for expected ENV VARS - these might be redundant if using BindEnv)

	// Set defaults (these are lower precedence than ENV and config file)
	viper.SetDefault("server.address", ":8080")
	viper.SetDefault("jwt.expiration", "1h")
	// ... other defaults ...

	// Attempt to read the config file
	// If found, its values are merged. ENV vars (especially with BindEnv) should override.
	if errRead := viper.ReadInConfig(); errRead != nil {
			if _, ok := errRead.(viper.ConfigFileNotFoundError); ok {
					log.Println("Viper: Config file not found. Relying on ENV vars and defaults.")
					// This is fine if config file is optional
			} else {
					log.Printf("Viper: Error reading config file: %v. Relying on ENV vars and defaults.", errRead)
					// Decide if this is a fatal error or not.
			}
	} else {
			log.Println("Viper: Successfully read config file.")
	}

	// Unmarshal the config
	err = viper.Unmarshal(&config)
	if err != nil {
			log.Fatalf("Viper: Unable to decode into struct, %v", err) // Make this fatal if unmarshal fails
	}

	log.Printf("Loaded config: JWT Secret length: %d", len(config.JWT.Secret)) // Check length instead of value for sensitive
	log.Printf("Loaded config: S3 Bucket is '%s'", config.S3.BucketName)
	log.Printf("Loaded config: S3 Endpoint is '%s'", config.S3.Endpoint)

	return
}
