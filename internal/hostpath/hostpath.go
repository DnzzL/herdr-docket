// Package hostpath answers "where is it" for everything the plugin needs to
// find: the herdr binary, the config directory, the state directory, the
// plugin checkout.
//
// Every one of those follows the same shape: Herdr's environment variable
// wins, then we ask the herdr CLI, then we fall back to Herdr's own layout.
// Keeping them in one package is the point — answered ad hoc, two callers
// eventually disagree about where the config lives, and a daemon reading the
// wrong directory looks exactly like an empty fleet.
package hostpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PluginID must match the id in herdr-plugin.toml: it locates the same
// directories Herdr hands the daemon when the CLI is run from a plain shell.
const PluginID = "dnzzl.herdr-docket"

// Bin locates the herdr CLI. HERDR_BIN_PATH is herdr telling a plugin exactly
// which binary to call back into, so it wins — but only if it still exists. A
// long-running herdr whose binary was moved or uninstalled under it (a package
// manager switch, say) keeps advertising the old path, and taking it on faith
// made every call fail with a fork/exec error the user could do nothing about.
func Bin() string {
	if b := os.Getenv("HERDR_BIN_PATH"); b != "" && usable(b) {
		return b
	}
	return "herdr"
}

// usable reports whether path is something we could actually execute.
func usable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// ConfigDir resolves the config directory. Under Herdr the env var is
// authoritative; from a plain shell we ask the herdr CLI, so
// `herdr-docket list` in a terminal always sees what the daemon sees.
func ConfigDir() string {
	if d := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); d != "" {
		return d
	}
	if out, err := exec.Command(Bin(), "plugin", "config-dir", PluginID).Output(); err == nil {
		if d := strings.TrimSpace(string(out)); d != "" {
			return d
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "herdr", "plugins", "config", PluginID)
}

// StateDir resolves where run history and scheduler state live. Herdr exposes
// no state-dir command, so outside a plugin context we mirror its layout.
func StateDir() string {
	if d := os.Getenv("HERDR_PLUGIN_STATE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "herdr", "plugins", PluginID)
}

// Root locates the plugin checkout: Herdr sets it, otherwise derive it from
// the binary's location (bin/herdr-docket lives inside the checkout).
func Root() (string, error) {
	if r := os.Getenv("HERDR_PLUGIN_ROOT"); r != "" {
		return r, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}
