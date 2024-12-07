package lib

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	DefaultTLS     = false
	DefaultCert    = "cert.pem"
	DefaultKey     = "key.pem"
	DefaultAddress = "0.0.0.0"
	DefaultPort    = 6065
	DefaultPrefix  = "/"
)

type Config struct {
	UserPermissions `mapstructure:",squash"`
	Debug           bool
	Address         string
	Port            int
	TLS             bool
	Cert            string
	Key             string
	Prefix          string
	NoSniff         bool
	NoPassword      bool
	BehindProxy     bool
	Log             Log
	CORS            CORS
	Users           []User
}

func ParseConfig(filename string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	// Configure flags bindings
	if flags != nil {
		err := v.BindPFlags(flags)
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
	// TODO: use new env struct bind feature when it's released in viper.
	// This should make it redundant to set defaults for things that are
	// empty or false.

	// Defaults shared with flags
	v.SetDefault("TLS", DefaultTLS)
	v.SetDefault("Cert", DefaultCert)
	v.SetDefault("Key", DefaultKey)
	v.SetDefault("Address", DefaultAddress)
	v.SetDefault("Port", DefaultPort)
	v.SetDefault("Prefix", DefaultPrefix)

	// Other defaults
	v.SetDefault("RulesBehavior", RulesOverwrite)
	v.SetDefault("Directory", ".")
	v.SetDefault("Permissions", "R")
	v.SetDefault("Debug", false)
	v.SetDefault("NoSniff", false)
	v.SetDefault("NoPassword", false)
	v.SetDefault("Log.Format", "console")
	v.SetDefault("Log.Outputs", []string{"stderr"})
	v.SetDefault("Log.Colors", true)
	v.SetDefault("CORS.Allowed_Headers", []string{"*"})
	v.SetDefault("CORS.Allowed_Hosts", []string{"*"})
	v.SetDefault("CORS.Allowed_Methods", []string{"*"})

	// Read and unmarshal configuration
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	cfg := &Config{}
	err = v.Unmarshal(cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.TextUnmarshallerHookFunc(),
	)))
	if err != nil {
		return nil, err
	}

	// Cascade user settings
	for i := range cfg.Users {
		if !v.IsSet(fmt.Sprintf("Users.%d.Directory", i)) {
			cfg.Users[i].Directory = cfg.Directory
		}

		if !v.IsSet(fmt.Sprintf("Users.%d.Permissions", i)) {
			cfg.Users[i].Permissions = cfg.Permissions
		}

		if !v.IsSet(fmt.Sprintf("Users.%d.RulesBehavior", i)) {
			cfg.Users[i].RulesBehavior = cfg.RulesBehavior
		}

		if v.IsSet(fmt.Sprintf("Users.%d.Rules", i)) {
			switch cfg.Users[i].RulesBehavior {
			case RulesOverwrite:
				// Do nothing
			case RulesAppend:
				rules := append([]*Rule{}, cfg.Rules...)
				rules = append(rules, cfg.Users[i].Rules...)

				cfg.Users[i].Rules = rules
			}
		} else {
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

	c.Directory, err = filepath.Abs(c.Directory)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
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

	err = c.UserPermissions.Validate()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	for i := range c.Users {
		err := c.Users[i].Validate(c.NoPassword)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
	}

	return nil
}

func (cfg *Config) GetLogger() (*zap.Logger, error) {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.DisableCaller = true
	if cfg.Debug {
		loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	if cfg.Log.Colors && cfg.Log.Format != "json" {
		loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	loggerConfig.Encoding = cfg.Log.Format
	loggerConfig.OutputPaths = cfg.Log.Outputs
	return loggerConfig.Build()
}

type Log struct {
	Format  string
	Colors  bool
	Outputs []string
}

type CORS struct {
	Enabled        bool
	Credentials    bool
	AllowedHeaders []string `mapstructure:"allowed_headers"`
	AllowedHosts   []string `mapstructure:"allowed_hosts"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	ExposedHeaders []string `mapstructure:"exposed_headers"`
}
