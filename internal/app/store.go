package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	DB   *sql.DB
	AEAD cipher.AEAD
}

//go:embed migrations/001.sql
var schema string

type querier interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func OpenStore(dir, key string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	k, e := hex.DecodeString(key)
	if e != nil || len(k) != 32 {
		return nil, errors.New("APP_ENCRYPTION_KEY 必须是64位十六进制密钥")
	}
	block, e := aes.NewCipher(k)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "platform.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(schema); e != nil {
		db.Close()
		return nil, e
	}
	_ = os.Chmod(filepath.Join(dir, "platform.db"), 0600)
	s := &Store{DB: db, AEAD: g}
	marker, err := get[string](db, "meta", "encryption-key-check")
	if err == nil {
		var text string
		if s.Unseal(marker, &text) != nil || text != "campus-return-v1" {
			db.Close()
			return nil, errors.New("加密密钥与数据库不匹配，拒绝启动")
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		previous, e := records[string](db, "private", "LIMIT 1")
		if e != nil {
			db.Close()
			return nil, e
		}
		if len(previous) > 0 {
			var material Secret
			if s.Unseal(previous[0], &material) != nil {
				db.Close()
				return nil, errors.New("现有私密材料无法解密，请恢复原密钥")
			}
		}
		if e = put(db, "meta", "encryption-key-check", s.Seal("campus-return-v1")); e != nil {
			db.Close()
			return nil, e
		}
	} else {
		db.Close()
		return nil, err
	}
	return s, nil
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func uid() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func put(q querier, kind, id string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = q.Exec(`INSERT INTO records VALUES(?,?,?,?,?) ON CONFLICT(kind,id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, kind, id, string(b), now(), now())
	return e
}
func get[T any](q querier, kind, id string) (T, error) {
	var v T
	var b string
	e := q.QueryRow(`SELECT data FROM records WHERE kind=? AND id=?`, kind, id).Scan(&b)
	if e != nil {
		return v, e
	}
	e = json.Unmarshal([]byte(b), &v)
	return v, e
}
func records[T any](q querier, kind, where string, args ...any) ([]T, error) {
	a := []any{kind}
	a = append(a, args...)
	rows, e := q.Query(`SELECT data FROM records WHERE kind=? `+where, a...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var b string
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		var v T
		if e = json.Unmarshal([]byte(b), &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Seal(v any) string {
	b, _ := json.Marshal(v)
	nonce := make([]byte, s.AEAD.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		panic(e)
	}
	return base64.StdEncoding.EncodeToString(s.AEAD.Seal(nonce, nonce, b, nil))
}
func (s *Store) Unseal(raw string, v any) error {
	b, e := base64.StdEncoding.DecodeString(raw)
	if e != nil {
		return e
	}
	n := s.AEAD.NonceSize()
	if len(b) < n {
		return errors.New("私密记录损坏")
	}
	b, e = s.AEAD.Open(nil, b[:n], b[n:], nil)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (s *Store) Transaction(fn func(*sql.Tx) error) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit()
}
