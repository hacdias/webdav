package lib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeAndParseConfig(t *testing.T, content string) *Config {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yml")

	err := os.WriteFile(tmpFile, []byte(content), 0666)
	require.NoError(t, err)

	cfg, err := ParseConfig(tmpFile, nil)
	require.NoError(t, err)

	return cfg
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := writeAndParseConfig(t, "")
	require.NoError(t, cfg.Validate())

	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedHeaders)
	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedHosts)
	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedMethods)
}

func TestConfigCascade(t *testing.T) {
	t.Parallel()

	content := `
auth: true
scope: /
modify: true
rules:
  - path: /public/access/
    modify: true

users:
  - username: admin
    password: admin
  - username: basic
    password: basic
    scope: /basic
    modify: false
    rules: []`

	cfg := writeAndParseConfig(t, content)
	require.NoError(t, cfg.Validate())

	require.True(t, cfg.Modify)
	require.Equal(t, "/", cfg.Scope)
	require.Len(t, cfg.Rules, 1)

	require.Len(t, cfg.Users, 2)

	require.True(t, cfg.Users[0].Modify)
	require.Equal(t, "/", cfg.Users[0].Scope)
	require.Len(t, cfg.Users[0].Rules, 1)

	require.False(t, cfg.Users[1].Modify)
	require.Equal(t, "/basic", cfg.Users[1].Scope)
	require.Len(t, cfg.Users[1].Rules, 0)
}
