package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptReadsStdin(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetIn(strings.NewReader("secret\r\n"))
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"bcrypt", "-c", "4"})
	require.NoError(t, rootCmd.Execute())

	hash := strings.TrimSpace(out.String())
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret")))
}
