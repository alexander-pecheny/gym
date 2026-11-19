package plan

import (
	"encoding/csv"
	"io"
	"regexp"
	"strings"
	"unicode"
)

// The coach's sheet (the "training without a coach" tab) is read as it is laid
// out: a Session starts at a header row whose second cell is headerMarker
// ("set 1"), the row under it names the Session in its first cell (its date),
// and the columns are the exercise, four weight/reps pairs and the note. A row
// whose only cell is in capitals is a Block's title; any other row with a name
// is an Exercise.
//
// A superset is not drawn in the sheet, only written: the coach appends
// supersetMarker to each of its exercises, with or without brackets.
// Consecutive marked rows pair up in order, and an odd run ends in a group of
// three, which is how every superset in the sheet so far reads.

// Column layout of the sheet.
const (
	colName  = 0
	colFirst = 1 // the first set's weight; reps follow, then the next set
	setCount = 4
	colNote  = colFirst + 2*setCount
)

// The sheet's own words, matched as input.
const (
	headerMarker    = "1 ПОДХОД"
	subheaderMarker = "ВЕС"
)

var supersetMarker = regexp.MustCompile(`(?i)\(?\s*суперсет\s*\)?`)

// ParseCSV reads the sheet as Google exports it.
func ParseCSV(r io.Reader) ([]Session, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	return Parse(rows), nil
}

// Parse turns the sheet's rows into Sessions, in the sheet's order.
func Parse(rows [][]string) []Session {
	var p parser
	for i := 0; i < len(rows); i++ {
		row := rows[i]
		switch {
		case isHeader(row):
			p.startSession(cell(row, colName))
			if i+1 < len(rows) && strings.EqualFold(cell(rows[i+1], colFirst), subheaderMarker) {
				p.cur.Label = cell(rows[i+1], colName)
				i++
			}
		case p.cur == nil, blank(row):
			continue
		case onlyName(row) && isHeading(cell(row, colName)):
			p.startBlock(cell(row, colName))
		default:
			p.addExercise(row)
		}
	}
	p.endSession()
	return p.out
}

type parser struct {
	out    []Session
	cur    *Session
	block  *Block
	marked []Exercise // the superset run still open in the current Block
}

func (p *parser) startSession(title string) {
	p.endSession()
	p.cur = &Session{ID: len(p.out) + 1, Title: title}
}

func (p *parser) endSession() {
	if p.cur == nil {
		return
	}
	p.endBlock()
	p.out = append(p.out, *p.cur)
	p.cur = nil
}

func (p *parser) startBlock(title string) {
	p.endBlock()
	p.block = &Block{Title: title}
}

func (p *parser) endBlock() {
	if p.block == nil {
		return
	}
	p.flushSuperset()
	p.cur.Blocks = append(p.cur.Blocks, *p.block)
	p.block = nil
}

func (p *parser) addExercise(row []string) {
	if p.block == nil {
		p.block = &Block{}
	}
	name := cell(row, colName)
	ex := Exercise{Note: cell(row, colNote)}
	for k := range setCount {
		ex.Sets = append(ex.Sets, Set{Weight: cell(row, colFirst+2*k), Reps: cell(row, colFirst+2*k+1)})
	}
	for len(ex.Sets) > 0 && ex.Sets[len(ex.Sets)-1].Empty() {
		ex.Sets = ex.Sets[:len(ex.Sets)-1]
	}
	if supersetMarker.MatchString(name) {
		ex.Name = squash(supersetMarker.ReplaceAllString(name, " "))
		p.marked = append(p.marked, ex)
		return
	}
	ex.Name = name
	p.flushSuperset()
	p.block.Groups = append(p.block.Groups, Group{Exercises: []Exercise{ex}})
}

// flushSuperset closes the open run of marked exercises into pairs, the last
// taking a third when the run is odd.
func (p *parser) flushSuperset() {
	run := p.marked
	p.marked = nil
	for len(run) > 0 {
		n := 2
		if len(run) <= 3 {
			n = len(run)
		}
		p.block.Groups = append(p.block.Groups, Group{Exercises: run[:n]})
		run = run[n:]
	}
}

func isHeader(row []string) bool { return strings.EqualFold(cell(row, colFirst), headerMarker) }

func cell(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return squash(row[i])
}

// squash trims a cell and collapses its runs of spaces, which the sheet is
// full of (a double space before a superset marker is common).
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func blank(row []string) bool {
	for i := range row {
		if cell(row, i) != "" {
			return false
		}
	}
	return true
}

func onlyName(row []string) bool {
	for i := range row {
		if i != colName && cell(row, i) != "" {
			return false
		}
	}
	return true
}

// isHeading is a Block title: letters, all of them capitals.
func isHeading(s string) bool {
	letters := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters = true
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return letters
}
