// Package store is gym's persistence. For now that is only who may log in:
// the plans themselves are read from the coach's sheet (gym/server/sheet.go).
package store

import (
	"database/sql"

	"code.pecheny.me/pecheny/dopesuite/dopecore/schema"
)

// Migrations is the schema as a list (dopecore/schema): each step runs once
// and is recorded in schema_versions; a new step goes at the end.
// sloplint: ignore[magic-numbers] a step's version is its id in schema_versions, not a quantity
var Migrations = []schema.Migration{
	{Version: 1, Name: "accounts", Up: schema.Exec(`
-- The shape dopecore/tglogin and dopecore/authcred expect.
create table if not exists users(
  id integer primary key,
  telegram_user_id integer unique,
  telegram_username text,
  telegram_name text,
  username text unique,
  password_hash text,
  password_salt text,
  created_at text not null,
  updated_at text not null
);

create table if not exists sessions(
  id integer primary key,
  user_id integer not null references users(id) on delete cascade,
  token_hash text not null unique,
  created_at text not null,
  expires_at text not null,
  last_seen_at text not null
);

create table if not exists telegram_login_codes(
  id integer primary key,
  code text not null unique,
  kind text not null check (kind in ('register','login')),
  user_id integer references users(id),
  telegram_user_id integer,
  telegram_username text,
  telegram_name text,
  created_at text not null,
  expires_at text not null,
  consumed_at text
);
`)},
}

// Migrate applies Migrations; sqlitex.Open calls it.
func Migrate(db *sql.DB) error { return schema.Apply(db, Migrations) }
