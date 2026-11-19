package gymserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"code.pecheny.me/pecheny/dopesuite/dopecore/authcred"
	corei18n "code.pecheny.me/pecheny/dopesuite/dopecore/i18nstrings"
	"code.pecheny.me/pecheny/dopesuite/dopecore/session"
	"code.pecheny.me/pecheny/dopesuite/dopecore/tglogin"
)

// Login is telegram only, through the shared handshake (dopecore/tglogin);
// this file is gym's adapter over it. Who may come in is GYM_ALLOWED_TELEGRAM:
// an account is only ever created for a handle on that list, and every page
// checks the list again, so taking a handle off it locks its owner out.

var gymSessions = authcred.Sessions{
	UserColumns: "u.username, u.telegram_username",
	UserDest:    func(u *session.User) ([]any, func()) { return []any{&u.Username, &u.Telegram}, nil },
}

// lookupSession resolves the session cookie to a user, sliding a live session
// so a weekly visitor is never logged out.
func (s *server) lookupSession(w http.ResponseWriter, r *http.Request) (session.User, bool) {
	c, err := r.Cookie(session.CookieName)
	if err != nil || c.Value == "" {
		return session.User{}, false
	}
	now := time.Now()
	u, state, slide := gymSessions.Lookup(r.Context(), s.db, c.Value, now)
	switch state {
	case authcred.NoSession:
		return session.User{}, false
	case authcred.Expired:
		logDropped("session-expire", s.withWriteTx(r.Context(), "session-expire", func(ctx context.Context, tx *sql.Tx) error {
			return authcred.DeleteSession(ctx, tx, u.SessionID)
		}))
		return session.User{}, false
	}
	if slide {
		logDropped("session-refresh", s.withWriteTx(r.Context(), "session-refresh", func(ctx context.Context, tx *sql.Tx) error {
			return authcred.SlideSession(ctx, tx, u.SessionID, now)
		}))
		session.SetCookie(w, c.Value)
	}
	return u, true
}

// mayView is the allowlist, asked of the handle the account carries now.
func (s *server) mayView(u session.User) bool {
	return u.Telegram.Valid && s.allowed[normHandle(u.Telegram.String)]
}

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

// ---- what the login page and the menu ask ----

type meResponse struct {
	UserID   int64   `json:"user_id"`
	Username *string `json:"username"`
	Telegram *string `json:"telegram"`
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) error {
	u, ok := s.lookupSession(w, r)
	if !ok {
		return statusError{http.StatusUnauthorized, ""}
	}
	out := meResponse{UserID: u.UserID}
	if u.Username.Valid {
		out.Username = &u.Username.String
	}
	if u.Telegram.Valid {
		out.Telegram = &u.Telegram.String
	}
	return writeJSON(w, out)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if u, ok := s.lookupSession(w, r); ok {
		logDropped("logout", s.withWriteTx(r.Context(), "logout", func(ctx context.Context, tx *sql.Tx) error {
			return authcred.DeleteSession(ctx, tx, u.SessionID)
		}))
	}
	session.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func botUsername() string { return strings.TrimSpace(os.Getenv("GYM_BOT_USERNAME")) }

func (s *server) handleLoginMethods(w http.ResponseWriter, _ *http.Request) error {
	st := "ok"
	switch {
	case s.bot == nil || botUsername() == "":
		st = "misconfigured"
	case !s.botPolling():
		st = "unreachable"
	}
	return writeJSON(w, map[string]any{"telegram": st == "ok", "telegram_status": st})
}

// handleLoginPassword answers the login page's password form: gym has no
// password accounts.
func (s *server) handleLoginPassword(http.ResponseWriter, *http.Request) error {
	return corei18n.User(str.Auth.Login.TelegramOnly())
}

// ---- the telegram handshake ----

func (s *server) handshake() tglogin.Handshake {
	return tglogin.Handshake{Users: gymUsers{s}}
}

func (s *server) handleTgStart(w http.ResponseWriter, r *http.Request) error {
	if s.bot == nil {
		return statusError{http.StatusServiceUnavailable, str.Auth.Tg.NotConfigured()}
	}
	var out struct {
		Code        string `json:"code"`
		ExpiresAt   string `json:"expires_at"`
		BotUsername string `json:"bot_username,omitempty"`
	}
	err := s.withWriteTx(r.Context(), "tg-start", func(ctx context.Context, tx *sql.Tx) error {
		res, err := s.handshake().Start(ctx, tx, time.Now())
		out.Code, out.ExpiresAt, out.BotUsername = res.Code, rfc3339(res.ExpiresAt), botUsername()
		return err
	})
	if err != nil {
		return err
	}
	return writeJSON(w, out)
}

func (s *server) handleTgStatus(w http.ResponseWriter, r *http.Request) error {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		return corei18n.User(str.Auth.Tg.CodeMissing())
	}
	var out tglogin.Outcome
	err := s.withWriteTx(r.Context(), "tg-status", func(ctx context.Context, tx *sql.Tx) (err error) {
		out, err = s.handshake().Resolve(ctx, tx, code, time.Now())
		return err
	})
	return writeOutcome(w, out, err)
}

func (s *server) handleTgClaim(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		return err
	}
	name := strings.TrimSpace(req.Username)
	if !usernameRe.MatchString(name) {
		return corei18n.User(str.Auth.Username.Format())
	}
	var out tglogin.Outcome
	err := s.withWriteTx(r.Context(), "tg-claim", func(ctx context.Context, tx *sql.Tx) (err error) {
		out, err = s.handshake().Claim(ctx, tx, req.Code, name, req.Password, time.Now())
		switch {
		case errors.Is(err, tglogin.ErrCodeNotFound):
			return corei18n.User(str.Auth.Tg.CodeMissing())
		case errors.Is(err, tglogin.ErrWrongPassword), errors.Is(err, tglogin.ErrTelegramLinked):
			return corei18n.User(str.Auth.Tg.TelegramTaken())
		}
		return err
	})
	return writeOutcome(w, out, err)
}

func writeOutcome(w http.ResponseWriter, out tglogin.Outcome, err error) error {
	if err != nil {
		return err
	}
	if out.Token != "" {
		session.SetCookie(w, out.Token)
	}
	return writeJSON(w, struct {
		Status   string  `json:"status"`
		Username *string `json:"username,omitempty"`
	}{out.Status, out.Username})
}

// gymUsers is gym's users table as the handshake needs it.
type gymUsers struct{ s *server }

const accountColumns = `id, username, password_hash, password_salt, telegram_user_id`

func (gymUsers) ByTelegram(ctx context.Context, tx tglogin.Tx, tg int64) (tglogin.Account, bool, error) {
	return scanAccount(tx.QueryRowContext(ctx, `select `+accountColumns+` from users where telegram_user_id = ?`, tg))
}

func (gymUsers) ByUsername(ctx context.Context, tx tglogin.Tx, username string) (tglogin.Account, bool, error) {
	return scanAccount(tx.QueryRowContext(ctx, `select `+accountColumns+` from users where username = ?`, username))
}

func scanAccount(row *sql.Row) (tglogin.Account, bool, error) {
	var a tglogin.Account
	switch err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.PasswordSalt, &a.TelegramUserID); {
	case errors.Is(err, sql.ErrNoRows):
		return a, false, nil
	case err != nil:
		return a, false, err
	}
	return a, true, nil
}

// Create is where the allowlist keeps strangers out: nobody else gets a row.
func (u gymUsers) Create(ctx context.Context, tx tglogin.Tx, id tglogin.Identity, username string, now time.Time) (int64, error) {
	if !id.Username.Valid || !u.s.allowed[normHandle(id.Username.String)] {
		return 0, statusError{http.StatusForbidden, str.Auth.Access.Denied()}
	}
	res, err := tx.ExecContext(ctx, `
insert into users(telegram_user_id, telegram_username, telegram_name, username, created_at, updated_at)
values(?, ?, ?, ?, ?, ?)`, id.TelegramUserID, id.Username, id.Name, username, rfc3339(now), rfc3339(now))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (gymUsers) Attach(ctx context.Context, tx tglogin.Tx, userID int64, id tglogin.Identity, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
update users set telegram_user_id = ?, telegram_username = ?, telegram_name = ?, updated_at = ? where id = ?`,
		id.TelegramUserID, id.Username, id.Name, rfc3339(now), userID)
	return err
}
