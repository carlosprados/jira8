package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Config holds the Jira CLI configuration.
type Config struct {
	URL      string `mapstructure:"url"`
	Token    string `mapstructure:"token"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Project  string `mapstructure:"project"`
}

// Load reads the configuration (see Read) and checks it is usable.
func Load(configFile string) (*Config, error) {
	cfg, err := Read(configFile)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Read reads configuration from flags, environment variables (and .env in the
// current directory) and the config file, without checking it is complete.
func Read(configFile string) (*Config, error) {
	_ = gotenv.Load()

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName(".jira")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME")
	}

	_ = viper.BindEnv("url", "JIRA_URL")
	_ = viper.BindEnv("token", "JIRA_TOKEN")
	_ = viper.BindEnv("user", "JIRA_USER")
	_ = viper.BindEnv("password", "JIRA_PASSWORD")
	_ = viper.BindEnv("project", "JIRA_PROJECT")

	// A missing ~/.jira.yaml is fine (env vars may be enough); a broken one,
	// or a missing --config file, is not.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// File is the config file Read loaded, or "" when there was none.
func File() string { return viper.ConfigFileUsed() }

// Source tells where a key's value came from: "flag", "env", "file" or ""
// (unset). changedFlag reports whether the user passed the matching flag.
func Source(key string, changedFlag bool) string {
	switch {
	case changedFlag:
		return "flag"
	case os.Getenv("JIRA_"+strings.ToUpper(key)) != "":
		return "env"
	case viper.InConfig(key):
		return "file"
	}
	return ""
}

// Validate checks there is a URL and a complete set of credentials.
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("url required: set 'url' in ~/.jira.yaml, JIRA_URL or --url")
	}
	if c.Auth() == "" {
		return fmt.Errorf("auth required: set 'token' (Bearer) or 'user'+'password' (Basic) in ~/.jira.yaml, env vars, or flags")
	}
	return nil
}

// Auth is the authentication method the client will use: "token" (Bearer),
// "basic" (user + password) or "" when the credentials are incomplete.
func (c *Config) Auth() string {
	switch {
	case c.Token != "":
		return "token"
	case c.User != "" && c.Password != "":
		return "basic"
	}
	return ""
}
