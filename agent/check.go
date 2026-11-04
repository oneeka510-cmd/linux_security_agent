package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Status is the outcome of one security check.
type Status string

const (
	StatusPass          Status = "PASS"
	StatusFail          Status = "FAIL"
	StatusNotApplicable Status = "NOT_APPLICABLE" // the thing being checked is not installed
	StatusError         Status = "ERROR"          // the agent could not find out
)

// CheckResult is what every check returns.
type CheckResult struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	CISSection string `json:"cis_section"` // CIS Ubuntu 22.04 v3.0.0 section
	Status     Status `json:"status"`
	Evidence   string `json:"evidence"`
}

// allChecks lists every check the agent runs, in order.
func allChecks() []func() CheckResult {
	return []func() CheckResult{
		checkSSHRootLogin,
		checkSSHEmptyPasswords,
		checkUnusedFilesystems,
		checkFirewall,
		checkAuditd,
		checkTimeSync,
		checkAppArmor,
		checkGDMAutoLogin,
		checkWorldWritableFiles,
		checkPasswordComplexity,
		checkPasswordExpiration,
	}
}

// findTool returns the path of a program. Admin tools live in /usr/sbin or /sbin,
// which are not always in a normal user's PATH.
func findTool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, dir := range []string{"/usr/sbin", "/sbin"} {
		path := dir + "/" + name
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found", name)
}

// runChecks runs every check and collects the results.
func runChecks() []CheckResult {
	var results []CheckResult
	for _, check := range allChecks() {
		results = append(results, check())
	}
	return results
}
