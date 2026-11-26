package server

import (
	"context"
	"errors"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/updates"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCommandsRequireOwner(t *testing.T) {
	s := fixture(t, "sqlite")
	for _, op := range []string{"update_status", "check_update", "install_update"} {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/"+op, strings.NewReader("{}")))
		if w.Code != 403 {
			t.Fatal(op, w.Code)
		}
	}
	a, _ := owner(t, s)
	if err := s.installUpdate(context.Background(), a, "v2.0.0"); !errors.Is(err, updates.ErrUnsupported) {
		t.Fatal(err)
	}
	s.EnableManagedUpdates()
	if err := s.installUpdate(context.Background(), feed.Actor{Owner: true}, "v2.0.0"); !errors.Is(err, feed.Denied) {
		t.Fatal(err)
	}
}
