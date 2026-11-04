package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// Package is one installed program on the machine.
type Package struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
}

// collectPackages runs dpkg-query and turns each output line into a Package.
func collectPackages() ([]Package, error) {
	// Each output line looks like: "ii \tadduser\t3.137ubuntu1\tall"
	format := "${db:Status-Abbrev}\t${Package}\t${Version}\t${Architecture}\n"
	out, err := exec.Command("dpkg-query", "-W", "-f="+format).Output()
	if err != nil {
		return nil, fmt.Errorf("running dpkg-query: %w", err)
	}

	var packages []Package
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue // blank line at the end, or something unexpected
		}
		// "ii" means installed. Other states (e.g. removed, config files left) are skipped.
		if !strings.HasPrefix(fields[0], "ii") {
			continue
		}
		packages = append(packages, Package{
			Name:         fields[1],
			Version:      fields[2],
			Architecture: fields[3],
		})
	}
	return packages, nil
}
