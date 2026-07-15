package config

import (
	"social-network-system/pkg/config"
)

// Config holds all configurations for the Post Service.
type Config struct {
	Port          string `mapstructure:"PORT"`
	MongoURI      string `mapstructure:"MONGO_URI"`
	MongoDBName   string `mapstructure:"MONGO_DB_NAME"`
	RedisURI      string `mapstructure:"REDIS_URI"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`
	JWTSecret     string `mapstructure:"JWT_SECRET"`
	KafkaBrokers  string `mapstructure:"KAFKA_BROKERS"`
	PostCreatedTopic string `mapstructure:"KAFKA_TOPIC_POST_CREATED"`
}

// Load loads the configurations using Viper.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := config.LoadConfig(path, ".env", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
