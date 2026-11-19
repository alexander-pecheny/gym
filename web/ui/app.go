// Package ui is gym's thin overlay over DopeUIKit's kit: the "plan" page kind
// and the four primitives a training Session is drawn with. A Block is the
// kit's results table, the one dope's results page uses: an exercise is a paper row of
// its own, and a superset is glued like tied places, hairlines between its
// rows and no gap.
package ui

//go:generate go run code.pecheny.me/pecheny/dopesuite/dopeuikit/cmd/uigen -core ../../dopesuite/dopeuikit/kit/vocab.json -overlay vocab.json -pkg ui -base code.pecheny.me/pecheny/dopesuite/dopeuikit/kit -out tags_gen.go

import (
	_ "embed"
	"strconv"

	base "code.pecheny.me/pecheny/dopesuite/dopeuikit/kit"

	gymstrings "code.pecheny.me/pecheny/gym/i18nstrings"
)

//go:embed vocab.json
var overlayVocab []byte

var app = mustApp()

func mustApp() *base.App {
	a, err := base.NewApp(base.Options{
		Strings:      gymstrings.Default,
		VocabOverlay: overlayVocab,
		Expand: map[string]base.ExpandFunc{
			"planblock": expandBlock,
			// A Group, its exercises and their sets are drawn by the Block's
			// table; on their own they have nothing to say.
			"plangroup": expandEmpty,
			"exercise":  expandEmpty,
			"setcell":   expandEmpty,
		},
		Chrome: base.CoreChrome().With(base.Chrome{
			// Read on a phone between sets: the notch is real estate.
			Viewport: "width=device-width, initial-scale=1, viewport-fit=cover",
			PageKinds: map[string]base.PageKind{
				"plan": {Body: []string{"host", "plan-host"}, Main: []string{"match-main", "plan-main"}},
			},
		}),
	})
	if err != nil {
		panic(err)
	}
	return a
}

// expandBlock draws a Block as one of the kit's results tables (core.css):
// the Block's title and the set numbers in the head, then each Group as a
// paper block of its own, an 8px results-group-gap row between Groups. A
// Group of one is one rounded row; a superset's rows are glued, hairlines
// between them, the way tied places are on dope's results page.
func expandBlock(c *base.ExpandCtx, p *base.Element) []base.Node {
	title, _ := base.Get(p, "title")
	cols, err := strconv.Atoi(mustGet(p, "cols"))
	if err != nil || cols < 0 {
		cols = 0
	}
	folded := base.Flag(p, "folded")
	// The head carries the Block's title, and under it the note the coach
	// wrote on several of its exercises, said once, the way a team's city
	// sits under its name.
	titleCell := []base.Node{base.Inl("span", nil, base.Text(title))}
	if note, ok := base.Get(p, "note"); ok && note != "" {
		titleCell = append(titleCell, base.Inl("span", []base.Attr{base.ClassAttr("results-team-city")}, base.Text(note)))
	}
	headAttrs := []base.Attr{base.ClassAttr("results-team-head")}
	if folded {
		// Every row is one "3 × 15" cell, so there are no sets to number.
		headAttrs = append(headAttrs, base.At("colspan", "2"))
	}
	head := []base.Node{base.El("th", headAttrs, titleCell...)}
	for k := range cols {
		if folded {
			break
		}
		head = append(head, base.Inl("th", []base.Attr{base.ClassAttr("results-tour-head")}, base.Text(strconv.Itoa(k+1))))
	}
	// A folded Block's rows are the name and one "3 × 15" cell.
	rowCols := cols
	if folded {
		rowCols = 1
	}
	var rows []base.Node
	for i, g := range p.Block {
		group, ok := g.(*base.Element)
		if !ok {
			continue
		}
		if i > 0 {
			rows = append(rows, base.El("tr", []base.Attr{base.ClassAttr("results-group-gap")},
				base.El("td", []base.Attr{base.At("colspan", strconv.Itoa(rowCols+1))})))
		}
		rows = append(rows, groupRows(c, group, rowCols)...)
	}
	// A fixed table takes its widths from here, not from a head cell that
	// spans the row: the name takes what the set columns leave.
	colgroup := []base.Node{base.El("col", nil)}
	if folded {
		colgroup = append(colgroup, base.El("col", []base.Attr{base.ClassAttr("plan-repeat-col")}))
	} else {
		for range cols {
			colgroup = append(colgroup, base.El("col", []base.Attr{base.ClassAttr("plan-set-col")}))
		}
	}
	table := []base.Node{base.El("colgroup", nil, colgroup...), base.El("thead", nil, base.El("tr", nil, head...))}
	if len(rows) > 0 {
		table = append(table, base.El("tbody", nil, rows...))
	}
	return []base.Node{base.El("table", []base.Attr{base.ClassAttr("results-table", "plan-table")}, table...)}
}

func groupRows(_ *base.ExpandCtx, g *base.Element, cols int) []base.Node {
	var exercises []*base.Element
	for _, n := range g.Block {
		if e, ok := n.(*base.Element); ok {
			exercises = append(exercises, e)
		}
	}
	rows := make([]base.Node, 0, len(exercises))
	for i, e := range exercises {
		classes := []string{"results-row"}
		if i == 0 {
			classes = append(classes, "results-group-first")
		}
		if i == len(exercises)-1 {
			classes = append(classes, "results-group-last")
		}
		rows = append(rows, base.El("tr", []base.Attr{base.ClassAttr(classes...)}, exerciseCells(e, cols)...))
	}
	return rows
}

// exerciseCells is the name with the coach's note under it, the way a team's
// city sits under its name, then one cell per set: the weight, and the reps
// under it in the same quiet line.
func exerciseCells(e *base.Element, cols int) []base.Node {
	name := []base.Node{base.Inl("span", nil, base.Text(mustGet(e, "name")))}
	if note, ok := base.Get(e, "note"); ok && note != "" {
		name = append(name, base.Inl("span", []base.Attr{base.ClassAttr("results-team-city")}, base.Text(note)))
	}
	cells := []base.Node{base.El("td", []base.Attr{base.ClassAttr("results-team", "plan-name")}, name...)}
	// The same reps every set and nothing to load: one cell across the sets.
	if repeat, ok := base.Get(e, "repeat"); ok && repeat != "" {
		return append(cells, base.Inl("td", []base.Attr{base.ClassAttr("results-tour", "plan-set"), base.At("colspan", strconv.Itoa(cols))}, base.Text(repeat)))
	}
	var sets []*base.Element
	for _, n := range e.Block {
		if s, ok := n.(*base.Element); ok {
			sets = append(sets, s)
		}
	}
	for k := range cols {
		var kids []base.Node
		if k < len(sets) {
			weight, _ := base.Get(sets[k], "weight")
			reps, _ := base.Get(sets[k], "reps")
			switch {
			case weight != "" && reps != "":
				kids = []base.Node{base.Inl("span", nil, base.Text(weight)),
					base.Inl("span", []base.Attr{base.ClassAttr("results-team-city")}, base.Text(reps))}
			case weight != "":
				kids = []base.Node{base.Inl("span", nil, base.Text(weight))}
			case reps != "":
				kids = []base.Node{base.Inl("span", nil, base.Text(reps))}
			}
		}
		cells = append(cells, base.El("td", []base.Attr{base.ClassAttr("results-tour", "plan-set")}, kids...))
	}
	return cells
}

func mustGet(p *base.Element, name string) string {
	v, _ := base.Get(p, name)
	return v
}

func expandEmpty(*base.ExpandCtx, *base.Element) []base.Node { return nil }

// Compile parses, validates and expands a .dopeui page (the kit's login page).
func Compile(name string, src []byte) ([]byte, error) { return app.Compile(name, src) }

// Render validates and prints a builder-made page.
func Render(doc *Doc) ([]byte, error) { return app.Render(doc) }
