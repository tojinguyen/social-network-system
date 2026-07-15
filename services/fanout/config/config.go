package config

import (
	"social-network-system/pkg/config"
)

// Config holds all configurations for the Fan-out Worker.
type Config struct {
	MongoURI           string `mapstructure:"MONGO_URI"`
	MongoDBName        string `mapstructure:"MONGO_DB_NAME"`
	RedisURI           string `mapstructure:"REDIS_URI"`
	RedisPassword      string `mapstructure:"REDIS_PASSWORD"`
	KafkaBrokers       string `mapstructure:"KAFKA_BROKERS"`
	PostCreatedTopic   string `mapstructure:"KAFKA_TOPIC_POST_CREATED"`
	CelebrityThreshold int    `mapstructure:"CELEBRITY_THRESHOLD"`
	WorkerPoolSize     int    `mapstructure:"WORKER_POOL_SIZE"`
}

// Load loads the configurations using Viper.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := config.LoadConfig(path, ".env", &cfg); err != nil {
		return nil, err
	}
	if cfg.CelebrityThreshold == 0 {
		cfg.CelebrityThreshold = 10000
	}
	if cfg.WorkerPoolSize == 0 {
		cfg.WorkerPoolSize = 10
	}
	return &cfg, nil
}
