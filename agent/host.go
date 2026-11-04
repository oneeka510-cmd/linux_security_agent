package main

import (
	"os"
	"runtime"
	"strings"
	"time"
)

// agentVersion is a variable (not a constant) so the package build can set it:
// go build -ldflags "-X main.agentVersion=1.2.3"
var agentVersion = "0.1.0"

// HostInfo describes the machine the agent is running on.
type HostInfo struct {
	MachineID    string `json:"machine_id"` // unique per machine, used as the database key
	Hostname     string `json:"hostname"`
	OSName       string `json:"os_name"`
	OSVersion    string `json:"os_version"`
	OSPrettyName string `json:"os_pretty_name"`
	Kernel       string `json:"kernel"`
	Architecture string `json:"architecture"`
	AgentVersion string `json:"agent_version"`
	CollectedAt  string `json:"collected_at"` // UTC, RFC 3339
}

// readFileTrimmed returns a file's contents without surrounding whitespace, or "" on error.
func readFileTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readOSRelease parses /etc/os-release, whose lines look like KEY="value".
func readOSRelease() map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(readFileTrimmed("/etc/os-release"), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			values[key] = strings.Trim(value, `"`)
		}
	}
	return values
}

// collectHostInfo gathers the details shown in the "hosts" view.
func collectHostInfo() HostInfo {
	hostname, _ := os.Hostname()
	osRelease := readOSRelease()

	return HostInfo{
		MachineID:    readFileTrimmed("/etc/machine-id"),
		Hostname:     hostname,
		OSName:       osRelease["NAME"],
		OSVersion:    osRelease["VERSION_ID"],
		OSPrettyName: osRelease["PRETTY_NAME"],
		Kernel:       readFileTrimmed("/proc/sys/kernel/osrelease"),
		Architecture: runtime.GOARCH,
		AgentVersion: agentVersion,
		CollectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}
