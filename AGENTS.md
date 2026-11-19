# gym — Agent Notes

The coach's training plans, one Session per phone screen, at gym.pecheny.me.
A prototype: step 1 is reading the plans while training; step 2 (not built) is
a way for the coach to enter them here instead of in the Google sheet.

- **dopesuite is a git submodule** (`dopesuite/`), pinned to a commit, and
  `go.mod` points dopecore and dopeuikit at it with `replace`: the kit embeds
  its built JS (`dopeuikit/assets/js/dist`), which dopesuite does not commit
  (its ADR-0001), so a plain `go get` of dopeuikit would not compile. `just
  build-web` builds that JS inside the submodule; every recipe that compiles
  depends on it. Clone with `--recurse-submodules`. To move to a newer
  dopesuite: `git -C dopesuite pull origin main`, `just check`, commit the new
  pointer. The generators (`go generate ./...`) also run from the submodule.
- **No real training data in this repo.** It is public; the sheet's URL and the
  bot token live only in `/etc/gym.env` on vps-he (and a local `.env`).

- **The data is the coach's Google sheet**, the "ТРЕНИНГ (без тренера)" tab,
  read as its CSV export (`GYM_SHEET_CSV`) at most once a minute. The last good
  export is kept at `GYM_SHEET_CACHE` and shown, with a note, when Google does
  not answer. `domain/plan/sheet.go` is the reader; its rules (header rows,
  Block titles in capitals, supersets marked in the name and paired in order)
  are in its comment. `testdata/sample.csv` is a made-up sheet in the tab's
  exact layout; keep it made up.
- **A Session's id is its position in the sheet.** The coach appends, so `/`
  shows the last one. Inserting a Session in the middle renumbers the rest.
- **Who may look** is `GYM_ALLOWED_TELEGRAM` (comma separated handles). Login
  is telegram only (`dopecore/tglogin`); an account is only created for a
  handle on the list, and every page checks the list again.
- **The look is dope's Итог**, and it is the kit's own classes, not a copy:
  each Block is a `.results-table` (core.css), every exercise a paper row, a
  superset's rows glued like tied places (`.results-group-first/last`, the
  1px structure hairline), and an 8px `.results-group-gap` row between
  groups. The coach's note sits under the name as `.results-team-city`. The
  phone padding and type are dope's own mobile values, copied from its layer.
  Nothing here should be invented: no accents, markers or labels dope lacks.
  The primitives are `planblock`, `plangroup`, `exercise` and `setcell`
  (`web/ui`); the Block's expander draws the whole table.
- **What folds, so a Session fits a phone screen:** an exercise with no weight
  and the same reps every set is one "3 × 15" cell (`Exercise.Uniform`); a
  Block whose exercises all fold drops its set numbers and its four columns
  (`Block.Folded`); a note the coach wrote on several exercises of a Block is
  said once, under the Block's title (`Block.SharedNote`).
- Pages are built server-side with the typed builder (like qy); there is no
  TypeScript. Port 9689 on vps-he, unit `deploy/gym.service`, shipped by
  `just deploy` (`deploy/deploy.sh`).
