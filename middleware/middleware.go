package middleware

import (
	"net/http"
	"time"

	"ridgway.dev/personal/internal/auth"
	"ridgway.dev/personal/internal/traffic"
)

// TrafficTracking records page views after each request.
func TrafficTracking(svc *traffic.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !traffic.ShouldTrack(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			sessionID := traffic.SessionID(r)
			// Set session cookie if new
			if _, err := r.Cookie("site_session"); err != nil {
				http.SetCookie(w, &http.Cookie{
					Name:     "site_session",
					Value:    sessionID,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Expires:  time.Now().Add(24 * time.Hour),
				})
			}

			rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rw, r)
			elapsed := time.Since(start).Seconds() * 1000

			go svc.Track(traffic.PageView{
				IPAddress:    traffic.ClientIP(r),
				UserAgent:    r.UserAgent(),
				Path:         r.URL.Path,
				Method:       r.Method,
				Referrer:     r.Referer(),
				Timestamp:    time.Now(),
				ResponseTime: elapsed,
				StatusCode:   rw.code,
				SessionID:    sessionID,
			})
		})
	}
}

// AdminOnly redirects unauthenticated users to /admin/login.
func AdminOnly(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !svc.IsAuthenticated(r) {
				http.Redirect(w, r, "/admin/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}
