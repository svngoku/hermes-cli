package engine

import (
	"context"

	"github.com/svngoku/hermes-cli/internal/config"
)

type Engine interface {
	Name() string
	CheckInstalled(ctx context.Context) (bool, string, error)
	Install(ctx context.Context) error
	ServeCommand(cfg config.ServeConfig) (string, []string)
}

func Get(name config.Engine) Engine {
	switch name {
	case config.EngineSGLang:
		return &SGLangEngine{}
	case config.EngineVLLM:
		return &VLLMEngine{}
	default:
		return nil
	}
}
