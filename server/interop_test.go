package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sametcn99/th0ughts/feed"
)

func TestCLIAndStdio(t *testing.T) {
	bin := os.Getenv("TEST_BINARY")
	if bin == "" {
		t.Skip("set TEST_BINARY to the compiled th0ughts binary")
	}
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	h := httptest.NewServer(s)
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	token, e := s.A.Token(ctx, a, feed.Access{Name: "publisher", Scopes: []string{"posts:write", "posts:read"}, Expires: time.Now().Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "credentials")
	if e = os.WriteFile(path, []byte(token), 0600); e != nil {
		t.Fatal(e)
	}
	cli := func(args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, bin, append([]string{"-url", h.URL, "-credentials", path, "-json"}, args...)...)
	}
	draft := cli("save_post", `{"original":"en","text":{"en":{"body":"Interoperability draft","published":false}}}`)
	b, e := draft.Output()
	if e != nil {
		t.Fatal(e)
	}
	var p feed.Post
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "publisher", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.CommandTransport{Command: cli("mcp")}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	list, e := session.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tool := range list.Tools {
		if tool.Name == "save_settings" {
			t.Fatal("irrelevant scoped tool exposed")
		}
	}
	p.Text["tr"] = feed.Translation{Body: "Birlikte çalışabilirlik", Published: true}
	p.Text["en"] = feed.Translation{Body: "Published through MCP", Published: true}
	res, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "save_post", Arguments: p})
	if e != nil || res.IsError {
		t.Fatal(e, res)
	}
	resp, e := h.Client().Get(h.URL + "/post/" + p.ID + ".md?lang=tr")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(raw), "Birlikte çalışabilirlik") {
		t.Fatal(string(raw))
	}
	if e = s.S.DB.Delete(ctx, "token", feed.Digest(token)); e != nil {
		t.Fatal(e)
	}
	res, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "delete_post", Arguments: map[string]any{"id": p.ID}})
	if e != nil || !res.IsError {
		t.Fatal("revocation did not affect live MCP session", e)
	}
}

type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(req)
}

func TestMCPBearerRevocation(t *testing.T) {
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	token, err := s.A.Token(ctx, a, feed.Access{Name: "read only", Scopes: []string{"posts:read"}, Expires: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(s)
	defer h.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "reader", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: h.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{token}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	args := &mcp.CallToolParams{Name: "list_posts", Arguments: map[string]bool{"private": true}}
	result, err := session.CallTool(ctx, args)
	if err != nil || result.IsError {
		t.Fatal("authorized read", result, err)
	}
	if err = s.S.DB.Delete(ctx, "token", feed.Digest(token)); err != nil {
		t.Fatal(err)
	}
	if result, err = session.CallTool(ctx, args); err == nil && !result.IsError {
		t.Fatal("revoked grant read private timeline")
	}
}
