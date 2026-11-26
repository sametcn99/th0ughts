package server

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sametcn99/th0ughts/feed"
)

type testMailDNS struct {
	mx      []*net.MX
	mxErr   error
	ips     []net.IPAddr
	ipErr   error
	queries []string
}

func (d *testMailDNS) LookupMX(_ context.Context, host string) ([]*net.MX, error) {
	d.queries = append(d.queries, "MX "+host)
	return d.mx, d.mxErr
}
func (d *testMailDNS) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	d.queries = append(d.queries, "IP "+host)
	return d.ips, d.ipErr
}

func TestCommentMailRoutes(t *testing.T) {
	missing := &net.DNSError{IsNotFound: true}
	temporary := &net.DNSError{IsTemporary: true}
	for _, tc := range []struct {
		name     string
		dns      testMailDNS
		want     error
		lookupIP bool
	}{
		{"explicit MX", testMailDNS{mx: []*net.MX{{Host: "mx.example.test."}}}, nil, false},
		{"null MX with address", testMailDNS{mx: []*net.MX{{Host: "."}}, ips: []net.IPAddr{{IP: net.IPv4(192, 0, 2, 5)}}}, invalidEmail, false},
		{"implicit IPv4 MX", testMailDNS{mxErr: missing, ips: []net.IPAddr{{IP: net.IPv4(192, 0, 2, 5)}}}, nil, true},
		{"implicit IPv6 MX", testMailDNS{ips: []net.IPAddr{{IP: net.ParseIP("2001:db8::5")}}}, nil, true},
		{"missing domain", testMailDNS{mxErr: missing, ipErr: missing}, invalidEmail, true},
		{"MX lookup unavailable", testMailDNS{mxErr: temporary}, dnsUnavailable, false},
		{"fallback timeout", testMailDNS{mxErr: missing, ipErr: context.DeadlineExceeded}, dnsUnavailable, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address, err := mailbox("Guest@BÜCHER.example")
			if err != nil {
				t.Fatal(err)
			}
			err = checkMailDomain(context.Background(), address, &tc.dns)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v, want %v", err, tc.want)
			}
			want := "MX xn--bcher-kva.example."
			if tc.lookupIP {
				want += " IP xn--bcher-kva.example."
			}
			if got := strings.Join(tc.dns.queries, " "); got != want {
				t.Fatalf("DNS queries %q, want %q", got, want)
			}
		})
	}
}

func TestGuestCommentEmailPolicies(t *testing.T) {
	ctx := context.Background()
	s := fixture(t, "sqlite")
	a, _ := owner(t, s)
	site, err := s.S.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	site.Comments.Mode = "open"
	if site.Comments.RequireEmailCode {
		t.Fatal("new sites should use DNS only")
	}
	if err = s.S.SaveSettings(ctx, a, site); err != nil {
		t.Fatal(err)
	}
	p, err := s.S.Save(ctx, a, feed.Post{Original: "en", Text: map[string]feed.Translation{"en": {Body: "Comments", Published: true}}})
	if err != nil {
		t.Fatal(err)
	}
	dns := &testMailDNS{}
	s.guest.resolver = dns
	post := func(email string) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		form.WriteField("body", "Test comment")
		if email != "" {
			form.WriteField("email", email)
		}
		form.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/comments/"+p.ID, &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		return w
	}
	code := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/comment-code", strings.NewReader(`{"post":"`+p.ID+`","email":"guest@example.test"}`))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		return w
	}
	if w := code(); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, email := range []string{"", "bogus", "guest@example.test"} {
		dns.mx = []*net.MX{{Host: "."}}
		if w := post(email); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_email") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	dns.mx = nil
	dns.mxErr = context.DeadlineExceeded
	if w := post("guest@example.test"); w.Code != 503 || !strings.Contains(w.Body.String(), "comment_dns_unavailable") {
		t.Fatal(w.Code, w.Body.String())
	}
	q := feed.CommentQuery{Post: p.ID}
	items, err := s.S.CommentList(ctx, a, q, false)
	if err != nil || len(items.Items) != 0 {
		t.Fatal(items, err)
	}
	dns.mxErr = nil
	dns.mx = []*net.MX{{Host: "mail.example.test."}}
	if w := post("guest@example.test"); w.Code != 200 || strings.Contains(w.Body.String(), "guest@example.test") {
		t.Fatal(w.Code, w.Body.String())
	}
	site.Comments.RequireEmailCode = true
	if err = s.S.SaveSettings(ctx, a, site); err != nil {
		t.Fatal(err)
	}
	if w := post("guest@example.test"); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_comment_code") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = s.L.SubmitDomainCheckedComment(ctx, p.ID, "", "", "ignored", "text", nil); !errors.Is(err, feed.EmailCodeRequired) {
		t.Fatal(err)
	}
	dns.mx = []*net.MX{{Host: "."}}
	if w := code(); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_email") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestDNSCommentRequestBudget(t *testing.T) {
	var g guestMail
	for i := 0; i < 12; i++ {
		if err := g.allowRequest("192.0.2.1:123"); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(g.allowRequest("192.0.2.1:456"), commentRate) {
		t.Fatal("source port reset the limit")
	}
	if err := g.allowRequest("192.0.2.2:123"); err != nil {
		t.Fatal(err)
	}
}
