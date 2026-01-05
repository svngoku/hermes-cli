package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/svngoku/hermes-cli/internal/app"
	"github.com/svngoku/hermes-cli/internal/ui"
)

type VerifyResult struct {
	Endpoint   string `json:"endpoint"`
	Status     string `json:"status"`
	ModelsOK   bool   `json:"models_ok"`
	HealthOK   bool   `json:"health_ok"`
	ChatOK     bool   `json:"chat_ok,omitempty"`
	Message    string `json:"message,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

func Verify(ctx *app.AppContext, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	host := fs.String("host", "127.0.0.1", "Server host")
	port := fs.Int("port", 30000, "Server port")
	timeout := fs.Int("timeout", 60, "Timeout in seconds")
	noVerify := fs.Bool("no-verify", false, "Skip verification (no-op for compatibility)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	chat := fs.Bool("chat", false, "Also test chat completion endpoint")
	fs.Usage = func() {
		fmt.Fprintln(ctx.Stdout, "Usage: hermes verify [flags]")
		fmt.Fprintln(ctx.Stdout)
		fmt.Fprintln(ctx.Stdout, "Verify server is responding")
		fmt.Fprintln(ctx.Stdout)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *noVerify {
		if !*jsonOutput {
			fmt.Fprintln(ctx.Stdout, ui.Info("Verification skipped (--no-verify)"))
		}
		return nil
	}

	base := fmt.Sprintf("http://%s:%d", *host, *port)
	result := runVerify(ctx, base, time.Duration(*timeout)*time.Second, *chat, *jsonOutput)

	if *jsonOutput {
		enc := json.NewEncoder(ctx.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Status == "ok" {
		return nil
	}
	return fmt.Errorf("verification failed: %s", result.Message)
}

func runVerify(ctx *app.AppContext, base string, timeout time.Duration, testChat, jsonOut bool) VerifyResult {
	start := time.Now()
	result := VerifyResult{
		Endpoint: base,
	}

	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Banner())
		fmt.Fprintln(ctx.Stdout, ui.Step(fmt.Sprintf("Verifying server at %s...", base)))
		fmt.Fprintln(ctx.Stdout, ui.HR())
	}

	client := &http.Client{Timeout: timeout}

	modelsOK := checkModels(ctx, client, base, jsonOut)
	result.ModelsOK = modelsOK

	healthOK := checkHealth(ctx, client, base, jsonOut)
	result.HealthOK = healthOK

	if testChat {
		chatOK := checkChatCompletion(ctx, client, base, jsonOut)
		result.ChatOK = chatOK
	}

	result.DurationMs = time.Since(start).Milliseconds()

	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.HR())
	}

	if result.ModelsOK || result.HealthOK {
		result.Status = "ok"
		result.Message = "Server is operational"
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Server operational (%dms)", result.DurationMs)))
		}
	} else {
		result.Status = "fail"
		result.Message = "Server not responding"
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail("Server not responding"))
		}
	}

	return result
}

func checkModels(ctx *app.AppContext, client *http.Client, base string, jsonOut bool) bool {
	url := base + "/v1/models"
	resp, err := client.Get(url)
	if err != nil {
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail(fmt.Sprintf("GET /v1/models: %v", err)))
		}
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if !jsonOut {
			preview := string(body)
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Fprintln(ctx.Stdout, ui.Ok("GET /v1/models: OK"))
			fmt.Fprintln(ctx.Stdout, "    "+strings.ReplaceAll(preview, "\n", "\n    "))
		}
		return true
	}

	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Fail(fmt.Sprintf("GET /v1/models: %d", resp.StatusCode)))
	}
	return false
}

func checkHealth(ctx *app.AppContext, client *http.Client, base string, jsonOut bool) bool {
	url := base + "/health"
	resp, err := client.Get(url)
	if err != nil {
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Warn(fmt.Sprintf("GET /health: %v", err)))
		}
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Ok("GET /health: OK"))
		}
		return true
	}

	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Warn(fmt.Sprintf("GET /health: %d", resp.StatusCode)))
	}
	return false
}

func checkChatCompletion(ctx *app.AppContext, client *http.Client, base string, jsonOut bool) bool {
	url := base + "/v1/chat/completions"
	payload := `{
		"model": "default",
		"messages": [{"role": "user", "content": "Return OK"}],
		"max_tokens": 8,
		"temperature": 0
	}`

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail(fmt.Sprintf("POST /v1/chat/completions: %v", err)))
		}
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer DUMMY")

	resp, err := client.Do(req)
	if err != nil {
		if !jsonOut {
			fmt.Fprintln(ctx.Stdout, ui.Fail(fmt.Sprintf("POST /v1/chat/completions: %v", err)))
		}
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if !jsonOut {
			preview := string(body)
			if len(preview) > 300 {
				preview = preview[:300] + "..."
			}
			fmt.Fprintln(ctx.Stdout, ui.Ok("POST /v1/chat/completions: OK"))
			fmt.Fprintln(ctx.Stdout, "    "+strings.ReplaceAll(preview, "\n", "\n    "))
		}
		return true
	}

	if !jsonOut {
		fmt.Fprintln(ctx.Stdout, ui.Fail(fmt.Sprintf("POST /v1/chat/completions: %d", resp.StatusCode)))
	}
	return false
}
