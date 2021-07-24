package cmd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	v "github.com/spf13/viper"
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
}

var rootCmd = &cobra.Command{
	Use:   "lib_official_webdav",
	Short: "A simple to use WebDAV server",
	Long: `If you don't set "config", it will look for a configuration file called
config.{json, toml, yaml, yml} in the following directories:

- ./
- /etc/lib_official_webdav/

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
		if err != nil {
			log.Fatal(err)
		}

		// Tell the user the port in which is listening.
		fmt.Println("Listening on", listener.Addr().String())

		// Starts the server.
		if getOptB(flags, "tls") {
			if err := http.ServeTLS(listener, cfg, getOpt(flags, "cert"), getOpt(flags, "key")); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := http.Serve(listener, cfg); err != nil {
				log.Fatal(err)
			}
		}
	},
}

func initConfig() {
	if cfgFile == "" {
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/lib_official_webdav/")
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
