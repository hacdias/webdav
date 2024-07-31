package lib

import (
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
	if strings.HasPrefix(u.Password, "{bcrypt}") {
		savedPassword := strings.TrimPrefix(u.Password, "{bcrypt}")
		return bcrypt.CompareHashAndPassword([]byte(savedPassword), []byte(input)) == nil
	}

	return u.Password == input
}

func (u *User) Validate() error {
	if u.Username == "" {
		return errors.New("invalid user: username must be set")
	}

	if u.Password == "" {
		return fmt.Errorf("invalid user %q: password must be set", u.Username)
	} else if strings.HasPrefix(u.Password, "{env}") {

		env := strings.TrimPrefix(u.Password, "{env}")
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
