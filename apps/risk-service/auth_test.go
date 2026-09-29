package main

// Tests for control-plane operator authentication.
// Run: go test -run 'TestResolve|TestVerify|TestMint|TestControlAuth' -v

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-that-is-long-enough-0123456789"

func setTestSecret(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)
}

// ── Secret resolution ─────────────────────────────────────────────────────────

func TestResolveOperatorSecret(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty", "", true},
		{"default change-me", "change-me", true},
		{"default change-me-before-production", "change-me-before-production", true},
		{"dev-secret", "dev-secret", true},
		{"too short", "short-secret-123", true},
		{"31 chars rejected", strings.Repeat("a", 31), true},
		{"32 chars accepted", strings.Repeat("a", 32), false},
		{"strong secret accepted", testSecret, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", tc.value)
			_, err := resolveOperatorSecret()
			if (err != nil) != tc.wantErr {
				t.Fatalf("resolveOperatorSecret(%q) err=%v, wantErr=%v", tc.value, err, tc.wantErr)
			}
		})
	}
}

func TestResolveOperatorSecretWhitespaceTrimmed(t *testing.T) {
	t.Setenv("JWT_SECRET", "  change-me  ")
	if _, err := resolveOperatorSecret(); err == nil {
		t.Fatal("expected denylisted value with whitespace to be rejected")
	}
}

// ── Token mint / verify ───────────────────────────────────────────────────────

func TestMintAndVerifyRoundtrip(t *testing.T) {
	for _, role := range []string{"operator", "admin"} {
		tok, err := mintOperatorToken(testSecret, "devon", role, []string{"risk-service"}, time.Hour)
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		sub, rl, err := verifyOperatorToken(testSecret, tok)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if sub != "devon" || rl.String() != role {
			t.Fatalf("got sub=%q role=%q, want devon/%s", sub, rl, role)
		}
	}
}

func TestVerifyRejects(t *testing.T) {
	good, _ := mintOperatorToken(testSecret, "devon", "operator", []string{"risk-service"}, time.Hour)

	cases := []struct {
		name  string
		token func() string
	}{
		{"empty", func() string { return "" }},
		{"garbage", func() string { return "not-a-token" }},
		{"two parts", func() string { return "a.b" }},
		{"bad base64 segment", func() string { return "%%%.%%%.%%%" }},
		{"tampered signature", func() string {
			p := strings.Split(good, ".")
			return p[0] + "." + p[1] + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}},
		{"wrong secret", func() string {
			tok, _ := mintOperatorToken(strings.Repeat("b", 32), "devon", "operator", []string{"risk-service"}, time.Hour)
			return tok
		}},
		{"expired", func() string {
			tok, _ := mintOperatorToken(testSecret, "devon", "operator", []string{"risk-service"}, -time.Hour)
			return tok
		}},
		{"nbf in the future", func() string {
			now := time.Now()
			payload, _ := json.Marshal(operatorClaims{
				Sub: "devon", Role: "operator",
				Aud: json.RawMessage(`["risk-service"]`),
				Exp: now.Add(time.Hour).Unix(),
				Nbf: now.Add(time.Hour).Unix(),
				Iat: now.Unix(),
			})
			return mintRawToken(testSecret, `{"alg":"HS256","typ":"JWT"}`, string(payload))
		}},
		{"iat in the future", func() string {
			now := time.Now()
			payload, _ := json.Marshal(operatorClaims{
				Sub: "devon", Role: "operator",
				Aud: json.RawMessage(`["risk-service"]`),
				Exp: now.Add(2 * time.Hour).Unix(),
				Iat: now.Add(time.Hour).Unix(),
			})
			return mintRawToken(testSecret, `{"alg":"HS256","typ":"JWT"}`, string(payload))
		}},
		{"unknown role", func() string {
			tok, _ := mintOperatorToken(testSecret, "devon", "superuser", []string{"risk-service"}, time.Hour)
			return tok
		}},
		{"empty sub", func() string {
			tok, _ := mintOperatorToken(testSecret, "", "operator", []string{"risk-service"}, time.Hour)
			return tok
		}},
		{"alg none", func() string {
			// Craft an unsigned token: header claims alg=none.
			p := strings.Split(good, ".")
			return "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + p[1] + "."
		}},
		{"alg RS256 rejected", func() string {
			// Alg-confusion attempt: header says RS256, body HMAC-signed with
			// the real secret. There is no RSA verification path, and the
			// pinned-alg check rejects the header before signature comparison.
			now := time.Now()
			payload, _ := json.Marshal(operatorClaims{
				Sub: "devon", Role: "admin",
				Aud: json.RawMessage(`["risk-service"]`),
				Exp: now.Add(time.Hour).Unix(),
				Iat: now.Unix(),
			})
			return mintRawToken(testSecret, `{"alg":"RS256","typ":"JWT"}`, string(payload))
		}},
		{"unexpected typ", func() string {
			now := time.Now()
			payload, _ := json.Marshal(operatorClaims{
				Sub: "devon", Role: "operator",
				Aud: json.RawMessage(`["risk-service"]`),
				Exp: now.Add(time.Hour).Unix(),
				Iat: now.Unix(),
			})
			return mintRawToken(testSecret, `{"alg":"HS256","typ":"at+jwt"}`, string(payload))
		}},
		{"missing aud", func() string {
			tok, _ := mintOperatorToken(testSecret, "devon", "operator", nil, time.Hour)
			return tok
		}},
		{"aud for wrong service", func() string {
			// A websocket-gateway token must not work on the risk-service.
			tok, _ := mintOperatorToken(testSecret, "devon", "admin", []string{"ws-gateway"}, time.Hour)
			return tok
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := verifyOperatorToken(testSecret, tc.token()); err == nil {
				t.Fatal("expected verification to fail")
			}
		})
	}
}

func TestVerifyAcceptsAudVariants(t *testing.T) {
	now := time.Now()
	mk := func(audJSON string) string {
		payload, _ := json.Marshal(operatorClaims{
			Sub: "devon", Role: "operator",
			Aud: json.RawMessage(audJSON),
			Exp: now.Add(time.Hour).Unix(),
			Iat: now.Unix(),
		})
		return mintRawToken(testSecret, `{"alg":"HS256","typ":"JWT"}`, string(payload))
	}
	// aud as a bare string is valid per RFC 7519.
	if _, _, err := verifyOperatorToken(testSecret, mk(`"risk-service"`)); err != nil {
		t.Fatalf("string aud rejected: %v", err)
	}
	// aud as a list containing risk-service alongside others is valid.
	if _, _, err := verifyOperatorToken(testSecret, mk(`["ws-gateway","risk-service"]`)); err != nil {
		t.Fatalf("list aud rejected: %v", err)
	}
	// absent typ is accepted (typ is informational).
	payload, _ := json.Marshal(operatorClaims{
		Sub: "devon", Role: "operator",
		Aud: json.RawMessage(`["risk-service"]`),
		Exp: now.Add(time.Hour).Unix(),
		Iat: now.Unix(),
	})
	if _, _, err := verifyOperatorToken(testSecret, mintRawToken(testSecret, `{"alg":"HS256"}`, string(payload))); err != nil {
		t.Fatalf("missing typ rejected: %v", err)
	}
}

// ── Middleware ────────────────────────────────────────────────────────────────

func testEngine() *RiskEngine {
	return &RiskEngine{redis: newFakeRedis()}
}

func authedRequest(t *testing.T, method, path, token string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return httptest.NewRecorder(), req
}

func TestControlAuthMiddleware(t *testing.T) {
	setTestSecret(t)
	secret, err := resolveOperatorSecret()
	if err != nil {
		t.Fatal(err)
	}
	engine := testEngine()

	operatorTok, _ := mintOperatorToken(secret, "op1", "operator", []string{"risk-service"}, time.Hour)
	adminTok, _ := mintOperatorToken(secret, "admin1", "admin", []string{"risk-service"}, time.Hour)
	expiredTok, _ := mintOperatorToken(secret, "op1", "operator", []string{"risk-service"}, -time.Hour)
	// A gateway token (aud ws-gateway) must be rejected by the risk-service.
	gatewayTok, _ := mintOperatorToken(secret, "gw1", "admin", []string{"ws-gateway"}, time.Hour)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	cases := []struct {
		name     string
		action   string
		minRole  roleLevel
		method   string
		token    string
		wantCode int
	}{
		{"kill: no token -> 401", "kill", roleOperator, "POST", "", 401},
		{"kill: garbage token -> 401", "kill", roleOperator, "POST", "garbage", 401},
		{"kill: expired token -> 401", "kill", roleOperator, "POST", expiredTok, 401},
		{"kill: GET rejected -> 405", "kill", roleOperator, "GET", operatorTok, 405},
		{"kill: operator allowed", "kill", roleOperator, "POST", operatorTok, 204},
		{"kill: admin allowed", "kill", roleOperator, "POST", adminTok, 204},
		{"reset: operator forbidden -> 403", "reset", roleAdmin, "POST", operatorTok, 403},
		{"reset: admin allowed", "reset", roleAdmin, "POST", adminTok, 204},
		{"reset: no token -> 401", "reset", roleAdmin, "POST", "", 401},
		{"kill: gateway token rejected -> 401", "kill", roleOperator, "POST", gatewayTok, 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := requireControlAuth(engine, secret, tc.minRole, tc.action, next)
			rr, req := authedRequest(t, tc.method, "/"+tc.action, tc.token)
			h.ServeHTTP(rr, req)
			if rr.Code != tc.wantCode {
				t.Fatalf("got %d, want %d (body: %s)", rr.Code, tc.wantCode, rr.Body.String())
			}
		})
	}
}

func TestControlAuthQueryTokenRejected(t *testing.T) {
	setTestSecret(t)
	secret, _ := resolveOperatorSecret()
	engine := testEngine()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := requireControlAuth(engine, secret, roleOperator, "kill", next)

	tok, _ := mintOperatorToken(secret, "op1", "operator", []string{"risk-service"}, time.Hour)
	req := httptest.NewRequest("POST", "/kill?token="+tok, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("query token: got %d, want 401 (tokens are header-only)", rr.Code)
	}
}

func TestControlAuthSetsActor(t *testing.T) {
	setTestSecret(t)
	secret, _ := resolveOperatorSecret()
	engine := testEngine()
	var gotActor string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotActor = actorFromContext(r)
	})
	h := requireControlAuth(engine, secret, roleOperator, "kill", next)

	tok, _ := mintOperatorToken(secret, "devon", "operator", []string{"risk-service"}, time.Hour)
	rr, req := authedRequest(t, "POST", "/kill", tok)
	h.ServeHTTP(rr, req)
	if gotActor != "devon" {
		t.Fatalf("actor = %q, want devon", gotActor)
	}
}
