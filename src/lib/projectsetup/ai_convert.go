package projectsetup

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed setup-script-ai-prompt.txt
var embeddedSetupScriptAIPrompt string

type AIConfig struct {
	Provider  string `json:"provider"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	APIKey    string `json:"apiKey,omitempty"`
	APIKeyEnv string `json:"apiKeyEnv,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

type AIRefineResult struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	PromptPath string `json:"promptPath,omitempty"`
}

func SetupScriptAIPrompt() string { return embeddedSetupScriptAIPrompt }

func RefineSetupYAMLWithAI(ctx context.Context, root, scriptPath, draft string) (string, AIRefineResult, error) {
	cfg, err := loadAIConfig()
	if err != nil {
		return "", AIRefineResult{}, err
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", AIRefineResult{}, err
	}
	promptTemplate, promptPath, err := loadSetupScriptPrompt()
	if err != nil {
		return "", AIRefineResult{}, err
	}
	prompt := strings.NewReplacer(
		"{{PROJECT_ROOT}}", root,
		"{{SETUP_SCRIPT}}", string(script),
		"{{DRAFT_YAML}}", draft,
	).Replace(promptTemplate)
	content, err := callAI(ctx, cfg, prompt)
	if err != nil {
		return "", AIRefineResult{}, err
	}
	content = normalizeAIYAML(content)
	if content == "" {
		return "", AIRefineResult{}, errors.New("AI hat kein update-cli.yaml zurückgegeben")
	}
	if err := validateManifestText(root, content); err != nil {
		return "", AIRefineResult{}, fmt.Errorf("AI-Ergebnis wurde verworfen: %w", err)
	}
	return content, AIRefineResult{Provider: cfg.Provider, Model: cfg.Model, PromptPath: promptPath}, nil
}

func loadAIConfig() (AIConfig, error) {
	cfg := AIConfig{}
	path := strings.TrimSpace(os.Getenv("UPDATE_CLI_AI_CONFIG"))
	if path == "" {
		configRoot := strings.TrimSpace(os.Getenv("UPDATE_CLI_CONFIG_PATH"))
		if configRoot == "" {
			configRoot = "/usr/local/etc/update-cli"
		}
		path = filepath.Join(configRoot, "ai.json")
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("AI-Konfiguration ungültig (%s): %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	override := func(env string, target *string) {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			*target = v
		}
	}
	override("UPDATE_CLI_AI_PROVIDER", &cfg.Provider)
	override("UPDATE_CLI_AI_BASE_URL", &cfg.BaseURL)
	override("UPDATE_CLI_AI_MODEL", &cfg.Model)
	override("UPDATE_CLI_AI_API_KEY", &cfg.APIKey)
	override("UPDATE_CLI_AI_API_KEY_ENV", &cfg.APIKeyEnv)
	if cfg.APIKey == "" && cfg.APIKeyEnv != "" {
		cfg.APIKey = strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	}
	if cfg.APIKey == "" {
		cfg.APIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if cfg.Provider == "" {
		if strings.TrimSpace(os.Getenv("OLLAMA_HOST")) != "" {
			cfg.Provider = "ollama"
		} else {
			cfg.Provider = "openai-compatible"
		}
	}
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Model == "" {
		return cfg, errors.New("--with-ai benötigt ein Modell: UPDATE_CLI_AI_MODEL setzen oder /usr/local/etc/update-cli/ai.json konfigurieren")
	}
	switch cfg.Provider {
	case "ollama":
		if cfg.BaseURL == "" {
			cfg.BaseURL = strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
		}
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://localhost:11434"
		}
	case "openai", "openai-compatible", "nvidia":
		if cfg.BaseURL == "" {
			return cfg, errors.New("--with-ai benötigt UPDATE_CLI_AI_BASE_URL oder ai.json baseUrl für OpenAI-kompatible Provider")
		}
	default:
		return cfg, fmt.Errorf("unbekannter AI-Provider %q; unterstützt: ollama, openai-compatible, nvidia", cfg.Provider)
	}
	return cfg, nil
}

func loadSetupScriptPrompt() (string, string, error) {
	if explicit := strings.TrimSpace(os.Getenv("UPDATE_CLI_AI_PROMPT")); explicit != "" {
		data, err := os.ReadFile(explicit)
		if err != nil {
			return "", explicit, err
		}
		return string(data), explicit, nil
	}
	configRoot := strings.TrimSpace(os.Getenv("UPDATE_CLI_CONFIG_PATH"))
	if configRoot == "" {
		configRoot = "/usr/local/etc/update-cli"
	}
	path := filepath.Join(configRoot, "prompts", "setup-script-to-yaml.txt")
	if data, err := os.ReadFile(path); err == nil {
		return string(data), path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", path, err
	}
	return embeddedSetupScriptAIPrompt, "", nil
}

func callAI(ctx context.Context, cfg AIConfig, prompt string) (string, error) {
	timeout := 2 * time.Minute
	if strings.TrimSpace(cfg.Timeout) != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		} else {
			return "", fmt.Errorf("AI timeout ungültig: %w", err)
		}
	}
	client := &http.Client{Timeout: timeout}
	if cfg.Provider == "ollama" {
		body := map[string]any{
			"model":    cfg.Model,
			"stream":   false,
			"messages": []map[string]string{{"role": "user", "content": prompt}},
		}
		data, _ := json.Marshal(body)
		url := strings.TrimRight(cfg.BaseURL, "/") + "/api/chat"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("Ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
		}
		var out struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(payload, &out); err != nil {
			return "", err
		}
		return out.Message.Content, nil
	}
	body := map[string]any{
		"model":       cfg.Model,
		"temperature": 0.1,
		"messages": []map[string]string{
			{"role": "system", "content": "Return only valid Update CLI update-cli.yaml schemaVersion 2 YAML."},
			{"role": "user", "content": prompt},
		},
	}
	data, _ := json.Marshal(body)
	base := strings.TrimRight(cfg.BaseURL, "/")
	url := base
	if !strings.HasSuffix(url, "/chat/completions") {
		url += "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("AI-Antwort enthält keine choices")
	}
	return out.Choices[0].Message.Content, nil
}

func normalizeAIYAML(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "```") {
		lines := strings.Split(value, "\n")
		if len(lines) > 0 {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		value = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return value + "\n"
}
