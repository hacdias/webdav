package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	flags := bcryptCmd.Flags()
	flags.IntP("cost", "c", bcrypt.DefaultCost, "cost used to generate password, higher cost leads to slower verification times")

	rootCmd.AddCommand(bcryptCmd)
}

var bcryptCmd = &cobra.Command{
	Use:   "bcrypt",
	Short: "Generate a bcrypt encrypted password",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cost, err := cmd.Flags().GetInt("cost")
		if err != nil {
			return err
		}

		if cost < bcrypt.MinCost {
			return fmt.Errorf("given cost cannot be under minimum cost of %d", bcrypt.MinCost)
		}

		if cost > bcrypt.MaxCost {
			return fmt.Errorf("given cost cannot be over maximum cost of %d", bcrypt.MaxCost)
		}

		pwd := args[0]
		if pwd == "" {
			return errors.New("password argument must not be empty")
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(pwd), cost)
		if err != nil {
			return err
		}

		fmt.Println(string(hash))
		return nil
	},
}
