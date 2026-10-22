package updater

import (
	"context"
	"errors"
	"fmt"
	"github.com/r14r/update-cli/lib/config"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runHealthcheck(ctx context.Context, c config.Config) error {
	h := c.Healthcheck
	if h.Type == "" || h.Type == "none" {
		return nil
	}
	timeout := time.Duration(h.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	hcCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	switch h.Type {
	case "http":
		client := &http.Client{Timeout: 5 * time.Second}
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		var last error
		for {
			req, err := http.NewRequestWithContext(hcCtx, http.MethodGet, h.URL, nil)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					return nil
				}
				last = fmt.Errorf("HTTP %s", resp.Status)
			} else {
				last = err
			}
			select {
			case <-hcCtx.Done():
				return fmt.Errorf("Healthcheck %s fehlgeschlagen: %v", h.URL, last)
			case <-ticker.C:
			}
		}
	case "command":
		bash, err := exec.LookPath("bash")
		if err != nil {
			return errors.New("Healthcheck command benötigt bash")
		}
		cmd := exec.CommandContext(hcCtx, bash, "-c", h.Command)
		cmd.Dir = c.CurrentDir
		cmd.Env = os.Environ()
		var b strings.Builder
		cmd.Stdout = &b
		cmd.Stderr = &b
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Healthcheck command fehlgeschlagen: %s", strings.TrimSpace(b.String()))
		}
		return nil
	default:
		return fmt.Errorf("unbekannter Healthcheck-Typ %q", h.Type)
	}
}
