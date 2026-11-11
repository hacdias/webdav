package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/hacdias/webdav/v5/lib"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func init() {
	flags := rootCmd.Flags()
	flags.StringP("config", "c", "", "config file path")
	flags.StringP("address", "a", lib.DefaultAddress, "address to listen on")
	flags.IntP("port", "p", lib.DefaultPort, "port to listen on")
	flags.BoolP("tls", "t", lib.DefaultTLS, "enable TLS")
	flags.String("cert", lib.DefaultCert, "path to TLS certificate")
	flags.String("key", lib.DefaultKey, "path to TLS key")
	flags.StringP("prefix", "P", lib.DefaultPrefix, "URL path prefix")
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
		// Flags parsed fine, so usage would not help with what fails from here.
		cmd.SilenceUsage = true

		flags := cmd.Flags()

		cfgFilename, _ := flags.GetString("config")

		cfg, err := lib.ParseConfig(cfgFilename, flags)
		if err != nil {
			return err
		}

		// Setup the logger based on the configuration
		logger, err := cfg.GetLogger()
		if err != nil {
			return err
		}
		zap.ReplaceGlobals(logger)

		// Create HTTP handler from the config
		handler, err := lib.NewReloadableHandler(cfg)
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

		// No read or write timeouts, which would cut off large transfers.
		server := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
			IdleTimeout:       5 * time.Minute,
		}

		// Trap exiting signals
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

		failed := make(chan error, 1)
		go func() {
			zap.L().Info("listening", zap.String("address", listener.Addr().String()))

			if cfg.TLS {
				failed <- server.ServeTLS(listener, cfg.Cert, cfg.Key)
			} else {
				failed <- server.Serve(listener)
			}
		}()

		applied := cfg
		stop := lib.WatchConfig(cfg, flags, func(next *lib.Config, err error) {
			var logger *zap.Logger
			if err == nil && (next.Debug != applied.Debug || !reflect.DeepEqual(next.Log, applied.Log)) {
				logger, err = next.GetLogger()
			}
			if err == nil {
				err = handler.Reload(next)
			}
			if err != nil {
				zap.L().Error("failed to reload config, keeping the previous one", zap.Error(err))
				return
			}
			if logger != nil {
				previous := zap.L()
				zap.ReplaceGlobals(logger)
				// zap has no way to close the previous output; a log file stays open.
				_ = previous.Sync()
			}
			if next.Address != cfg.Address || next.Port != cfg.Port || next.TLS != cfg.TLS ||
				(cfg.TLS && (next.Cert != cfg.Cert || next.Key != cfg.Key)) {
				zap.L().Warn("listener settings changed, restart to apply them")
			}
			applied = next
			zap.L().Info("config reloaded")
		})
		defer stop()

		select {
		case err := <-failed:
			// Shutdown has not run, so this is never [http.ErrServerClosed].
			zap.L().Error("failed to start server", zap.Error(err))
			return err
		case sig := <-quit:
			zap.L().Info("caught signal, shutting down", zap.Stringer("signal", sig))
		case <-cmd.Context().Done():
		}
		_ = server.Shutdown(context.Background())

		return nil
	},
}

func getListener(cfg *lib.Config) (net.Listener, error) {
	var (
		address string
		network string
	)

	if strings.HasPrefix(cfg.Address, "sd-listen-fd:") {
		listeners, err := activation.ListenersWithNames()
		if err != nil {
			return nil, err
		}

		address := cfg.Address[13:]
		listener, ok := listeners[address]

		if !ok || len(listener) < 1 {
			return nil, errors.New("unknown sd-listen-fd address '" + address + "'")
		}

		return listener[0], nil
	} else if strings.HasPrefix(cfg.Address, "unix:") {
		address = cfg.Address[5:]
		network = "unix"
	} else {
		address = fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
		network = "tcp"
	}

	return net.Listen(network, address)
}
