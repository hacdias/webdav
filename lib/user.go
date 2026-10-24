package lib

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	UserPermissions `mapstructure:",squash"`
	Username        string
	Password        string
}

func (u User) checkPassword(input string) bool {
	if savedPassword, ok := strings.CutPrefix(u.Password, "{bcrypt}"); ok {
		return bcrypt.CompareHashAndPassword([]byte(savedPassword), []byte(input)) == nil
	}

	return subtle.ConstantTimeCompare([]byte(u.Password), []byte(input)) == 1
}

func (u *User) Validate(noPassword bool) error {
	if u.Username == "" {
		return errors.New("invalid user: username must be set")
	} else if env, ok := strings.CutPrefix(u.Username, "{env}"); ok {
		if env == "" {
			return fmt.Errorf("invalid user %q: username environment variable not set", u.Username)
		}

		u.Username = os.Getenv(env)
		if u.Username == "" {
			return fmt.Errorf("invalid user %q: username environment variable is empty", u.Username)
		}
	}

	if u.Password == "" && !noPassword {
		return fmt.Errorf("invalid user %q: password must be set", u.Username)
	} else if env, ok := strings.CutPrefix(u.Password, "{env}"); ok {
		if env == "" {
			return fmt.Errorf("invalid user %q: password environment variable not set", u.Username)
		}

		u.Password = os.Getenv(env)
		if u.Password == "" {
			return fmt.Errorf("invalid user %q: password environment variable is empty", u.Username)
		}
	}

	if err := u.UserPermissions.Validate(); err != nil {
		return fmt.Errorf("invalid user %q: %w", u.Username, err)
	}

	return nil
}
