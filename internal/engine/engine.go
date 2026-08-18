package engine

import (
	"context"

	"github.com/svngoku/hermes-cli/internal/config"
)

type Engine interface {
	Name() string
	Profile() RuntimeProfile
	CheckInstalled(ctx context.Context) (bool, string, error)
	Install(ctx context.Context) error
	ServeCommand(cfg config.ServeConfig) (string, []string)
}

type EnvironmentEngine interface {
	CheckInstalledIn(ctx context.Context, venvPath string) (bool, string, error)
	InstallIn(ctx context.Context, venvPath string) error
}

type RuntimeKind string

const (
	RuntimeUVPython RuntimeKind = "uv-python"
	RuntimeNative   RuntimeKind = "preinstalled-native"
)

type RuntimeProfile struct {
	Kind                   RuntimeKind
	RequiresNVIDIA         bool
	SupportsTensorParallel bool
}

func CheckInstalledIn(ctx context.Context, eng Engine, venvPath string) (bool, string, error) {
	if managed, ok := eng.(EnvironmentEngine); ok && venvPath != "" {
		return managed.CheckInstalledIn(ctx, venvPath)
	}
	return eng.CheckInstalled(ctx)
}

func InstallIn(ctx context.Context, eng Engine, venvPath string) error {
	if managed, ok := eng.(EnvironmentEngine); ok && venvPath != "" {
		return managed.InstallIn(ctx, venvPath)
	}
	return eng.Install(ctx)
}

func Get(name config.Engine) Engine {
	switch name {
	case config.EngineSGLang:
		return &SGLangEngine{}
	case config.EngineVLLM:
		return &VLLMEngine{}
	case config.EngineLlamaCpp:
		return &LlamaCppEngine{}
	default:
		return nil
	}
}
