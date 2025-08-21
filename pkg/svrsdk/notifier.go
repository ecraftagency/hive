package svrsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AgentNotifier gửi shutdown event tới Agent REST API
type AgentNotifier struct {
	HTTP *http.Client
}

func (n *AgentNotifier) client() *http.Client {
	if n.HTTP != nil {
		return n.HTTP
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func (n *AgentNotifier) Notify(ctx context.Context, cfg Config, ev ShutdownEvent) error {
	if cfg.RoomID == "" || cfg.Token == "" || cfg.AgentBaseURL == "" {
		return fmt.Errorf("missing required config for notifier")
	}
	body, _ := json.Marshal(ev)
	url := fmt.Sprintf("%s/rooms/%s/shutdown", cfg.AgentBaseURL, cfg.RoomID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	resp, err := n.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("agent returned status %d", resp.StatusCode)
	}
	return nil
}
