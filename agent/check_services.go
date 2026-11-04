package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// systemdRunning reports whether this machine is managed by systemd.
func systemdRunning() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

// unitState asks systemd about a service. what is "is-active" or "is-enabled".
// systemctl exits non-zero for "inactive", so only the printed word is used.
func unitState(what, unit string) string {
	out, _ := exec.Command("systemctl", what, unit).Output()
	state := strings.TrimSpace(string(out))
	if state == "" {
		return "not-found"
	}
	return state
}

// systemdMissing fills in an ERROR result when systemd is not running.
func systemdMissing(result CheckResult) (CheckResult, bool) {
	if systemdRunning() {
		return result, false
	}
	result.Status = StatusError
	result.Evidence = "systemd is not running, so service state cannot be read"
	return result, true
}

// Firewall: CIS 4.1, "Configure UncomplicatedFirewall"
func checkFirewall() CheckResult {
	result := CheckResult{ID: "firewall-enabled", Title: "Firewall (ufw) is enabled", CISSection: "4.1"}

	ufw, err := findTool("ufw")
	if err != nil {
		result.Status = StatusFail
		result.Evidence = "ufw is not installed, so no firewall was found"
		return result
	}

	out, err := exec.Command(ufw, "status").CombinedOutput()
	if err != nil {
		result.Status = StatusError
		result.Evidence = fmt.Sprintf("ufw status failed (needs root): %v: %s", err, strings.TrimSpace(string(out)))
		return result
	}

	firstLine, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	result.Evidence = "ufw " + firstLine
	if strings.Contains(firstLine, "Status: active") {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}
	return result
}

// auditd: CIS 6.2.1, "Configure auditd Service"
func checkAuditd() CheckResult {
	result := CheckResult{ID: "auditd-running", Title: "auditd service is enabled and running", CISSection: "6.2.1"}
	if r, missing := systemdMissing(result); missing {
		return r
	}

	active := unitState("is-active", "auditd")
	enabled := unitState("is-enabled", "auditd")
	result.Evidence = fmt.Sprintf("auditd: active=%s, enabled=%s", active, enabled)
	if active == "active" && enabled == "enabled" {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}
	return result
}

// Time synchronization: CIS 2.3, chrony or systemd-timesyncd
func checkTimeSync() CheckResult {
	result := CheckResult{ID: "time-sync", Title: "Time synchronization is running", CISSection: "2.3"}
	if r, missing := systemdMissing(result); missing {
		return r
	}

	chrony := unitState("is-active", "chrony")
	timesyncd := unitState("is-active", "systemd-timesyncd")
	result.Evidence = fmt.Sprintf("chrony=%s, systemd-timesyncd=%s", chrony, timesyncd)
	if chrony == "active" || timesyncd == "active" {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}
	return result
}

// AppArmor: CIS 1.3.1, "Configure AppArmor"
func checkAppArmor() CheckResult {
	result := CheckResult{ID: "apparmor-enabled", Title: "AppArmor is enabled", CISSection: "1.3.1"}

	data, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
	if err != nil {
		result.Status = StatusFail
		result.Evidence = "AppArmor is not present in the running kernel"
		return result
	}

	value := strings.TrimSpace(string(data))
	result.Evidence = "/sys/module/apparmor/parameters/enabled = " + value
	if value == "Y" {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}
	return result
}
