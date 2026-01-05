package tui

import (
	"github.com/charmbracelet/huh"
)

type ServeFormResult struct {
	Engine string
	Model  string
	TP     string
}

func RunInteractiveSetup() (*ServeFormResult, error) {
	result := &ServeFormResult{}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select inference engine").
				Options(
					huh.NewOption("SGLang (sgl-project/sglang)", "sglang"),
					huh.NewOption("vLLM (vllm-project/vllm)", "vllm"),
				).
				Value(&result.Engine),

			huh.NewInput().
				Title("Model (HuggingFace repo or local path)").
				Placeholder("meta-llama/Llama-3-8B").
				Value(&result.Model),

			huh.NewSelect[string]().
				Title("Tensor parallel size").
				Description("Number of GPUs to use").
				Options(
					huh.NewOption("1 GPU", "1"),
					huh.NewOption("2 GPUs", "2"),
					huh.NewOption("4 GPUs", "4"),
					huh.NewOption("8 GPUs", "8"),
				).
				Value(&result.TP),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ConfirmAction(title, description string) (bool, error) {
	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Affirmative("Yes").
				Negative("No").
				Value(&confirm),
		),
	)

	err := form.Run()
	if err != nil {
		return false, err
	}

	return confirm, nil
}
