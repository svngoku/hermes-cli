package engine

import (
	"context"
	"io"

	"github.com/svngoku/hermes-cli/internal/config"
)

type Engine interface {
	Name() string
	Profile() RuntimeProfile
	CheckInstalled(ctx context.Context) (bool, string, error)
	// Install streams all subprocess output to stdout/stderr so long installs
	// stay visible to the operator.
	Install(ctx context.Context, stdout, stderr io.Writer) error
	ServeCommand(cfg config.ServeConfig) (string, []string)
}

type RuntimeKind string

const (
	RuntimePythonVenv RuntimeKind = "python-venv"
	RuntimeNative     RuntimeKind = "preinstalled-native"
)

type RuntimeProfile struct {
	Kind                   RuntimeKind
	RequiresNVIDIA         bool
	SupportsTensorParallel bool
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
