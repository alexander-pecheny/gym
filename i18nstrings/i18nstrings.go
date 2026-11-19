// Package i18nstrings is gym's Catalog (root docs/adr/0006): every string it
// shows, one TOML file per Surface under ru/, generated into the typed Strings
// below. Edit the TOML, then `just generate-strings`.
//
//go:generate go -C ../dopesuite/scripts/i18nstringsgen run . -dir ../i18nstrings
package i18nstrings

// Default is what a caller renders in when nothing chose a language.
var Default = RU
