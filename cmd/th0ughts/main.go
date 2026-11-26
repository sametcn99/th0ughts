// Command th0ughts runs the web server, remote CLI, or stdio MCP adapter.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/server"
	"github.com/sametcn99/th0ughts/storage"
)

type options struct {
	dir, listen, base, driver, address, auth, lang, file string
	json                                                 bool
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	var o options
	flag.StringVar(&o.dir, "data", "data", "Persistent data directory")
	flag.StringVar(&o.listen, "listen", ":8080", "HTTP listen address")
	flag.StringVar(&o.base, "url", "http://localhost:8080", "Remote site URL")
	flag.StringVar(&o.driver, "driver", "", "Initial storage driver")
	flag.StringVar(&o.address, "address", "", "Initial database address")
	flag.StringVar(&o.auth, "credentials", "", "File containing a scoped token or saved owner session")
	flag.StringVar(&o.lang, "lang", "en", "Output language")
	flag.StringVar(&o.file, "file", "", "File to upload, restore or download")
	flag.BoolVar(&o.json, "json", false, "Machine-readable output")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), server.Labels(o.lang)["cli_help"])
		labels := server.Labels(o.lang)
		for _, name := range append([]string{"serve", "setup", "setup-storage", "recover", "theme-reset", "login", "mcp", "upload", "backup", "restore"}, server.CommandNames()...) {
			description := labels["cli_"+name]
			if description == "" {
				description = labels[name]
			}
			fmt.Fprintf(flag.CommandLine.Output(), "  %-16s %s\n", name, description)
		}
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(flag.CommandLine.Output(), "  -%-15s %s\n", f.Name, labels["cli_"+f.Name])
		})
	}
	flag.Parse()
	cmd := flag.Arg(0)
	if cmd == "" {
		flag.Usage()
		return nil
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if cmd == "setup-storage" {
		return prepare(o)
	}
	if cmd == "serve" || cmd == "recover" || cmd == "theme-reset" {
		c := storage.Config{Driver: o.driver, Address: o.address}
		if c.Address == "" {
			switch c.Driver {
			case "json":
				c.Address = filepath.Join(o.dir, "feed.json")
			case "json-dir":
				c.Address = filepath.Join(o.dir, "records")
			case "sqlite":
				c.Address = filepath.Join(o.dir, "feed.db")
			}
		}
		s, e := server.Open(ctx, o.dir, os.Getenv("TH0UGHTS_MASTER_KEY"), c)
		if e != nil {
			return e
		}
		defer s.Close()
		if cmd == "recover" {
			b, e := io.ReadAll(io.LimitReader(os.Stdin, 258))
			if e != nil {
				return e
			}
			return s.Recover(ctx, strings.TrimSpace(string(b)))
		}
		if cmd == "theme-reset" {
			v, e := s.S.Settings(ctx)
			if e != nil {
				return e
			}
			v.Theme = "paper"
			if _, e = s.T.Install(ctx, feed.Actor{Owner: true}, s.DefaultTheme(), ""); e != nil {
				return e
			}
			return s.S.SaveSettings(ctx, feed.Actor{Owner: true}, v)
		}
		httpd := http.Server{Addr: o.listen, Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 3 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
		go func() {
			<-ctx.Done()
			c, done := context.WithTimeout(context.Background(), 10*time.Second)
			defer done()
			httpd.Shutdown(c)
		}()
		fmt.Fprintln(os.Stderr, "Listening on", o.listen)
		e = httpd.ListenAndServe()
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	}
	jar, _ := cookiejar.New(nil)
	client := http.Client{Jar: jar, Timeout: 3 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(ctx context.Context, route string, r io.Reader, method string) (*http.Response, error) {
		req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(o.base, "/")+route, r)
		if e != nil {
			return nil, e
		}
		req.Header.Set("X-Th0ughts-Request", "1")
		req.Header.Set("Content-Type", "application/json")
		if o.auth != "" && cmd != "login" && cmd != "setup" {
			b, e := os.ReadFile(o.auth)
			if e != nil {
				return nil, e
			}
			v := strings.TrimSpace(string(b))
			if strings.HasPrefix(v, "session=") || strings.HasPrefix(v, "__Host-session=") {
				req.Header.Set("Cookie", v)
			} else {
				req.Header.Set("Authorization", "Bearer "+v)
			}
		}
		resp, e := client.Do(req)
		if e != nil {
			return nil, e
		}
		if resp.StatusCode >= 300 {
			defer resp.Body.Close()
			var v map[string]string
			json.NewDecoder(resp.Body).Decode(&v)
			if label := server.Labels(o.lang)[v["error"]]; label != "" {
				return nil, errors.New(label)
			}
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return resp, nil
	}
	remote := func(ctx context.Context, name string, raw json.RawMessage) (any, error) {
		if len(raw) == 0 {
			raw = []byte("{}")
		}
		resp, e := call(ctx, "/api/v1/"+name, bytes.NewReader(raw), "POST")
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		var out any
		e = json.NewDecoder(resp.Body).Decode(&out)
		return out, e
	}
	if cmd == "mcp" {
		res, e := call(ctx, "/api/v1/tools?lang="+url.QueryEscape(o.lang), nil, "GET")
		if e != nil {
			return e
		}
		defer res.Body.Close()
		var tools []struct{ Name string }
		if e = json.NewDecoder(res.Body).Decode(&tools); e != nil {
			return e
		}
		names := map[string]bool{}
		for _, t := range tools {
			names[t.Name] = true
		}
		return server.RemoteMCP(o.lang, names, remote).Run(ctx, &mcp.StdioTransport{})
	}
	if cmd == "login" || cmd == "setup" {
		if o.auth == "" {
			return errors.New(server.Labels(o.lang)["cli_need_credentials"])
		}
		var reader io.Reader = os.Stdin
		route := "/auth/login"
		if cmd == "setup" {
			var in struct{ Master, Password, Driver, Address string }
			if e := json.NewDecoder(io.LimitReader(os.Stdin, 65536)).Decode(&in); e != nil {
				return feed.Invalid
			}
			b, _ := json.Marshal(map[string]string{"master": in.Master})
			res, e := call(ctx, "/auth/bootstrap", bytes.NewReader(b), "POST")
			if e != nil {
				return e
			}
			res.Body.Close()
			in.Master = ""
			b, _ = json.Marshal(in)
			reader = bytes.NewReader(b)
			route = "/auth/setup"
		}
		res, e := call(ctx, route, reader, "POST")
		if e != nil {
			return e
		}
		defer res.Body.Close()
		for _, c := range res.Cookies() {
			if c.Name == "session" || c.Name == "__Host-session" {
				return storage.Atomic(o.auth, []byte(c.Name+"="+c.Value))
			}
		}
		return feed.Denied
	}
	if cmd == "backup" || cmd == "restore" || cmd == "upload" {
		if o.file == "" {
			return errors.New(server.Labels(o.lang)["cli_need_file"])
		}
		var r io.Reader
		method := "GET"
		if cmd != "backup" {
			f, e := os.Open(o.file)
			if e != nil {
				return e
			}
			defer f.Close()
			r = f
			method = "POST"
		}
		resp, e := call(ctx, "/api/v1/"+cmd, r, method)
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		if cmd == "backup" {
			f, e := os.OpenFile(o.file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			_, e = io.Copy(f, resp.Body)
			closeErr := f.Close()
			if e != nil {
				os.Remove(o.file)
				return e
			}
			return closeErr
		}
		_, e = io.Copy(os.Stdout, resp.Body)
		return e
	}
	if !slices.Contains(server.CommandNames(), cmd) {
		return errors.New(server.Labels(o.lang)["cli_unknown"])
	}
	var raw []byte
	if flag.NArg() > 1 {
		raw = []byte(flag.Arg(1))
	} else if stat, _ := os.Stdin.Stat(); stat.Mode()&os.ModeCharDevice == 0 {
		raw, _ = io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	}
	v, e := remote(ctx, cmd, raw)
	if e != nil {
		return e
	}
	enc := json.NewEncoder(os.Stdout)
	if !o.json {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}
func prepare(o options) error {
	key := os.Getenv("TH0UGHTS_MASTER_KEY")
	v, e := feed.NewVault(key)
	if e != nil {
		return e
	}
	p := filepath.Join(o.dir, "connection.enc")
	secretFile := filepath.Join(o.file, "db-password")
	b, e := os.ReadFile(secretFile)
	if errors.Is(e, os.ErrNotExist) {
		b = []byte(feed.ID(32))
		if e = storage.Atomic(secretFile, b); e != nil {
			return e
		}
		if e = os.Chmod(secretFile, 0444); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}
	adminFile := filepath.Join(o.file, "admin-password")
	if _, err := os.Stat(adminFile); errors.Is(err, os.ErrNotExist) {
		if err = storage.Atomic(adminFile, []byte(feed.ID(32))); err != nil {
			return err
		}
		if err = os.Chmod(adminFile, 0444); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, e = os.Stat(p); e == nil {
		return nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	secret := url.QueryEscape(string(b))
	addr := ""
	switch o.driver {
	case "postgres":
		addr = "postgres://th0ughts:" + secret + "@db:5432/th0ughts?sslmode=disable"
	case "mysql", "mariadb":
		addr = "th0ughts:" + secret + "@tcp(db:3306)/th0ughts"
	case "mongodb":
		addr = "mongodb://th0ughts:" + secret + "@db:27017/th0ughts?authSource=th0ughts"
	default:
		return feed.Invalid
	}
	raw, _ := json.Marshal(storage.Config{Driver: o.driver, Address: addr})
	return storage.Atomic(p, v.Seal("database", raw))
}
