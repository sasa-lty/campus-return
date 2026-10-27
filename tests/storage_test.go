package tests

import (
	"campusreturn/internal/app"
	"database/sql"
	_ "modernc.org/sqlite"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupSnapshotAndKeySeparation(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("c", 64)
	s, e := app.OpenStore(dir, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(`INSERT INTO records VALUES('test','record','{"valid":true}','now','now')`); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "snapshot.db")
	if _, e = s.DB.Exec("VACUUM INTO ?", out); e != nil {
		t.Fatal(e)
	}
	copy, e := sql.Open("sqlite", out)
	if e != nil {
		t.Fatal(e)
	}
	var status string
	if e = copy.QueryRow("PRAGMA integrity_check").Scan(&status); e != nil || status != "ok" {
		t.Fatal(status, e)
	}
	var n int
	if e = copy.QueryRow(`SELECT count(*) FROM records WHERE kind='test'`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	copy.Close()
	s.DB.Close()
	if wrong, e := app.OpenStore(dir, strings.Repeat("d", 64)); e == nil {
		wrong.DB.Close()
		t.Fatal("wrong encryption key accepted")
	}
}
