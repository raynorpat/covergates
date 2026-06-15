package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

// SlackNotifier posts a coverage summary to a repo's Slack incoming webhook,
// gated by the repo's notify trigger.
type SlackNotifier struct {
	Repos  core.RepoStore
	Config *config.Config
	Client *http.Client // nil -> http.DefaultClient
}

// Notify posts to the webhook when the trigger passes and a webhook is configured.
func (n *SlackNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	setting, err := n.Repos.Setting(repo)
	if err != nil || setting == nil {
		return nil
	}
	if !shouldNotify(setting.NotifyTrigger, verdict) || setting.SlackWebhook == "" {
		return nil
	}
	text := fmt.Sprintf("%s\n%s", summaryLine(repo, build, verdict), buildURL(n.Config, repo, build))
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.SlackWebhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(ioutil.Discard, resp.Body) // drain so the connection can be reused
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}
