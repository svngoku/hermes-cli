package commands

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/engine"
	"github.com/svngoku/hermes-cli/internal/gpu"
	"github.com/svngoku/hermes-cli/internal/pidfile"
	"github.com/svngoku/hermes-cli/internal/ui"
)

// recordDaemon persists a pid record for a started engine process. Best-effort:
// a failure to record does not stop the server.
func recordDaemon(ctx *app.AppContext, cfg config.ServeConfig, pid int) {
	rec := pidfile.Record{
		PID:       pid,
		Engine:    string(cfg.Engine),
		Model:     cfg.Model,
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
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	engineName := fs.String("engine", "sglang", "Engine: sglang|vllm")
	model := fs.String("model", "", "Model path or HuggingFace repo")
	tp := fs.Int("tp", 1, "Tensor parallel size")
	host := fs.String("host", "0.0.0.0", "Bind host")
	port := fs.Int("port", 30000, "Bind port")
	daemon := fs.Bool("daemon", false, "Run in daemon mode")
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

	if *model == "" {
		return fmt.Errorf("--model is required")
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

	cfg := config.ServeConfig{
		Engine:    eng,
		Model:     *model,
		TP:        *tp,
		Host:      *host,
		Port:      *port,
		Daemon:    *daemon,
		ExtraArgs: *extraArgs,
		LogFile:   ctx.LogFile,
	}

	if err := validateTensorParallel(ctx, cfg.TP); err != nil {
		return err
	}

	return runServe(ctx, cfg)
}

func validateTensorParallel(ctx *app.AppContext, tp int) error {
	gpuCount, err := gpu.Count(ctx.Ctx)
	if err != nil {
		ctx.Logger.Warn("could not query GPU count; skipping tensor parallel capacity validation", "error", err)
		gpuCount = -1
	}
	return config.ValidateTP(tp, gpuCount)
}

func runServe(ctx *app.AppContext, cfg config.ServeConfig) error {
	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step(fmt.Sprintf("Starting %s server...", cfg.Engine)))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Engine: %s", cfg.Engine)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Model:  %s", cfg.Model)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("TP:     %d", cfg.TP)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Host:   %s", cfg.Host)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Port:   %d", cfg.Port)))
	if cfg.ExtraArgs != "" {
		fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Extra:  %s", cfg.ExtraArgs)))
	}
	fmt.Fprintln(ctx.Stdout, ui.HR())

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
	ctx.Logger.Debug("serve command", "cmd", cmdName, "args", cmdArgs)

	execCtx := ctx.Ctx
	if cfg.Daemon {
		execCtx = context.Background()
	}
	cmd := exec.CommandContext(execCtx, cmdName, cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
