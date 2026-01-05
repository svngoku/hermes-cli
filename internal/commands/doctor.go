package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/execx"
	"github.com/svngoku/hermes-cli/internal/ui"
)

type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusWarning CheckStatus = "warning"
	StatusFail    CheckStatus = "fail"
	StatusSkipped CheckStatus = "skipped"
)

type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message,omitempty"`
	Details string      `json:"details,omitempty"`
}

type DoctorReport struct {
	Checks   []CheckResult `json:"checks"`
	Summary  string        `json:"summary"`
	ExitCode int           `json:"exit_code"`
}

func Doctor(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	strict := fs.Bool("strict", false, "Fail if any check is missing")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes doctor [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Check GPU, CUDA, and system requirements")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	report := DoctorReport{
		Checks: make([]CheckResult, 0),
	}

	if !*jsonOutput {
		fmt.Fprintln(ctx.Stdout, ui.Banner())
		fmt.Fprintln(ctx.Stdout, ui.Step("Running doctor checks..."))
		fmt.Fprintln(ctx.Stdout, ui.HR())
	}

	report.Checks = append(report.Checks, checkNvidiaSMI(ctx, *jsonOutput))
	report.Checks = append(report.Checks, checkCUDA(ctx, *jsonOutput))
	report.Checks = append(report.Checks, checkGPUs(ctx, *jsonOutput))
	report.Checks = append(report.Checks, checkUV(ctx, *jsonOutput))
	report.Checks = append(report.Checks, checkPython(ctx, *jsonOutput))

	hasOK := false
	hasWarn := false
	hasFail := false
	for _, c := range report.Checks {
		switch c.Status {
		case StatusOK:
			hasOK = true
		case StatusWarning:
			hasWarn = true
		case StatusFail:
			hasFail = true
		}
	}

	if hasFail {
		report.ExitCode = 3
		report.Summary = "Some checks failed"
	} else if hasWarn {
		if *strict {
			report.ExitCode = 2
			report.Summary = "Some checks have warnings (strict mode)"
		} else {
			report.ExitCode = 0
			report.Summary = "All required checks passed (with warnings)"
		}
	} else if hasOK {
		report.ExitCode = 0
		report.Summary = "All checks passed"
	} else {
		report.ExitCode = 3
		report.Summary = "No checks ran"
	}

	if *jsonOutput {
		enc := json.NewEncoder(ctx.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Fprintln(ctx.Stdout, ui.HR())
	if report.ExitCode == 0 {
		fmt.Fprintln(ctx.Stdout, ui.Ok(report.Summary))
	} else if report.ExitCode == 2 {
		fmt.Fprintln(ctx.Stdout, ui.Warn(report.Summary))
	} else {
		fmt.Fprintln(ctx.Stdout, ui.Fail(report.Summary))
	}

	os.Exit(report.ExitCode)
	return nil
}

func checkNvidiaSMI(ctx *app.AppContext, jsonOut bool) CheckResult {
	check := CheckResult{Name: "nvidia-smi"}

	if !execx.CommandExists("nvidia-smi") {
		check.Status = StatusFail
		check.Message = "nvidia-smi not found"
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail("nvidia-smi not found; GPU not visible"))
		}
		return check
	}

	result := execx.Run(ctx.Ctx, "nvidia-smi", "--query-gpu=name,memory.total,driver_version", "--format=csv,noheader")
	if result.ExitCode != 0 {
		check.Status = StatusFail
		check.Message = "nvidia-smi failed"
		check.Details = result.Stderr
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail("nvidia-smi failed: "+result.Stderr))
		}
		return check
	}

	check.Status = StatusOK
	check.Message = "GPU detected"
	check.Details = result.Stdout
	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Ok("nvidia-smi: GPU detected"))
		for _, line := range strings.Split(result.Stdout, "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Fprintln(ctx.Stdout, "    "+line)
			}
		}
	}
	return check
}

func checkCUDA(ctx *app.AppContext, jsonOut bool) CheckResult {
	check := CheckResult{Name: "cuda"}

	if !execx.CommandExists("nvcc") {
		check.Status = StatusWarning
		check.Message = "nvcc not found (runtime-only image is fine)"
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Warn("nvcc not found (runtime-only image is fine)"))
		}
		return check
	}

	result := execx.Run(ctx.Ctx, "nvcc", "--version")
	if result.ExitCode != 0 {
		check.Status = StatusWarning
		check.Message = "nvcc failed"
		check.Details = result.Stderr
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Warn("nvcc failed: "+result.Stderr))
		}
		return check
	}

	versionRe := regexp.MustCompile(`release (\d+\.\d+)`)
	matches := versionRe.FindStringSubmatch(result.Stdout)
	version := "unknown"
	if len(matches) > 1 {
		version = matches[1]
	}

	check.Status = StatusOK
	check.Message = "CUDA " + version
	check.Details = result.Stdout
	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Ok("CUDA toolchain: "+version))
	}
	return check
}

func checkGPUs(ctx *app.AppContext, jsonOut bool) CheckResult {
	check := CheckResult{Name: "gpu_count"}

	result := execx.Run(ctx.Ctx, "nvidia-smi", "--query-gpu=count", "--format=csv,noheader")
	if result.ExitCode != 0 {
		check.Status = StatusSkipped
		check.Message = "Could not query GPU count"
		return check
	}

	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	count := len(lines)
	if count > 0 && lines[0] != "" {
		check.Status = StatusOK
		check.Message = fmt.Sprintf("%d GPU(s) available", count)
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("%d GPU(s) available", count)))
		}
	} else {
		check.Status = StatusFail
		check.Message = "No GPUs detected"
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail("No GPUs detected"))
		}
	}
	return check
}

func checkUV(ctx *app.AppContext, jsonOut bool) CheckResult {
	check := CheckResult{Name: "uv"}

	if !execx.CommandExists("uv") {
		check.Status = StatusWarning
		check.Message = "uv not found (will install during hermes install)"
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Warn("uv not found (will install during hermes install)"))
		}
		return check
	}

	result := execx.Run(ctx.Ctx, "uv", "--version")
	version := strings.TrimSpace(result.Stdout)
	check.Status = StatusOK
	check.Message = version
	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Ok("uv: "+version))
	}
	return check
}

func checkPython(ctx *app.AppContext, jsonOut bool) CheckResult {
	check := CheckResult{Name: "python"}

	pythonCmd := "python3"
	if !execx.CommandExists("python3") {
		if !execx.CommandExists("python") {
			check.Status = StatusWarning
			check.Message = "python not found"
			if !jsonOut {
				fmt.Fprintln(ctx.Stdout, ui.Warn("python not found"))
			}
			return check
		}
		pythonCmd = "python"
	}

	result := execx.Run(ctx.Ctx, pythonCmd, "--version")
	version := strings.TrimSpace(result.Stdout)
	check.Status = StatusOK
	check.Message = version
	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Ok(version))
	}
	return check
}
