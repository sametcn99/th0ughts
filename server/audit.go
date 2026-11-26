package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type auditedResponse struct {
	http.ResponseWriter
	status int
}

func (w *auditedResponse) WriteHeader(n int)           { w.status = n; w.ResponseWriter.WriteHeader(n) }
func (w *auditedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// The allowlist prevents user-controlled paths and secrets from entering logs.
func auditAction(r *http.Request) string {
	if r.Method == "GET" && r.URL.Path == "/api/v1/backup" {
		return "backup"
	}
	if r.Method == "GET" || r.Method == "HEAD" {
		return ""
	}
	for _, name := range []string{"bootstrap", "setup", "login", "logout", "otp-begin", "otp-finish", "passkey-register", "passkey-login", "register-finish", "login-finish"} {
		if r.URL.Path == "/auth/"+name {
			return name
		}
	}
	for _, name := range []string{"configure_comment_webhook", "save_settings", "configure_media", "create_token", "revoke_token", "install_theme", "publish_theme", "delete_theme", "restore"} {
		if r.URL.Path == "/api/v1/"+name {
			return name
		}
	}
	return ""
}
func logSecurity(action string, w *auditedResponse, r *http.Request, started time.Time) {
	kind := "anonymous"
	if cookie(r, "session") != "" {
		kind = "session"
	}
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		kind = "bearer"
	}
	slog.Info("security event", "action", action, "credential_type", kind, "status", w.status, "duration_ms", time.Since(started).Milliseconds())
}
