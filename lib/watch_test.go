package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func shortConfigWatchIntervals(t *testing.T) {
	t.Helper()
	delay, interval := configReloadDelay, configRearmInterval
	configReloadDelay, configRearmInterval = 20*time.Millisecond, 30*time.Millisecond
	t.Cleanup(func() { configReloadDelay, configRearmInterval = delay, interval })
}

type configChange struct {
	cfg *Config
	err error
}

func watchTestConfig(t *testing.T) (string, string, <-chan configChange) {
	t.Helper()
	shortConfigWatchIntervals(t)
	text := fmt.Sprintf("directory: %q\nusers:\n  - username: alice\n    password: pass\n", t.TempDir())
	cfg := writeAndParseConfig(t, text, ".yml")
	changes := make(chan configChange, 32)
	t.Cleanup(WatchConfig(cfg, nil, func(next *Config, err error) { changes <- configChange{next, err} }))
	return cfg.file, text, changes
}

func awaitConfigChange(t *testing.T, changes <-chan configChange, user string) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case change := <-changes:
			return change.err == nil && change.cfg != nil && len(change.cfg.Users) > 0 && change.cfg.Users[len(change.cfg.Users)-1].Username == user
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
}

func TestWatchConfigInPlace(t *testing.T) {
	path, text, changes := watchTestConfig(t)
	require.NoError(t, os.WriteFile(path, []byte(text+"  - username: bob\n    password: pass\n"), 0600))
	awaitConfigChange(t, changes, "bob")
}

func TestWatchConfigRecreate(t *testing.T) {
	path, text, changes := watchTestConfig(t)
	text += "  - username: bob\n    password: pass\n"
	require.NoError(t, os.WriteFile(path+".new", []byte(text), 0600))
	// Windows briefly refuses these while the file is open.
	require.Eventually(t, func() bool { return os.Remove(path) == nil }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return os.Rename(path+".new", path) == nil }, 2*time.Second, 10*time.Millisecond)
	awaitConfigChange(t, changes, "bob")
	require.NoError(t, os.WriteFile(path, []byte(text+"  - username: carol\n    password: pass\n"), 0600))
	awaitConfigChange(t, changes, "carol")
}

func TestWatchConfigRejectsInvalid(t *testing.T) {
	for _, name := range []string{"empty", "served"} {
		t.Run(name, func(t *testing.T) {
			path, _, changes := watchTestConfig(t)
			text, message := "", "config file is empty"
			if name == "served" {
				text, message = fmt.Sprintf("directory: %q", filepath.Dir(path)), "inside a served directory"
			}
			require.NoError(t, os.WriteFile(path, []byte(text), 0600))
			var change configChange
			require.Eventually(t, func() bool {
				select {
				case change = <-changes:
					return true
				default:
					return false
				}
			}, 5*time.Second, 10*time.Millisecond)
			require.Nil(t, change.cfg)
			require.ErrorContains(t, change.err, message)
		})
	}
}

func TestWatchConfigDisabled(t *testing.T) {
	shortConfigWatchIntervals(t)
	var calls atomic.Int32
	callback := func(*Config, error) { calls.Add(1) }
	stop := WatchConfig(&Config{}, nil, callback)
	stop()
	stop()
	cfg := writeAndParseConfig(t, "", ".yml")
	cfg.Directory = filepath.Dir(cfg.file)
	t.Cleanup(WatchConfig(cfg, nil, callback))
	require.NoError(t, os.WriteFile(cfg.file, []byte("port: 12345"), 0600))
	require.Never(t, func() bool { return calls.Load() != 0 }, 200*time.Millisecond, 10*time.Millisecond)
}

func TestWatchConfigRearmCap(t *testing.T) {
	previous := watchConfigArm
	var arms atomic.Int32
	watchConfigArm = func(*viper.Viper) { arms.Add(1) }
	t.Cleanup(func() { watchConfigArm = previous })
	path, text, changes := watchTestConfig(t)
	user := func(i int) string {
		// Sizes differ, so a coarse modification time can't hide a change.
		return text + fmt.Sprintf("  - username: user%d\n    password: pass%s\n", i, strings.Repeat("x", i))
	}

	// Edits in place without a callback reload, but don't re-arm.
	for i := range 3 {
		require.NoError(t, os.WriteFile(path, []byte(user(i)), 0600))
		awaitConfigChange(t, changes, fmt.Sprintf("user%d", i))
	}
	require.EqualValues(t, 1, arms.Load())

	// Replacements re-arm, up to the limit.
	for i := range configMaxRearms + 2 {
		require.NoError(t, os.WriteFile(path+".tmp", []byte(user(10+i)), 0600))
		// Windows briefly refuses these while the file is open.
		require.Eventually(t, func() bool { return os.Rename(path+".tmp", path) == nil }, 2*time.Second, 10*time.Millisecond)
		awaitConfigChange(t, changes, fmt.Sprintf("user%d", 10+i))
	}
	require.EqualValues(t, 1+configMaxRearms, arms.Load())
}

func TestWatchConfigChangedBeforeWatch(t *testing.T) {
	shortConfigWatchIntervals(t)
	text := fmt.Sprintf("directory: %q\nusers:\n  - username: alice\n    password: pass\n", t.TempDir())
	cfg := writeAndParseConfig(t, text, ".yml")
	// Replaced after parsing but before the watch started, so the file system
	// watcher can't report it.
	require.NoError(t, os.WriteFile(cfg.file+".new", []byte(text+"  - username: bob\n    password: pass\n"), 0600))
	require.NoError(t, os.Rename(cfg.file+".new", cfg.file))
	changes := make(chan configChange, 32)
	t.Cleanup(WatchConfig(cfg, nil, func(next *Config, err error) { changes <- configChange{next, err} }))
	awaitConfigChange(t, changes, "bob")
}

func TestParseConfigContent(t *testing.T) {
	cfg := writeAndParseConfig(t, "port: 10001", ".yml")
	next, err := parseConfig(cfg.file, nil, []byte("port: 10002"))
	require.NoError(t, err)
	require.Equal(t, 10002, next.Port)
	require.Equal(t, cfg.file, next.ConfigFile())
}

func TestConfigFingerprintMissing(t *testing.T) {
	info, err := configFingerprint(filepath.Join(t.TempDir(), "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, info)
}

func TestConfigFileIsServed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	for _, tc := range []struct {
		name, text string
		served     bool
	}{
		{"global directory", fmt.Sprintf("directory: %q", dir), true},
		{"global mount", fmt.Sprintf("directories:\n  - name: config\n    path: %q", dir), true},
		{"user directory", fmt.Sprintf("directory: %q\nusers:\n  - username: alice\n    password: pass\n    directory: %q", data, dir), true},
		{"user mount", fmt.Sprintf("directory: %q\nusers:\n  - username: alice\n    password: pass\n    directories:\n      - name: config\n        path: %q", data, dir), true},
		{"unreachable global directory", fmt.Sprintf("directory: %q\nusers:\n  - username: alice\n    password: pass\n    directory: %q", dir, data), false},
		{"unreachable global mount", fmt.Sprintf("directories:\n  - name: config\n    path: %q\nusers:\n  - username: alice\n    password: pass\n    directory: %q", dir, data), false},
		{"inherited user directory", fmt.Sprintf("directory: %q\nusers:\n  - username: alice\n    password: pass", dir), true},
		{"outside", fmt.Sprintf("directory: %q", data), false},
		{"sibling prefix", fmt.Sprintf("directory: %q", dir+"-other"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(path, []byte(tc.text), 0600))
			cfg, err := ParseConfig(path, nil)
			require.NoError(t, err)
			require.Equal(t, tc.served, configFileIsServed(cfg))
		})
	}
}

func TestConfigFileIsServedDirectoryIdentity(t *testing.T) {
	t.Parallel()
	served := t.TempDir()
	nested := filepath.Join(served, "nested")
	require.NoError(t, os.Mkdir(nested, 0700))
	alias := filepath.Join(t.TempDir(), "served-alias")
	if err := os.Symlink(served, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	path := filepath.Join(nested, "config.yml")
	require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, "directory: %q", alias), 0600))
	cfg, err := ParseConfig(path, nil)
	require.NoError(t, err)
	require.True(t, configFileIsServed(cfg), "a served directory alias must match a config ancestor by identity")
	cfg.Directory = t.TempDir()
	require.False(t, configFileIsServed(cfg), "a different directory must not match")
}

func TestConfigFileIsServedSymlinkEntry(t *testing.T) {
	served, external := t.TempDir(), t.TempDir()
	target := filepath.Join(external, "settings.yml")
	require.NoError(t, os.WriteFile(target, fmt.Appendf(nil, "directory: %q", served), 0600))
	entry := filepath.Join(served, "config.yml")
	if err := os.Symlink(target, entry); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg, err := ParseConfig(entry, nil)
	require.NoError(t, err)
	require.True(t, configFileIsServed(cfg), "the served symlink can write through to an external target")
}
