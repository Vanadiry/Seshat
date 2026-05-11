// Package db 管理 SQLite 数据库连接、Schema 迁移和写入操作。
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	database.SetMaxOpenConns(4)
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := migrate(database); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return database, nil
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	for i, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	for _, stmt := range additiveMigrations {
		db.Exec(stmt)
	}
	return nil
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS subject (
		id INTEGER PRIMARY KEY, type INTEGER NOT NULL DEFAULT 2,
		name TEXT NOT NULL, name_cn TEXT NOT NULL DEFAULT '',
		summary TEXT NOT NULL DEFAULT '', date TEXT,
		platform TEXT NOT NULL DEFAULT '', eps INTEGER NOT NULL DEFAULT 0,
		total_episodes INTEGER NOT NULL DEFAULT 0, volumes INTEGER NOT NULL DEFAULT 0,
		series INTEGER NOT NULL DEFAULT 0, locked INTEGER NOT NULL DEFAULT 0,
		nsfw INTEGER NOT NULL DEFAULT 0, score REAL, rank INTEGER,
		rating_total INTEGER, wish_count INTEGER DEFAULT 0,
		collect_count INTEGER DEFAULT 0, doing_count INTEGER DEFAULT 0,
		on_hold_count INTEGER DEFAULT 0, dropped_count INTEGER DEFAULT 0,
		image_path TEXT NOT NULL DEFAULT '', image_grid_path TEXT NOT NULL DEFAULT '',
		infobox TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')))`,

	`CREATE TABLE IF NOT EXISTS character (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL,
		type INTEGER NOT NULL DEFAULT 1, summary TEXT NOT NULL DEFAULT '',
		gender TEXT, blood_type INTEGER, birth_year INTEGER,
		birth_mon INTEGER, birth_day INTEGER, locked INTEGER NOT NULL DEFAULT 0,
		nsfw INTEGER NOT NULL DEFAULT 0, image_path TEXT NOT NULL DEFAULT '',
		image_grid_path TEXT NOT NULL DEFAULT '', infobox TEXT NOT NULL DEFAULT '[]',
		comment_count INTEGER DEFAULT 0, collect_count INTEGER DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')))`,

	`CREATE TABLE IF NOT EXISTS person (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL,
		type INTEGER NOT NULL DEFAULT 1, summary TEXT NOT NULL DEFAULT '',
		gender TEXT, blood_type INTEGER, birth_year INTEGER,
		birth_mon INTEGER, birth_day INTEGER, locked INTEGER NOT NULL DEFAULT 0,
		image_path TEXT NOT NULL DEFAULT '', image_grid_path TEXT NOT NULL DEFAULT '',
		infobox TEXT NOT NULL DEFAULT '[]', career TEXT NOT NULL DEFAULT '[]',
		comment_count INTEGER DEFAULT 0, collect_count INTEGER DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')))`,

	`CREATE TABLE IF NOT EXISTS tag (name TEXT PRIMARY KEY)`,

	`CREATE TABLE IF NOT EXISTS subject_tag (
		subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		tag_name TEXT NOT NULL REFERENCES tag(name) ON DELETE CASCADE,
		count INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (subject_id, tag_name))`,

	`CREATE TABLE IF NOT EXISTS episode (
		id INTEGER PRIMARY KEY, subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		type INTEGER NOT NULL DEFAULT 0, sort REAL NOT NULL, ep REAL,
		name TEXT NOT NULL DEFAULT '', name_cn TEXT NOT NULL DEFAULT '',
		duration TEXT NOT NULL DEFAULT '', airdate TEXT,
		"desc" TEXT NOT NULL DEFAULT '', disc INTEGER NOT NULL DEFAULT 0)`,

	`CREATE TABLE IF NOT EXISTS subject_character (
		subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		character_id INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
		relation TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (subject_id, character_id))`,

	`CREATE TABLE IF NOT EXISTS subject_person (
		subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		person_id INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
		relation TEXT NOT NULL DEFAULT '', eps TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (subject_id, person_id, relation))`,

	`CREATE TABLE IF NOT EXISTS character_person (
		character_id INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
		person_id INTEGER NOT NULL REFERENCES person(id) ON DELETE CASCADE,
		subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		PRIMARY KEY (character_id, person_id, subject_id))`,

	`CREATE TABLE IF NOT EXISTS subject_relation (
		subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		related_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		relation TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (subject_id, related_id))`,

	`CREATE TABLE IF NOT EXISTS custom_field (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL DEFAULT 'text')`,

	`CREATE TABLE IF NOT EXISTS user_subject_data (
		subject_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		field_id INTEGER NOT NULL REFERENCES custom_field(id) ON DELETE CASCADE,
		value TEXT NOT NULL DEFAULT '', PRIMARY KEY (subject_id, field_id))`,

	`CREATE TABLE IF NOT EXISTS collection (
		subject_id INTEGER PRIMARY KEY REFERENCES subject(id) ON DELETE CASCADE,
		type INTEGER NOT NULL, rate INTEGER DEFAULT 0,
		comment TEXT DEFAULT '', tags TEXT DEFAULT '[]',
		private INTEGER DEFAULT 0, updated_at TEXT)`,

	`CREATE TABLE IF NOT EXISTS elo_comparison (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		winner_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		loser_id INTEGER NOT NULL REFERENCES subject(id) ON DELETE CASCADE,
		created_at TEXT NOT NULL DEFAULT (datetime('now')))`,

	`CREATE TABLE IF NOT EXISTS elo_rating (
		subject_id INTEGER PRIMARY KEY REFERENCES subject(id) ON DELETE CASCADE,
		rating REAL NOT NULL DEFAULT 1500.0,
		updated_at TEXT NOT NULL DEFAULT (datetime('now')))`,
}

var additiveMigrations = []string{
	"ALTER TABLE subject ADD COLUMN image_grid_path TEXT NOT NULL DEFAULT ''",
	"ALTER TABLE character ADD COLUMN image_grid_path TEXT NOT NULL DEFAULT ''",
	"ALTER TABLE person ADD COLUMN image_grid_path TEXT NOT NULL DEFAULT ''",
}
