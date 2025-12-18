package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	path string
	conn *sql.DB
}

func Open() (*DB, error) {
	return OpenAt(DefaultPath())
}

func OpenAt(path string) (*DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path is required")
	}

	clean := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(clean), 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	conn, err := openAndInit(clean)
	if err == nil {
		return &DB{path: clean, conn: conn}, nil
	}

	// Graceful handling: if the database is corrupt, preserve it and recreate.
	if !isCorruptSQLiteError(err) {
		return nil, err
	}

	if _, statErr := os.Stat(clean); statErr == nil {
		backupPath := clean + ".corrupt." + time.Now().UTC().Format("20060102T150405Z")
		if renameErr := os.Rename(clean, backupPath); renameErr != nil {
			return nil, fmt.Errorf("db appears corrupt (%v), and rename failed: %w", err, renameErr)
		}
	}

	conn, err = openAndInit(clean)
	if err != nil {
		return nil, err
	}
	return &DB{path: clean, conn: conn}, nil
}

func (d *DB) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}
	return d.conn.Close()
}

func (d *DB) Conn() *sql.DB {
	if d == nil {
		return nil
	}
	return d.conn
}

func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func DefaultPath() string {
	if caamHome := os.Getenv("CAAM_HOME"); caamHome != "" {
		return filepath.Join(caamHome, "data", "caam.db")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".caam", "data", "caam.db")
	}
	return filepath.Join(homeDir, ".caam", "data", "caam.db")
}

func openAndInit(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite PRAGMAs are per-connection; keep a single shared connection.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	// Ensure we don't leak file descriptors on init errors.
	initErr := func() error {
		if err := conn.Ping(); err != nil {
			return fmt.Errorf("ping: %w", err)
		}

		if err := enableWAL(conn); err != nil {
			return err
		}
		if err := RunMigrations(conn); err != nil {
			return err
		}
		return nil
	}()

	if initErr != nil {
		_ = conn.Close()
		return nil, initErr
	}

	return conn, nil
}

func dsn(path string) string {
	// Use an explicit file: DSN so we can pass mode=rwc for auto-create.
	return "file:" + filepath.ToSlash(path) + "?mode=rwc"
}

func enableWAL(conn *sql.DB) error {
	if conn == nil {
		return fmt.Errorf("conn is nil")
	}

	// Enable WAL mode for concurrent reads.
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("set journal_mode=WAL: %w", err)
	}
	// Foreign keys are off by default in SQLite.
	if _, err := conn.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		return fmt.Errorf("set foreign_keys=ON: %w", err)
	}
	return nil
}

func isCorruptSQLiteError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrInvalid) {
		return true
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "file is not a database"):
		return true
	case strings.Contains(msg, "database disk image is malformed"):
		return true
	case strings.Contains(msg, "malformed"):
		return true
	default:
		return false
	}
}
