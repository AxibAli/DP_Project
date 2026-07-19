// Package middleware enforces authentication and role-based authorization
// as chi-compatible HTTP middleware. It is the single, centralized place
// where "is this request logged in" and "is this request allowed to do X"
// are decided — handlers never need to re-implement those checks.
package middleware

import (
	"encoding/json"
	"net/http"

	"livestock/internal/auth"
	"livestock/internal/models"
)

const (
	SessionCookieName = "session_token"
	CSRFCookieName    = "csrf_token"
	CSRFHeaderName    = "X-CSRF-Token"
)

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Authenticate validates the session cookie against the database on every
// request. A missing, unknown, or expired session is rejected with 401.
// On success the authenticated user and session are attached to the
// request context for downstream handlers and RequireRole.
func Authenticate(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			user, session, err := svc.ValidateSession(cookie.Value)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "session is invalid or expired")
				return
			}

			ctx := withUser(r.Context(), user)
			ctx = withSession(ctx, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole enforces that the authenticated user (attached by
// Authenticate, which must run first) has exactly the given role.
// Anything else — including roles the client might claim in a request
// body — is never trusted; only the role stored in the DB and loaded by
// ValidateSession is checked here.
func RequireRole(role models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if user.Role != role {
				writeJSONError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF implements the double-submit-cookie pattern: on any state-changing
// request (anything but GET/HEAD/OPTIONS), the client must echo the value
// of its (non-HttpOnly) csrf_token cookie back in the X-CSRF-Token header.
// A cross-site form or script can trigger the request and thus resend the
// cookie automatically, but it cannot read the cookie's value to put it in
// the header (same-origin policy), so forged requests fail this check.
func CSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(CSRFCookieName)
			if err != nil || cookie.Value == "" {
				writeJSONError(w, http.StatusForbidden, "missing CSRF token")
				return
			}
			header := r.Header.Get(CSRFHeaderName)
			if header == "" || header != cookie.Value {
				writeJSONError(w, http.StatusForbidden, "invalid CSRF token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
