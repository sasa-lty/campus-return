package ledger

import (
	"campusreturn/internal/domain"
	"strings"
	"testing"
)

func TestLifecycleAndPermissions(t *testing.T) {
	hash := strings.Repeat("a", 64)
	create := domain.Command{ID: "op1", Action: "CREATE", ActorID: "finder", ItemID: "item1", Data: domain.Payload{ContentHash: hash, AnswerHash: hash, QuestionHash: hash}}
	if _, e := Apply(nil, create, "Org1MSP", "tx", "2026-10-23T00:00:00Z"); e == nil {
		t.Fatal("wrong MSP accepted")
	}
	it, e := Apply(nil, create, "Org2MSP", "tx1", "2026-10-23T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	act := func(action, actor, claim, msp string) error {
		c := domain.Command{ID: action + claim, Action: action, ActorID: actor, ItemID: it.ID, ExpectedVersion: it.Version, Data: domain.Payload{ClaimID: claim, EvidenceHash: hash, ReasonHash: hash, ReceiptHash: hash}}
		next, e := Apply(&it, c, msp, action+claim, "2026-10-23T00:00:01Z")
		if e == nil {
			it = next
		}
		return e
	}
	if e = act("CLAIM", "finder", "self", "Org2MSP"); e == nil {
		t.Fatal("self claim accepted")
	}
	if e = act("CLAIM", "bob", "c1", "Org2MSP"); e != nil {
		t.Fatal(e)
	}
	old := it
	if e = act("CLAIM", "carol", "c2", "Org2MSP"); e != nil {
		t.Fatal(e)
	}
	if old.Claims[0].Status != "PENDING" || len(old.Claims) != 1 {
		t.Fatal("simulation modified old version")
	}
	c := domain.Command{ID: "stale", Action: "APPROVE", ActorID: "admin", ItemID: it.ID, ExpectedVersion: old.Version, Data: domain.Payload{ClaimID: "c1", ReasonHash: hash}}
	if _, e = Apply(&it, c, "Org1MSP", "tx", "x"); e == nil {
		t.Fatal("stale approval accepted")
	}
	if e = act("APPROVE", "admin", "missing", "Org1MSP"); e == nil {
		t.Fatal("wrong claim accepted")
	}
	if e = act("APPROVE", "admin", "c2", "Org1MSP"); e != nil {
		t.Fatal(e)
	}
	if it.OwnerID != "carol" || it.Claims[0].Status != "SUPERSEDED" {
		t.Fatal("candidate selection failed")
	}
	if e = act("RETURN", "admin", "c1", "Org1MSP"); e == nil {
		t.Fatal("return of wrong claim accepted")
	}
	if e = act("RETURN", "admin", "c2", "Org1MSP"); e != nil {
		t.Fatal(e)
	}
	if it.Status != "RETURNED" {
		t.Fatal(it.Status)
	}
	if e = act("REVOKE", "admin", "c2", "Org1MSP"); e == nil {
		t.Fatal("returned history rewritten")
	}
	if e = act("FLAG_FRAUD", "admin", "c2", "Org1MSP"); e != nil {
		t.Fatal(e)
	}
	if it.Status != "RETURNED" {
		t.Fatal("fraud correction erased return")
	}
}
func TestRejectWithdrawAndQuestionProtection(t *testing.T) {
	h := strings.Repeat("b", 64)
	it := domain.Item{ID: "i", ReporterID: "f", Status: "CLAIM_PENDING", Visibility: "PUBLIC", Version: 3, QuestionHash: h, AnswerHash: h, Claims: []domain.Claim{{ID: "c", UserID: "u", Status: "PENDING"}}}
	c := domain.Command{ID: "o", ItemID: "i", ActorID: "a", Action: "UPDATE", ExpectedVersion: 3, Data: domain.Payload{ContentHash: h, AnswerHash: h, QuestionHash: strings.Repeat("c", 64), ReasonHash: h}}
	if _, e := Apply(&it, c, "Org1MSP", "tx", "at"); e == nil {
		t.Fatal("question changed after claim")
	}
	c.Action = "REJECT"
	c.Data.ClaimID = "c"
	next, e := Apply(&it, c, "Org1MSP", "tx", "at")
	if e != nil || next.Status != "FOUND" || next.Claims[0].Status != "REJECTED" {
		t.Fatal(next, e)
	}
	c.Action = "WITHDRAW"
	c.ActorID = "outsider"
	if _, e = Apply(&it, c, "Org2MSP", "tx", "at"); e == nil {
		t.Fatal("other user withdrew claim")
	}
}
