package feed

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sametcn99/th0ughts/storage"
)

const hookConfigKey = "comment-webhook"

type CommentWebhookSettings struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	Secret  string `json:"secret,omitempty"`
}

type webhookConfig struct {
	Settings   CommentWebhookSettings
	Generation string
}

type deliveryReceipt struct {
	Generation string `json:"generation,omitempty"`
	State      string `json:"state"`
	At         int64  `json:"at"`
	Code       int    `json:"code,omitempty"`
	Attempt    int    `json:"attempt"`
}

type WebhookInfo struct {
	Enabled   bool             `json:"enabled"`
	URL       string           `json:"url"`
	HasSecret bool             `json:"hasSecret"`
	Last      *deliveryReceipt `json:"last,omitempty"`
}

func (l *Library) hookConfig(ctx context.Context) (webhookConfig, error) {
	var cfg webhookConfig
	sealed, err := Read[[]byte](ctx, l.Service.DB, "config", hookConfigKey)
	if errors.Is(err, storage.ErrMissing) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	raw, err := l.Vault.Open(hookConfigKey, sealed)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(raw, &cfg)
	return cfg, err
}

func (l *Library) CommentWebhook(ctx context.Context, a Actor) (WebhookInfo, error) {
	var info WebhookInfo
	if !a.Owner {
		return info, Denied
	}
	cfg, err := l.hookConfig(ctx)
	if err != nil {
		return info, err
	}
	info.Enabled, info.URL, info.HasSecret = cfg.Settings.Enabled, cfg.Settings.URL, cfg.Settings.Secret != ""
	last, err := Read[deliveryReceipt](ctx, l.Service.DB, "system", "webhook-last")
	if errors.Is(err, storage.ErrMissing) {
		return info, nil
	}
	if err == nil && last.Generation == cfg.Generation {
		last.Generation = ""
		info.Last = &last
	}
	return info, err
}

func (l *Library) ConfigureCommentWebhook(ctx context.Context, a Actor, settings CommentWebhookSettings) error {
	if !a.Recent() {
		return Denied
	}
	s := l.Service
	// Finish any in-flight request before acknowledging a destination or key change.
	s.hookDelivery.Lock()
	defer s.hookDelivery.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := l.hookConfig(ctx)
	if err != nil {
		return err
	}
	settings.URL = strings.TrimSpace(settings.URL)
	if settings.URL != "" {
		u, err := url.ParseRequestURI(settings.URL)
		if err != nil || len(settings.URL) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || strings.Contains(settings.URL, "#") {
			return Invalid
		}
	} else if settings.Enabled || settings.Secret != "" {
		return Invalid
	}
	if settings.URL != "" && old.Settings.URL == settings.URL && settings.Secret == "" {
		settings.Secret = old.Settings.Secret
	}
	if settings.URL != "" && (len(settings.Secret) < 32 || len(settings.Secret) > 512) {
		return Invalid
	}
	generation := old.Generation
	if old.Settings.URL != settings.URL || old.Settings.Enabled != settings.Enabled {
		generation = ID(16)
	}
	raw, err := json.Marshal(webhookConfig{Settings: settings, Generation: generation})
	if err != nil {
		return err
	}
	return Write(ctx, s.DB, "config", hookConfigKey, l.Vault.Seal(hookConfigKey, raw), 0, false)
}

type commentEvent struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       Comment   `json:"data"`
}
type webhookJob struct {
	Event      commentEvent
	Generation string
	Attempts   int
}

// The intent and the comment write share the service lock. On restart the
// observed comment state distinguishes a committed mutation from an abandoned one.
func (s *Service) resolveHookIntent(ctx context.Context) error {
	job, err := Read[webhookJob](ctx, s.DB, "system", "webhook-intent")
	if errors.Is(err, storage.ErrMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	ev := job.Event
	c, err := Read[Comment](ctx, s.DB, "comment", ev.Data.ID)
	missing := errors.Is(err, storage.ErrMissing)
	if err != nil && !missing {
		return err
	}
	committed := !missing && c.Status == ev.Data.Status || missing && ev.Type == "comment.deleted"
	if committed {
		// Delivery always resolves the intent before consuming queue entries.
		if err = Write(ctx, s.DB, "hook_queue", ev.ID, job, -ev.OccurredAt.UnixMilli(), false); err != nil {
			return err
		}
	}
	return s.DB.Delete(ctx, "system", "webhook-intent")
}

// The worker or the next mutation promotes this intent after the caller releases the lock.
func (l *Library) stageCommentEvent(ctx context.Context, kind string, c Comment) error {
	s := l.Service
	if err := s.resolveHookIntent(ctx); err != nil {
		return err
	}
	cfg, err := l.hookConfig(ctx)
	if err != nil || !cfg.Settings.Enabled {
		return err
	}
	ev := commentEvent{ID: ID(16), Type: kind, OccurredAt: time.Now().UTC(), Data: c}
	err = Write(ctx, s.DB, "system", "webhook-intent", webhookJob{Generation: cfg.Generation, Event: ev}, 0, false)
	if err == nil {
		select {
		case s.hookWake <- struct{}{}:
		default:
		}
	}
	return err
}

func (s *Service) WebhookWake() <-chan struct{} { return s.hookWake }

func sendWebhook(ctx context.Context, transport http.RoundTripper, cfg CommentWebhookSettings, ev commentEvent) int {
	body, err := json.Marshal(ev)
	if err != nil {
		return 0
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return 0
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(cfg.Secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Th0ughts-Timestamp", ts)
	req.Header.Set("X-Th0ughts-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
	client := http.Client{Transport: transport, Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

var webhookDelays = [...]time.Duration{10 * time.Second, time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour}

// DeliverCommentWebhook runs one delivery for a single dispatcher. A zero wait
// means more work may be ready. The transport may be nil to use system TLS.
func (l *Library) DeliverCommentWebhook(ctx context.Context, transport http.RoundTripper) (time.Duration, error) {
	s := l.Service
	s.hookDelivery.Lock()
	defer s.hookDelivery.Unlock()
	s.mu.Lock()
	err := s.resolveHookIntent(ctx)
	s.mu.Unlock()
	if err != nil {
		return 0, err
	}
	rows, err := s.DB.List(ctx, storage.Query{Kind: "hook_queue", Limit: 1})
	if err != nil || len(rows) == 0 {
		return time.Minute, err
	}
	r := rows[0]
	if delay := time.Until(time.UnixMilli(-r.Rank)); delay > 0 {
		return delay, nil
	}
	var job webhookJob
	if err = json.Unmarshal(r.Data, &job); err != nil {
		return 0, err
	}
	cfg, err := l.hookConfig(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.Settings.Enabled || cfg.Generation != job.Generation {
		return 0, s.DB.Delete(ctx, "hook_queue", r.ID)
	}
	code := sendWebhook(ctx, transport, cfg.Settings, job.Event)
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	job.Attempts++
	last := deliveryReceipt{Generation: cfg.Generation, At: time.Now().UnixMilli(), Code: code, Attempt: job.Attempts, State: "delivered"}
	if code < 200 || code >= 300 {
		last.State = "failed"
		if job.Attempts <= len(webhookDelays) {
			last.State = "retrying"
		}
	}
	if err = Write(ctx, s.DB, "system", "webhook-last", last, 0, false); err != nil {
		return 0, err
	}
	if last.State == "retrying" {
		next := time.Now().Add(webhookDelays[job.Attempts-1])
		return 0, Write(ctx, s.DB, "hook_queue", r.ID, job, -next.UnixMilli(), false)
	}
	return 0, s.DB.Delete(ctx, "hook_queue", r.ID)
}
