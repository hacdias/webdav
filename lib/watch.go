package lib

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// sameConfigPath follows the platform's default filename casing semantics.
func sameConfigPath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if defaultCaseInsensitiveFS {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func realConfigPath(name string) string {
	if real, err := filepath.EvalSymlinks(name); err == nil {
		name = real
	}
	if absolute, err := filepath.Abs(name); err == nil {
		name = absolute
	}
	return filepath.Clean(name)
}

func configFileIsServed(cfg *Config) bool {
	if cfg.file == "" {
		return false
	}
	// Preserve the final component too: a served symlink entry can be written
	// through even when its target is outside the served directory.
	files := [2]string{
		filepath.Join(realConfigPath(filepath.Dir(cfg.file)), filepath.Base(cfg.file)),
		realConfigPath(cfg.file),
	}
	contains := func(directory string) bool {
		directoryInfo, directoryErr := os.Stat(directory)
		directory = realConfigPath(directory)
		for _, file := range files {
			for name := filepath.Dir(file); ; name = filepath.Dir(name) {
				// Identity also covers aliases that EvalSymlinks cannot normalize,
				// such as case-insensitive directories on Linux or bind mounts.
				if directoryErr == nil {
					if info, err := os.Stat(name); err == nil && os.SameFile(info, directoryInfo) {
						return true
					}
				}
				if sameConfigPath(name, directory) {
					return true
				}
				if filepath.Dir(name) == name {
					break
				}
			}
		}
		return false
	}
	serves := func(p UserPermissions) bool {
		if p.useDirectories {
			for _, mount := range p.Directories {
				if contains(mount.Path) {
					return true
				}
			}
			return false
		}
		return contains(p.Directory)
	}
	if len(cfg.Users) == 0 {
		return serves(cfg.UserPermissions)
	}
	for _, user := range cfg.Users {
		if serves(user.UserPermissions) {
			return true
		}
	}
	return false
}

var configReloadDelay, configRearmInterval = 100 * time.Millisecond, time.Second

// configMaxRearms bounds re-arms over the lifetime of a watch: a Viper watcher
// re-armed by mistake while still alive can't be stopped. Past it, changes are
// still picked up by the periodic check.
const configMaxRearms = 10

// watchConfigArm starts Viper's watcher. Viper exits the process when it can't
// create a file system watcher, so that is checked first.
var watchConfigArm = func(v *viper.Viper) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		zap.L().Error("failed to watch config file, checking periodically", zap.Error(err))
		return
	}
	_ = w.Close()
	v.WatchConfig()
}

// configFingerprint stats an open file, so that on Windows the file identity
// is captured now rather than when it is first compared.
func configFingerprint(name string) (os.FileInfo, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.Stat()
}

// WatchConfig watches the configuration file with Viper and calls onChange with
// the new configuration, or an error to keep the previous one. Changes are
// debounced and a failed or empty read is retried once.
//
// Viper stops watching for good when the file is removed, which atomic saves
// do on Windows and macOS. The file is therefore checked periodically: when it
// changed without a Viper callback, it is reloaded, and if it was replaced by
// another file, Viper's watch is re-armed.
//
// Stop waits for a running onChange and must not be called from it. Viper's
// own watchers cannot be stopped and live until the process exits.
func WatchConfig(cfg *Config, flags *pflag.FlagSet, onChange func(*Config, error)) (stop func()) {
	if cfg.file == "" {
		return func() {}
	}
	if configFileIsServed(cfg) {
		zap.L().Warn("config file is inside a served directory, not watching it for changes")
		return func() {}
	}
	done, exited := make(chan struct{}), make(chan struct{})
	changes := make(chan os.FileInfo, 1)
	delay, interval := configReloadDelay, configRearmInterval
	var identity os.FileInfo
	arm := func() {
		v := viper.New()
		v.SetConfigFile(cfg.file)
		v.OnConfigChange(func(fsnotify.Event) {
			info, _ := configFingerprint(cfg.file)
			select {
			case changes <- info:
			case <-done:
			}
		})
		watchConfigArm(v)
	}
	// The baseline is the file as parsed, so a change made before the watch
	// started is still picked up.
	identity = cfg.fingerprint
	arm()
	go func() {
		defer close(exited)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var pending <-chan time.Time
		rearms, retry, changed, missing := 0, false, false, false
		replaced := func(info os.FileInfo) bool { return identity == nil || !os.SameFile(identity, info) }
		for {
			select {
			case <-done:
				return
			case info := <-changes:
				identity, changed, retry = info, false, false
				pending = time.After(delay)
			case <-ticker.C:
				info, err := configFingerprint(cfg.file)
				if err != nil {
					missing, changed = true, false
					continue
				}
				if identity != nil && os.SameFile(identity, info) &&
					identity.ModTime().Equal(info.ModTime()) && identity.Size() == info.Size() {
					changed = false
					continue
				}
				// Wait for a second check, so that a live watcher's callback
				// for the same change wins.
				if changed {
					if (missing || replaced(info)) && rearms < configMaxRearms {
						arm()
						rearms, missing = rearms+1, false
						if rearms == configMaxRearms {
							zap.L().Warn("config file changes are not reported by the file system watcher, checking periodically")
						}
					}
					identity, retry = info, false
					pending = time.After(delay)
				}
				changed = !changed
			case <-pending:
				pending = nil
				var next *Config
				content, err := os.ReadFile(cfg.file)
				if err == nil && strings.TrimSpace(string(content)) == "" {
					err = errors.New("config file is empty")
				}
				if err == nil {
					next, err = parseConfig(cfg.file, flags, content)
				}
				if err != nil && !retry {
					retry = true
					pending = time.After(delay)
					continue
				}
				if err == nil && configFileIsServed(next) {
					next, err = nil, errors.New("config file is inside a served directory")
				}
				onChange(next, err)
			}
		}
	}()
	return sync.OnceFunc(func() { close(done); <-exited })
}
