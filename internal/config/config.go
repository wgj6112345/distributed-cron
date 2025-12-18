// internal/config/config.go
package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for our application.
// The mapstructure tags are used by Viper to unmarshal the data.
type Config struct {
	EtcdEndpoints      []string      `mapstructure:"etcd_endpoints"`
	EtcdTimeout        time.Duration `mapstructure:"etcd_timeout"`
	HttpListenAddr     string        `mapstructure:"http_listen_addr"`
	LeaderElectionTTL  time.Duration `mapstructure:"leader_election_ttl"`
}

// Load loads configuration from file and environment variables.
func Load() (*Config, error) {
	// Set default values
	viper.SetDefault("etcd_timeout", "5s")
	viper.SetDefault("http_listen_addr", ":8080")
	viper.SetDefault("leader_election_ttl", "10s")

	// Set config file details
	viper.SetConfigName("config")    // name of config file (without extension)
	viper.SetConfigType("yaml")      // or "json", "toml"
	viper.AddConfigPath("./configs") // path to look for the config file in
	viper.AddConfigPath(".")         // optionally look for config in the working directory

	// Read environment variables
	viper.AutomaticEnv()

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			// We can rely on defaults and env vars
		} else {
			// Config file was found but another error was produced
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
