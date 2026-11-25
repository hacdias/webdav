package cmd

import (
	"bufio"
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
	Use:   "bcrypt [password]",
	Short: "Generate a bcrypt encrypted password",
	Long:  "Generate a bcrypt encrypted password. Without an argument, the password is read from the first line of standard input, which keeps it out of the shell history.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cost, err := cmd.Flags().GetInt("cost")
		if err != nil {
			return err
		}

		// [bcrypt.GenerateFromPassword] silently uses the default cost below the minimum.
		if cost < bcrypt.MinCost {
			return fmt.Errorf("given cost cannot be under minimum cost of %d", bcrypt.MinCost)
		}

		var pwd string
		if len(args) == 1 {
			pwd = args[0]
		} else {
			scanner := bufio.NewScanner(cmd.InOrStdin())
			scanner.Scan()
			if err := scanner.Err(); err != nil {
				return err
			}
			pwd = scanner.Text()
		}

		if pwd == "" {
			return errors.New("password must not be empty")
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(pwd), cost)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(hash))
		return err
	},
}
