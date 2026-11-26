package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
	theme "github.com/sametcn99/th0ughts/themes"
)

type Command struct {
	Name   string                                                                   `json:"name"`
	Scope  string                                                                   `json:"-"`
	Read   bool                                                                     `json:"-"`
	Schema *jsonschema.Schema                                                       `json:"inputSchema"`
	Run    func(context.Context, *Server, feed.Actor, json.RawMessage) (any, error) `json:"-"`
}

func command[I any](name, scope string, read bool, fn func(context.Context, *Server, feed.Actor, I) (any, error)) Command {
	schema, e := jsonschema.For[I](nil)
	if e != nil {
		panic(e)
	}
	return Command{name, scope, read, schema, func(ctx context.Context, s *Server, a feed.Actor, b json.RawMessage) (any, error) {
		if scope == "owner" && !a.Owner || scope != "" && scope != "owner" && !a.Can(scope) {
			return nil, feed.Denied
		}
		var x I
		if len(b) > 0 {
			d := json.NewDecoder(bytes.NewReader(b))
			d.DisallowUnknownFields()
			if e := d.Decode(&x); e != nil {
				return nil, feed.Invalid
			}
		}
		return fn(ctx, s, a, x)
	}}
}

type empty struct{}
type identify struct {
	ID      string `json:"id"`
	Private bool   `json:"private,omitempty"`
}
type themeInput struct {
	Source   string `json:"source,omitempty"`
	Rollback string `json:"rollback,omitempty"`
}

var commands = []Command{
	command("list_posts", "", true, func(c context.Context, s *Server, a feed.Actor, x feed.Filter) (any, error) { return s.S.List(c, a, x) }),
	command("get_post", "", true, func(c context.Context, s *Server, a feed.Actor, x identify) (any, error) {
		return s.S.Get(c, a, x.ID, x.Private)
	}),
	command("save_post", "posts:write", false, func(c context.Context, s *Server, a feed.Actor, x feed.Post) (any, error) { return s.S.Save(c, a, x) }),
	command("delete_post", "posts:write", false, func(c context.Context, s *Server, a feed.Actor, x identify) (any, error) {
		return true, s.S.Delete(c, a, x.ID)
	}),
	command("get_settings", "", true, func(c context.Context, s *Server, a feed.Actor, _ empty) (any, error) { return s.S.Settings(c) }),
	command("save_settings", "settings", false, func(c context.Context, s *Server, a feed.Actor, x feed.Settings) (any, error) {
		if _, e := s.T.Load(c, x.Theme); e != nil {
			return nil, feed.Invalid
		}
		return true, s.S.SaveSettings(c, a, x)
	}),
	command("list_themes", "themes", true, func(c context.Context, s *Server, a feed.Actor, _ empty) (any, error) { return s.T.List(c) }),
	command("preview_theme", "owner", false, func(_ context.Context, s *Server, a feed.Actor, x struct {
		Source string `json:"source"`
	}) (any, error) { design, err := theme.Compile(x.Source); if err != nil {
		return nil, feed.Invalid
	}; token := s.issueTicket(design, "preview:"+a.Session); return map[string]any{"theme": design, "ticket": token}, nil }),
	command("install_theme", "themes", false, func(c context.Context, s *Server, a feed.Actor, x themeInput) (any, error) {
		return s.T.Install(c, a, x.Source, x.Rollback)
	}),
	command("create_token", "owner", false, func(c context.Context, s *Server, a feed.Actor, x feed.Access) (any, error) {
		return s.A.Token(c, a, x)
	}),
	command("list_tokens", "owner", true, func(c context.Context, s *Server, a feed.Actor, _ empty) (any, error) {
		rows, e := s.S.DB.List(c, storage.Query{Kind: "token"})
		out := []any{}
		for _, r := range rows {
			var t feed.Access
			if e := json.Unmarshal(r.Data, &t); e != nil {
				return nil, e
			}
			out = append(out, map[string]any{"id": r.ID, "grant": t})
		}
		return out, e
	}),
	command("revoke_token", "owner", false, func(c context.Context, s *Server, a feed.Actor, x identify) (any, error) {
		if !a.Recent() {
			return nil, feed.Denied
		}
		return true, s.S.DB.Delete(c, "token", x.ID)
	}),
	command("configure_media", "owner", false, func(c context.Context, s *Server, a feed.Actor, x feed.S3) (any, error) {
		return true, s.L.Configure(c, a, x)
	}),
}

func visible(a feed.Actor, op Command) bool {
	return op.Scope == "" || a.Owner || op.Scope != "owner" && a.Can(op.Scope)
}
func (s *Server) operation(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("op")
	for _, op := range commands {
		if op.Name != name {
			continue
		}
		if r.Method == "GET" && !op.Read {
			w.WriteHeader(405)
			return
		}
		var raw json.RawMessage
		if r.Method == "GET" {
			raw = []byte(r.URL.Query().Get("input"))
		} else if e := decode(w, r, &raw); e != nil {
			fail(w, e)
			return
		}
		v, e := op.Run(r.Context(), s, s.actor(r), raw)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, v)
		return
	}
	fail(w, storage.ErrMissing)
}
func MCP(list []Command, labels map[string]string, call func(context.Context, string, json.RawMessage) (any, error)) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "th0ughts", Version: "1"}, nil)
	for _, op := range list {
		srv.AddTool(&mcp.Tool{Name: op.Name, Description: labels[op.Name], InputSchema: op.Schema}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			v, e := call(ctx, op.Name, req.Params.Arguments)
			if e != nil {
				_, code := errorStatus(e)
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: labels[code]}}}, nil
			}
			b, e := json.Marshal(v)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, e
		})
	}
	return srv
}
func (s *Server) mcpHTTP(w http.ResponseWriter, r *http.Request) {
	a := s.actor(r)
	if r.Header.Get("Authorization") != "" && a.Session == "" {
		fail(w, feed.Denied)
		return
	}
	l := r.URL.Query().Get("lang")
	if !feed.Locale(l) {
		l = "en"
	}
	allowed := []Command{}
	for _, op := range commands {
		if visible(a, op) {
			allowed = append(allowed, op)
		}
	}
	srv := MCP(allowed, s.catalog[l], func(ctx context.Context, name string, b json.RawMessage) (any, error) {
		for _, op := range allowed {
			if op.Name == name {
				return op.Run(ctx, s, a, b)
			}
		}
		return nil, feed.Denied
	})
	mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20}).ServeHTTP(w, r)
}
func CommandNames() []string {
	var names []string
	for _, op := range commands {
		names = append(names, op.Name)
	}
	sort.Strings(names)
	return names
}
func RemoteMCP(lang string, names map[string]bool, call func(context.Context, string, json.RawMessage) (any, error)) *mcp.Server {
	list := []Command{}
	for _, op := range commands {
		if names[op.Name] {
			list = append(list, op)
		}
	}
	return MCP(list, Labels(lang), call)
}
func Labels(lang string) map[string]string {
	if !feed.Locale(lang) {
		lang = "en"
	}
	b, _ := assets.ReadFile("locales/" + lang + ".json")
	var m map[string]string
	json.Unmarshal(b, &m)
	return m
}
