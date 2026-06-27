package lib

import (
	"os"
	"path/filepath"
	"strconv"
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

func writeAndParseConfigWithError(t *testing.T, content, extension, error string) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config"+extension)

	err := os.WriteFile(tmpFile, []byte(content), 0666)
	require.NoError(t, err)

	_, err = ParseConfig(tmpFile, nil)
	require.ErrorContains(t, err, error)
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := writeAndParseConfig(t, "", ".yml")
	require.NoError(t, cfg.Validate())

	require.EqualValues(t, DefaultTLS, cfg.TLS)
	require.EqualValues(t, DefaultAddress, cfg.Address)
	require.EqualValues(t, DefaultPort, cfg.Port)
	require.EqualValues(t, DefaultPrefix, cfg.Prefix)
	require.EqualValues(t, "console", cfg.Log.Format)
	require.EqualValues(t, true, cfg.Log.Colors)
	require.EqualValues(t, []string{"stderr"}, cfg.Log.Outputs)

	dir, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, dir, cfg.Directory)

	require.EqualValues(t, []string{"*"}, cfg.CORS.AllowedHosts)
	require.EqualValues(t, []string{"Authorization", "Content-Type", "Content-Range", "Depth", "Destination", "If", "Lock-Token", "Overwrite", "X-Update-Range"}, cfg.CORS.AllowedHeaders)
	require.EqualValues(t, []string{"COPY", "DELETE", "GET", "HEAD", "LOCK", "MKCOL", "MOVE", "OPTIONS", "PATCH", "POST", "PROPFIND", "PROPPATCH", "PUT", "UNLOCK"}, cfg.CORS.AllowedMethods)
}

func TestConfigCascade(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, cfg *Config) {
		require.True(t, cfg.Permissions.Read)
		require.True(t, cfg.Permissions.Create)
		require.False(t, cfg.Permissions.Delete)
		require.False(t, cfg.Permissions.Update)
		require.Equal(t, "/", cfg.Directory)
		require.Len(t, cfg.Rules, 1)

		require.Len(t, cfg.Users, 2)
		require.True(t, cfg.Users[0].Permissions.Read)
		require.True(t, cfg.Users[0].Permissions.Create)
		require.False(t, cfg.Users[0].Permissions.Delete)
		require.False(t, cfg.Users[0].Permissions.Update)
		require.Equal(t, "/", cfg.Users[0].Directory)
		require.Len(t, cfg.Users[0].Rules, 1)

		require.True(t, cfg.Users[1].Permissions.Read)
		require.False(t, cfg.Users[1].Permissions.Create)
		require.False(t, cfg.Users[1].Permissions.Delete)
		require.False(t, cfg.Users[1].Permissions.Update)
		require.Equal(t, "/basic", cfg.Users[1].Directory)
		require.Len(t, cfg.Users[1].Rules, 0)
	}

	t.Run("YAML", func(t *testing.T) {
		content := `
directory: /
permissions: CR
rules:
  - path: /public/access/
    permissions: R

users:
  - username: admin
    password: admin
  - username: basic
    password: basic
    directory: /basic
    permissions: R
    rules: []`

		cfg := writeAndParseConfig(t, content, ".yml")
		require.NoError(t, cfg.Validate())

		check(t, cfg)
	})

	t.Run("JSON", func(t *testing.T) {
		content := `{
	"directory": "/",
	"permissions": "CR",
	"rules": [
		{
			"path": "/public/access/",
			"permissions": "R"
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
			"directory": "/basic",
			"permissions": "R",
			"rules": []
		}
	]
}`

		cfg := writeAndParseConfig(t, content, ".json")
		require.NoError(t, cfg.Validate())

		check(t, cfg)
	})

	t.Run("`TOML", func(t *testing.T) {
		content := `
directory = "/"
permissions = "CR"

[[rules]]
path = "/public/access/"
permissions = "R"

[[users]]
username = "admin"
password = "admin"

[[users]]
username = "basic"
password = "basic"
directory = "/basic"
permissions = "R"
rules = []
`

		cfg := writeAndParseConfig(t, content, ".toml")
		require.NoError(t, cfg.Validate())

		check(t, cfg)
	})
}

func TestConfigDirectories(t *testing.T) {
	t.Parallel()

	t.Run("Mixed Entries", func(t *testing.T) {
		t.Parallel()

		dirC := t.TempDir()
		dirD := t.TempDir()
		dirE := t.TempDir()

		cfg := writeAndParseConfig(t, `
directories:
  - `+dirC+`
  - d2: `+dirD+`
  - name: archive
    path: `+dirE+`
`, ".yml")
		require.NoError(t, cfg.Validate())

		require.True(t, cfg.useDirectories)
		require.Equal(t, filepath.Base(dirC), cfg.Directories[0].Name)
		require.Equal(t, dirC, cfg.Directories[0].Path)
		require.Equal(t, "d2", cfg.Directories[1].Name)
		require.Equal(t, dirD, cfg.Directories[1].Path)
		require.Equal(t, "archive", cfg.Directories[2].Name)
		require.Equal(t, dirE, cfg.Directories[2].Path)
	})

	t.Run("JSON", func(t *testing.T) {
		t.Parallel()

		dirC := t.TempDir()
		dirD := t.TempDir()
		dirE := t.TempDir()

		cfg := writeAndParseConfig(t, `{
	"directories": [
		`+strconv.Quote(dirC)+`,
		{ "d2": `+strconv.Quote(dirD)+` },
		{ "name": "archive", "path": `+strconv.Quote(dirE)+` }
	]
}`, ".json")
		require.NoError(t, cfg.Validate())

		require.True(t, cfg.useDirectories)
		require.Equal(t, filepath.Base(dirC), cfg.Directories[0].Name)
		require.Equal(t, dirC, cfg.Directories[0].Path)
		require.Equal(t, "d2", cfg.Directories[1].Name)
		require.Equal(t, dirD, cfg.Directories[1].Path)
		require.Equal(t, "archive", cfg.Directories[2].Name)
		require.Equal(t, dirE, cfg.Directories[2].Path)
	})

	t.Run("TOML", func(t *testing.T) {
		t.Parallel()

		dirD := t.TempDir()
		dirE := t.TempDir()

		cfg := writeAndParseConfig(t, `
[[directories]]
d2 = `+strconv.Quote(dirD)+`

[[directories]]
name = "archive"
path = `+strconv.Quote(dirE)+`
`, ".toml")
		require.NoError(t, cfg.Validate())

		require.True(t, cfg.useDirectories)
		require.Equal(t, "d2", cfg.Directories[0].Name)
		require.Equal(t, dirD, cfg.Directories[0].Path)
		require.Equal(t, "archive", cfg.Directories[1].Name)
		require.Equal(t, dirE, cfg.Directories[1].Path)
	})

	t.Run("Mutually Exclusive Global Directory Fields", func(t *testing.T) {
		t.Parallel()

		writeAndParseConfigWithError(t, `
directory: /tmp
directories:
  - /tmp
`, ".yml", "directory and directories cannot both be defined")
	})

	t.Run("Mutually Exclusive User Directory Fields", func(t *testing.T) {
		t.Parallel()

		writeAndParseConfigWithError(t, `
users:
  - username: basic
    password: basic
    directory: /tmp
    directories:
      - /tmp
`, ".yml", "cannot define both directory and directories")
	})

	t.Run("Duplicate Mount Names", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		dir := filepath.Join(parent, "dup")
		require.NoError(t, os.Mkdir(dir, 0775))

		writeAndParseConfigWithError(t, `
directories:
  - `+dir+`
  - dup: /tmp
`, ".yml", "duplicate mount name")
	})

	t.Run("Cascade Mode", func(t *testing.T) {
		t.Parallel()

		global := t.TempDir()
		single := t.TempDir()
		userMulti := t.TempDir()

		cfg := writeAndParseConfig(t, `
directories:
  - global: `+global+`
users:
  - username: inherited
    password: inherited
  - username: single
    password: single
    directory: `+single+`
  - username: multi
    password: multi
    directories:
      - owned: `+userMulti+`
`, ".yml")
		require.NoError(t, cfg.Validate())

		require.True(t, cfg.useDirectories)
		require.True(t, cfg.Users[0].useDirectories)
		require.Equal(t, DirectoryMounts{{Name: "global", Path: global}}, cfg.Users[0].Directories)
		require.False(t, cfg.Users[1].useDirectories)
		require.Equal(t, single, cfg.Users[1].Directory)
		require.True(t, cfg.Users[2].useDirectories)
		require.Equal(t, DirectoryMounts{{Name: "owned", Path: userMulti}}, cfg.Users[2].Directories)
	})
}

func TestConfigDirectoriesEnvOverrides(t *testing.T) {
	global := t.TempDir()
	single := t.TempDir()
	userMulti := t.TempDir()

	t.Setenv("WD_DIRECTORIES", global)
	t.Setenv("WD_USERS_1_DIRECTORY", single)
	t.Setenv("WD_USERS_2_DIRECTORIES", userMulti)

	cfg := writeAndParseConfig(t, `
users:
  - username: inherited
    password: inherited
  - username: single
    password: single
  - username: multi
    password: multi
`, ".yml")
	require.NoError(t, cfg.Validate())

	require.True(t, cfg.useDirectories)
	require.Equal(t, DirectoryMounts{{Name: filepath.Base(global), Path: global}}, cfg.Directories)
	require.True(t, cfg.Users[0].useDirectories)
	require.Equal(t, DirectoryMounts{{Name: filepath.Base(global), Path: global}}, cfg.Users[0].Directories)
	require.False(t, cfg.Users[1].useDirectories)
	require.Equal(t, single, cfg.Users[1].Directory)
	require.True(t, cfg.Users[2].useDirectories)
	require.Equal(t, DirectoryMounts{{Name: filepath.Base(userMulti), Path: userMulti}}, cfg.Users[2].Directories)
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
	t.Run("Only Regex or Path", func(t *testing.T) {
		content := `
directory: /
rules:
  - regex: '^.+\.js$'
    path: /public/access/`

		writeAndParseConfigWithError(t, content, ".yaml", "cannot define both regex and path")
	})

	t.Run("Regex or Path Required", func(t *testing.T) {
		content := `
directory: /
rules:
  - permissions: CRUD`

		writeAndParseConfigWithError(t, content, ".yaml", "must either define a path of a regex")
	})

	t.Run("Parse", func(t *testing.T) {
		content := `
directory: /
rules:
  - regex: '^.+\.js$'
  - path: /public/access/`

		cfg := writeAndParseConfig(t, content, ".yaml")
		require.NoError(t, cfg.Validate())

		require.Len(t, cfg.Rules, 2)

		require.Empty(t, cfg.Rules[0].Path)
		require.NotNil(t, cfg.Rules[0].Regex)
		require.True(t, cfg.Rules[0].Regex.MatchString("/my/path/to/file.js"))
		require.False(t, cfg.Rules[0].Regex.MatchString("/my/path/to/file.ts"))

		require.NotEmpty(t, cfg.Rules[1].Path)
		require.Nil(t, cfg.Rules[1].Regex)
	})

	t.Run("Rules Behavior (Default: Overwrite)", func(t *testing.T) {
		content := `
directory: /
rules:
  - regex: '^.+\.js$'
  - path: /public/access/

users:
  - username: foo
    password: bar
    rules:
    - path: /private/access/`

		cfg := writeAndParseConfig(t, content, ".yaml")
		require.NoError(t, cfg.Validate())

		require.Len(t, cfg.Rules, 2)

		require.Empty(t, cfg.Rules[0].Path)
		require.NotNil(t, cfg.Rules[0].Regex)
		require.True(t, cfg.Rules[0].Regex.MatchString("/my/path/to/file.js"))
		require.False(t, cfg.Rules[0].Regex.MatchString("/my/path/to/file.ts"))

		require.EqualValues(t, "/public/access/", cfg.Rules[1].Path)
		require.Nil(t, cfg.Rules[1].Regex)

		require.Len(t, cfg.Users, 1)
		require.Len(t, cfg.Users[0].Rules, 1)
		require.EqualValues(t, "/private/access/", cfg.Users[0].Rules[0].Path)
	})

	t.Run("Rules Behavior (Append)", func(t *testing.T) {
		content := `
directory: /
rules:
  - regex: '^.+\.js$'
  - path: /public/access/
rulesBehavior: append

users:
  - username: foo
    password: bar
    rules:
    - path: /private/access/`

		cfg := writeAndParseConfig(t, content, ".yaml")
		require.NoError(t, cfg.Validate())

		require.Len(t, cfg.Rules, 2)

		require.Empty(t, cfg.Rules[0].Path)
		require.NotNil(t, cfg.Rules[0].Regex)
		require.True(t, cfg.Rules[0].Regex.MatchString("/my/path/to/file.js"))
		require.False(t, cfg.Rules[0].Regex.MatchString("/my/path/to/file.ts"))

		require.EqualValues(t, "/public/access/", cfg.Rules[1].Path)
		require.Nil(t, cfg.Rules[1].Regex)

		require.Len(t, cfg.Users, 1)
		require.Len(t, cfg.Users[0].Rules, 3)

		require.EqualValues(t, cfg.Rules[0], cfg.Users[0].Rules[0])
		require.EqualValues(t, cfg.Rules[1], cfg.Users[0].Rules[1])
		require.EqualValues(t, "/private/access/", cfg.Users[0].Rules[2].Path)
	})
}

func TestConfigEnv(t *testing.T) {
	require.NoError(t, os.Setenv("WD_PORT", "1234"))
	require.NoError(t, os.Setenv("WD_DEBUG", "true"))
	require.NoError(t, os.Setenv("WD_PERMISSIONS", "CRUD"))
	require.NoError(t, os.Setenv("WD_DIRECTORY", "/test"))

	cfg, err := ParseConfig("", nil)
	require.NoError(t, err)

	assert.Equal(t, 1234, cfg.Port)
	assert.Equal(t, "/test", cfg.Directory)
	assert.Equal(t, true, cfg.Debug)
	require.True(t, cfg.Permissions.Read)
	require.True(t, cfg.Permissions.Create)
	require.True(t, cfg.Permissions.Delete)
	require.True(t, cfg.Permissions.Update)

	// Reset
	require.NoError(t, os.Setenv("WD_PORT", ""))
	require.NoError(t, os.Setenv("WD_DEBUG", ""))
	require.NoError(t, os.Setenv("WD_PERMISSIONS", ""))
	require.NoError(t, os.Setenv("WD_DIRECTORY", ""))
}

func TestConfigParseUserPasswordEnvironment(t *testing.T) {
	content := `
directory: /
users:
  - username: '{env}USER1_USERNAME'
    password: '{env}USER1_PASSWORD'
  - username: basic
    password: basic
`

	writeAndParseConfigWithError(t, content, ".yml", "username environment variable is empty")

	err := os.Setenv("USER1_USERNAME", "admin")
	require.NoError(t, err)

	writeAndParseConfigWithError(t, content, ".yml", "password environment variable is empty")

	err = os.Setenv("USER1_PASSWORD", "admin")
	require.NoError(t, err)

	cfg := writeAndParseConfig(t, content, ".yaml")
	require.NoError(t, cfg.Validate())

	require.Equal(t, "admin", cfg.Users[0].Username)
	require.Equal(t, "basic", cfg.Users[1].Username)

	require.True(t, cfg.Users[0].checkPassword("admin"))
	require.True(t, cfg.Users[1].checkPassword("basic"))
}
