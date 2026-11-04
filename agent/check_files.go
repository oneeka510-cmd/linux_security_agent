package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const worldWritableTimeout = 2 * time.Minute

// World-writable files: CIS 7.1, "System File Permissions"
func checkWorldWritableFiles() CheckResult {
	result := CheckResult{ID: "no-world-writable-files", Title: "No world-writable files exist", CISSection: "7.1"}

	ctx, cancel := context.WithTimeout(context.Background(), worldWritableTimeout)
	defer cancel()

	// -xdev stays on the root filesystem; -perm -0002 matches files others can write.
	out, err := exec.CommandContext(ctx, "find", "/", "-xdev", "-type", "f", "-perm", "-0002").Output()

	var execErr *exec.Error
	switch {
	case ctx.Err() != nil:
		result.Status = StatusError
		result.Evidence = fmt.Sprintf("search took longer than %s", worldWritableTimeout)
		return result
	case errors.As(err, &execErr):
		result.Status = StatusError
		result.Evidence = err.Error()
		return result
	}
	// find exits non-zero for unreadable directories but still prints what it found.

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}

	if len(files) == 0 {
		result.Status = StatusPass
		result.Evidence = "no world-writable files found (find / -xdev -type f -perm -0002)"
		return result
	}

	result.Status = StatusFail
	examples := files
	if len(examples) > 5 {
		examples = examples[:5]
	}
	result.Evidence = fmt.Sprintf("%d world-writable file(s), e.g. %s", len(files), strings.Join(examples, ", "))
	return result
}
