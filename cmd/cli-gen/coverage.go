package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/sivchari/kumo/cli"
	"github.com/sivchari/kumo/internal/cligen"
	"github.com/sivchari/kumo/internal/service"
)

// restCLIAliases maps a kumo service name to additional cli.NewRootCmd()
// top-level command names (beyond the service name itself) that also count
// toward that service's hand-written CLI coverage. Needed for hand-written
// services split across more than one top-level cobra command; currently
// only s3, which registers both "s3" (high-level, e.g. `mb`) and "s3api"
// (API-level, e.g. `put-object`). Add an entry here if another hand-written
// service is split the same way.
var restCLIAliases = map[string][]string{
	"s3": {"s3api"},
}

// skipReasonBuckets classifies a cli-gen skip Diagnostic.Reason into the
// small, stable set of causes cligen actually produces (see
// internal/cligen/discover.go and flags.go), so -coverage can summarize N
// skipped actions instead of printing N distinct per-action reasons (most
// reasons embed the specific field or action name).
var skipReasonBuckets = []struct {
	substr string
	label  string
}{
	{substr: "no matching aws-sdk-go-v2 client method", label: "no matching SDK method"},
	{substr: "manually overridden in cli/", label: "manual override file"},
	{substr: "needs a hand-written override", label: "unsupported field type"},
}

func classifySkipReason(reason string) string {
	for _, b := range skipReasonBuckets {
		if strings.Contains(reason, b.substr) {
			return b.label
		}
	}

	return "other"
}

// serviceCoverage is one row of the -coverage table: how kumo's CLI covers a
// single registered service.
type serviceCoverage struct {
	name      string
	category  string // "auto-generated", "manual", or "uncovered"
	commands  int    // commands reachable from the kumo CLI for this service
	generated int
	skipped   int
	note      string
}

// runCoverage prints a per-service CLI coverage table to w: for every
// service.Services() entry, whether its CLI commands come from cli-gen
// auto-discovery (internal/cligen), a hand-written cli/*.go file, or
// neither, followed by service and command totals.
func runCoverage(w io.Writer, services []service.Service) error {
	_, diagnostics, err := cligen.Render(services)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	autoGen, restReason := groupDiagnostics(diagnostics)
	cmdCounts := cliTopLevelCommandCounts()

	rows := make([]serviceCoverage, 0, len(services))

	for _, svc := range services {
		name := svc.Name()

		if sc, ok := autoGen[name]; ok {
			rows = append(rows, *sc)

			continue
		}

		if n, ok := manualCommandCount(name, cmdCounts); ok {
			rows = append(rows, serviceCoverage{name: name, category: "manual", commands: n})

			continue
		}

		reason := restReason[name]
		if reason == "" {
			reason = "no CLI coverage"
		}

		rows = append(rows, serviceCoverage{name: name, category: "uncovered", note: reason})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	return printCoverageTable(w, rows)
}

// groupDiagnostics splits a cligen.Render diagnostics list into per-service
// auto-generation coverage (services with at least one per-action
// diagnostic) and service-level skip reasons (REST-only services, or
// Query/JSON services with no sdkBindings entry).
func groupDiagnostics(diagnostics []cligen.Diagnostic) (autoGen map[string]*serviceCoverage, restReason map[string]string) {
	autoGen = make(map[string]*serviceCoverage)
	restReason = make(map[string]string)
	skipReasons := make(map[string]map[string]int)

	for _, d := range diagnostics {
		if d.Action == "" {
			restReason[d.Service] = d.Reason

			continue
		}

		sc, ok := autoGen[d.Service]
		if !ok {
			sc = &serviceCoverage{name: d.Service, category: "auto-generated"}
			autoGen[d.Service] = sc
			skipReasons[d.Service] = make(map[string]int)
		}

		if d.Level == cligen.LevelGenerated {
			sc.generated++
		} else {
			sc.skipped++
			skipReasons[d.Service][classifySkipReason(d.Reason)]++
		}
	}

	for name, sc := range autoGen {
		sc.commands = sc.generated
		sc.note = summarizeSkipReasons(skipReasons[name])
	}

	return autoGen, restReason
}

// summarizeSkipReasons renders a "label (n), label2 (n2)" summary, sorted
// for deterministic output.
func summarizeSkipReasons(reasons map[string]int) string {
	if len(reasons) == 0 {
		return ""
	}

	labels := make([]string, 0, len(reasons))
	for label := range reasons {
		labels = append(labels, label)
	}

	sort.Strings(labels)

	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, fmt.Sprintf("%s (%d)", label, reasons[label]))
	}

	return strings.Join(parts, ", ")
}

// cliTopLevelCommandCounts builds the fully-assembled kumo CLI command tree
// (both cli-gen-generated and hand-written commands) and returns, per
// top-level command name (e.g. "s3", "s3api", "amplify"), the number of
// leaf commands (actual invokable actions) registered under it.
func cliTopLevelCommandCounts() map[string]int {
	root := cli.NewRootCmd()
	counts := make(map[string]int, len(root.Commands()))

	for _, top := range root.Commands() {
		counts[top.Name()] = countLeafCommands(top)
	}

	return counts
}

// countLeafCommands returns the number of leaf commands (no children, i.e.
// an invokable action rather than a grouping command) reachable under cmd,
// including cmd itself if it has no children.
func countLeafCommands(cmd *cobra.Command) int {
	children := cmd.Commands()
	if len(children) == 0 {
		return 1
	}

	var n int

	for _, c := range children {
		n += countLeafCommands(c)
	}

	return n
}

// manualCommandCount reports the hand-written CLI command count for
// serviceName (its own top-level command plus any restCLIAliases), and
// whether serviceName has any hand-written CLI coverage at all.
func manualCommandCount(serviceName string, cmdCounts map[string]int) (count int, found bool) {
	if n, ok := cmdCounts[serviceName]; ok {
		count += n
		found = true
	}

	for _, alias := range restCLIAliases[serviceName] {
		if n, ok := cmdCounts[alias]; ok {
			count += n
			found = true
		}
	}

	return count, found
}

// printCoverageTable writes rows as an aligned table to w, followed by
// service and command totals.
func printCoverageTable(w io.Writer, rows []serviceCoverage) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, "SERVICE\tCATEGORY\tCOMMANDS\tGENERATED\tSKIPPED\tNOTES"); err != nil {
		return fmt.Errorf("write coverage table header: %w", err)
	}

	var coveredServices, totalCommands int

	for _, r := range rows {
		if r.commands > 0 {
			coveredServices++
			totalCommands += r.commands
		}

		generated, skipped := "-", "-"
		if r.category == "auto-generated" {
			generated = fmt.Sprintf("%d", r.generated)
			skipped = fmt.Sprintf("%d", r.skipped)
		}

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n", r.name, r.category, r.commands, generated, skipped, r.note); err != nil {
			return fmt.Errorf("write coverage table row for %s: %w", r.name, err)
		}
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush coverage table: %w", err)
	}

	if _, err := fmt.Fprintf(w, "\ntotal: %d/%d service(s) covered, %d command(s)\n", coveredServices, len(rows), totalCommands); err != nil {
		return fmt.Errorf("write coverage table totals: %w", err)
	}

	return nil
}
