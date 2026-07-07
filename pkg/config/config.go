package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// LoadConfig loads configuration from a specified file path and unmarshals it into the out struct.
// It also reads system environment variables automatically.
func LoadConfig(path string, name string, out interface{}) error {
	if path != "" && name != "" {
		viper.AddConfigPath(path)
		viper.SetConfigName(name)
		viper.SetConfigType("env")

		if err := viper.ReadInConfig(); err != nil {
			log.Printf("Warning: Could not read config file %s/%s: %v. Falling back to environment variables.", path, name, err)
		}
	} else {
		log.Println("No config file specified. Loading configurations solely from environment variables.")
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.Unmarshal(out); err != nil {
		return err
	}

	return nil
}
