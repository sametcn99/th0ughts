// Package updates checks official th0ughts releases and stages verified Linux binaries.
package updates

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sametcn99/th0ughts/feed"
	"golang.org/x/mod/semver"
)

//go:embed VERSION
var embeddedVersion string

func Version() string { return strings.TrimSpace(embeddedVersion) }

const Repository = "https://github.com/sametcn99/th0ughts"
const latestURL = "https://api.github.com/repos/sametcn99/th0ughts/releases/latest"
const maxAsset = 128 << 20

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func stable(tag string) bool { return len(tag) <= 64 && tagPattern.MatchString(tag) }
func Supported() bool {
	return runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64")
}
func assetName() string { return "th0ughts-" + runtime.GOOS + "-" + runtime.GOARCH }

var ErrCheck = errors.New("release_check_failed")
var ErrInstall = errors.New("release_install_failed")
var ErrChanged = errors.New("release_changed")
var ErrUnsupported = errors.New("release_unmanaged")

type Manager struct {
	dir       string
	vault     feed.Vault
	client    *http.Client
	checkMu   sync.Mutex
	cached    release
	expires   time.Time
	installMu sync.Mutex
}

func New(dir string, vault feed.Vault) *Manager {
	return &Manager{dir: filepath.Join(dir, "updates"), vault: vault, client: &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Hostname()
		if len(via) > 5 || req.URL.Scheme != "https" || req.URL.User != nil || req.URL.Port() != "" || !(host == "github.com" || host == "release-assets.githubusercontent.com" || host == "objects.githubusercontent.com") {
			return errors.New("unexpected release redirect")
		}
		return nil
	}}}
}

type asset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}
type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

func (r release) binary() (asset, bool) {
	for _, a := range r.Assets {
		if a.Name != assetName() {
			continue
		}
		hash, err := hex.DecodeString(strings.TrimPrefix(a.Digest, "sha256:"))
		if strings.HasPrefix(a.Digest, "sha256:") && err == nil && len(hash) == sha256.Size && a.Size > 0 && a.Size <= maxAsset {
			return a, true
		}
	}
	return asset{}, false
}
func (m *Manager) latest(ctx context.Context, refresh bool) (release, error) {
	m.checkMu.Lock()
	defer m.checkMu.Unlock()
	if !refresh && time.Now().Before(m.expires) {
		return m.cached, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return release{}, ErrCheck
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "th0ughts/"+Version())
	resp, err := m.client.Do(req)
	if err != nil {
		return release{}, ErrCheck
	}
	defer resp.Body.Close()
	var r release
	switch resp.StatusCode {
	case http.StatusOK:
		raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		if err != nil || len(raw) > 1<<20 || json.Unmarshal(raw, &r) != nil || !stable(r.Tag) || r.Draft || r.Prerelease {
			return release{}, ErrCheck
		}
	case http.StatusNotFound: // A new public repository has no stable release yet.
	default:
		return release{}, ErrCheck
	}
	m.cached = r
	m.expires = time.Now().Add(5 * time.Minute)
	return r, nil
}

type Status struct {
	Current     string `json:"current"`
	Managed     bool   `json:"managed"`
	Latest      string `json:"latest,omitempty"`
	Available   bool   `json:"available"`
	Installable bool   `json:"installable"`
	Failed      string `json:"failed,omitempty"`
}

func (m *Manager) Status(managed bool) Status {
	s := Status{Current: Version(), Managed: managed && Supported()}
	if s.Managed {
		if trial, err := m.read("trial"); err == nil && semver.Compare(trial.Tag, s.Current) > 0 {
			s.Failed = trial.Tag
		}
	}
	return s
}
func (m *Manager) Check(ctx context.Context, managed bool) (Status, error) {
	s := m.Status(managed)
	r, err := m.latest(ctx, false)
	if err != nil {
		return s, err
	}
	s.Latest = r.Tag
	s.Available = semver.Compare(r.Tag, s.Current) > 0
	_, hasAsset := r.binary()
	s.Installable = s.Available && s.Managed && hasAsset
	return s, nil
}

// Stage accepts only the exact stable version previously presented to the owner.
// It constructs the download URL locally and verifies both byte count and digest.
func (m *Manager) Stage(ctx context.Context, tag string) error {
	if !Supported() {
		return ErrUnsupported
	}
	if !m.installMu.TryLock() {
		return feed.Conflict
	}
	defer m.installMu.Unlock()
	if _, err := os.Stat(filepath.Join(m.dir, "pending")); err == nil {
		return feed.Conflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrInstall
	}
	if !stable(tag) || semver.Compare(tag, Version()) <= 0 {
		return ErrChanged
	}
	r, err := m.latest(ctx, true)
	if err != nil {
		return err
	}
	a, ok := r.binary()
	if r.Tag != tag || !ok {
		return ErrChanged
	}
	if err = os.MkdirAll(m.dir, 0700); err != nil {
		return ErrInstall
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, Repository+"/releases/download/"+tag+"/"+assetName(), nil)
	if err != nil {
		return ErrInstall
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return ErrInstall
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength >= 0 && resp.ContentLength != a.Size {
		return ErrInstall
	}
	tmp, err := os.CreateTemp(m.dir, ".download-")
	if err != nil {
		return ErrInstall
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, a.Size+1))
	sum := fmt.Sprintf("%x", hash.Sum(nil))
	if err != nil || n != a.Size || "sha256:"+sum != a.Digest {
		return ErrInstall
	}
	if tmp.Chmod(0500) != nil || tmp.Sync() != nil || tmp.Close() != nil {
		return ErrInstall
	}
	record := build{Tag: tag, Hash: sum}
	if err = os.Rename(tmp.Name(), m.path(record)); err != nil {
		return ErrInstall
	}
	if err = m.write("pending", record); err != nil {
		return ErrInstall
	}
	return nil
}
