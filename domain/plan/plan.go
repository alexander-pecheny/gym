// Package plan is a training Session as the trainee reads it in the gym: its
// Blocks (warm-up, strength, cardio), and in each Block the Groups of
// Exercises. A Group of one is an exercise done on its own; a Group of two or
// more is a superset, done back to back. An Exercise carries up to four Sets,
// each a weight and a number of repetitions written the way the coach writes
// them (a weight of "2,5+1", reps of "12-15" or thirty seconds), because they
// are read, not computed.
package plan

// Set is one approach of an Exercise. Either half may be empty: a warm-up
// carries repetitions only, and a coach sometimes leaves a set out.
type Set struct {
	Weight string `json:"weight,omitempty"`
	Reps   string `json:"reps,omitempty"`
}

// Empty reports whether the coach wrote nothing in this set.
func (s Set) Empty() bool { return s.Weight == "" && s.Reps == "" }

// Exercise is one row of the plan: its name, the coach's note on technique,
// and its Sets in order. Sets is trimmed of trailing empty sets.
type Exercise struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	Sets []Set  `json:"sets,omitempty"`
}

// HasWeight reports whether any set names a weight, which decides whether the
// set cells carry one line or two.
func (e Exercise) HasWeight() bool {
	for _, s := range e.Sets {
		if s.Weight != "" {
			return true
		}
	}
	return false
}

// Uniform reports the reps every set shares when the Exercise has no weight
// and more than one set, all alike: a warm-up's "3 × 15", read as one cell.
func (e Exercise) Uniform() (string, bool) {
	if len(e.Sets) < 2 || e.HasWeight() {
		return "", false
	}
	for _, s := range e.Sets[1:] {
		if s.Reps != e.Sets[0].Reps {
			return "", false
		}
	}
	return e.Sets[0].Reps, e.Sets[0].Reps != ""
}

// Group is what is done before resting: one Exercise, or a superset of more.
type Group struct {
	Exercises []Exercise `json:"exercises"`
}

// Superset reports whether the Group is done back to back.
func (g Group) Superset() bool { return len(g.Exercises) > 1 }

// Block is a titled part of a Session. A Block may be empty: the coach writes
// a cardio block's title on its own to mean "do your cardio".
type Block struct {
	Title  string  `json:"title,omitempty"`
	Groups []Group `json:"groups,omitempty"`
}

// Folded reports whether every Exercise of the Block reads as one cell, so
// its head has no set columns to number. An empty Block is not folded.
func (b Block) Folded() bool {
	n := 0
	for _, g := range b.Groups {
		for _, e := range g.Exercises {
			if _, ok := e.Uniform(); !ok {
				return false
			}
			n++
		}
	}
	return n > 0
}

// SharedNote is a note the coach wrote on more than one Exercise of the
// Block, the most frequent one, so it can be said once for the Block. Empty
// when no note repeats.
func (b Block) SharedNote() string {
	count := map[string]int{}
	best := ""
	for _, g := range b.Groups {
		for _, e := range g.Exercises {
			if e.Note == "" {
				continue
			}
			count[e.Note]++
			if count[e.Note] > count[best] {
				best = e.Note
			}
		}
	}
	if count[best] < 2 {
		return ""
	}
	return best
}

// Session is one training: a label (usually its date), a title (the place, or
// the program's name) and its Blocks. ID is its 1-based position in the sheet,
// which is stable as long as the coach appends.
type Session struct {
	ID     int     `json:"id"`
	Label  string  `json:"label,omitempty"`
	Title  string  `json:"title,omitempty"`
	Blocks []Block `json:"blocks"`
}

// Name is what a Session is called in a list and a title bar.
func (s Session) Name() string {
	switch {
	case s.Label != "":
		return s.Label
	case s.Title != "":
		return s.Title
	}
	return ""
}

// MaxSets is the widest Exercise of the Session, so every row of the sheet
// draws the same number of columns.
func (s Session) MaxSets() int {
	n := 0
	for _, b := range s.Blocks {
		for _, g := range b.Groups {
			for _, e := range g.Exercises {
				n = max(n, len(e.Sets))
			}
		}
	}
	return n
}
