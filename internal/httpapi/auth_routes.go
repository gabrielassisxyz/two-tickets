package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/respond"
)

const (
	// SessionCookieName is free to choose: the frontend never reads it, it
	// only forwards it on server-side calls.
	SessionCookieName = "two_tickets_session"

	loginBodyLimitBytes = 8 << 10
	// loginFailureMessage mirrors Seerr's own answer to a failed local login.
	loginFailureMessage = "Access denied."

	routeLogin  = "/api/v1/auth/local"
	routeLogout = "/api/v1/auth/logout"
	routeMe     = "/api/v1/auth/me"
)

// authRoutes is what the login, logout and identity handlers share.
type authRoutes struct {
	service        *auth.Service
	trustedProxies []netip.Prefix
	logger         *slog.Logger
}

func (a authRoutes) register(mux *http.ServeMux) {
	mux.Handle(routeLogin, allowMethod(http.MethodPost, http.HandlerFunc(a.login)))
	mux.Handle(routeLogout, allowMethod(http.MethodPost, http.HandlerFunc(a.logout)))
	mux.Handle(routeMe, allowRead(http.HandlerFunc(a.me)))
}

// isPublicRoute is the whole unauthenticated surface under the session gate;
// the health routes never reach the gate. Logout is here because it must
// answer 200 even once the session is gone.
func isPublicRoute(r *http.Request) bool {
	switch r.URL.Path {
	case routeLogin, routeLogout:
		return r.Method == http.MethodPost
	case "/api/v1/settings/public", "/api/v1/backdrops":
		return r.Method == http.MethodGet || r.Method == http.MethodHead
	}
	return false
}

// requireSession answers 401 to every request outside the public surface that
// carries no live session, and hands the account to the handler otherwise.
func (a authRoutes) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		session, renewed, err := a.service.Authenticate(r.Context(), sessionTokenFrom(r))
		if errors.Is(err, auth.ErrNoSession) {
			writeStatusError(w, http.StatusUnauthorized)
			return
		}
		if err != nil {
			a.logger.ErrorContext(r.Context(), "authenticate", "error", err)
			writeStatusError(w, http.StatusInternalServerError)
			return
		}
		if renewed {
			http.SetCookie(w, sessionCookie(session, requestIsHTTPS(r, a.trustedProxies)))
		}
		next.ServeHTTP(w, r.WithContext(auth.ContextWithUser(r.Context(), session.User)))
	})
}

type loginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login takes the identifier from the form's email field, which carries a
// username or an email. An unreadable body becomes an empty identifier and
// takes the same path as a wrong password, so it answers the same way.
func (a authRoutes) login(w http.ResponseWriter, r *http.Request) {
	var body loginRequestBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, loginBodyLimitBytes)).Decode(&body); err != nil {
		body = loginRequestBody{}
	}
	session, err := a.service.Login(r.Context(), auth.LoginAttempt{
		Identifier:    body.Email,
		Password:      body.Password,
		Client:        clientAddress(r, a.trustedProxies),
		PreviousToken: sessionTokenFrom(r),
	})
	var limited *auth.RateLimitedError
	switch {
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(limited.RetryAfter.Seconds()))))
		writeStatusError(w, http.StatusTooManyRequests)
		return
	case errors.Is(err, auth.ErrInvalidCredentials):
		WriteError(w, http.StatusForbidden, loginFailureMessage)
		return
	case err != nil:
		a.logger.ErrorContext(r.Context(), "login", "error", err)
		writeStatusError(w, http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, sessionCookie(session, requestIsHTTPS(r, a.trustedProxies)))
	writeUser(w, session.User)
}

// logout always clears the cookie and answers {"status":"ok"}: the kept user
// menu only refreshes identity when it sees that body.
func (a authRoutes) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.service.Logout(r.Context(), sessionTokenFrom(r)); err != nil {
		a.logger.ErrorContext(r.Context(), "logout", "error", err)
		writeStatusError(w, http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, clearedSessionCookie(requestIsHTTPS(r, a.trustedProxies)))
	writeJSONBody(w, map[string]string{"status": "ok"})
}

func (a authRoutes) me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeStatusError(w, http.StatusUnauthorized)
		return
	}
	writeUser(w, user)
}

func writeJSONBody(w http.ResponseWriter, body any) {
	respond.JSON(w, body)
}

func sessionTokenFrom(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// sessionCookie is host-only (no Domain), so it is never sent to a sibling
// host. Secure follows the browser's own connection: the LAN address is plain
// HTTP and must keep working.
func sessionCookie(session auth.Session, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		MaxAge:   int(auth.SessionLifetime / time.Second),
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// clearedSessionCookie carries the same name, path, Secure and SameSite as
// the cookie it replaces; a browser keeps a cookie whose attributes differ.
func clearedSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// allowMethod answers 405 in the error shape for any other method.
func allowMethod(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeStatusError(w, http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}
