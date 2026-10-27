package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSixCharacterPasswordLifecycle(t *testing.T) {
	store, err := OpenStore(t.TempDir(), strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	srv, err := New(store, &DemoChain{S: store}, t.TempDir(), "123456")
	if err != nil {
		t.Fatal(err)
	}
	request := func(token, method, path string, value any, want int) map[string]any {
		t.Helper()
		data, _ := json.Marshal(value)
		req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		srv.Router.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: got HTTP %d, want %d", method, path, response.Code, want)
		}
		var out map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, tc := range []struct {
		name, password string
		code           int
	}{
		{"short", "12345", 400},
		{"short_unicode", "密码长五个", 400},
		{"six_unicode", "这是六个汉字", 201},
		{"too_many_bytes", strings.Repeat("字", 25), 400},
	} {
		request("", "POST", "/auth/register", map[string]string{"username": tc.name, "password": tc.password}, tc.code)
	}
	created := request("", "POST", "/auth/register", map[string]string{"username": "six_chars", "password": "123456"}, 201)
	recovery := created["recoveryCode"].(string)
	login := func(password string) string {
		t.Helper()
		return request("", "POST", "/auth/login", map[string]string{"username": "six_chars", "password": password}, 200)["token"].(string)
	}
	token := login("123456")
	request(token, "POST", "/me/password", map[string]string{"oldPassword": "123456", "password": "12345"}, 400)
	request(token, "POST", "/me/password", map[string]string{"oldPassword": "123456", "password": "654321"}, 200)
	request(token, "GET", "/me", nil, 401)
	token = login("654321")
	request("", "POST", "/auth/recover", map[string]string{"username": "six_chars", "recoveryCode": recovery, "password": "12345"}, 400)
	request("", "POST", "/auth/recover", map[string]string{"username": "six_chars", "recoveryCode": recovery, "password": "abcdef"}, 200)
	request(token, "GET", "/me", nil, 401)
	_ = login("abcdef")
}
