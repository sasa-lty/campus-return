package app

import (
	"campusreturn/internal/domain"
	"strings"
	"testing"
)

func TestClaimNoticesAndLegacyRepair(t *testing.T) {
	store, err := OpenStore(t.TempDir(), strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	srv, err := New(store, &DemoChain{S: store}, t.TempDir(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"finder", "alice", "bob"} {
		if err = put(store.DB, "user", id, User{ID: id, Username: id, Role: "USER", Status: "ACTIVE"}); err != nil {
			t.Fatal(err)
		}
	}
	users, _ := records[User](store.DB, "user", `AND json_extract(data,'$.role')='SUPER_ADMIN'`)
	adminID := users[0].ID
	item := domain.Item{ID: "item", ReporterID: "finder", Version: 1, Claims: []domain.Claim{{ID: "old", UserID: "alice", Status: "PENDING"}, {ID: "new", UserID: "bob", Status: "PENDING"}}, Events: []domain.Event{{Action: "CLAIM", ActorID: "bob", OperationID: "claim-op", TxID: "tx", At: now()}}}
	if err = srv.applyReceipt(domain.Receipt{Item: item}); err != nil {
		t.Fatal(err)
	}
	if err = srv.applyReceipt(domain.Receipt{Item: item}); err != nil {
		t.Fatal(err)
	}
	for user, title := range map[string]string{"finder": "收到新的认领申请", "bob": "认领申请已提交，等待管理员审核", adminID: "有新的认领申请待审核"} {
		n, err := records[Notice](store.DB, "notice", `AND json_extract(data,'$.userId')=?`, user)
		if err != nil || len(n) != 1 || n[0].Title != title {
			t.Fatalf("incorrect or duplicated notification for %s", user)
		}
	}
	if n, _ := records[Notice](store.DB, "notice", `AND json_extract(data,'$.userId')='alice'`); len(n) != 0 {
		t.Fatal("unrelated claimant received another person's application")
	}
	legacy := Operation{ID: "legacy", UserID: "bob", Command: domain.Command{Action: "CLAIM"}}
	if _, err = store.DB.Exec(`INSERT INTO operations VALUES(?,?,?,?,?)`, "legacy", "bob", "legacy-key", "hash", jsonValue(legacy)); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"bob", "alice", "finder"} {
		if err = put(store.DB, "notice", "legacy:"+user, Notice{ID: "legacy:" + user, UserID: user, ItemID: "item", Title: "收到新的认领申请", Read: true, At: "2026-01-01T00:00:00Z"}); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = store.DB.Exec(`DELETE FROM records WHERE kind='meta' AND id='claim-notices-v2'`)
	if err = srv.repairClaimNotices(); err != nil {
		t.Fatal(err)
	}
	n, err := get[Notice](store.DB, "notice", "legacy:bob")
	if err != nil || n.Title != "认领申请已提交，等待管理员审核" || !n.Read || n.At != "2026-01-01T00:00:00Z" {
		t.Fatal("legacy repair lost notice state")
	}
	if _, err = get[Notice](store.DB, "notice", "legacy:alice"); err == nil {
		t.Fatal("legacy unrelated notice remains")
	}
}
