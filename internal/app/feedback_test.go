package app

import (
	"campusreturn/internal/domain"
	"campusreturn/internal/ledger"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeedbackSnapshotsRecipientsAndGroupedReads(t *testing.T) {
	store, e := OpenStore(t.TempDir(), strings.Repeat("c", 64))
	if e != nil {
		t.Fatal(e)
	}
	defer store.DB.Close()
	srv, e := New(store, &DemoChain{S: store}, t.TempDir(), "Test12")
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"finder", "alice", "bob", "observer", "manager"} {
		role := "USER"
		if id == "manager" {
			role = "ADMIN"
		}
		check(put(store.DB, "user", id, User{ID: id, Username: id, DisplayName: "昵称" + id, Role: role, Status: "ACTIVE"}))
		check(put(store.DB, "session", domain.Hash(id), Session{UserID: id, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)}))
	}
	content := domain.Content{Name: "反馈回归杯", Category: "生活用品", Description: "虚拟反馈测试物品", Location: "测试服务台", FoundAt: "2026-01-01T00:00:00Z", Question: "物品上的暗记"}
	it := domain.Item{}
	seq := 0
	run := func(action, actor, claim, reason string) {
		t.Helper()
		seq++
		c := domain.Command{ID: fmt.Sprint("op", seq), ItemID: "test-item", ActorID: actor, Action: action, ExpectedVersion: it.Version, Data: domain.Payload{ClaimID: claim, ReasonHash: domain.Hash(reason)}}
		o := Operation{ID: c.ID, UserID: actor, ItemID: c.ItemID, Reason: reason, Status: "SUCCEEDED"}
		sec := Secret{Salt: "salt", Text: "private-proof-" + actor}
		if action == "CREATE" {
			c.Data.ContentHash = domain.Digest(content)
			c.Data.QuestionHash = domain.Hash(content.Question)
			c.Data.AnswerHash = domain.Hash(sec.Salt + ":" + sec.Text)
			check(put(store.DB, "content", c.ItemID+":"+c.Data.ContentHash, ContentVersion{Content: content}))
			o.Secret = store.Seal(sec)
		}
		if action == "CLAIM" {
			c.Data.EvidenceHash = domain.Hash(sec.Salt + ":" + sec.Text)
			o.Secret = store.Seal(sec)
		}
		if action == "RETURN" {
			c.Data.ReceiptHash = domain.Hash("receipt")
		}
		o.Command = c
		_, e := store.DB.Exec(`INSERT INTO operations VALUES(?,?,?,?,?)`, o.ID, actor, o.ID, "hash", jsonValue(o))
		if e != nil {
			t.Fatal(e)
		}
		var old *domain.Item
		if it.ID != "" {
			old = &it
		}
		msp := "Org1MSP"
		if domain.StudentAction(action) {
			msp = "Org2MSP"
		}
		next, e := ledger.Apply(old, c, msp, "tx"+c.ID, fmt.Sprintf("2026-10-26T01:00:%02dZ", seq))
		if e != nil {
			t.Fatal(e)
		}
		it = next
		if e = srv.applyReceipt(domain.Receipt{Item: next}); e != nil {
			t.Fatal(e)
		}
		if e = srv.applyReceipt(domain.Receipt{Item: next}); e != nil {
			t.Fatal(e)
		}
	}
	run("CREATE", "finder", "", "")
	run("CLAIM", "alice", "a1", "")
	run("REJECT", "manager", "a1", "特征描述不足，请补充细节")
	run("CLAIM", "bob", "b1", "")
	run("CLAIM", "alice", "a2", "")
	run("APPROVE", "manager", "b1", "材料与线下核对相符")
	run("RETURN", "manager", "b1", "交接完成")
	all := claimFeedback(store.DB, it.ID, "alice")
	if len(all) != 2 {
		t.Fatalf("expected two outcomes: %+v", all)
	}
	if all[1].Reason != "特征描述不足，请补充细节" || !all[1].ReasonAvailable {
		t.Fatal("original refusal reason lost")
	}
	if all[0].Status != "SUPERSEDED" || strings.Contains(all[0].Reason, "线下核对") {
		t.Fatal("other applicant decision leaked")
	}
	bobNotices, _ := records[Notice](store.DB, "notice", `AND json_extract(data,'$.userId')='bob'`)
	for _, n := range bobNotices {
		if n.Action == "REJECT" {
			t.Fatal("unrelated rejection broadcast")
		}
	}
	finderNotices, _ := records[Notice](store.DB, "notice", `AND json_extract(data,'$.userId')='finder'`)
	for _, n := range finderNotices {
		if n.Reason == "特征描述不足，请补充细节" {
			t.Fatal("applicant-only reason leaked to reporter")
		}
	}
	request := func(user, path string) (int, string) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1"+path, nil)
		if user != "" {
			r.Header.Set("Authorization", "Bearer "+user)
		}
		w := httptest.NewRecorder()
		srv.Router.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	code, raw := request("alice", "/me/claims?grouped=true")
	if code != 200 {
		t.Fatal(raw)
	}
	var list struct {
		Total int
		Items []struct {
			Attempts []json.RawMessage `json:"attempts"`
		}
	}
	json.Unmarshal([]byte(raw), &list)
	if list.Total != 1 || len(list.Items[0].Attempts) != 2 {
		t.Fatal(raw)
	}
	_, public := request("", "/items/"+it.ID)
	if strings.Contains(public, "特征描述不足") || strings.Contains(public, "private-proof") {
		t.Fatal("private outcome exposed publicly")
	}
	ns, _ := records[Notice](store.DB, "notice", `AND json_extract(data,'$.userId')='alice' AND json_extract(data,'$.action')='REJECT'`)
	if len(ns) != 1 || ns[0].Read {
		t.Fatal("result missing or duplicate")
	}
	if status, _ := request("bob", "/notifications/"+ns[0].ID); status != 403 {
		t.Fatal("cross-account notice readable")
	}
	if status, raw := request("alice", "/notifications/"+ns[0].ID); status != 200 || !strings.Contains(raw, "特征描述不足") {
		t.Fatal(raw)
	}
	grouped := groupedNotices(finderNotices)
	processGroups := 0
	for _, g := range grouped {
		if g.Process {
			processGroups++
			if len(g.Entries) < 3 {
				t.Fatal("process events lost")
			}
		}
	}
	if processGroups != 1 {
		t.Fatal("process not grouped")
	}
	// 旧记录补齐仅使用各自的成功版本和管理原因，保留账本及原通知已读状态。
	ns[0].Read = true
	check(put(store.DB, "notice", ns[0].ID, ns[0]))
	store.DB.Exec(`DELETE FROM records WHERE kind='claim_feedback' OR (kind='meta' AND id='feedback-v1')`)
	if e = srv.repairFeedback(); e != nil {
		t.Fatal(e)
	}
	after := claimFeedback(store.DB, it.ID, "alice")
	if len(after) != 2 || after[1].Reason != all[1].Reason || after[1].At != all[1].At {
		t.Fatal("legacy reconstruction changed result")
	}
	restored, _ := get[Notice](store.DB, "notice", ns[0].ID)
	if !restored.Read || restored.At != ns[0].At {
		t.Fatal("legacy read/time overwritten")
	}
	latest := findItem(store.DB, it.ID)
	if latest.Version != it.Version || latest.Status != "RETURNED" {
		t.Fatal("migration touched business state")
	}
	for _, v := range []Evaluation{{ID: "ev1", UserID: "bob", ItemID: it.ID, Stars: 5, Text: "交接顺利", Status: "PUBLIC"}, {ID: "ev2", UserID: "finder", ItemID: it.ID, Stars: 3, Text: "隐藏测试", Status: "HIDDEN"}} {
		check(put(store.DB, "evaluation", v.ID, v))
	}
	if status, raw := request("manager", "/admin/evaluations?status=HIDDEN&q=隐藏"); status != 200 || !strings.Contains(raw, "隐藏测试") || strings.Contains(raw, "交接顺利") {
		t.Fatal(raw)
	}
	if _, raw := request("bob", "/items/"+it.ID+"/evaluations"); !strings.Contains(raw, `"mine":true`) || strings.Contains(raw, "隐藏测试") || strings.Contains(raw, "passwordHash") {
		t.Fatal(raw)
	}
}
