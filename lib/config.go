package lib

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	Permissions `mapstructure:",squash"`
	Debug       bool
	Address     string
	Port        int
	TLS         bool
	Cert        string
	Key         string
	Prefix      string
	NoSniff     bool
	LogFormat   string
	Auth        bool
	CORS        CORS
	Users       []User
}

func ParseConfig(filename string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	// Configure flags bindings
	if flags != nil {
		err := v.BindPFlags(flags)
		if err != nil {
			return nil, err
		}

		err = v.BindPFlag("LogFormat", flags.Lookup("log_format"))
		if err != nil {
			return nil, err
		}
	}

	// Configuration file settings
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/webdav/")
	v.SetConfigName("config")
	if filename != "" {
		v.SetConfigFile(filename)
	}

	// Environment settings
	v.SetEnvPrefix("wd")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("CORS.AllowedHeaders", []string{"*"})
	v.SetDefault("CORS.AllowedHosts", []string{"*"})
	v.SetDefault("CORS.AllowedMethods", []string{"*"})

	// Read and unmarshal configuration
	err := v.ReadInConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, err
	}

	// Cascade user settings
	for i := range cfg.Users {
		if !v.IsSet(fmt.Sprintf("Users.%d.Scope", i)) {
			cfg.Users[i].Scope = cfg.Scope
		}

		if !v.IsSet(fmt.Sprintf("Users.%d.Modify", i)) {
			cfg.Users[i].Modify = cfg.Modify
		}

		if !v.IsSet(fmt.Sprintf("Users.%d.Rules", i)) {
			cfg.Users[i].Rules = cfg.Rules
		}
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var err error

	if c.Auth && len(c.Users) == 0 {
		return errors.New("invalid config: auth cannot be enabled without users")
	}

	if c.TLS {
		if c.Cert == "" {
			return errors.New("invalid config: Cert must be defined if TLS is activated")
		}

		if c.Key == "" {
			return errors.New("invalid config: Key must be defined if TLS is activated")
		}

		c.Cert, err = filepath.Abs(c.Cert)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}

		c.Key, err = filepath.Abs(c.Key)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
	}

	err = c.Permissions.Validate()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	for _, u := range c.Users {
		err := u.Validate()
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
	}

	return nil
}

type CORS struct {
	Enabled        bool
	Credentials    bool
	AllowedHeaders []string
	AllowedHosts   []string
	AllowedMethods []string
	ExposedHeaders []string
}