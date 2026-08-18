package config

type Settings struct {
	Engine      Engine  `json:"engine,omitempty"`
	Model       string  `json:"model,omitempty"`
	HFRepo      string  `json:"hf_repo,omitempty"`
	ModelURL    string  `json:"model_url,omitempty"`
	GPULayers   *int    `json:"gpu_layers,omitempty"`
	TP          int     `json:"tp,omitempty"`
	Host        string  `json:"host,omitempty"`
	Port        int     `json:"port,omitempty"`
	CUDADevices *string `json:"cuda_devices,omitempty"`
	VenvPath    string  `json:"venv_path,omitempty"`
}

func (s Settings) Apply(target *ServeConfig) {
	if s.Engine != "" {
		target.Engine = s.Engine
	}
	if s.Model != "" || s.HFRepo != "" || s.ModelURL != "" {
		target.Model = ""
		target.HFRepo = ""
		target.ModelURL = ""
	}
	if s.Model != "" {
		target.Model = s.Model
	}
	if s.HFRepo != "" {
		target.HFRepo = s.HFRepo
	}
	if s.ModelURL != "" {
		target.ModelURL = s.ModelURL
	}
	if s.GPULayers != nil {
		target.GPULayers = *s.GPULayers
	}
	if s.TP > 0 {
		target.TP = s.TP
	}
	if s.Host != "" {
		target.Host = s.Host
	}
	if s.Port > 0 {
		target.Port = s.Port
	}
	if s.CUDADevices != nil {
		target.CUDADevices = *s.CUDADevices
	}
	if s.VenvPath != "" {
		target.VenvPath = s.VenvPath
	}
}

func MergeSettings(target *Settings, source Settings) {
	gpuLayers := target.GPULayers
	cudaDevices := target.CUDADevices
	cfg := ServeConfig{}
	target.Apply(&cfg)
	if source.Engine != "" && source.Engine != cfg.Engine {
		cfg.Model = ""
		cfg.HFRepo = ""
		cfg.ModelURL = ""
		gpuLayers = nil
	}
	source.Apply(&cfg)
	*target = Settings{
		Engine:   cfg.Engine,
		Model:    cfg.Model,
		HFRepo:   cfg.HFRepo,
		ModelURL: cfg.ModelURL,
		TP:       cfg.TP,
		Host:     cfg.Host,
		Port:     cfg.Port,
		VenvPath: cfg.VenvPath,
	}
	if source.CUDADevices != nil {
		value := *source.CUDADevices
		cudaDevices = &value
	}
	target.CUDADevices = cudaDevices
	if source.GPULayers != nil {
		value := *source.GPULayers
		gpuLayers = &value
	}
	target.GPULayers = gpuLayers
}
