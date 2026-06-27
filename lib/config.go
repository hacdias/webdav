package lib

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

var errDirectoryConflict = errors.New("directory and directories cannot both be defined")

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
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())

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
	if path, err := os.Executable(); err == nil {
		v.AddConfigPath(filepath.Dir(path))
	}

	v.SetConfigName("config")
	if filename != "" {
		v.SetConfigFile(filename)
	}

	// Environment settings
	v.SetEnvPrefix("wd")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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
	v.SetDefault("CORS.Allowed_Hosts", []string{"*"})
	v.SetDefault("CORS.Allowed_Headers", []string{"Authorization", "Content-Type", "Content-Range", "Depth", "Destination", "If", "Lock-Token", "Overwrite", "X-Update-Range"})
	v.SetDefault("CORS.Allowed_Methods", []string{"COPY", "DELETE", "GET", "HEAD", "LOCK", "MKCOL", "MOVE", "OPTIONS", "PATCH", "POST", "PROPFIND", "PROPPATCH", "PUT", "UNLOCK"})

	// Read and unmarshal configuration
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	cfg := &Config{}
	err = v.Unmarshal(cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		directoryMountsDecodeHook(),
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.TextUnmarshallerHookFunc(),
	)))
	if err != nil {
		return nil, err
	}

	err = applyDirectoryConfig(v, flags, &cfg.UserPermissions, "directory", "directories", nil)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Cascade user settings
	for i := range cfg.Users {
		userDirectoryKey := fmt.Sprintf("Users.%d.Directory", i)
		userDirectoriesKey := fmt.Sprintf("Users.%d.Directories", i)

		if !v.IsSet(userDirectoryKey) {
			cfg.Users[i].Directory = cfg.Directory
		}

		err := applyDirectoryConfig(v, flags, &cfg.Users[i].UserPermissions, userDirectoryKey, userDirectoriesKey, &cfg.UserPermissions)
		if err != nil {
			if errors.Is(err, errDirectoryConflict) {
				return nil, fmt.Errorf("invalid config: user %q cannot define both directory and directories", cfg.Users[i].Username)
			}
			return nil, fmt.Errorf("invalid config: user %q: %w", cfg.Users[i].Username, err)
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

func applyDirectoryConfig(v *viper.Viper, flags *pflag.FlagSet, permissions *UserPermissions, directoryKey, directoriesKey string, inherited *UserPermissions) error {
	permissions.directoryExplicit = isExplicitlySet(v, flags, directoryKey)
	permissions.directoriesExplicit = isExplicitlySet(v, flags, directoriesKey)
	if permissions.directoryExplicit && permissions.directoriesExplicit {
		return errDirectoryConflict
	}

	switch {
	case permissions.directoryExplicit:
		permissions.Directory = v.GetString(directoryKey)
		permissions.useDirectories = false
	case permissions.directoriesExplicit:
		directories, err := getDirectoryMounts(v, directoriesKey, permissions.Directories)
		if err != nil {
			return err
		}
		permissions.Directories = directories
		permissions.useDirectories = true
	case inherited != nil:
		permissions.Directories = append(DirectoryMounts{}, inherited.Directories...)
		permissions.useDirectories = inherited.useDirectories
	}

	return nil
}

func isExplicitlySet(v *viper.Viper, flags *pflag.FlagSet, key string) bool {
	if flags != nil && flags.Changed(key) {
		return true
	}

	if v.InConfig(key) {
		return true
	}

	envKey := "WD_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	value, ok := os.LookupEnv(envKey)
	return ok && value != ""
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

func directoryMountsDecodeHook() mapstructure.DecodeHookFunc {
	mountsType := reflect.TypeOf(DirectoryMounts{})

	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if to != mountsType {
			return data, nil
		}

		return decodeDirectoryMounts(data)
	}
}

func getDirectoryMounts(v *viper.Viper, key string, fallback DirectoryMounts) (DirectoryMounts, error) {
	value := v.Get(key)
	if value == nil {
		return fallback, nil
	}

	return decodeDirectoryMounts(value)
}

func decodeDirectoryMounts(data any) (DirectoryMounts, error) {
	switch value := data.(type) {
	case nil:
		return DirectoryMounts{}, nil
	case DirectoryMounts:
		return value, nil
	case []DirectoryMount:
		return DirectoryMounts(value), nil
	case string:
		if value == "" {
			return DirectoryMounts{}, nil
		}

		parts := strings.Split(value, ",")
		mounts := make(DirectoryMounts, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			mounts = append(mounts, DirectoryMount{Path: part})
		}
		return mounts, nil
	case []any:
		mounts := make(DirectoryMounts, 0, len(value))
		for _, item := range value {
			mount, err := decodeDirectoryMount(item)
			if err != nil {
				return nil, err
			}
			mounts = append(mounts, mount)
		}
		return mounts, nil
	case []string:
		mounts := make(DirectoryMounts, 0, len(value))
		for _, item := range value {
			mounts = append(mounts, DirectoryMount{Path: item})
		}
		return mounts, nil
	default:
		return nil, fmt.Errorf("invalid directories: unsupported value %T", data)
	}
}

func decodeDirectoryMount(data any) (DirectoryMount, error) {
	switch value := data.(type) {
	case string:
		return DirectoryMount{Path: value}, nil
	case map[string]any:
		return decodeDirectoryMountMap(value)
	case map[any]any:
		m := map[string]any{}
		for key, value := range value {
			keyString, ok := key.(string)
			if !ok {
				return DirectoryMount{}, errors.New("invalid directories: mount keys must be strings")
			}
			m[keyString] = value
		}
		return decodeDirectoryMountMap(m)
	default:
		return DirectoryMount{}, fmt.Errorf("invalid directories: unsupported mount entry %T", data)
	}
}

func decodeDirectoryMountMap(data map[string]any) (DirectoryMount, error) {
	_, hasName := data["name"]
	_, hasPath := data["path"]
	if hasName || hasPath {
		name, nameOK := data["name"].(string)
		path, pathOK := data["path"].(string)
		if !nameOK || !pathOK || len(data) != 2 {
			return DirectoryMount{}, errors.New("invalid directories: explicit mount objects must define name and path")
		}
		return DirectoryMount{Name: name, Path: path}, nil
	}

	if len(data) != 1 {
		return DirectoryMount{}, errors.New("invalid directories: mapped mount entries must have exactly one key")
	}

	for name, path := range data {
		pathString, ok := path.(string)
		if !ok {
			return DirectoryMount{}, errors.New("invalid directories: mapped mount paths must be strings")
		}
		return DirectoryMount{Name: name, Path: pathString}, nil
	}

	return DirectoryMount{}, errors.New("invalid directories: empty mount entry")
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
