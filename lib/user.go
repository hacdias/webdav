package lib

import (
	"crypto/sha256"
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

// checkPassword remembers the last accepted password, as bcrypt is slow by design.
func (u *handlerUser) checkPassword(input string) bool {
	sum := sha256.Sum256([]byte(input))
	if last := u.verified.Load(); last != nil && subtle.ConstantTimeCompare(last[:], sum[:]) == 1 {
		return true
	}

	if !u.User.checkPassword(input) {
		return false
	}

	u.verified.Store(&sum)
	return true
}

func (u *User) Validate(noPassword bool) error {
	if u.Username == "" {
		return errors.New("invalid user: username must be set")
	} else if env, ok := strings.CutPrefix(u.Username, "{env}"); ok {
		if env == "" {
			return fmt.Errorf("invalid user %q: username environment variable not set", u.Username)
		}

		username := os.Getenv(env)
		if username == "" {
			return fmt.Errorf("invalid user %q: username environment variable is empty", u.Username)
		}
		u.Username = username
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

	if hash, ok := strings.CutPrefix(u.Password, "{bcrypt}"); ok {
		if _, err := bcrypt.Cost([]byte(hash)); err != nil {
			return fmt.Errorf("invalid user %q: %w", u.Username, err)
		}
	}

	if err := u.UserPermissions.Validate(); err != nil {
		return fmt.Errorf("invalid user %q: %w", u.Username, err)
	}

	return nil
}
