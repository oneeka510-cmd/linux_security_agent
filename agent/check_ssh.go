package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const cisSSH = "5.1" // CIS Ubuntu 22.04 v3.0.0, "Configure SSH Server"

var errSSHNotInstalled = errors.New("sshd (SSH server) is not installed")

// sshdSettings returns the settings the SSH server is really using.
// "sshd -T" prints them all, one "name value" per line, with defaults filled in.
func sshdSettings() (map[string]string, error) {
	path, err := exec.LookPath("sshd")
	if err != nil {
		// sshd lives in /usr/sbin, which is not always in a normal user's PATH.
		if _, statErr := os.Stat("/usr/sbin/sshd"); statErr != nil {
			return nil, errSSHNotInstalled
		}
		path = "/usr/sbin/sshd"
	}

	out, err := exec.Command(path, "-T").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running sshd -T failed (it needs root and the sshd runtime directory): %v: %s", err, strings.TrimSpace(string(out)))
	}

	settings := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		name, value, found := strings.Cut(line, " ")
		if found {
			settings[name] = strings.TrimSpace(value)
		}
	}
	return settings, nil
}

// checkSSHSetting is shared by the SSH checks: the setting must equal "want".
func checkSSHSetting(id, title, setting, want string) CheckResult {
	result := CheckResult{ID: id, Title: title, CISSection: cisSSH}

	settings, err := sshdSettings()
	switch {
	case errors.Is(err, errSSHNotInstalled):
		result.Status = StatusNotApplicable
		result.Evidence = err.Error()
		return result
	case err != nil:
		result.Status = StatusError
		result.Evidence = err.Error()
		return result
	}

	got := settings[setting]
	result.Evidence = fmt.Sprintf("sshd effective config: %s %s", setting, got)
	if got == want {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
		result.Evidence += fmt.Sprintf(" (CIS requires %s)", want)
	}
	return result
}

func checkSSHRootLogin() CheckResult {
	return checkSSHSetting("ssh-root-login", "SSH root login is disabled", "permitrootlogin", "no")
}

func checkSSHEmptyPasswords() CheckResult {
	return checkSSHSetting("ssh-empty-passwords", "SSH does not allow empty passwords", "permitemptypasswords", "no")
}
