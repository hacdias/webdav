package cmd

import (
	"log"

	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
)

// getOption returns a parameter as a string.
//
// NOTE: we could simply bind the flags to viper and use IsSet.
// Although there is a bug on Viper that always returns true on IsSet
// if a flag is binded. Our alternative way is to manually check
// the flag and then the value from env/config/gotten by viper.
// https://github.com/spf13/viper/pull/331
func getOpt(flags *pflag.FlagSet, key string) string {
	value, _ := flags.GetString(key)

	// If set on Flags, use it.
	if flags.Changed(key) {
		return value
	}

	// If set through viper (env, config), return it.
	if v.IsSet(key) {
		return v.GetString(key)
	}

	// Otherwise use default value on flags.
	return value
}

func getOptB(flags *pflag.FlagSet, key string) bool {
	value, _ := flags.GetBool(key)

	// If set on Flags, use it.
	if flags.Changed(key) {
		return value
	}

	// If set through viper (env, config), return it.
	if v.IsSet(key) {
		return v.GetBool(key)
	}

	// Otherwise use default value on flags.
	return value
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
