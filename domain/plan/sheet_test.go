package plan

import (
	"os"
	"reflect"
	"testing"
)

// testdata/sample.csv is a made-up sheet laid out the way the coach's tab is
// exported: the same columns, markers, quoting and stray spaces.
func parseFixture(t *testing.T) []Session {
	t.Helper()
	f, err := os.Open("testdata/sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sessions, err := ParseCSV(f)
	if err != nil {
		t.Fatal(err)
	}
	return sessions
}

func TestParsesEverySession(t *testing.T) {
	sessions := parseFixture(t)
	if len(sessions) != 3 {
		t.Fatalf("got %d sessions, want 3", len(sessions))
	}
	for i, s := range sessions {
		if s.ID != i+1 {
			t.Errorf("session %d has ID %d", i, s.ID)
		}
		if s.Name() == "" {
			t.Errorf("session %d has no name", s.ID)
		}
	}
}

func TestTheLastSession(t *testing.T) {
	sessions := parseFixture(t)
	s := sessions[len(sessions)-1]
	if s.Label != "10 марта" {
		t.Fatalf("last session is %q", s.Label)
	}
	var titles []string
	for _, b := range s.Blocks {
		titles = append(titles, b.Title)
	}
	if want := []string{"РАЗМИНОЧНЫЙ БЛОК", "СИЛОВОЙ БЛОК", "КАРДИО БЛОК"}; !reflect.DeepEqual(titles, want) {
		t.Fatalf("blocks %q, want %q", titles, want)
	}
	if !s.Blocks[0].Folded() || s.Blocks[1].Folded() || s.Blocks[2].Folded() {
		t.Errorf("only the warm-up folds")
	}
	if got := s.Blocks[1].SharedNote(); got != "темп медленный" {
		t.Errorf("strength shared note %q", got)
	}
	if s.Blocks[0].SharedNote() != "" {
		t.Errorf("the warm-up has no notes")
	}
	if n := len(s.Blocks[0].Groups); n != 4 {
		t.Errorf("warm-up has %d groups, want 4", n)
	}
	if got := s.Blocks[0].Groups[0].Exercises[0]; got.HasWeight() || len(got.Sets) != 3 || got.Sets[0].Reps != "12 х2" || got.Name != `Скручивание "Велосипед"` {
		t.Errorf("first warm-up exercise: %+v", got)
	}

	strength := s.Blocks[1].Groups
	var shape []int
	for _, g := range strength {
		shape = append(shape, len(g.Exercises))
	}
	// Three on their own, then two supersets of two.
	if want := []int{1, 1, 1, 2, 2}; !reflect.DeepEqual(shape, want) {
		t.Fatalf("strength groups %v, want %v", shape, want)
	}
	legs := strength[3].Exercises
	if legs[0].Name != "Сгибание ног лёжа" || legs[1].Name != "Подъём таза у скамьи" {
		t.Errorf("superset names: %q, %q", legs[0].Name, legs[1].Name)
	}
	if got := legs[0].Sets[3]; got != (Set{Weight: "35", Reps: "15"}) {
		t.Errorf("leg extension set 4: %+v", got)
	}
	if legs[0].Note == "" || legs[1].Note != "" {
		t.Errorf("notes: %q, %q", legs[0].Note, legs[1].Note)
	}
	if len(s.Blocks[2].Groups) != 0 {
		t.Errorf("cardio block should be empty, got %+v", s.Blocks[2].Groups)
	}
	if s.MaxSets() != 4 {
		t.Errorf("max sets %d", s.MaxSets())
	}
}

func TestAnOddSupersetRunEndsInThree(t *testing.T) {
	ex := func(name string) []string { return []string{name, "1", "10"} }
	rows := [][]string{
		{"", "1 ПОДХОД"},
		{"1 мая", "ВЕС"},
		{"СИЛОВОЙ БЛОК"},
		ex("a СУПЕРСЕТ"), ex("b (суперсет)"), ex("c (Суперсет)"),
		ex("d"),
		{"КАРДИО БЛОК"},
		{"10 минут любого кардио-тренажера "},
	}
	s := Parse(rows)[0]
	g := s.Blocks[0].Groups
	if len(g) != 2 || len(g[0].Exercises) != 3 || g[0].Exercises[2].Name != "c" || len(g[1].Exercises) != 1 {
		t.Fatalf("groups: %+v", g)
	}
	cardio := s.Blocks[1].Groups
	if len(cardio) != 1 || cardio[0].Exercises[0].Name != "10 минут любого кардио-тренажера" || len(cardio[0].Exercises[0].Sets) != 0 {
		t.Fatalf("cardio: %+v", cardio)
	}
}

func TestTheFirstSessionsOfTheSample(t *testing.T) {
	sessions := parseFixture(t)
	first := sessions[0]
	if first.Label != "" || first.Name() != "ПРОБНАЯ ПРОГРАММА" {
		t.Errorf("first session: label %q, name %q", first.Label, first.Name())
	}
	if got := first.Blocks[0].Groups[1].Exercises[0].Sets; len(got) != 2 {
		t.Errorf("trailing empty sets kept: %+v", got)
	}
	g := sessions[1].Blocks[0].Groups
	if len(g) != 2 || len(g[0].Exercises) != 3 || g[0].Exercises[2].Name != "Подъём гантелей в стороны" {
		t.Fatalf("an odd run in brackets: %+v", g)
	}
	if got := g[0].Exercises[2].Sets[0]; got != (Set{Weight: "3,5", Reps: "15"}) {
		t.Errorf("a decimal comma: %+v", got)
	}
}
