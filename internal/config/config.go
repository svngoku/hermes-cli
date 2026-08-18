package config

import (
	"fmt"
	"strings"
)

type Engine string

const (
	EngineSGLang   Engine = "sglang"
	EngineVLLM     Engine = "vllm"
	EngineLlamaCpp Engine = "llamacpp"
)

type InstallMode string

const (
	InstallSGLang   InstallMode = "sglang"
	InstallVLLM     InstallMode = "vllm"
	InstallLlamaCpp InstallMode = "llamacpp"
	InstallBoth     InstallMode = "both"
	InstallNone     InstallMode = "none"
)

type ServeConfig struct {
	Engine      Engine
	Model       string
	HFRepo      string
	ModelURL    string
	GPULayers   int
	TP          int
	Host        string
	Port        int
	Daemon      bool
	CUDADevices string
	VenvPath    string
	ExtraArgs   []string
	LogFile     string
}

type InstallConfig struct {
	Mode  InstallMode
	Check bool
}

type VerifyConfig struct {
	Host    string
	Port    int
	Timeout int
	Skip    bool
}

type StudioConfig struct {
	Enabled  bool
	Port     int
	Frontend bool
}

func DefaultServeConfig() ServeConfig {
	return ServeConfig{
		Engine:    EngineSGLang,
		GPULayers: -1,
		TP:        1,
		Host:      "0.0.0.0",
		Port:      30000,
	}
}

func ParseEngine(value string) (Engine, error) {
	switch Engine(strings.ToLower(strings.TrimSpace(value))) {
	case EngineSGLang:
		return EngineSGLang, nil
	case EngineVLLM:
		return EngineVLLM, nil
	case EngineLlamaCpp:
		return EngineLlamaCpp, nil
	default:
		return "", fmt.Errorf("invalid engine: %s (use sglang, vllm, or llamacpp)", value)
	}
}

func ValidateTP(tp, gpuCount int) error {
	if tp < 1 {
		return fmt.Errorf("tensor parallel size must be at least 1 (got %d; available GPU count: %d)", tp, gpuCount)
	}
	if gpuCount >= 0 && tp > gpuCount {
		return fmt.Errorf("tensor parallel size %d exceeds available GPU count %d", tp, gpuCount)
	}
	return nil
}

func DefaultInstallConfig() InstallConfig {
	return InstallConfig{
		Mode: InstallBoth,
	}
}

func DefaultVerifyConfig() VerifyConfig {
	return VerifyConfig{
		Host:    "127.0.0.1",
		Port:    30000,
		Timeout: 60,
	}
}

func DefaultStudioConfig() StudioConfig {
	return StudioConfig{
		Enabled: true,
		Port:    8000,
	}
}
