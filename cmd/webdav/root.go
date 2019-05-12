package main

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
	flags.Bool("auth", true, "whether to use authentication or not")
	flags.BoolP("tls", "t", false, "enable tls")
	flags.String("cert", "cert.pem", "TLS certificate")
	flags.String("key", "key.pem", "TLS key")
	flags.StringP("address", "a", "0.0.0.0", "address to listen to")
	flags.StringP("port", "p", "0", "port to listen to")
}

var rootCmd = &cobra.Command{
	Use:   "webdav",
	Short: "A simple to use Web Dav server",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()

		cfg := readConfig(flags)

		// Builds the address and a listener.
		laddr := getOpt(flags, "address") + ":" + getOpt(flags, "port")
		listener, err := net.Listen("tcp", laddr)
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
		v.AddConfigPath("/etc/webdav/")
		v.SetConfigName("config")
	} else {
		v.SetConfigFile(cfgFile)
	}

	v.SetEnvPrefix("WD")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := v.ReadInConfig()
	checkErr(err)
}
