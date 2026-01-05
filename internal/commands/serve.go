package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/engine"
	"github.com/svngoku/hermes-cli/internal/ui"
)

func Serve(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	engineName := fs.String("engine", "sglang", "Engine: sglang|vllm")
	model := fs.String("model", "", "Model path or HuggingFace repo")
	tp := fs.Int("tp", 4, "Tensor parallel size")
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

	return runServe(ctx, cfg)
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

	eng := engine.Get(cfg.Engine)
	if eng == nil {
		return fmt.Errorf("unknown engine: %s", cfg.Engine)
	}

	cmdName, cmdArgs := eng.ServeCommand(cfg)

	ctx.Logger.Debug("serve command", "cmd", cmdName, "args", cmdArgs)

	cmd := exec.CommandContext(ctx.Ctx, cmdName, cmdArgs...)

	var logFile *os.File
	var err error
	if cfg.LogFile != "" {
		logFile, err = os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logFile.Close()
	}

	if cfg.Daemon {
		return runDaemon(ctx, cmd, logFile, cfg)
	}

	return runForeground(ctx, cmd, logFile, cfg)
}

func runDaemon(ctx *app.AppContext, cmd *exec.Cmd, logFile *os.File, cfg config.ServeConfig) error {
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Daemon started (pid=%d)", cmd.Process.Pid)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Endpoint: http://%s:%d", cfg.Host, cfg.Port)))
	if cfg.LogFile != "" {
		fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Logs: tail -f %s", cfg.LogFile)))
	}

	return nil
}

func runForeground(ctx *app.AppContext, cmd *exec.Cmd, logFile *os.File, cfg config.ServeConfig) error {
	var writers []io.Writer
	writers = append(writers, ctx.Stdout)
	if logFile != nil {
		writers = append(writers, logFile)
	}
	multiWriter := io.MultiWriter(writers...)

	cmd.Stdout = multiWriter
	cmd.Stderr = multiWriter

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Server started (pid=%d)", cmd.Process.Pid)))
	fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("Endpoint: http://%s:%d", cfg.Host, cfg.Port)))
	fmt.Fprintln(ctx.Stdout, ui.Info("Ctrl+C to stop"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case sig := <-sigChan:
		ctx.Logger.Info("received signal, shutting down", "signal", sig)
		cmd.Process.Signal(syscall.SIGTERM)
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
