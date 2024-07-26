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
)

const (
	DefaultDirectory = "."
	DefaultModify    = false
	DefaultDebug     = false
	DefaultNoSniff   = false
	DefaultTLS       = false
	DefaultCert      = "cert.pem"
	DefaultKey       = "key.pem"
	DefaultAddress   = "0.0.0.0"
	DefaultPort      = 6065
	DefaultPrefix    = "/"
	DefaultLogFormat = "console"
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
	LogFormat   string `mapstructure:"log_format"`
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
	// TODO: use new env struct bind feature when it's released in viper.
	// This should make it redundant to set defaults for things that are
	// empty or false.

	// Defaults shared with flags
	v.SetDefault("Directory", DefaultDirectory)
	v.SetDefault("Modify", DefaultModify)
	v.SetDefault("Debug", DefaultDebug)
	v.SetDefault("NoSniff", DefaultNoSniff)
	v.SetDefault("TLS", DefaultTLS)
	v.SetDefault("Cert", DefaultCert)
	v.SetDefault("Key", DefaultKey)
	v.SetDefault("Address", DefaultAddress)
	v.SetDefault("Port", DefaultPort)
	v.SetDefault("Prefix", DefaultPrefix)
	v.SetDefault("Log_Format", DefaultLogFormat)

	// Other defaults
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

	if len(c.Users) == 0 {
		zap.L().Warn("unprotected config: no users have been set, so no authentication will be used")
	}

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
	AllowedHeaders []string `mapstructure:"allowed_headers"`
	AllowedHosts   []string `mapstructure:"allowed_hosts"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	ExposedHeaders []string `mapstructure:"exposed_headers"`
}
