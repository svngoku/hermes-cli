package commands

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/ui"
)

func Run(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	engineName := fs.String("engine", "", "Engine: sglang|vllm (required)")
	model := fs.String("model", "", "Model path or HuggingFace repo (required)")
	tp := fs.Int("tp", 4, "Tensor parallel size")
	host := fs.String("host", "0.0.0.0", "Bind host")
	port := fs.Int("port", 30000, "Bind port")
	daemon := fs.Bool("daemon", false, "Run in daemon mode")
	installMode := fs.String("install", "both", "Install mode: sglang|vllm|both|none")
	noVerify := fs.Bool("no-verify", false, "Skip verification")
	extraArgs := fs.String("extra-args", "", "Additional engine arguments")
	readinessTimeout := fs.Int("readiness-timeout", 300, "Readiness check timeout in seconds")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes run [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Run full pipeline: doctor → install → serve → verify")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *engineName == "" || *model == "" {
		return fmt.Errorf("--engine and --model are required")
	}

	var eng config.Engine
	switch *engineName {
	case "sglang":
		eng = config.EngineSGLang
	case "vllm":
		eng = config.EngineVLLM
	default:
		return fmt.Errorf("invalid engine: %s (use sglang or vllm)", *engineName)
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("Hermes Pipeline: doctor → install → serve → verify"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 1: Doctor"))
	fmt.Fprintln(ctx.Stdout, ui.HR())
	if err := runDoctorPhase(ctx); err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 2: Install"))
	fmt.Fprintln(ctx.Stdout, ui.HR())
	if err := runInstallPhase(ctx, config.InstallMode(*installMode)); err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 3: Serve"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	serveCfg := config.ServeConfig{
		Engine:    eng,
		Model:     *model,
		TP:        *tp,
		Host:      *host,
		Port:      *port,
		Daemon:    *daemon,
		ExtraArgs: *extraArgs,
		LogFile:   ctx.LogFile,
	}

	if err := runServePhase(ctx, serveCfg); err != nil {
		return err
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", *port)

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 4: Readiness"))
	fmt.Fprintln(ctx.Stdout, ui.HR())
	if err := waitForReadiness(ctx, base, time.Duration(*readinessTimeout)*time.Second); err != nil {
		return err
	}

	if !*noVerify {
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, ui.Step("Phase 5: Verify"))
		fmt.Fprintln(ctx.Stdout, ui.HR())
		result := runVerify(ctx, base, 60*time.Second, true, false)
		if result.Status != "ok" {
			return fmt.Errorf("verification failed: %s", result.Message)
		}
	}

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Hermes is operational: %s", base)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Logs: tail -f %s", ctx.LogFile)))

	if !*daemon {
		fmt.Fprintln(ctx.Stdout, ui.Info("Foreground mode: Ctrl+C to stop"))
		select {}
	}

	return nil
}

func runDoctorPhase(ctx *app.AppContext) error {
	checks := []struct {
		name  string
		check func() (bool, string)
	}{
		{"nvidia-smi", func() (bool, string) {
			result := checkNvidiaSMI(ctx, true)
			return result.Status == StatusOK, result.Message
		}},
		{"uv", func() (bool, string) {
			result := checkUV(ctx, true)
			return result.Status == StatusOK || result.Status == StatusWarning, result.Message
		}},
		{"python", func() (bool, string) {
			result := checkPython(ctx, true)
			return result.Status == StatusOK, result.Message
		}},
	}

	allPassed := true
	for _, c := range checks {
		ok, msg := c.check()
		if ok {
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("%s: %s", c.name, msg)))
		} else {
			fmt.Fprintln(ctx.Stdout, ui.Warn(fmt.Sprintf("%s: %s", c.name, msg)))
			if c.name == "nvidia-smi" {
				allPassed = false
			}
		}
	}

	if !allPassed {
		return fmt.Errorf("critical doctor checks failed")
	}
	return nil
}

func runInstallPhase(ctx *app.AppContext, mode config.InstallMode) error {
	if mode == config.InstallNone {
		fmt.Fprintln(ctx.Stdout, ui.Info("Skipping installation (--install none)"))
		return nil
	}

	return Install(ctx, []string{"--install", string(mode)})
}

func runServePhase(ctx *app.AppContext, cfg config.ServeConfig) error {
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Starting %s with model %s", cfg.Engine, cfg.Model)))

	cfg.Daemon = true

	return runServe(ctx, cfg)
}

func waitForReadiness(ctx *app.AppContext, base string, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	checkInterval := 2 * time.Second

	endpoints := []string{"/v1/models", "/health"}

	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Waiting for server at %s (timeout: %s)", base, timeout)))

	for time.Now().Before(deadline) {
		for _, endpoint := range endpoints {
			resp, err := client.Get(base + endpoint)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				fmt.Fprintln(ctx.Stdout, ui.Ok("Server is ready"))
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		time.Sleep(checkInterval)
		fmt.Fprint(ctx.Stdout, ".")
	}

	fmt.Fprintln(ctx.Stdout)
	return fmt.Errorf("timeout waiting for server readiness")
}
