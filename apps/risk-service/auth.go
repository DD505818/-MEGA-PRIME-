package main

// Control-plane operator authentication for the risk-service.
//
// /kill  requires a valid JWT with role "operator" (or "admin").
// /reset requires a valid JWT with role "admin" — strictly stronger than kill,
// because reset re-arms the system after a kill switch activation.
//
// Tokens are HS256 JWTs signed with JWT_SECRET. The service refuses to start
// unless JWT_SECRET is set to a non-default value of at least 32 characters.
// There are no backdoors: no dev-token bypass, no hardcoded fallbacks.
//
// Algorithm confusion is impossible by construction: the verifier has a
// single code path that always computes HMAC-SHA256 with the configured
// symmetric secret. It never dispatches on the token header's "alg" value,
// so there is no RSA branch an attacker could reach with an RS256 header
// (the classic HMAC-secret-as-RSA-public-key attack). As defense in depth
// the header is still required to say alg=HS256; anything else is rejected
// before signature verification.
//
// Cross-service token reuse is blocked by audience restriction: this service
// only accepts tokens whose "aud" claim contains "risk-service". A token
// minted for the websocket-gateway (aud "ws-gateway") is rejected here even
// though both services share JWT_SECRET. Distinct secrets per service remain
// the stronger option and are recommended before live; aud is the floor.
//
// Fail-closed: missing/invalid/expired tokens -> 401; valid token with
// insufficient role -> 403. Denied attempts are audit-logged.
//
// Token TTL policy: see docs/operator-token-policy.md (referenced from here
// so the policy lives next to the enforcement).

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type roleLevel int

const (
	roleNone     roleLevel = 0
	roleOperator roleLevel = 1
	roleAdmin    roleLevel = 2
)

func (r roleLevel) String() string {
	switch r {
	case roleOperator:
		return "operator"
	case roleAdmin:
		return "admin"
	default:
		return "none"
	}
}

func parseRole(s string) roleLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "operator":
		return roleOperator
	case "admin":
		return roleAdmin
	default:
		return roleNone
	}
}

// operatorSecretDenylist contains values that must never be accepted as
// JWT_SECRET. These are historical defaults that shipped in this repo.
var operatorSecretDenylist = map[string]bool{
	"":                            true,
	"change-me":                   true,
	"change-me-before-production": true,
	"dev-secret":                  true,
	"secret":                      true,
	"password":                    true,
}

const minOperatorSecretLen = 32

// resolveOperatorSecret returns the JWT secret or an error describing why the
// process must not start. Callers should log.Fatalf on error.
func resolveOperatorSecret() (string, error) {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if s == "" {
		return "", fmt.Errorf("JWT_SECRET is not set — refusing to start control plane without operator auth (see .env.example)")
	}
	if operatorSecretDenylist[s] {
		return "", fmt.Errorf("JWT_SECRET uses a forbidden default value — generate a fresh secret (see .env.example)")
	}
	if len(s) < minOperatorSecretLen {
		return "", fmt.Errorf("JWT_SECRET must be at least %d characters (got %d)", minOperatorSecretLen, len(s))
	}
	return s, nil
}

// riskServiceAudience is the audience this service requires in operator
// tokens. It blocks cross-service token reuse (e.g. a websocket-gateway
// token being replayed against /kill or /reset).
const riskServiceAudience = "risk-service"

// operatorClaims are the JWT claims this service understands.
type operatorClaims struct {
	Sub  string          `json:"sub"`
	Role string          `json:"role"`
	Aud  json.RawMessage `json:"aud"`
	Exp  int64           `json:"exp"`
	Nbf  int64           `json:"nbf,omitempty"`
	Iat  int64           `json:"iat,omitempty"`
}

// audienceContains reports whether an "aud" claim (string or array of
// strings, per RFC 7519) contains the required audience.
func audienceContains(raw json.RawMessage, want string) bool {
	if len(raw) == 0 {
		return false
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single == want
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, a := range list {
			if a == want {
				return true
			}
		}
	}
	return false
}

// verifyOperatorToken validates an HS256 JWT and returns (sub, roleLevel).
// Rejects: malformed tokens, non-HS256 alg (including "none"), unexpected typ,
// bad signature, missing/expired exp, nbf in the future, iat in the future,
// unknown role, empty sub, missing/wrong aud.
func verifyOperatorToken(secret, token string) (string, roleLevel, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", roleNone, fmt.Errorf("malformed token")
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return "", roleNone, fmt.Errorf("bad token header: %w", err)
	}
	// Pin the algorithm. The signature is ALWAYS verified as HMAC-SHA256 with
	// the configured secret; the header's alg is never used to select a
	// verification method, so an RS256 (or "none") header cannot route into an
	// RSA code path — there is no RSA code path.
	if header.Alg != "HS256" {
		return "", roleNone, fmt.Errorf("unexpected signing algorithm %q", header.Alg)
	}
	// typ is informational; a present-but-unexpected value is a forgery smell.
	if header.Typ != "" && !strings.EqualFold(header.Typ, "JWT") {
		return "", roleNone, fmt.Errorf("unexpected token type %q", header.Typ)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", roleNone, fmt.Errorf("bad token signature encoding: %w", err)
	}
	if !hmac.Equal(sig, expected) {
		return "", roleNone, fmt.Errorf("invalid token signature")
	}

	var claims operatorClaims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return "", roleNone, fmt.Errorf("bad token claims: %w", err)
	}
	if claims.Exp == 0 {
		return "", roleNone, fmt.Errorf("token missing exp")
	}
	// 60s leeway for clock skew.
	now := time.Now().Unix()
	if now > claims.Exp+60 {
		return "", roleNone, fmt.Errorf("token expired")
	}
	if claims.Nbf != 0 && now < claims.Nbf-60 {
		return "", roleNone, fmt.Errorf("token not yet valid")
	}
	if claims.Iat != 0 && now < claims.Iat-60 {
		return "", roleNone, fmt.Errorf("token issued in the future")
	}
	if !audienceContains(claims.Aud, riskServiceAudience) {
		return "", roleNone, fmt.Errorf("token audience does not include %q", riskServiceAudience)
	}
	role := parseRole(claims.Role)
	if role == roleNone {
		return "", roleNone, fmt.Errorf("token has unknown or missing role %q", claims.Role)
	}
	if strings.TrimSpace(claims.Sub) == "" {
		return "", roleNone, fmt.Errorf("token missing sub")
	}
	return claims.Sub, role, nil
}

func decodeSegment(seg string, v interface{}) error {
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// mintRawToken builds a token from arbitrary header/payload JSON, signing the
// result with HMAC-SHA256. It exists so tests can craft adversarial tokens
// (wrong alg, wrong typ, missing aud, future nbf/iat) that the normal minter
// would never produce.
func mintRawToken(secret, headerJSON, payloadJSON string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + payload + "." + sig
}

// mintOperatorToken creates an HS256 operator JWT for the given audiences.
// Used by tests and (via the python mint script for operators) out-of-band
// issuance. Tokens are short-lived; see docs/operator-token-policy.md.
func mintOperatorToken(secret, sub, role string, aud []string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := operatorClaims{
		Sub:  sub,
		Role: role,
		Exp:  now.Add(ttl).Unix(),
		Iat:  now.Unix(),
	}
	if len(aud) > 0 {
		raw, err := json.Marshal(aud)
		if err != nil {
			return "", err
		}
		claims.Aud = raw
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return mintRawToken(secret, `{"alg":"HS256","typ":"JWT"}`, string(payloadBytes)), nil
}

type ctxKey string

const actorCtxKey ctxKey = "operator_actor"

// actorFromContext returns the authenticated operator subject, or "".
func actorFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(actorCtxKey).(string); ok {
		return v
	}
	return ""
}

// extractBearerToken reads the token from Authorization: Bearer only.
// Tokens are never accepted via query parameters: URLs are logged by proxies,
// browsers, and servers, and a control-plane credential must not appear in them.
func extractBearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := strings.CutPrefix(h, "Bearer "); ok && strings.TrimSpace(rest) != "" {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func writeAuthJSON(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// requireControlAuth enforces operator authentication and minimum role on a
// control-plane endpoint. Denied attempts are audit-logged. Only POST is
// accepted (405 otherwise).
func requireControlAuth(engine *RiskEngine, secret string, minRole roleLevel, action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAuthJSON(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := extractBearerToken(r)
		if token == "" {
			engine.recordControlAudit(action, "unknown", false, "denied", "missing token")
			writeAuthJSON(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sub, role, err := verifyOperatorToken(secret, token)
		if err != nil {
			engine.recordControlAudit(action, "unknown", false, "denied", "invalid token: "+err.Error())
			writeAuthJSON(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if role < minRole {
			engine.recordControlAudit(action, sub, false, "denied", "insufficient role "+role.String())
			writeAuthJSON(w, http.StatusForbidden, "forbidden")
			return
		}
		ctx := context.WithValue(r.Context(), actorCtxKey, sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
