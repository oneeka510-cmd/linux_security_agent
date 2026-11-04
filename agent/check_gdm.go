package main

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

const gdmDir = "/etc/gdm3"

// GDM settings files, in the order GDM reads them.
var gdmConfigFiles = []string{"/etc/gdm3/custom.conf", "/etc/gdm3/daemon.conf"}

// autoLoginEnabled reports whether a GDM config file turns auto-login on.
// The setting is "AutomaticLoginEnable=true" inside the [daemon] section.
func autoLoginEnabled(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	inDaemon := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";"):
			continue // blank line or comment
		case strings.HasPrefix(line, "["):
			inDaemon = strings.EqualFold(line, "[daemon]")
		case inDaemon:
			key, value, found := strings.Cut(line, "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "AutomaticLoginEnable") &&
				strings.EqualFold(strings.TrimSpace(value), "true") {
				return true, nil
			}
		}
	}
	return false, nil
}

// GDM auto-login: CIS 1.7, "Configure GNOME Display Manager"
func checkGDMAutoLogin() CheckResult {
	result := CheckResult{ID: "gdm-autologin", Title: "GDM automatic login is disabled", CISSection: "1.7"}

	if _, err := os.Stat(gdmDir); errors.Is(err, fs.ErrNotExist) {
		result.Status = StatusNotApplicable
		result.Evidence = "GDM is not installed (" + gdmDir + " does not exist)"
		return result
	}

	var checked []string
	for _, path := range gdmConfigFiles {
		enabled, err := autoLoginEnabled(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			result.Status = StatusError
			result.Evidence = err.Error()
			return result
		case enabled:
			result.Status = StatusFail
			result.Evidence = "AutomaticLoginEnable=true found in " + path
			return result
		}
		checked = append(checked, path)
	}

	result.Status = StatusPass
	if len(checked) == 0 {
		result.Evidence = "no GDM config file sets AutomaticLoginEnable (off by default)"
	} else {
		result.Evidence = "AutomaticLoginEnable is not set to true in " + strings.Join(checked, ", ")
	}
	return result
}
