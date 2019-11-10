package lib

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func checkPassword(saved, input string) bool {
	if strings.HasPrefix(saved, "{bcrypt}") {
		savedPassword := strings.TrimPrefix(saved, "{bcrypt}")
		return bcrypt.CompareHashAndPassword([]byte(savedPassword), []byte(input)) == nil
	}

	return saved == input
}

func isAllowedHost(allowedHosts []string, origin string) bool {
	for _, host := range allowedHosts {
		if host == origin {
			return true
		}
	}
	return false
}
