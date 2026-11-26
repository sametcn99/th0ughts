package updates

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/mod/semver"
)

type build struct{ Tag, Hash string }

func (m *Manager) path(b build) string { return filepath.Join(m.dir, "th0ughts-"+b.Tag+"-"+b.Hash) }
func (m *Manager) read(name string) (build, error) {
	var b build
	f, err := os.Open(filepath.Join(m.dir, name))
	if err != nil {
		return b, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 2049))
	if err != nil || len(data) > 2048 {
		return b, ErrInstall
	}
	// The three pointers share a label so atomic renames can advance a build.
	raw, err := m.vault.Open("th0ughts-release", data)
	if err != nil || json.Unmarshal(raw, &b) != nil || !stable(b.Tag) {
		return build{}, ErrInstall
	}
	hash, err := hex.DecodeString(b.Hash)
	if err != nil || len(hash) != sha256.Size {
		return build{}, ErrInstall
	}
	return b, nil
}
func (m *Manager) write(name string, b build) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return storage.Atomic(filepath.Join(m.dir, name), m.vault.Seal("th0ughts-release", data))
}
func (m *Manager) verified(b build) bool {
	f, err := os.Open(m.path(b))
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxAsset+1))
	return err == nil && n > 0 && n <= maxAsset && fmt.Sprintf("%x", h.Sum(nil)) == b.Hash
}
func (m *Manager) LockLauncher() (*flock.Flock, error) {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(m.dir, "launcher.lock"))
	ok, err := lock.TryLock()
	if err == nil && !ok {
		err = errors.New("managed launcher already running")
	}
	return lock, err
}

type Boot struct {
	Path, Version string
	Trial         bool
}

// Boots returns the candidate, the last successful release and the bundled
// executable in fallback order. A claimed trial is attempted once across restarts.
func (m *Manager) Boots(bundled string) ([]Boot, error) {
	boots := []Boot{{Path: bundled, Version: Version()}}
	if b, err := m.read("active"); err == nil && semver.Compare(b.Tag, Version()) > 0 && m.verified(b) {
		boots = append([]Boot{{Path: m.path(b), Version: b.Tag}}, boots...)
	}
	b, err := m.read("pending")
	if errors.Is(err, os.ErrNotExist) {
		return boots, nil
	}
	if err != nil {
		return boots, err
	}
	if err = os.Rename(filepath.Join(m.dir, "pending"), filepath.Join(m.dir, "trial")); err != nil {
		return boots, err
	}
	if semver.Compare(b.Tag, boots[0].Version) > 0 && m.verified(b) {
		boots = append([]Boot{{Path: m.path(b), Version: b.Tag, Trial: true}}, boots...)
	}
	return boots, nil
}

func (m *Manager) Confirm(boot Boot) error {
	if !boot.Trial {
		return nil
	}
	b, err := m.read("trial")
	if err != nil {
		return err
	}
	if m.path(b) != boot.Path || b.Tag != boot.Version {
		return ErrInstall
	}
	return os.Rename(filepath.Join(m.dir, "trial"), filepath.Join(m.dir, "active"))
}

// Prune only updater-owned binaries while the launcher has no running worker.
func (m *Manager) Prune() {
	keep := map[string]bool{}
	for _, name := range []string{"active", "pending", "trial"} {
		if b, err := m.read(name); err == nil {
			keep[filepath.Base(m.path(b))] = true
		}
	}
	files, _ := os.ReadDir(m.dir)
	for _, f := range files {
		if !f.IsDir() && !keep[f.Name()] && (strings.HasPrefix(f.Name(), "th0ughts-v") || strings.HasPrefix(f.Name(), ".download-")) {
			os.Remove(filepath.Join(m.dir, f.Name()))
		}
	}
}
