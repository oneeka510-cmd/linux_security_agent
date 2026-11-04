package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Report is everything the agent collects. It is what will be sent to the cloud.
type Report struct {
	Host     HostInfo      `json:"host"`
	Packages []Package     `json:"packages,omitempty"`
	Checks   []CheckResult `json:"checks,omitempty"`
}

// buildReport collects the parts selected by "what": all, packages or checks.
func buildReport(what string) (Report, error) {
	report := Report{Host: collectHostInfo()}

	if what == "all" || what == "packages" {
		packages, err := collectPackages()
		if err != nil {
			return report, err
		}
		report.Packages = packages
	}
	if what == "all" || what == "checks" {
		report.Checks = runChecks()
	}
	return report, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go-agent [all|packages|checks|send]")
	fmt.Fprintln(os.Stderr, "  all (default), packages, checks   print the report as JSON")
	fmt.Fprintln(os.Stderr, "  send                              collect everything and upload it to the cloud")
	os.Exit(2)
}

func main() {
	what := "all"
	if len(os.Args) > 1 {
		what = os.Args[1]
	}

	switch what {
	case "all", "packages", "checks":
		report, err := buildReport(what)
		if err != nil {
			fail(err)
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fail(err)
		}
		fmt.Println(string(data))

	case "send":
		// Check the settings first so a missing API key fails fast, before the slow scan.
		cfg, err := loadConfig()
		if err != nil {
			fail(err)
		}
		report, err := buildReport("all")
		if err != nil {
			fail(err)
		}
		reply, err := sendReport(cfg, report)
		if err != nil {
			fail(err)
		}
		fmt.Printf("report sent to %s: %s\n", cfg.APIURL, reply)

	default:
		usage()
	}
}
