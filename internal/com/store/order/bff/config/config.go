// File: config/config.go

package config

import (
	"os"
)

type Config struct {
	Backend struct {
		Host string
		Port string
	}
	BFFServer struct {
		Port string
		Host string
	}
	KafkaService struct {
		Port    string
		Host    string
		Topic   string
		ApiPort string
	}
}

func LoadConfig() (*Config, error) {
	config := &Config{
		Backend: struct {
			Host string
			Port string
		}{
			Port: getEnvOrDefault("DOMAIN_SERVER_PORT", "9000"),
			Host: getEnvOrDefault("DOMAIN_SERVER_HOST", "order-api-mock"),
		},
		BFFServer: struct {
			Port string
			Host string
		}{
			Port: getEnvOrDefault("SERVER_PORT", "8080"),
			Host: getEnvOrDefault("SERVER_HOST", "bff-service"),
		},
		KafkaService: struct {
			Port    string
			Host    string
			Topic   string
			ApiPort string
		}{
			Port:    getEnvOrDefault("KAFKA_PORT", "9093"),
			Host:    getEnvOrDefault("KAFKA_HOST", "specmatic-kafka"),
			Topic:   getEnvOrDefault("KAFKA_TOPIC", "product-queries"),
			ApiPort: getEnvOrDefault("KAFKA_API_PORT", "9094"),
		},
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
