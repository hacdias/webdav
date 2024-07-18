package cmd

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	v "github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	cfgFile string
)

func init() {
	cobra.OnInitialize(initConfig)

	flags := rootCmd.Flags()
	flags.StringVarP(&cfgFile, "config", "c", "", "config file path")
	flags.BoolP("tls", "t", false, "enable tls")
	flags.Bool("auth", true, "enable auth")
	flags.String("cert", "cert.pem", "TLS certificate")
	flags.String("key", "key.pem", "TLS key")
	flags.StringP("address", "a", "0.0.0.0", "address to listen to")
	flags.StringP("port", "p", "0", "port to listen to")
	flags.StringP("prefix", "P", "/", "URL path prefix")
	flags.String("log_format", "console", "logging format")
}

var rootCmd = &cobra.Command{
	Use:   "webdav",
	Short: "A simple to use WebDAV server",
	Long: `If you don't set "config", it will look for a configuration file called
config.{json, toml, yaml, yml} in the following directories:

- ./
- /etc/webdav/

The precedence of the configuration values are as follows:

- flags
- environment variables
- configuration file
- defaults

The environment variables are prefixed by "WD_" followed by the option
name in caps. So to set "cert" via an env variable, you should
set WD_CERT.`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()

		cfg := readConfig(flags)

		// Build address and listener
		laddr := getOpt(flags, "address")
		var lnet string
		if strings.HasPrefix(laddr, "unix:") {
			laddr = laddr[5:]
			lnet = "unix"
		} else {
			laddr = laddr + ":" + getOpt(flags, "port")
			lnet = "tcp"
		}
		listener, err := net.Listen(lnet, laddr)

		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)

		go func(c chan os.Signal) {
			// Wait for a SIGINT or SIGKILL:
			sig := <-c
			log.Printf("Caught signal %s: shutting down.", sig)
			// Stop listening (and unlink the socket if unix type):
			listener.Close()
			// And we're done:
			os.Exit(0)
		}(sigc)

		if err != nil {
			log.Fatal(err)
		}
		loggerConfig := zap.NewProductionConfig()
		loggerConfig.DisableCaller = true
		if cfg.Debug {
			loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		}
		loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		loggerConfig.Encoding = cfg.LogFormat
		logger, err := loggerConfig.Build()
		if err != nil {
			// if we fail to configure proper logging, then the user has deliberately
			// misconfigured the logger. Abort.
			panic(err)
		}
		zap.ReplaceGlobals(logger)
		defer func() {
			_ = zap.L().Sync()
		}()
		// Tell the user the port in which is listening.
		zap.L().Info("Listening", zap.String("address", listener.Addr().String()))

		// Starts the server.
		if getOptB(flags, "tls") {
			if err := http.ServeTLS(listener, cfg, getOpt(flags, "cert"), getOpt(flags, "key")); err != nil {
				zap.L().Fatal("shutting server", zap.Error(err))
			}
		} else {
			if err := http.Serve(listener, cfg); err != nil {
				zap.L().Fatal("shutting server", zap.Error(err))
			}
		}
	},
}

func initConfig() {
	if cfgFile == "" {
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/webdav/")
		v.SetConfigName("config")
	} else {
		v.SetConfigFile(cfgFile)
	}

	v.SetEnvPrefix("WD")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(v.ConfigParseError); ok {
			panic(err)
		}
		cfgFile = "No config file used"
	} else {
		cfgFile = "Using config file: " + v.ConfigFileUsed()
	}
}
