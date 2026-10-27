package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlatformLifecycle(t *testing.T) {
	store, e := OpenStore(t.TempDir(), strings.Repeat("b", 64))
	if e != nil {
		t.Fatal(e)
	}
	defer store.DB.Close()
	srv, e := New(store, &DemoChain{S: store}, t.TempDir(), "Test-only-Admin-2026")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.Run(ctx)
	ts := httptest.NewServer(srv.Router)
	defer ts.Close()
	req := func(token, method, path string, payload any) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(payload)
		r, e := http.NewRequest(method, ts.URL+"/api/v1"+path, bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", uid())
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		v := map[string]any{}
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatalf("%s %d %s", path, res.StatusCode, raw)
		}
		return res.StatusCode, v
	}
	login := func(name, pw string) string {
		code, v := req("", "POST", "/auth/login", map[string]string{"username": name, "password": pw})
		if code != 200 {
			t.Fatalf("login %d %v", code, v)
		}
		return v["token"].(string)
	}
	register := func(name string) (string, string) {
		code, v := req("", "POST", "/auth/register", map[string]string{"username": name, "password": "Sample-pass-2026"})
		if code != 201 {
			t.Fatal(v)
		}
		return login(name, "Sample-pass-2026"), v["user"].(map[string]any)["id"].(string)
	}
	admin := login("admin", "Test-only-Admin-2026")
	finder, finderID := register("finder")
	bob, bobID := register("bob")
	carol, _ := register("carol")
	wait := func(token string, op map[string]any) map[string]any {
		t.Helper()
		id, ok := op["id"].(string)
		if !ok {
			t.Fatalf("operation: %v", op)
		}
		for i := 0; i < 100; i++ {
			_, v := req(token, "GET", "/operations/"+id, nil)
			if v["status"] == "SUCCEEDED" {
				return v
			}
			if v["status"] == "FAILED" {
				t.Fatalf("job: %v", v)
			}
			time.Sleep(40 * time.Millisecond)
		}
		t.Fatalf("timeout %v", op)
		return nil
	}
	createPayload := map[string]any{"content": map[string]string{"name": "验收保温杯", "category": "生活用品", "description": "课程验收用白色保温杯", "location": "图书馆一楼", "foundAt": "2026-01-01T00:00:00Z", "question": "杯底刻字是什么"}, "answer": "Secret-Unpublished-Answer"}
	code, op := req(finder, "POST", "/items", createPayload)
	if code != 202 {
		t.Fatal(code, op)
	}
	wait(finder, op)
	id := op["itemId"].(string)
	item := func() map[string]any {
		code, v := req(finder, "GET", "/items/"+id, nil)
		if code != 200 {
			t.Fatal(v)
		}
		return v
	}
	version := func() float64 { return item()["version"].(float64) }
	_, op = req(bob, "POST", "/items/"+id+"/claims", map[string]any{"evidence": "My-private-evidence", "expectedVersion": version()})
	wait(bob, op)
	claim1 := op["command"].(map[string]any)["data"].(map[string]any)["claimId"].(string)
	_, op = req(carol, "POST", "/items/"+id+"/claims", map[string]any{"evidence": "Second-private-proof", "expectedVersion": version()})
	wait(carol, op)
	code, _ = req(bob, "GET", "/admin/items/"+id+"/private", nil)
	if code != 403 {
		t.Fatal("private permission", code)
	}
	oldVersion := version() - 1
	code, _ = req(admin, "POST", "/admin/items/"+id+"/actions", map[string]any{"action": "APPROVE", "claimId": claim1, "expectedVersion": oldVersion, "note": "核验材料相符"})
	if code != 409 {
		t.Fatal("stale request", code)
	}
	_, op = req(admin, "POST", "/admin/items/"+id+"/actions", map[string]any{"action": "APPROVE", "claimId": claim1, "expectedVersion": version(), "note": "核验材料相符"})
	wait(admin, op)
	_, handover := req(bob, "POST", "/items/"+id+"/handover-code", map[string]any{})
	_, op = req(admin, "POST", "/admin/items/"+id+"/actions", map[string]any{"action": "RETURN", "claimId": claim1, "code": handover["code"], "expectedVersion": version(), "note": "现场完成实物交接"})
	wait(admin, op)
	_, points := req(finder, "GET", "/me/points", nil)
	if points["score"] != float64(10) {
		t.Fatal(points)
	}
	_, points = req(bob, "GET", "/me/points", nil)
	if points["score"] != float64(2) {
		t.Fatal(points)
	}
	code, _ = req(bob, "POST", "/items/"+id+"/evaluations", map[string]any{"stars": 5, "text": "交接顺利，信息准确"})
	if code != 201 {
		t.Fatal("evaluation", code)
	}
	code, _ = req(admin, "PATCH", "/admin/users/"+bobID, map[string]any{"status": "DISABLED", "reason": "验收账号禁用"})
	if code != 200 {
		t.Fatal(code)
	}
	code, _ = req(bob, "GET", "/me", nil)
	if code != 401 {
		t.Fatal("disabled token", code)
	}
	_, op = req(admin, "POST", "/admin/items/"+id+"/actions", map[string]any{"action": "FLAG_FRAUD", "claimId": claim1, "expectedVersion": version(), "note": "根据争议核实为测试虚假交接"})
	wait(admin, op)
	_, points = req(finder, "GET", "/me/points", nil)
	if points["score"] != float64(0) {
		t.Fatal("correction", points)
	}
	if item()["status"] != "RETURNED" {
		t.Fatal("history overwritten")
	}
	rows := must(records[string](store.DB, "private", ""))
	for _, raw := range rows {
		if strings.Contains(raw, "Secret-Unpublished-Answer") {
			t.Fatal("plaintext persisted")
		}
	}
	checkSession, e := New(store, &DemoChain{S: store}, t.TempDir(), "")
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	r.Header.Set("Authorization", "Bearer "+finder)
	w := httptest.NewRecorder()
	checkSession.Router.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("session did not survive new server")
	}
	_ = finderID
}
