package bdd

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Eventually   EventuallyConfig
	Consistently ConsistentlyConfig
}

type EventuallyConfig struct {
	Timeout  time.Duration
	Interval time.Duration
}

type ConsistentlyConfig struct {
	Timeout  time.Duration
	Interval time.Duration
}

// LoadBDDConfig creates a new viper configuration instance with default values
// for BDD testing framework timeouts and intervals.
func LoadBDDConfig() (Config, error) {
	v := viper.New()

	v.SetDefault("eventually.timeout", 30*time.Second)
	v.SetDefault("eventually.interval", 1*time.Second)

	v.SetDefault("consistently.timeout", 10*time.Second)
	v.SetDefault("consistently.interval", 500*time.Millisecond)

	v.SetEnvPrefix("BDD")
	v.AutomaticEnv()

	cfg := Config{}

	err := v.Unmarshal(&cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}
