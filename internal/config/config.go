package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config holds the Jira CLI configuration.
type Config struct {
	URL     string `mapstructure:"url"`
	Email   string `mapstructure:"email"`
	Token   string `mapstructure:"token"`
	Project string `mapstructure:"project"`
}

// Load reads configuration from file, environment variables, and defaults.
func Load(configFile string) (*Config, error) {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName(".jira")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME")
	}

	viper.SetDefault("url", "https://jira.amplia.es/jira")
	viper.SetDefault("project", "ESA")

	_ = viper.BindEnv("url", "JIRA_URL")
	_ = viper.BindEnv("email", "JIRA_EMAIL")
	_ = viper.BindEnv("token", "JIRA_TOKEN")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only fail on parse errors, not missing file
			if configFile != "" {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required: set JIRA_TOKEN, add 'token' to ~/.jira.yaml, or use --token flag")
	}

	return &cfg, nil
}
