package commands

import (
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/pidfile"
	"github.com/svngoku/hermes-cli/internal/ui"
)

// Stop terminates a recorded daemon by sending SIGTERM to its process group.
func Stop(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	port := fs.Int("port", 0, "Port of the daemon to stop (required)")
	all := fs.Bool("all", false, "Stop all recorded daemons")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes stop [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Stop a background engine daemon")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*all && *port == 0 {
		return fmt.Errorf("--port is required (or use --all)")
	}

	records, err := pidfile.List()
	if err != nil {
		return fmt.Errorf("failed to list daemons: %w", err)
	}

	targets := records
	if !*all {
		targets = nil
		for _, r := range records {
			if r.Port == *port {
				targets = append(targets, r)
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no daemon recorded on port %d", *port)
		}
	}

	stopped := 0
	for _, r := range targets {
		if !pidfile.Alive(r.PID) {
			_ = pidfile.Remove(r.Port)
			fmt.Fprintln(ctx.Stdout, ui.Warn(fmt.Sprintf("port %d: pid %d not running (record removed)", r.Port, r.PID)))
			continue
		}
		if err := syscall.Kill(-r.PID, syscall.SIGTERM); err != nil {
			_ = r.PID
			if err := syscall.Kill(r.PID, syscall.SIGTERM); err != nil {
				fmt.Fprintln(ctx.Stdout, ui.Fail(fmt.Sprintf("port %d: failed to signal pid %d: %v", r.Port, r.PID, err)))
				continue
			}
		}
		_ = pidfile.Remove(r.Port)
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("port %d: stopped pid %d (%s %s)", r.Port, r.PID, r.Engine, r.Model)))
		stopped++
	}

	if stopped == 0 {
		fmt.Fprintln(ctx.Stdout, ui.Info("No running daemons stopped"))
	}
	return nil
}

type statusRow struct {
	port    int
	engine  string
	model   string
	pid     int
	alive   bool
	healthy bool
	logFile string
}

// Status reports on recorded daemons: which are alive and which are responding.
func Status(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes status [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "List recorded engine daemons")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	records, err := pidfile.List()
	if err != nil {
		return fmt.Errorf("failed to list daemons: %w", err)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	rows := make([]statusRow, 0, len(records))
	pruned := 0
	for _, r := range records {
		if !pidfile.Alive(r.PID) {
			_ = pidfile.Remove(r.Port)
			pruned++
			continue
		}
		row := statusRow{
			port:    r.Port,
			engine:  r.Engine,
			model:   r.Model,
			pid:     r.PID,
			alive:   true,
			logFile: r.LogFile,
		}
		resp, err := client.Get(fmt.Sprintf("http://%s:%d/health", r.Host, r.Port))
		if err == nil {
			row.healthy = resp.StatusCode == http.StatusOK
			resp.Body.Close()
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].port < rows[j].port })

	if *jsonOutput {
		fmt.Fprintln(ctx.Stdout, formatStatusJSON(rows, pruned))
		return nil
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("Engine daemons"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	if len(rows) == 0 {
		fmt.Fprintln(ctx.Stdout, ui.Info("No running daemons"))
		return nil
	}

	for _, row := range rows {
		health := "down"
		if row.healthy {
			health = "healthy"
		}
		fmt.Fprintf(ctx.Stdout, "  port %-6d pid %-7d %-8s %-10s %s\n",
			row.port, row.pid, row.engine, health, truncate(row.model, 40))
		if row.logFile != "" {
			fmt.Fprintln(ctx.Stdout, "       logs: "+row.logFile)
		}
	}
	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintf(ctx.Stdout, "%d daemon(s) running\n", len(rows))
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func formatStatusJSON(rows []statusRow, pruned int) string {
	var b strings.Builder
	b.WriteString("[")
	for i, row := range rows {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"port":%d,"pid":%d,"engine":%q,"model":%q,"alive":true,"healthy":%t}`,
			row.port, row.pid, row.engine, row.model, row.healthy)
	}
	b.WriteString("]")
	if pruned > 0 {
		fmt.Fprintf(&b, "\n// pruned %d stale record(s)", pruned)
	}
	return b.String()
}
