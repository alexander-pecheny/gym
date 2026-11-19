package gymserver

import (
	"net/http"
	"net/url"

	"code.pecheny.me/pecheny/dopesuite/dopecore/sameorigin"
)

func routes(s *server) http.Handler {
	mux := http.NewServeMux()

	// ---- pages: the allowlist stands in front of each ----
	mux.HandleFunc("GET /{$}", s.viewer(s.handleLatest))
	mux.HandleFunc("GET /s/{id}", s.viewer(s.handleSession))
	mux.HandleFunc("GET /sessions", s.viewer(s.handleList))
	mux.HandleFunc("GET /login", s.handleLogin)

	// ---- auth: what the kit's login and menu scripts call ----
	mux.HandleFunc("GET /api/auth/methods", api(s.handleLoginMethods))
	mux.HandleFunc("POST /api/auth/tg/start", api(s.handleTgStart))
	mux.HandleFunc("GET /api/auth/tg/status", api(s.handleTgStatus))
	mux.HandleFunc("POST /api/auth/tg/claim", api(s.handleTgClaim))
	mux.HandleFunc("POST /api/auth/login-password", api(s.handleLoginPassword))
	mux.HandleFunc("GET /api/auth/me", api(s.handleMe))
	mux.HandleFunc("POST /api/auth/logout", api(s.handleLogout))

	// ---- static ----
	mux.HandleFunc("GET /static/styles.css", s.assets.ServeStylesheet())
	mux.HandleFunc("GET /static/login.js", s.assets.ServeShared("/static/login.js"))
	mux.HandleFunc("GET /static/menu.js", s.assets.ServeShared("/static/menu.js"))
	mux.Handle("GET /static/fonts/", s.assets.ServeFonts())
	mux.Handle("GET /static/", s.assets.FileServer())
	return sameorigin.Middleware(func() string { return "" }, mux)
}

// viewer is the door every page goes through: a visitor with no session is
// sent to log in and brought back, and one whose handle is not on the
// allowlist is told so.
func (s *server) viewer(h func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.lookupSession(w, r)
		if !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		if !s.mayView(u) {
			s.writePageError(w, r, statusError{http.StatusForbidden, str.Auth.Access.Denied()})
			return
		}
		if err := h(w, r); err != nil {
			s.writePageError(w, r, err)
		}
	}
}
