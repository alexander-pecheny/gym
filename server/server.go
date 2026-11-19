// Package gymserver is gym's HTTP server: the coach's training plans, read
// from the sheet the coach keeps, shown one Session to a phone screen, behind
// the shared Telegram login and an allowlist of who may see them.
package gymserver

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"code.pecheny.me/pecheny/dopesuite/dopecore/session"
	"code.pecheny.me/pecheny/dopesuite/dopecore/sqlitex"
	"code.pecheny.me/pecheny/dopesuite/dopecore/tgbot"
	"code.pecheny.me/pecheny/dopesuite/dopecore/webassets"
	kit "code.pecheny.me/pecheny/dopesuite/dopeuikit/kit"

	"code.pecheny.me/pecheny/gym/storage/store"
	"code.pecheny.me/pecheny/gym/web/assets"
	"code.pecheny.me/pecheny/gym/web/ui"

	gymstrings "code.pecheny.me/pecheny/gym/i18nstrings"
)

var str = gymstrings.Default

type server struct {
	db *sql.DB
	mu sync.Mutex // global write lock — serialises every write transaction

	assets *webassets.Assets
	pages  *kit.PageSet
	sheet  *sheetSource

	// allowed is the telegram handles that may log in and see the plans,
	// lower-cased and without the @.
	allowed map[string]bool

	// The login bot, polling in this process. nil without GYM_BOT_TOKEN.
	bot *tgbot.Client
}

func newServer() (*server, error) {
	if session.SecureCookies() && publicURL() == "" {
		return nil, errors.New("GYM_PUBLIC_URL is required (or set GYM_ENV=development)")
	}
	path := envOr("GYM_DB", "gym.db")
	db, err := sqlitex.Open(path, store.Migrate)
	if err != nil {
		return nil, err
	}
	s := &server{db: db, allowed: map[string]bool{}}
	for _, h := range strings.Split(os.Getenv("GYM_ALLOWED_TELEGRAM"), ",") {
		if h = normHandle(h); h != "" {
			s.allowed[h] = true
		}
	}
	s.sheet = newSheetSource(os.Getenv("GYM_SHEET_CSV"), envOr("GYM_SHEET_CACHE", "sheet.csv"))
	s.assets = kit.Assets(assets.FS, ".", "web/assets")
	s.pages = kit.NewPageSet(s.assets.Source, s.assets.NoCache, ui.Compile).
		Provide("ui/login.dopeui", kit.LoginPage(str.Auth.Page.Title(), "/"))
	if err := s.pages.Warm("ui/login.dopeui"); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *server) withWriteTx(reqCtx context.Context, label string, fn func(ctx context.Context, tx *sql.Tx) error) error {
	return sqlitex.Writer{DB: s.db, Mu: &s.mu}.Tx(reqCtx, label, fn)
}

func publicURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("GYM_PUBLIC_URL")), "/")
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func normHandle(h string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(h), "@"))
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }
