package main

import (
	"os"
	"os/exec"
	"strings"
)

const cisFilesystems = "1.1.1" // CIS Ubuntu 22.04 v3.0.0, "Configure Filesystem Kernel Modules"

// Filesystems CIS wants disabled when they are not needed.
var unusedFilesystems = []string{"cramfs", "squashfs"}

// moduleLoaded reports whether a kernel module is loaded right now.
func moduleLoaded(name string) bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, name+" ") {
			return true
		}
	}
	return false
}

// moduleDisabled says whether a kernel module is disabled, with a short explanation.
func moduleDisabled(modprobe, name string) (bool, string) {
	if moduleLoaded(name) {
		return false, name + ": module is loaded right now"
	}

	// -n = dry run, -v = print what would happen
	out, _ := exec.Command(modprobe, "-n", "-v", name).CombinedOutput()
	text := strings.TrimSpace(string(out))

	switch {
	case strings.Contains(text, "install /bin/false"), strings.Contains(text, "install /bin/true"):
		return true, name + ": loading is blocked (" + text + ")"
	case strings.Contains(text, "not found"):
		return true, name + ": module not available in this kernel"
	case text == "":
		// modprobe prints nothing for a module that is built into the kernel
		return false, name + ": built into the kernel, cannot be blocked with modprobe"
	default:
		return false, name + ": can be loaded (" + text + ")"
	}
}

func checkUnusedFilesystems() CheckResult {
	result := CheckResult{
		ID:         "unused-filesystems",
		Title:      "Unused filesystems (cramfs, squashfs) are disabled",
		CISSection: cisFilesystems,
	}

	modprobe, err := findTool("modprobe")
	if err != nil {
		result.Status = StatusError
		result.Evidence = err.Error()
		return result
	}

	result.Status = StatusPass
	var evidence []string
	for _, name := range unusedFilesystems {
		disabled, note := moduleDisabled(modprobe, name)
		evidence = append(evidence, note)
		if !disabled {
			result.Status = StatusFail
		}
	}
	result.Evidence = strings.Join(evidence, "; ")
	return result
}
