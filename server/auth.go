package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pquerna/otp/totp"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
)

func cookie(r *http.Request, name string) string {
	c, e := r.Cookie(cookieName(r, name))
	if e != nil {
		return ""
	}
	return c.Value
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	token, e := s.A.Session(r.Context())
	if e != nil {
		fail(w, e)
		return
	}
	if raw := cookie(r, "session"); raw != "" {
		if e = s.S.DB.Delete(r.Context(), "session", feed.Digest(raw)); e != nil {
			fail(w, e)
			return
		}
	}
	s.setCookie(w, r, "session", token, 43200)
	respond(w, map[string]bool{"ok": true})
}
func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	op := r.PathValue("op")
	actor := s.actor(r)
	if op == "logout" {
		if actor.Owner {
			if e := s.S.DB.Delete(ctx, "session", actor.Session); e != nil {
				fail(w, e)
				return
			}
		}
		s.setCookie(w, r, "session", "", -1)
		respond(w, true)
		return
	}
	if op == "passkey-register" || op == "passkey-login" || op == "register-finish" || op == "login-finish" {
		s.passkey(w, r, op, actor)
		return
	}
	if op == "otp-begin" {
		if !actor.Recent() {
			fail(w, feed.Denied)
			return
		}
		k, e := totp.Generate(totp.GenerateOpts{Issuer: "th0ughts", AccountName: "owner"})
		if e != nil {
			fail(w, e)
			return
		}
		s.setCookie(w, r, "otp", s.issueTicket(k.Secret(), actor.Session), 300)
		respond(w, map[string]string{"secret": k.Secret(), "url": k.URL()})
		return
	}
	var in struct{ Password, Code, Master, Driver, Address string }
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	switch op {
	case "bootstrap":
		if !s.allowedAttempt(r) {
			fail(w, feed.Denied)
			return
		}
		_, e := s.A.Account(ctx)
		if !errors.Is(e, storage.ErrMissing) || !s.masterOK(in.Master) {
			fail(w, feed.Denied)
			return
		}
		s.setCookie(w, r, "bootstrap", s.issueTicket(true, "setup"), 300)
		respond(w, true)
	case "setup":
		if _, ok := s.ticket(cookie(r, "bootstrap"), "setup", false); !ok {
			fail(w, feed.Denied)
			return
		}
		if _, e := s.A.Account(ctx); !errors.Is(e, storage.ErrMissing) {
			fail(w, feed.Conflict)
			return
		}
		if len(in.Password) < 12 || len(in.Password) > 256 {
			fail(w, feed.Invalid)
			return
		}
		if in.Driver != "" && in.Driver != s.cfg.Driver || in.Address != "" && in.Address != s.cfg.Address {
			addr := in.Address
			switch in.Driver {
			case "sqlite", "json", "json-dir":
				addr = filepath.Join(s.dir, map[string]string{"sqlite": "feed.db", "json": "feed.json", "json-dir": "records"}[in.Driver])
			}
			if e := s.connect(ctx, storage.Config{Driver: in.Driver, Address: addr}); e != nil {
				fail(w, feed.Invalid)
				return
			}
		}
		if e := s.A.Setup(ctx, in.Password, false); e != nil {
			fail(w, e)
			return
		}
		s.ticket(cookie(r, "bootstrap"), "setup", true)
		s.setCookie(w, r, "bootstrap", "", -1)
		s.login(w, r)
	case "login":
		if !s.allowedAttempt(r) {
			fail(w, feed.Denied)
			return
		}
		if e := s.A.Check(ctx, in.Password, in.Code); e != nil {
			fail(w, e)
			return
		}
		s.login(w, r)
	case "otp-finish":
		v, ok := s.ticket(cookie(r, "otp"), actor.Session, true)
		if !ok {
			fail(w, feed.Denied)
			return
		}
		codes, e := s.A.EnableOTP(ctx, actor, v.(string), in.Code)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, codes)
	default:
		fail(w, storage.ErrMissing)
	}
}
func (s *Server) passkey(w http.ResponseWriter, r *http.Request, op string, user feed.Actor) {
	register := op == "passkey-register" || op == "register-finish"
	if register && !user.Recent() || !register && !s.allowedAttempt(r) {
		fail(w, feed.Denied)
		return
	}
	u, e := s.A.Account(r.Context())
	if e != nil {
		fail(w, feed.Denied)
		return
	}
	origin := s.origin(r)
	o, e := url.Parse(origin)
	if e != nil {
		fail(w, feed.Invalid)
		return
	}
	if net.ParseIP(o.Hostname()) != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"passkey_domain"}`))
		return
	}
	wa, e := webauthn.New(&webauthn.Config{RPDisplayName: "th0ughts", RPID: o.Hostname(), RPOrigins: []string{origin}, AuthenticatorSelection: protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}})
	if e != nil {
		fail(w, e)
		return
	}
	binding := "passkey-login:" + u.ID
	if register {
		binding = "passkey-register:" + user.Session
	}
	if op == "passkey-login" || op == "passkey-register" {
		var data *webauthn.SessionData
		var opt any
		if register {
			opt, data, e = wa.BeginRegistration(u)
		} else {
			opt, data, e = wa.BeginLogin(u, webauthn.WithUserVerification(protocol.VerificationRequired))
		}
		if e != nil {
			fail(w, feed.Denied)
			return
		}
		s.setCookie(w, r, "ceremony", s.issueTicket(data, binding), 300)
		respond(w, opt)
		return
	}
	v, ok := s.ticket(cookie(r, "ceremony"), binding, true)
	if !ok {
		fail(w, feed.Denied)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var cred *webauthn.Credential
	if register {
		cred, e = wa.FinishRegistration(u, *v.(*webauthn.SessionData), r)
	} else {
		cred, e = wa.FinishLogin(u, *v.(*webauthn.SessionData), r)
	}
	if e != nil || cred.Authenticator.CloneWarning {
		fail(w, feed.Denied)
		return
	}
	if e = s.A.Credential(r.Context(), *cred, u.ID); e != nil {
		fail(w, e)
		return
	}
	if register {
		respond(w, true)
	} else {
		s.login(w, r)
	}
}
func (s *Server) Recover(ctx context.Context, password string) error {
	return s.A.Setup(ctx, password, true)
}
