package lib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeAndParseConfig(t *testing.T, content, extension string) *Config {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config"+extension)

	err := os.WriteFile(tmpFile, []byte(content), 0666)
	require.NoError(t, err)

	cfg, err := ParseConfig(tmpFile, nil)
	require.NoError(t, err)

	return cfg
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := writeAndParseConfig(t, "", ".yml")
	require.NoError(t, cfg.Validate())

	require.EqualValues(t, DefaultAuth, cfg.Auth)
	require.EqualValues(t, DefaultTLS, cfg.TLS)
	require.EqualValues(t, DefaultAddress, cfg.Address)
	require.EqualValues(t, DefaultPort, cfg.Port)
	require.EqualValues(t, DefaultPrefix, cfg.Prefix)
	require.EqualValues(t, DefaultLogFormat, cfg.LogFormat)
	require.NotEmpty(t, cfg.Scope)

	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedHeaders)
	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedHosts)
	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedMethods)
}

func TestConfigCascade(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, cfg *Config) {
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

	t.Run("YAML", func(t *testing.T) {
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

		cfg := writeAndParseConfig(t, content, ".yml")
		require.NoError(t, cfg.Validate())

		check(t, cfg)
	})

	t.Run("JSON", func(t *testing.T) {
		content := `{
	"auth": true,
	"scope": "/",
	"modify": true,
	"rules": [
		{
			"path": "/public/access/",
			"modify": true
		}
	],
	"users": [
		{
			"username": "admin",
			"password": "admin"
		},
		{
			"username": "basic",
			"password": "basic",
			"scope": "/basic",
			"modify": false,
			"rules": []
		}
	]
}`

		cfg := writeAndParseConfig(t, content, ".json")
		require.NoError(t, cfg.Validate())

		check(t, cfg)
	})

	t.Run("`TOML", func(t *testing.T) {
		content := `auth = true
scope = "/"
modify = true

[[rules]]
path = "/public/access/"
modify = true

[[users]]
username = "admin"
password = "admin"

[[users]]
username = "basic"
password = "basic"
scope = "/basic"
modify = false
rules = []
`

		cfg := writeAndParseConfig(t, content, ".toml")
		require.NoError(t, cfg.Validate())

		check(t, cfg)
	})
}

func TestConfigKeys(t *testing.T) {
	t.Parallel()

	cfg := writeAndParseConfig(t, `
cors:
  enabled: true
  credentials: true
  allowed_headers:
    - Depth
  allowed_hosts:
    - http://localhost:8080
  allowed_methods:
    - GET
  exposed_headers:
    - Content-Length
    - Content-Range`, ".yml")
	require.NoError(t, cfg.Validate())

	require.True(t, cfg.CORS.Enabled)
	require.True(t, cfg.CORS.Credentials)
	require.EqualValues(t, []string{"Content-Length", "Content-Range"}, cfg.CORS.ExposedHeaders)
	require.EqualValues(t, []string{"Depth"}, cfg.CORS.AllowedHeaders)
	require.EqualValues(t, []string{"http://localhost:8080"}, cfg.CORS.AllowedHosts)
	require.EqualValues(t, []string{"GET"}, cfg.CORS.AllowedMethods)
}

func TestConfigRules(t *testing.T) {
	content := `
auth: false
scope: /
modify: true
rules:
  - path: '^.+\.js$'
    regex: true
    modify: true
  - path: /public/access/
    regex: false
    modify: true`

	cfg := writeAndParseConfig(t, content, ".yaml")
	require.NoError(t, cfg.Validate())

	require.Len(t, cfg.Rules, 2)

	require.Empty(t, cfg.Rules[0].Path)
	require.NotNil(t, cfg.Rules[0].Regexp)
	require.True(t, cfg.Rules[0].Regexp.MatchString("/my/path/to/file.js"))
	require.False(t, cfg.Rules[0].Regexp.MatchString("/my/path/to/file.ts"))

	require.NotEmpty(t, cfg.Rules[1].Path)
	require.Nil(t, cfg.Rules[1].Regexp)
}

func TestConfigEnv(t *testing.T) {
	require.NoError(t, os.Setenv("WD_PORT", "1234"))
	require.NoError(t, os.Setenv("WD_DEBUG", "true"))
	require.NoError(t, os.Setenv("WD_MODIFY", "true"))
	require.NoError(t, os.Setenv("WD_SCOPE", "/test"))

	cfg, err := ParseConfig("", nil)
	require.NoError(t, err)

	assert.Equal(t, 1234, cfg.Port)
	assert.Equal(t, "/test", cfg.Scope)
	assert.Equal(t, true, cfg.Debug)
	assert.Equal(t, true, cfg.Modify)

	// Reset
	require.NoError(t, os.Setenv("WD_PORT", ""))
	require.NoError(t, os.Setenv("WD_DEBUG", ""))
	require.NoError(t, os.Setenv("WD_MODIFY", ""))
	require.NoError(t, os.Setenv("WD_SCOPE", ""))
}
