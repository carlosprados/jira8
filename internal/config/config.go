package config

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Config holds the Jira CLI configuration.
type Config struct {
	URL      string `mapstructure:"url"`
	Email    string `mapstructure:"email"`
	Token    string `mapstructure:"token"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Project  string `mapstructure:"project"`
}

// Load reads configuration from file, environment variables, and defaults.
// It also loads .env from the current directory if present.
func Load(configFile string) (*Config, error) {
	// Load .env file (silently ignore if not found)
	_ = gotenv.Load()

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
	_ = viper.BindEnv("user", "JIRA_USER")
	_ = viper.BindEnv("password", "JIRA_PASSWORD")
	_ = viper.BindEnv("project", "JIRA_PROJECT")

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

	hasToken := cfg.Token != ""
	hasBasic := cfg.User != "" && cfg.Password != ""
	if !hasToken && !hasBasic {
		return nil, fmt.Errorf("auth required: set 'token' (Bearer) or 'user'+'password' (Basic) in ~/.jira.yaml, env vars, or flags")
	}

	return &cfg, nil
}
