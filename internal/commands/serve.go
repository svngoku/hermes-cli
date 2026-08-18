package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/engine"
	"github.com/svngoku/hermes-cli/internal/gpu"
	"github.com/svngoku/hermes-cli/internal/pidfile"
	"github.com/svngoku/hermes-cli/internal/settingsstore"
	"github.com/svngoku/hermes-cli/internal/ui"
)

// recordDaemon persists a pid record for a started engine process. Best-effort:
// a failure to record does not stop the server.
func recordDaemon(ctx *app.AppContext, cfg config.ServeConfig, pid int) {
	rec := pidfile.Record{
		PID:       pid,
		Engine:    string(cfg.Engine),
		Model:     modelReference(cfg),
		Host:      cfg.Host,
		Port:      cfg.Port,
		LogFile:   cfg.LogFile,
		StartedAt: time.Now(),
	}
	if err := pidfile.Write(rec); err != nil {
		ctx.Logger.Warn("failed to record daemon", "error", err)
	}
}

func Serve(ctx *app.AppContext, args []string) error {
	defaults := config.DefaultServeConfig()
	settings, _, err := settingsstore.Load()
	if err != nil {
		return err
	}
	settings.Apply(&defaults)

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	engineName := fs.String("engine", string(defaults.Engine), "Engine: sglang|vllm|llamacpp")
	model := fs.String("model", defaults.Model, "Model path or HuggingFace repo")
	hfRepo := fs.String("hf-repo", defaults.HFRepo, "llama.cpp Hugging Face GGUF repo[:quant]")
	modelURL := fs.String("model-url", defaults.ModelURL, "llama.cpp public GGUF URL")
	gpuLayers := fs.Int("gpu-layers", defaults.GPULayers, "llama.cpp GPU layers (-1 uses engine default)")
	tp := fs.Int("tp", defaults.TP, "Tensor parallel size")
	host := fs.String("host", defaults.Host, "Bind host")
	port := fs.Int("port", defaults.Port, "Bind port")
	daemon := fs.Bool("daemon", false, "Run in daemon mode")
	bootTimeout := fs.Int("boot-timeout", 120, "Daemon boot timeout in seconds")
	cudaDevices := fs.String("cuda-devices", defaults.CUDADevices, "CUDA_VISIBLE_DEVICES list (e.g. 0,1); empty inherits environment")
	extraArgs := fs.String("extra-args", "", "Additional engine arguments")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes serve [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Start inference server")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	applyModelSourceOverrides(fs, model, hfRepo, modelURL)
	applyEngineSpecificOverrides(fs, string(defaults.Engine), *engineName, model, hfRepo, modelURL, gpuLayers)

	if *bootTimeout <= 0 {
		return fmt.Errorf("--boot-timeout must be greater than zero")
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

	cfg := config.ServeConfig{
		Engine:      eng,
		Model:       *model,
		HFRepo:      *hfRepo,
		ModelURL:    *modelURL,
		GPULayers:   *gpuLayers,
		TP:          *tp,
		Host:        *host,
		Port:        *port,
		Daemon:      *daemon,
		CUDADevices: normalizedCUDADevices,
		VenvPath:    defaults.VenvPath,
		ExtraArgs:   parsedExtraArgs,
		LogFile:     ctx.LogFile,
	}

	if err := validateServeConfig(ctx, &cfg); err != nil {
		return err
	}

	return runServe(ctx, cfg, time.Duration(*bootTimeout)*time.Second)
}

func applyEngineSpecificOverrides(
	fs *flag.FlagSet,
	defaultEngine, selectedEngine string,
	model, hfRepo, modelURL *string,
	gpuLayers *int,
) {
	if !flagWasSet(fs, "engine") || selectedEngine == defaultEngine {
		return
	}
	if !flagWasSet(fs, "model") && !flagWasSet(fs, "hf-repo") && !flagWasSet(fs, "model-url") {
		*model = ""
		*hfRepo = ""
		*modelURL = ""
	}
	if !flagWasSet(fs, "gpu-layers") {
		*gpuLayers = -1
	}
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func applyModelSourceOverrides(fs *flag.FlagSet, model, hfRepo, modelURL *string) {
	explicit := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicit[f.Name] = true
	})
	count := 0
	for _, name := range []string{"model", "hf-repo", "model-url"} {
		if explicit[name] {
			count++
		}
	}
	if count != 1 {
		return
	}
	if !explicit["model"] {
		*model = ""
	}
	if !explicit["hf-repo"] {
		*hfRepo = ""
	}
	if !explicit["model-url"] {
		*modelURL = ""
	}
}

func validateServeConfig(ctx *app.AppContext, cfg *config.ServeConfig) error {
	if cfg.GPULayers < -1 {
		return fmt.Errorf("--gpu-layers must be -1 or greater")
	}

	eng := engine.Get(cfg.Engine)
	if eng == nil {
		return fmt.Errorf("unknown engine: %s", cfg.Engine)
	}
	if eng.Profile().SupportsTensorParallel {
		if cfg.Model == "" {
			return fmt.Errorf("--model is required")
		}
		if cfg.HFRepo != "" || cfg.ModelURL != "" || cfg.GPULayers != -1 {
			return fmt.Errorf("--hf-repo, --model-url, and --gpu-layers are only supported by llamacpp")
		}
		return validateTensorParallel(ctx, *cfg)
	}

	if cfg.TP != 1 {
		return fmt.Errorf("%s requires --tp=1; use --extra-args for engine-specific multi-GPU controls", cfg.Engine)
	}
	if err := validateLlamaCppModelSource(cfg); err != nil {
		return err
	}
	return validateLlamaCppExtraArgs(cfg.ExtraArgs)
}

func validateLlamaCppModelSource(cfg *config.ServeConfig) error {
	sources := 0
	if cfg.Model != "" {
		sources++
	}
	if cfg.HFRepo != "" {
		sources++
	}
	if cfg.ModelURL != "" {
		sources++
	}
	if sources != 1 {
		return fmt.Errorf("llamacpp requires exactly one of --model, --hf-repo, or --model-url")
	}

	if cfg.Model != "" {
		info, err := os.Stat(cfg.Model)
		if err != nil {
			return fmt.Errorf("invalid --model: %w", err)
		}
		if !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(cfg.Model), ".gguf") {
			return fmt.Errorf("--model must be a regular .gguf file")
		}
		cfg.Model, err = filepath.Abs(cfg.Model)
		if err != nil {
			return fmt.Errorf("resolve --model: %w", err)
		}
	}
	if cfg.HFRepo != "" {
		repo := strings.SplitN(cfg.HFRepo, ":", 2)[0]
		parts := strings.Split(repo, "/")
		if strings.TrimSpace(cfg.HFRepo) != cfg.HFRepo ||
			strings.ContainsAny(cfg.HFRepo, " \t\r\n") ||
			len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("--hf-repo must use owner/repository[:quant] format")
		}
	}
	if cfg.ModelURL != "" {
		parsed, err := url.Parse(cfg.ModelURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("--model-url must be an absolute HTTP(S) URL")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("--model-url must not contain credentials, a query string, or a fragment")
		}
		if !isPublicModelHost(parsed.Hostname()) {
			return fmt.Errorf("--model-url must use a public host")
		}
	}
	return nil
}

func isPublicModelHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
			!ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
	}
	return strings.Contains(host, ".")
}

func validateLlamaCppExtraArgs(args []string) error {
	reserved := []string{
		"-m", "--model", "-hf", "-hfr", "--hf-repo", "-mu", "--model-url",
		"--host", "--port", "-ngl", "--gpu-layers", "--n-gpu-layers",
	}
	for _, arg := range args {
		for _, flag := range reserved {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return fmt.Errorf("--extra-args must not override Hermes-owned flag %s", flag)
			}
		}
	}
	return nil
}

func validateTensorParallel(ctx *app.AppContext, cfg config.ServeConfig) error {
	if cfg.CUDADevices != "" {
		gpuCount, _, err := gpu.ParseCUDADevices(cfg.CUDADevices)
		if err != nil {
			return fmt.Errorf("invalid CUDA devices: %w", err)
		}
		return config.ValidateTP(cfg.TP, gpuCount)
	}

	if inherited, present := os.LookupEnv("CUDA_VISIBLE_DEVICES"); present {
		inherited = strings.TrimSpace(inherited)
		if inherited == "" || inherited == "-1" {
			return config.ValidateTP(cfg.TP, 0)
		}

		gpuCount, _, err := gpu.ParseCUDADevices(inherited)
		if err != nil {
			ctx.Logger.Warn(
				"unsupported inherited CUDA_VISIBLE_DEVICES; skipping tensor parallel capacity validation",
				"value", inherited,
				"error", err,
			)
			return config.ValidateTP(cfg.TP, -1)
		}
		return config.ValidateTP(cfg.TP, gpuCount)
	}

	gpuCount, err := gpu.Count(ctx.Ctx)
	if err != nil {
		ctx.Logger.Warn("could not query GPU count; skipping tensor parallel capacity validation", "error", err)
		gpuCount = -1
	}
	return config.ValidateTP(cfg.TP, gpuCount)
}

func runServe(ctx *app.AppContext, cfg config.ServeConfig, bootTimeout time.Duration) error {
	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step(fmt.Sprintf("Starting %s server...", cfg.Engine)))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Engine: %s", cfg.Engine)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Model:  %s", modelReference(cfg))))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("TP:     %d", cfg.TP)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Host:   %s", cfg.Host)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Port:   %d", cfg.Port)))
	if cfg.CUDADevices != "" {
		fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("CUDA:   %s", cfg.CUDADevices)))
	}
	if len(cfg.ExtraArgs) > 0 {
		fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Extra:  %d argument(s)", len(cfg.ExtraArgs))))
	}
	fmt.Fprintln(ctx.Stdout, ui.HR())

	if err := assertPortAvailable(cfg.Host, cfg.Port); err != nil {
		return err
	}

	cmd, logFile, err := startEngine(ctx, cfg)
	if err != nil {
		return err
	}

	if cfg.Daemon {
		if logFile != nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			defer logFile.Close()
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		recordDaemon(ctx, cfg, cmd.Process.Pid)
		base := readinessBaseURL(cfg.Host, cfg.Port)
		if err := waitForBoot(ctx.Ctx, cmd, base, bootTimeout, cfg.LogFile); err != nil {
			_ = pidfile.Remove(cfg.Port)
			terminateAndReap(cmd)
			return err
		}
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Daemon started (pid=%d)", cmd.Process.Pid)))
		fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Endpoint: http://%s:%d", cfg.Host, cfg.Port)))
		fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Stop with: hermes stop --port %d", cfg.Port)))
		if cfg.LogFile != "" {
			fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Logs: tail -f %s", cfg.LogFile)))
		}
		return nil
	}

	if logFile != nil {
		defer logFile.Close()
	}
	var writers []io.Writer
	writers = append(writers, ctx.Stdout)
	if logFile != nil {
		writers = append(writers, logFile)
	}
	multiWriter := io.MultiWriter(writers...)
	cmd.Stdout = multiWriter
	cmd.Stderr = multiWriter

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	recordDaemon(ctx, cfg, cmd.Process.Pid)
	defer pidfile.Remove(cfg.Port)

	fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Server started (pid=%d)", cmd.Process.Pid)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Endpoint: http://%s:%d", cfg.Host, cfg.Port)))
	fmt.Fprintln(ctx.Stdout, ui.Info("Ctrl+C to stop"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	return waitForServer(ctx, cmd)
}

func modelReference(cfg config.ServeConfig) string {
	switch {
	case cfg.Model != "":
		return cfg.Model
	case cfg.HFRepo != "":
		return cfg.HFRepo
	default:
		return cfg.ModelURL
	}
}

// startEngine resolves the engine and builds its process. The process is always
// placed in its own process group so we can signal the whole tree on shutdown.
// Daemon processes are bound to context.Background() so they survive after the
// CLI exits; foreground processes inherit the cancelable app context.
func startEngine(ctx *app.AppContext, cfg config.ServeConfig) (*exec.Cmd, *os.File, error) {
	eng := engine.Get(cfg.Engine)
	if eng == nil {
		return nil, nil, fmt.Errorf("unknown engine: %s", cfg.Engine)
	}

	cmdName, cmdArgs := eng.ServeCommand(cfg)
	ctx.Logger.Debug("serve command prepared", "cmd", cmdName, "extra_args_present", len(cfg.ExtraArgs) > 0)

	execCtx := ctx.Ctx
	if cfg.Daemon {
		execCtx = context.Background()
	}
	cmd := exec.CommandContext(execCtx, cmdName, cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if cfg.CUDADevices != "" {
		cmd.Env = environmentWithCUDADevices(os.Environ(), cfg.CUDADevices)
	}

	var logFile *os.File
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logFile = f
	}

	return cmd, logFile, nil
}

func environmentWithCUDADevices(environ []string, devices string) []string {
	const key = "CUDA_VISIBLE_DEVICES="

	result := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if !strings.HasPrefix(entry, key) {
			result = append(result, entry)
		}
	}
	return append(result, key+devices)
}

// waitForServer blocks until the engine exits or the operator interrupts. On
// SIGINT/SIGTERM it terminates the whole process group and waits for exit.
func waitForServer(ctx *app.AppContext, cmd *exec.Cmd) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case sig := <-sigChan:
		ctx.Logger.Info("received signal, shutting down", "signal", sig)
		stopProcess(cmd)
		<-done
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, ui.Ok("Server stopped"))
		return nil
	case err := <-done:
		if err != nil {
			return fmt.Errorf("server exited with error: %w", err)
		}
		fmt.Fprintln(ctx.Stdout, ui.Ok("Server exited"))
		return nil
	}
}

// stopProcess sends SIGTERM to the process group (negative pid) so child
// workers spawned by the engine are also terminated.
func stopProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
}
