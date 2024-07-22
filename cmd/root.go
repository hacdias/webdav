package cmd

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hacdias/webdav/v4/lib"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	flags := rootCmd.Flags()
	flags.StringP("config", "c", "", "config file path")
	flags.BoolP("tls", "t", lib.DefaultTLS, "enable TLS")
	flags.Bool("auth", lib.DefaultAuth, "enable authentication")
	flags.String("cert", lib.DefaultCert, "path to TLS certificate")
	flags.String("key", lib.DefaultKey, "path to TLS key")
	flags.StringP("address", "a", lib.DefaultAddress, "address to listen on")
	flags.IntP("port", "p", lib.DefaultPort, "port to listen on")
	flags.StringP("prefix", "P", lib.DefaultPrefix, "URL path prefix")
	flags.String("log_format", lib.DefaultLogFormat, "logging format")
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
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()

		cfgFilename, _ := flags.GetString("config")

		cfg, err := lib.ParseConfig(cfgFilename, flags)
		if err != nil {
			return err
		}

		// Create HTTP handler from the config
		handler, err := lib.NewHandler(cfg)
		if err != nil {
			return err
		}

		// Setup the logger based on the configuration
		err = setupLogger(cfg)
		if err != nil {
			return err
		}

		defer func() {
			// Flush the logger at the end
			_ = zap.L().Sync()
		}()

		// Build listener
		listener, err := getListener(cfg)
		if err != nil {
			return err
		}

		// Trap exiting signals
		quit := make(chan os.Signal, 1)

		go func() {
			zap.L().Info("listening", zap.String("address", listener.Addr().String()))

			var err error
			if cfg.TLS {
				err = http.ServeTLS(listener, handler, cfg.Cert, cfg.Key)
			} else {
				err = http.Serve(listener, handler)
			}

			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				zap.L().Error("failed to start server", zap.Error(err))
			}

			quit <- os.Interrupt
		}()

		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		signal := <-quit

		zap.L().Info("caught signal, shutting down", zap.Stringer("signal", signal))
		_ = listener.Close()

		return nil
	},
}

func getListener(cfg *lib.Config) (net.Listener, error) {
	var (
		address string
		network string
	)

	if strings.HasPrefix(cfg.Address, "unix:") {
		address = cfg.Address[5:]
		network = "unix"
	} else {
		address = fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
		network = "tcp"
	}

	return net.Listen(network, address)
}

func setupLogger(cfg *lib.Config) error {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.DisableCaller = true
	if cfg.Debug {
		loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	loggerConfig.Encoding = cfg.LogFormat
	logger, err := loggerConfig.Build()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(logger)
	return nil
}
