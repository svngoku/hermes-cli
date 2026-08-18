package config

import "testing"

func TestMergeSettingsWithProjectOverride(t *testing.T) {
	gpuLayers := 12
	cudaDevices := "0,1"
	settings := Settings{
		Engine:      EngineSGLang,
		Model:       "user/model",
		TP:          4,
		Host:        "127.0.0.1",
		Port:        30000,
		CUDADevices: &cudaDevices,
	}
	MergeSettings(&settings, Settings{
		Engine:    EngineLlamaCpp,
		HFRepo:    "owner/repository:Q4_K_M",
		GPULayers: &gpuLayers,
		TP:        1,
		Port:      8080,
	})
	cfg := DefaultServeConfig()
	settings.Apply(&cfg)
	if cfg.Engine != EngineLlamaCpp || cfg.Model != "" || cfg.HFRepo != "owner/repository:Q4_K_M" {
		t.Errorf("model settings = %#v", cfg)
	}
	if cfg.TP != 1 || cfg.Port != 8080 || cfg.Host != "127.0.0.1" || cfg.GPULayers != 12 {
		t.Errorf("merged settings = %#v", cfg)
	}
	if cfg.CUDADevices != "0,1" {
		t.Errorf("CUDA devices = %q", cfg.CUDADevices)
	}
}
