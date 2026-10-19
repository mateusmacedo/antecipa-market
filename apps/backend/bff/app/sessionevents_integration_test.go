//go:build integration

package app_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

const backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"

func (r *fakeRealm) logoutToken(t *testing.T, subject, session string) string {
	t.Helper()
	now := time.Now()
	claims := map[string]any{
		"iss": r.issuer, "aud": "dmpf-portal", "sub": subject, "sid": session,
		"iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix(), "jti": unique("logout"), "typ": "Logout",
		"events": map[string]any{backchannelLogoutEvent: map[string]any{}},
	}
	segment := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal() = %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	signed := segment(map[string]string{"alg": "RS256", "kid": realmKeyID, "typ": "JWT"}) + "." + segment(claims)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, r.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() = %v", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (r *fakeRealm) serve(t *testing.T, path string, events any) {
	t.Helper()
	body, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("Marshal() = %v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.served[path] = body
}

func apiRevocations(t *testing.T, redisURL string) authn.RevocationChecker {
	t.Helper()
	redisCfg, err := redis.FromEnv(lookup("REDIS_URL", redisURL))
	if err != nil {
		t.Fatalf("redis.FromEnv() = %v", err)
	}
	cfg, closeStore, err := app.WithRevocations(app.Config{Redis: redisCfg})
	if err != nil {
		t.Fatalf("WithRevocations() = %v", err)
	}
	t.Cleanup(func() { _ = closeStore() })
	return cfg.Auth.Revocations
}

func requireNotRevoked(t *testing.T, store authn.RevocationChecker, subject, session string) {
	t.Helper()
	if kind, err := store.Revoked(context.Background(), authn.SessionRef{Subject: ports.SubjectID(subject), ID: session}); err != nil || kind != "" {
		t.Fatalf("Revoked() before the event = %q, %v, want not revoked", kind, err)
	}
}

func requireSessionRevoked(t *testing.T, store authn.RevocationChecker, subject, session string) {
	t.Helper()
	ref := authn.SessionRef{Subject: ports.SubjectID(subject), ID: session, IssuedAt: time.Now()}
	var kind authn.RevocationKind
	waitFor(t, "session "+session+" in the deny-list the API reads", func() bool {
		var err error
		if kind, err = store.Revoked(context.Background(), ref); err != nil {
			t.Fatalf("Revoked() = %v", err)
		}
		return kind == authn.RevocationSession
	})
}

func TestTheSessionEventsRoleFeedsTheDenyListTheAPIReads(t *testing.T) {
	redisURL := tb.Env(t, "REDIS_URL")
	realm := newFakeRealm(t)
	store := apiRevocations(t, redisURL)
	none := []map[string]any{}

	t.Run("a LOGOUT read by the polling", func(t *testing.T) {
		subject, session := unique("user"), unique("sid")
		requireNotRevoked(t, store, subject, session)
		realm.serve(t, adminEventsPath, none)
		realm.serve(t, logoutEventsPath, []map[string]any{{
			"id": unique("event"), "time": time.Now().UnixMilli(), "type": "LOGOUT", "userId": subject, "sessionId": session,
		}})
		e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer, "REDIS_URL", redisURL)

		requireSessionRevoked(t, store, subject, session)
		e.stop(t)
	})

	t.Run("a session removed through the admin console read by the polling", func(t *testing.T) {
		subject, session := unique("user"), unique("sid")
		requireNotRevoked(t, store, subject, session)
		realm.serve(t, logoutEventsPath, none)
		realm.serve(t, adminEventsPath, []map[string]any{{
			"id": unique("admin-event"), "time": time.Now().UnixMilli(), "operationType": "DELETE",
			"resourceType": "USER_SESSION", "resourcePath": "sessions/" + session,
		}})
		e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer, "REDIS_URL", redisURL)

		requireSessionRevoked(t, store, subject, session)
		e.stop(t)
	})

	t.Run("a logout token accepted by the receiver", func(t *testing.T) {
		realm.serve(t, adminEventsPath, none)
		realm.serve(t, logoutEventsPath, none)
		subject, session := unique("user"), unique("sid")
		requireNotRevoked(t, store, subject, session)
		e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer, "REDIS_URL", redisURL)

		if status := postForm(t, e.addr, app.BackchannelLogoutPath, url.Values{"logout_token": {realm.logoutToken(t, subject, session)}}); status != http.StatusOK {
			t.Fatalf("POST %s with a valid logout token = %d, want 200", app.BackchannelLogoutPath, status)
		}
		requireSessionRevoked(t, store, subject, session)
		e.stop(t)
	})
}
