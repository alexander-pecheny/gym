package gymserver

import (
	"net/http"
	"slices"
	"strconv"

	kit "code.pecheny.me/pecheny/dopesuite/dopeuikit/kit"

	"code.pecheny.me/pecheny/gym/domain/plan"
	"code.pecheny.me/pecheny/gym/web/ui"
)

// Every page but the login is built here with the typed builder: a topbar
// with the trail and a way to the list, then the page's body.

type view struct {
	title string
	crumb string // the trail's last step after 🏠; empty on the home page
	body  []ui.Item
}

func (s *server) render(w http.ResponseWriter, r *http.Request, status int, v view) error {
	trail := []ui.Item{ui.Crumb(ui.Home(), ui.Href("/"), ui.IconHouse, ui.Label(str.Plan.ToHome()))}
	if v.crumb != "" {
		trail = append(trail, ui.Crumb(ui.Text(v.crumb)))
	}
	title := str.Plan.SiteName()
	if v.title != "" {
		title = str.Plan.TitleSuffix(v.title, str.Plan.SiteName())
	}
	page := []ui.Item{ui.Title(title), ui.PagePlan,
		ui.Topbar(ui.Nosync(), ui.Crumbs(trail...),
			ui.Iconlink(ui.IconList, ui.Href("/sessions"), ui.Label(str.Plan.AllSessions()))),
	}
	page = append(page, v.body...)
	body, err := ui.Render(&ui.Doc{Nodes: []ui.Node{ui.Page(page...)}})
	if err != nil {
		return err
	}
	s.writePage(w, r, status, body)
	return nil
}

func (s *server) writePageError(w http.ResponseWriter, r *http.Request, err error) {
	code, msg := errorOf(err)
	logDropped("error page", s.render(w, r, code, view{
		title: str.Plan.ErrorTitle(),
		crumb: str.Plan.ErrorTitle(),
		body:  []ui.Item{ui.Section(ui.Hint(ui.HintDanger, ui.Text(msg)))},
	}))
}

func (s *server) handleLatest(w http.ResponseWriter, r *http.Request) error {
	v, err := s.sheet.Sessions(r.Context())
	if err != nil {
		return err
	}
	if len(v.Sessions) == 0 {
		return s.render(w, r, http.StatusOK, view{body: []ui.Item{ui.Section(ui.Hint(ui.Text(str.Plan.Empty())))}})
	}
	// The coach appends, so the last Session in the sheet is the next one.
	return s.renderSession(w, r, v, v.Sessions[len(v.Sessions)-1])
}

func (s *server) handleSession(w http.ResponseWriter, r *http.Request) error {
	v, err := s.sheet.Sessions(r.Context())
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 || id > len(v.Sessions) {
		return statusError{http.StatusNotFound, str.Server.Error.NotFound()}
	}
	return s.renderSession(w, r, v, v.Sessions[id-1])
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) error {
	v, err := s.sheet.Sessions(r.Context())
	if err != nil {
		return err
	}
	rows := make([]ui.Item, 0, len(v.Sessions))
	for _, sess := range slices.Backward(v.Sessions) {
		row := []ui.Item{ui.Href("/s/" + strconv.Itoa(sess.ID)), ui.Listtitle(ui.Text(sess.Name()))}
		if sess.Label != "" && sess.Title != "" {
			row = append(row, ui.Muted(ui.Text(sess.Title)))
		}
		rows = append(rows, ui.Listrow(row...))
	}
	body := staleNote(v)
	body = append(body, ui.Section(ui.List(rows...)))
	return s.render(w, r, http.StatusOK, view{title: str.Plan.AllSessions(), crumb: str.Plan.AllSessions(), body: body})
}

func staleNote(v sheetView) []ui.Item {
	if !v.Stale {
		return nil
	}
	return []ui.Item{ui.Hint(ui.Text(str.Plan.Stale(v.AsOf.Format("02.01 15:04"))))}
}

func (s *server) renderSession(w http.ResponseWriter, r *http.Request, v sheetView, sess plan.Session) error {
	body := staleNote(v)
	if sess.Label != "" && sess.Title != "" {
		body = append(body, ui.Muted(ui.Text(sess.Title)))
	}
	cols := strconv.Itoa(sess.MaxSets())
	for _, b := range sess.Blocks {
		items := []ui.Item{ui.Title(b.Title), ui.Cols(cols)}
		if b.Folded() {
			items = append(items, ui.Folded())
		}
		shared := b.SharedNote()
		if shared != "" {
			items = append(items, ui.Note(shared))
		}
		for _, g := range b.Groups {
			items = append(items, groupItem(g, shared))
		}
		body = append(body, ui.Planblock(items...))
	}
	return s.render(w, r, http.StatusOK, view{title: sess.Name(), crumb: sess.Name(), body: body})
}

// groupItem leaves out a note the Block's head already says.
func groupItem(g plan.Group, shared string) ui.Item {
	items := make([]ui.Item, 0, len(g.Exercises))
	for _, e := range g.Exercises {
		ex := []ui.Item{ui.Name(e.Name)}
		if e.Note != "" && e.Note != shared {
			ex = append(ex, ui.Note(e.Note))
		}
		if reps, ok := e.Uniform(); ok {
			ex = append(ex, ui.Repeat(str.Plan.Repeat(strconv.Itoa(len(e.Sets)), reps)))
			items = append(items, ui.Exercise(ex...))
			continue
		}
		for _, set := range e.Sets {
			ex = append(ex, ui.Setcell(ui.Weight(set.Weight), ui.Reps(set.Reps)))
		}
		items = append(items, ui.Exercise(ex...))
	}
	return ui.Plangroup(items...)
}

// handleLogin serves the kit's login page, compiled with its own destination
// when the visitor was sent here from a page.
func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	src := "ui/login.dopeui"
	var (
		body []byte
		err  error
	)
	if next := kit.SafeLoginRedirect(r.URL.Query().Get("next")); next != "" {
		body, err = ui.Compile(src, kit.LoginPage(str.Auth.Page.Title(), next))
	} else {
		body, err = s.pages.Bytes(src)
	}
	if err != nil {
		s.writePageError(w, r, err)
		return
	}
	s.writePage(w, r, http.StatusOK, body)
}

func (s *server) writePage(w http.ResponseWriter, r *http.Request, status int, body []byte) {
	body = s.assets.VersionRefs(body)
	h := w.Header()
	h.Set("Content-Security-Policy", contentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Cache-Control", "no-cache")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; " +
	"base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
