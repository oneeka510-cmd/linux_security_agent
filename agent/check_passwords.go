package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	loginDefsPath    = "/etc/login.defs"
	pwqualityPath    = "/etc/security/pwquality.conf"
	pwqualityDropins = "/etc/security/pwquality.conf.d/*.conf"
)

// readSettings reads a "name value" or "name = value" file into a map.
// Comments and blank lines are skipped; if a name repeats, the last one wins.
func readSettings(path string, settings map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(strings.Replace(line, "=", " ", 1))
		if len(fields) >= 2 {
			settings[fields[0]] = fields[1]
		}
	}
	return nil
}

// Password expiration: CIS 5.4.1.1, "Ensure password expiration is configured"
func checkPasswordExpiration() CheckResult {
	result := CheckResult{ID: "password-expiration", Title: "Passwords expire within 365 days", CISSection: "5.4.1.1"}

	settings := make(map[string]string)
	if err := readSettings(loginDefsPath, settings); err != nil {
		result.Status = StatusError
		result.Evidence = err.Error()
		return result
	}

	value, ok := settings["PASS_MAX_DAYS"]
	if !ok {
		result.Status = StatusFail
		result.Evidence = "PASS_MAX_DAYS is not set in " + loginDefsPath
		return result
	}

	days, err := strconv.Atoi(value)
	result.Evidence = fmt.Sprintf("PASS_MAX_DAYS = %s in %s (CIS requires 1-365)", value, loginDefsPath)
	if err == nil && days >= 1 && days <= 365 {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}
	return result
}

// Password complexity: CIS 5.3.3.2, "Configure pam_pwquality module"
//
// MVP rule: minlen >= 14, and either minclass >= 3 or all four *credit settings <= -1.
// This is a simplified reading of the CIS controls and does not check PAM wiring.
func checkPasswordComplexity() CheckResult {
	result := CheckResult{ID: "password-complexity", Title: "Password complexity is enforced", CISSection: "5.3.3.2"}

	settings := make(map[string]string)
	err := readSettings(pwqualityPath, settings)
	if errors.Is(err, fs.ErrNotExist) {
		result.Status = StatusFail
		result.Evidence = pwqualityPath + " does not exist, so no password quality rules are configured"
		return result
	}
	if err != nil {
		result.Status = StatusError
		result.Evidence = err.Error()
		return result
	}
	dropins, _ := filepath.Glob(pwqualityDropins)
	for _, path := range dropins {
		_ = readSettings(path, settings) // later files override earlier ones
	}

	number := func(name string) (int, bool) {
		n, err := strconv.Atoi(settings[name])
		return n, err == nil
	}

	minlen, hasMinlen := number("minlen")
	minclass, hasMinclass := number("minclass")
	creditsOK := true
	for _, name := range []string{"dcredit", "ucredit", "lcredit", "ocredit"} {
		if n, ok := number(name); !ok || n > -1 {
			creditsOK = false
		}
	}

	result.Evidence = fmt.Sprintf("minlen=%q minclass=%q dcredit=%q ucredit=%q lcredit=%q ocredit=%q",
		settings["minlen"], settings["minclass"], settings["dcredit"], settings["ucredit"], settings["lcredit"], settings["ocredit"])

	lengthOK := hasMinlen && minlen >= 14
	classOK := (hasMinclass && minclass >= 3) || creditsOK
	if lengthOK && classOK {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
		result.Evidence += " (CIS wants minlen >= 14 and minclass >= 3, or all credits <= -1)"
	}
	return result
}
