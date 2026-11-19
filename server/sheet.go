package gymserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"code.pecheny.me/pecheny/gym/domain/plan"
)

// The plans live in the coach's Google sheet, which the coach keeps editing,
// so gym reads it rather than copying it: the tab's CSV export (GYM_SHEET_CSV),
// at most once a minute. The last good export is kept on disk
// (GYM_SHEET_CACHE), so a Google outage in the middle of a training shows the
// plan as it last read rather than nothing.

const (
	sheetTTL      = time.Minute
	sheetTimeout  = 15 * time.Second
	sheetMaxBytes = 8 << 20
)

type sheetSource struct {
	url, cachePath string
	client         *http.Client

	mu      sync.Mutex
	view    sheetView
	have    bool
	checked time.Time
}

// sheetView is what a page draws from: the Sessions, and whether they are the
// sheet as it is now or a copy from AsOf.
type sheetView struct {
	Sessions []plan.Session
	Stale    bool
	AsOf     time.Time
}

func newSheetSource(url, cachePath string) *sheetSource {
	return &sheetSource{url: url, cachePath: cachePath, client: &http.Client{Timeout: sheetTimeout}}
}

func (s *sheetSource) Sessions(ctx context.Context) (sheetView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.have && now.Sub(s.checked) < sheetTTL {
		return s.view, nil
	}
	s.checked = now
	body, err := s.fetch(ctx)
	if err == nil {
		var sessions []plan.Session
		if sessions, err = plan.ParseCSV(bytes.NewReader(body)); err == nil {
			s.view, s.have = sheetView{Sessions: sessions, AsOf: now}, true
			s.keep(body)
			return s.view, nil
		}
	}
	log.Printf("sheet: %v", err)
	if !s.have {
		if cached, asOf, cerr := s.readCache(); cerr == nil {
			s.view, s.have = sheetView{Sessions: cached, AsOf: asOf}, true
		}
	}
	if !s.have {
		return sheetView{}, err
	}
	s.view.Stale = true
	return s.view, nil
}

func (s *sheetSource) fetch(ctx context.Context) ([]byte, error) {
	if s.url == "" {
		return nil, errors.New("GYM_SHEET_CSV is not set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("export answered %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, sheetMaxBytes))
}

func (s *sheetSource) keep(body []byte) {
	tmp := s.cachePath + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		log.Printf("sheet: keep copy: %v", err)
		return
	}
	logDropped("sheet: keep copy", os.Rename(tmp, s.cachePath))
}

func (s *sheetSource) readCache() ([]plan.Session, time.Time, error) {
	f, err := os.Open(s.cachePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	sessions, err := plan.ParseCSV(f)
	return sessions, st.ModTime(), err
}
