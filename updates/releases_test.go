package updates

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/sametcn99/th0ughts/feed"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestReleaseIntegrityAndSelection(t *testing.T) {
	if !Supported() {
		t.Skip("Linux release target")
	}
	vault, _ := feed.NewVault(feed.ID(32))
	m := New(t.TempDir(), vault)
	ctx := context.Background()
	content := []byte("fixture bytes")
	tag := "v9000.0.0"
	metadata := release{Tag: tag, Assets: []asset{{Name: assetName(), Size: int64(len(content)), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(content))}}}
	status := 200
	requests := 0
	m.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		var body []byte
		switch req.URL.String() {
		case latestURL:
			requests++
			body, _ = json.Marshal(metadata)
		case Repository + "/releases/download/" + tag + "/" + assetName():
			body = content
		default:
			t.Fatalf("unexpected URL %s", req.URL)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: http.Header{}}, nil
	})
	info, err := m.Check(ctx, false)
	if err != nil || !info.Available || info.Installable {
		t.Fatal(info, err)
	}
	info, err = m.Check(ctx, true)
	if err != nil || !info.Installable || requests != 1 {
		t.Fatal(info, err, requests)
	}
	if err = m.Stage(ctx, "v8000.0.0"); !errors.Is(err, ErrChanged) {
		t.Fatal(err)
	}
	content[0] = 'X'
	if err = m.Stage(ctx, tag); !errors.Is(err, ErrInstall) {
		t.Fatal("changed payload accepted", err)
	}
	if _, err = m.read("pending"); !os.IsNotExist(err) {
		t.Fatal("invalid pending file", err)
	}
	content[0] = 'f'
	if err = m.Stage(ctx, tag); err != nil {
		t.Fatal(err)
	}
	boots, err := m.Boots("bundled")
	if err != nil || len(boots) != 2 || !boots[0].Trial {
		t.Fatal(boots, err)
	}
	if fallback, err := m.Boots("bundled"); err != nil || len(fallback) != 1 || fallback[0].Path != "bundled" {
		t.Fatal(fallback, err)
	}
	if err = m.Confirm(boots[0]); err != nil {
		t.Fatal(err)
	}
	boots, err = m.Boots("bundled")
	if err != nil || boots[0].Trial || boots[0].Version != tag {
		t.Fatal(boots, err)
	}
	if err = os.Chmod(boots[0].Path, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(boots[0].Path, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if boots, err = m.Boots("bundled"); err != nil || boots[0].Path != "bundled" {
		t.Fatal("ran modified binary", boots, err)
	}
	status = 404
	r, err := m.latest(ctx, true)
	if err != nil || r.Tag != "" {
		t.Fatal(r, err)
	}
	status = 403
	if _, err = m.latest(ctx, true); !errors.Is(err, ErrCheck) {
		t.Fatal(err)
	}
	status = 200
	for _, bad := range []string{"v2.0.0-beta.1", "v01.2.3", "v1", "v2.0.0/other"} {
		metadata.Tag = bad
		if _, err = m.latest(ctx, true); !errors.Is(err, ErrCheck) {
			t.Fatal(bad, err)
		}
	}
	metadata.Tag = tag
	metadata.Prerelease = true
	if _, err = m.latest(ctx, true); !errors.Is(err, ErrCheck) {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "https://other.example.com/"+strings.Repeat("x", 10), nil)
	if m.client.CheckRedirect(req, nil) == nil {
		t.Fatal("followed external redirect")
	}
}
