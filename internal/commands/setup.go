package commands

import (
	"flag"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/engine"
	"github.com/svngoku/hermes-cli/internal/gpu"
	"github.com/svngoku/hermes-cli/internal/settingsstore"
	"github.com/svngoku/hermes-cli/internal/ui"
)

func Setup(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	engineName := fs.String("engine", "", "Engine: sglang|vllm|llamacpp")
	model := fs.String("model", "", "Model path or Hugging Face repo")
	hfRepo := fs.String("hf-repo", "", "llama.cpp Hugging Face GGUF repo[:quant]")
	modelURL := fs.String("model-url", "", "llama.cpp public GGUF URL")
	tp := fs.Int("tp", 0, "Tensor parallel size (0 auto-detects)")
	host := fs.String("host", "127.0.0.1", "Default bind host")
	port := fs.Int("port", 30000, "Default bind port")
	cudaDevices := fs.String("cuda-devices", "", "Default CUDA_VISIBLE_DEVICES list")
	gpuLayers := fs.Int("gpu-layers", -1, "llama.cpp GPU layers")
	scope := fs.String("scope", "", "Config scope: user|project")
	install := fs.Bool("install", true, "Install or validate the selected engine")
	nonInteractive := fs.Bool("non-interactive", false, "Require all choices through flags")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes setup [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Detect hardware, configure defaults, and prepare an inference engine")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Fprintln(ctx.Stdout, ui.Banner())
	fmt.Fprintln(ctx.Stdout, ui.Step("GPU host setup"))
	fmt.Fprintln(ctx.Stdout, ui.HR())

	if !*nonInteractive {
		if err := promptSetupChoices(engineName, scope); err != nil {
			return fmt.Errorf("setup prompt: %w", err)
		}
	}
	if *engineName == "" || *scope == "" {
		return fmt.Errorf("--engine and --scope are required in non-interactive mode")
	}

	eng, err := config.ParseEngine(*engineName)
	if err != nil {
		return err
	}
	if *scope != "user" && *scope != "project" {
		return fmt.Errorf("invalid config scope %q (use user or project)", *scope)
	}
	selectedAdapter := engine.Get(eng)
	selectedGPUCount, normalizedCUDADevices, err := gpu.ParseCUDADevices(*cudaDevices)
	if err != nil {
		return fmt.Errorf("invalid --cuda-devices: %w", err)
	}

	gpuCount, gpuErr := gpu.Count(ctx.Ctx)
	if gpuErr != nil {
		gpuCount = 0
		fmt.Fprintln(ctx.Stdout, ui.Warn("GPU auto-detection unavailable; defaulting to one worker"))
	} else {
		fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Detected %d NVIDIA GPU(s)", gpuCount)))
	}
	if *tp == 0 {
		*tp = gpuCount
		if normalizedCUDADevices != "" {
			*tp = selectedGPUCount
		}
		if *tp < 1 || !selectedAdapter.Profile().SupportsTensorParallel {
			*tp = 1
		}
	}

	if !*nonInteractive {
		if err := promptModelSource(eng, model, hfRepo, modelURL); err != nil {
			return fmt.Errorf("setup prompt: %w", err)
		}
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
		CUDADevices: normalizedCUDADevices,
	}
	if err := validateServeConfig(ctx, &serveCfg); err != nil {
		return err
	}

	if *install {
		mode := config.InstallMode(eng)
		fmt.Fprintln(ctx.Stdout, ui.Step("Preparing "+string(eng)+"..."))
		if err := Install(ctx, []string{"--install", string(mode)}); err != nil {
			return err
		}
	}

	gpuLayerValue := serveCfg.GPULayers
	cudaDeviceValue := serveCfg.CUDADevices
	settings := config.Settings{
		Engine:      serveCfg.Engine,
		Model:       serveCfg.Model,
		HFRepo:      serveCfg.HFRepo,
		ModelURL:    serveCfg.ModelURL,
		GPULayers:   &gpuLayerValue,
		TP:          serveCfg.TP,
		Host:        serveCfg.Host,
		Port:        serveCfg.Port,
		CUDADevices: &cudaDeviceValue,
	}
	path, err := settingsstore.Save(settings, *scope)
	if err != nil {
		return err
	}
	fmt.Fprintln(ctx.Stdout, ui.Ok("Configuration saved: "+path))
	fmt.Fprintln(ctx.Stdout, ui.Info("Run `hermes run` to use these defaults"))
	return nil
}

func promptSetupChoices(engineName, scope *string) error {
	fields := make([]huh.Field, 0, 2)
	if *engineName == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Select inference engine").
			Options(
				huh.NewOption("SGLang", "sglang"),
				huh.NewOption("vLLM", "vllm"),
				huh.NewOption("llama.cpp", "llamacpp"),
			).
			Value(engineName))
	}
	if *scope == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Save defaults").
			Options(
				huh.NewOption("User config", "user"),
				huh.NewOption("Project .hermes.json", "project"),
			).
			Value(scope))
	}
	if len(fields) == 0 {
		return nil
	}
	return huh.NewForm(huh.NewGroup(fields...)).Run()
}

func promptModelSource(eng config.Engine, model, hfRepo, modelURL *string) error {
	if *model != "" || *hfRepo != "" || *modelURL != "" {
		return nil
	}
	if eng != config.EngineLlamaCpp {
		return huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Model (Hugging Face repo or local path)").
				Value(model),
		)).Run()
	}

	source := "model"
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("GGUF model source").
			Options(
				huh.NewOption("Local .gguf file", "model"),
				huh.NewOption("Hugging Face GGUF repo", "hf-repo"),
				huh.NewOption("Public GGUF URL", "model-url"),
			).
			Value(&source),
	)).Run(); err != nil {
		return err
	}

	input := huh.NewInput()
	switch source {
	case "hf-repo":
		input.Title("Hugging Face repo (owner/repository[:quant])").Value(hfRepo)
	case "model-url":
		input.Title("Public GGUF URL").Value(modelURL)
	default:
		input.Title("Local .gguf file").Value(model)
	}
	return huh.NewForm(huh.NewGroup(input)).Run()
}
