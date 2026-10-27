package app

import (
	"bytes"
	"campusreturn/internal/domain"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPersonalOverviewIsolationAndUnpaginatedCounts(t *testing.T) {
	store, err := OpenStore(t.TempDir(), strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	srv, err := New(store, &DemoChain{S: store}, t.TempDir(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	request := func(token, method, path string, payload any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(payload)
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		srv.Router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: got %d, want %d, %s", path, w.Code, want, w.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	register := func(name string) (string, string) {
		v := request("", "POST", "/auth/register", map[string]string{"username": name, "password": "123456"}, 201)
		login := request("", "POST", "/auth/login", map[string]string{"username": name, "password": "123456"}, 200)
		return v["user"].(map[string]any)["id"].(string), login["token"].(string)
	}
	mine, token := register("overview_owner")
	other, otherToken := register("overview_other")
	for i := 0; i < 17; i++ {
		id := fmt.Sprintf("own-%02d", i)
		it := domain.Item{ID: id, ReporterID: mine, Status: "FOUND", Visibility: "HIDDEN", Version: 1}
		if err := put(store.DB, "item", id, it); err != nil {
			t.Fatal(err)
		}
		if err := put(store.DB, "notice", id, Notice{ID: id, UserID: mine, Title: "自己的消息", Read: i == 0, At: now()}); err != nil {
			t.Fatal(err)
		}
	}
	for i, state := range []string{"CLAIM_PENDING", "CLAIMED", "RETURNED"} {
		id := fmt.Sprint("claim-", i)
		cl := domain.Claim{ID: id, UserID: mine, Status: "PENDING", EvidenceHash: "PRIVATE-HASH"}
		it := domain.Item{ID: id, ReporterID: other, Status: state, Visibility: "PUBLIC", Version: 1, Claims: []domain.Claim{cl}}
		if state != "CLAIM_PENDING" {
			it.OwnerID = mine
			it.Claims[0].Status = "APPROVED"
		}
		if err := put(store.DB, "item", id, it); err != nil {
			t.Fatal(err)
		}
	}
	if err := put(store.DB, "notice", "other-secret", Notice{ID: "other-secret", UserID: other, Title: "OTHER-PRIVATE-NOTICE"}); err != nil {
		t.Fatal(err)
	}
	if err := put(store.DB, "report", "report", Report{ID: "report", UserID: mine, Status: "IN_PROGRESS", Reason: "PRIVATE-REPORT"}); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"QUEUED", "SUBMITTING", "PENDING", "SUCCEEDED", "FAILED"} {
		op := Operation{ID: status, UserID: mine, Status: status, Secret: "PRIVATE-SECRET"}
		if _, err := store.DB.Exec(`INSERT INTO operations VALUES(?,?,?,?,?)`, status, mine, status, status, jsonValue(op)); err != nil {
			t.Fatal(err)
		}
	}
	request("", "GET", "/me/overview", nil, 401)
	result := request(token, "GET", "/me/overview", nil, 200)
	counts := result["counts"].(map[string]any)
	for key, expected := range map[string]int{"items": 17, "claims": 3, "pendingClaims": 1, "handovers": 1, "unread": 16, "openReports": 1, "pendingOperations": 3} {
		if counts[key] != float64(expected) {
			t.Errorf("%s: got %v, want %d", key, counts[key], expected)
		}
	}
	if len(result["notices"].([]any)) != 5 {
		t.Fatal("recent notices should be bounded")
	}
	if len(result["handovers"].([]any)) != 1 {
		t.Fatal("only active owned handovers are actionable")
	}
	raw, _ := json.Marshal(result)
	for _, secret := range []string{"OTHER-PRIVATE-NOTICE", "PRIVATE-REPORT", "PRIVATE-SECRET", "PRIVATE-HASH", "passwordHash", "recoveryHash"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("overview leaked %s", secret)
		}
	}
	request(token, "GET", "/admin/operations?status=ATTENTION", nil, 403)
	adminToken := request("", "POST", "/auth/login", map[string]string{"username": "admin", "password": "123456"}, 200)["token"].(string)
	attention := request(adminToken, "GET", "/admin/operations?status=ATTENTION", nil, 200)
	if attention["total"] != float64(3) {
		t.Fatal("attention queue must match dashboard count")
	}
	otherResult := request(otherToken, "GET", "/me/overview", nil, 200)
	if otherResult["counts"].(map[string]any)["items"] != float64(3) {
		t.Fatal("cross-account count contamination")
	}
	// 忽略客户端提供的 userId，始终以会话身份为准。
	spoofed := request(otherToken, "GET", "/me/overview?userId="+mine, nil, 200)
	if spoofed["counts"].(map[string]any)["items"] != float64(3) {
		t.Fatal("client identity override")
	}
}
