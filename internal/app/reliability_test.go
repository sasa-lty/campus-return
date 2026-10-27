package app

import (
	"campusreturn/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type timeoutAfterCommit struct {
	*DemoChain
	count int
}

func (c *timeoutAfterCommit) Submit(o Operation, save func(string, []byte) error) (domain.Receipt, error) {
	c.count++
	r, e := c.DemoChain.Submit(o, save)
	if e != nil {
		return r, e
	}
	return domain.Receipt{}, uncertainError{errors.New("simulated lost response after successful commit")}
}
func TestReconcileCommitTimeoutAndEncryptedMaterial(t *testing.T) {
	s, e := OpenStore(t.TempDir(), strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	ch := &timeoutAfterCommit{DemoChain: &DemoChain{S: s}}
	srv, e := New(s, ch, t.TempDir(), "Test-only-Admin-2026")
	if e != nil {
		t.Fatal(e)
	}
	secret := Secret{Salt: uid(), Text: "private-answer-never-in-ledger"}
	if e = put(s.DB, "user", "user", User{ID: "user", Username: "timeout_actor", Role: "USER", Status: "ACTIVE"}); e != nil {
		t.Fatal(e)
	}
	content := domain.Content{Name: "test cup"}
	cmd := domain.Command{ID: uid(), ItemID: "timeout-item", ActorID: "user", Action: "CREATE", Data: domain.Payload{ContentHash: domain.Digest(content), AnswerHash: domain.Hash(secret.Salt + ":" + secret.Text), QuestionHash: domain.Hash("q")}}
	op := Operation{ID: cmd.ID, ItemID: cmd.ItemID, UserID: "user", Status: "QUEUED", Command: cmd, Secret: s.Seal(secret), At: now(), Content: &content}
	_, e = s.DB.Exec(`INSERT INTO operations VALUES(?,?,?,?,?)`, op.ID, "user", "timeout-test", "hash", jsonValue(op))
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.Run(ctx)
	for i := 0; i < 100; i++ {
		o, e := srv.operation(op.ID)
		if e == nil && o.Status == "SUCCEEDED" {
			if ch.count != 1 {
				t.Fatal("transaction resubmitted after confirmed commit")
			}
			v, e := srv.secret("item:"+op.ItemID, cmd.Data.AnswerHash)
			if e != nil || v.Text != secret.Text {
				t.Fatal("private material lost after timeout", e)
			}
			item, e := ch.Get(op.ItemID)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(item)
			if strings.Contains(string(b), secret.Text) {
				t.Fatal("plaintext entered ledger")
			}
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("committed transaction not reconciled")
}
func TestHiddenItemAnonymousAndRecoverySession(t *testing.T) {
	it := domain.Item{ReporterID: "reporter", Visibility: "HIDDEN"}
	if visible(it, User{}) {
		t.Fatal("anonymous empty id treated as owner")
	}
	if !visible(it, User{ID: "reporter"}) {
		t.Fatal("reporter lost access")
	}
}
