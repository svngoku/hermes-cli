package commands

import (
	"flag"
	"fmt"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/engine"
	"github.com/svngoku/hermes-cli/internal/gpu"
	"github.com/svngoku/hermes-cli/internal/pidfile"
	"github.com/svngoku/hermes-cli/internal/settingsstore"
	"github.com/svngoku/hermes-cli/internal/ui"
)

type ownershipGuard struct {
	owned   bool
	cleanup func()
}

func newOwnershipGuard(cleanup func()) *ownershipGuard {
	return &ownershipGuard{owned: true, cleanup: cleanup}
}

func (g *ownershipGuard) release() {
	g.owned = false
}

func (g *ownershipGuard) clean() {
	if !g.owned {
		return
	}
	g.owned = false
	g.cleanup()
}

func Run(ctx *app.AppContext, args []string) error {
	defaults := config.DefaultServeConfig()
	defaults.Engine = ""
	settings, _, err := settingsstore.Load()
	if err != nil {
		return err
	}
	settings.Apply(&defaults)

	fs := flag.NewFlagSet("run", flag.ExitOnError)
	engineName := fs.String("engine", string(defaults.Engine), "Engine: sglang|vllm|llamacpp (required unless configured)")
	model := fs.String("model", defaults.Model, "Model path or HuggingFace repo")
	hfRepo := fs.String("hf-repo", defaults.HFRepo, "llama.cpp Hugging Face GGUF repo[:quant]")
	modelURL := fs.String("model-url", defaults.ModelURL, "llama.cpp public GGUF URL")
	gpuLayers := fs.Int("gpu-layers", defaults.GPULayers, "llama.cpp GPU layers (-1 uses engine default)")
	tp := fs.Int("tp", defaults.TP, "Tensor parallel size")
	host := fs.String("host", defaults.Host, "Bind host")
	port := fs.Int("port", defaults.Port, "Bind port")
	daemon := fs.Bool("daemon", false, "Run in daemon mode")
	cudaDevices := fs.String("cuda-devices", defaults.CUDADevices, "CUDA_VISIBLE_DEVICES list (e.g. 0,1); empty inherits environment")
	installMode := fs.String("install", "", "Install mode: sglang|vllm|llamacpp|both|none")
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
	applyModelSourceOverrides(fs, model, hfRepo, modelURL)
	applyEngineSpecificOverrides(fs, string(defaults.Engine), *engineName, model, hfRepo, modelURL, gpuLayers)

	if *engineName == "" {
		return fmt.Errorf("--engine is required")
	}
	if *readinessTimeout <= 0 {
		return fmt.Errorf("--readiness-timeout must be greater than zero")
	}

	eng, err := config.ParseEngine(*engineName)
	if err != nil {
		return err
	}

	_, normalizedCUDADevices, err := gpu.ParseCUDADevices(*cudaDevices)
	if err != nil {
		return fmt.Errorf("invalid --cuda-devices: %w", err)
	}
	parsedExtraArgs, err := engine.ParseArgs(*extraArgs)
	if err != nil {
		return err
	}

	serveCfg := config.ServeConfig{
		Engine:      eng,
		Model:       *model,
		HFRepo:      *hfRepo,
		ModelURL:    *modelURL,
		GPULayers:   *gpuLayers,
		TP:          *tp,
		Host:        *host,
		Port:        *port,
		Daemon:      true, // run engine in the background while we poll and verify
		CUDADevices: normalizedCUDADevices,
		VenvPath:    defaults.VenvPath,
		ExtraArgs:   parsedExtraArgs,
		LogFile:     ctx.LogFile,
	}

	if err := validateServeConfig(ctx, &serveCfg); err != nil {
		return err
	}
	resolvedInstallMode, err := resolveRunInstallMode(eng, config.InstallMode(*installMode))
	if err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("Hermes Pipeline: doctor → install → serve → verify"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 1: Doctor"))
	fmt.Fprintln(ctx.Stdout, ui.HR())
	if err := runDoctorPhase(ctx, eng); err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 2: Install"))
	fmt.Fprintln(ctx.Stdout, ui.HR())
	if err := runInstallPhase(ctx, resolvedInstallMode, serveCfg.VenvPath); err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 3: Serve"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Starting %s with model %s", serveCfg.Engine, modelReference(serveCfg))))

	if err := assertPortAvailable(serveCfg.Host, serveCfg.Port); err != nil {
		return err
	}

	serveCmd, serveLog, err := startEngine(ctx, serveCfg)
	if err != nil {
		return err
	}
	if serveLog != nil {
		serveCmd.Stdout = serveLog
		serveCmd.Stderr = serveLog
		defer serveLog.Close()
	}
	if err := serveCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	recordDaemon(ctx, serveCfg, serveCmd.Process.Pid)
	ownership := newOwnershipGuard(func() {
		_ = pidfile.Remove(serveCfg.Port)
		terminateAndReap(serveCmd)
	})
	defer ownership.clean()
	fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Server started (pid=%d)", serveCmd.Process.Pid)))

	base := readinessBaseURL(serveCfg.Host, serveCfg.Port)

	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, ui.Step("Phase 4: Readiness"))
	fmt.Fprintln(ctx.Stdout, ui.HR())
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf(
		"Waiting for server at %s (timeout: %s)",
		base,
		time.Duration(*readinessTimeout)*time.Second,
	)))
	if err := waitForBoot(ctx.Ctx, serveCmd, base, time.Duration(*readinessTimeout)*time.Second, serveCfg.LogFile); err != nil {
		return err
	}
	fmt.Fprintln(ctx.Stdout, ui.Ok("Server is ready"))

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
		err := waitForServer(ctx, serveCmd)
		_ = pidfile.Remove(serveCfg.Port)
		ownership.release()
		return err
	}

	ownership.release()
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Stop with: hermes stop --port %d", *port)))
	return nil
}

func runDoctorPhase(ctx *app.AppContext, selectedEngine config.Engine) error {
	selectedAdapter := engine.Get(selectedEngine)
	if selectedAdapter != nil && !selectedAdapter.Profile().RequiresNVIDIA {
		result := checkEngine(ctx, selectedEngine, true)
		if result.Status != StatusOK {
			fmt.Fprintln(ctx.Stdout, ui.Warn(fmt.Sprintf("%s: %s", result.Name, result.Message)))
			return fmt.Errorf("critical doctor checks failed")
		}
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("%s: %s", result.Name, result.Message)))
		return nil
	}

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

func runInstallPhase(ctx *app.AppContext, mode config.InstallMode, venvPath string) error {
	if mode == config.InstallNone {
		fmt.Fprintln(ctx.Stdout, ui.Info("Skipping installation (--install none)"))
		return nil
	}

	installArgs := []string{"--install", string(mode)}
	if venvPath != "" {
		installArgs = append(installArgs, "--venv", venvPath)
	}
	return Install(ctx, installArgs)
}

func resolveRunInstallMode(selectedEngine config.Engine, requested config.InstallMode) (config.InstallMode, error) {
	if requested == "" {
		return config.InstallMode(selectedEngine), nil
	}
	if selectedEngine == config.EngineLlamaCpp &&
		requested != config.InstallLlamaCpp && requested != config.InstallNone {
		return "", fmt.Errorf("llamacpp only supports --install llamacpp or --install none")
	}
	switch requested {
	case config.InstallSGLang, config.InstallVLLM, config.InstallLlamaCpp, config.InstallBoth, config.InstallNone:
		return requested, nil
	default:
		return "", fmt.Errorf("invalid install mode: %s", requested)
	}
}
