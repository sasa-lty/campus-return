package app

import (
	"bytes"
	"campusreturn/internal/domain"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Server struct {
	S      *Store
	Chain  Chain
	Dir    string
	Mu     sync.Mutex
	Router *gin.Engine
	limits map[string]*bucket
}
type bucket struct {
	At time.Time
	N  int
}
type apiError struct {
	Code int
	Text string
}

func problem(code int, text string) { panic(apiError{code, text}) }
func require(ok bool, text string) {
	if !ok {
		problem(400, text)
	}
}
func must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}
func check(e error) {
	if e != nil {
		panic(e)
	}
}
func body[T any](c *gin.Context) T {
	var v T
	if e := c.ShouldBindJSON(&v); e != nil {
		problem(400, "请求格式不正确")
	}
	return v
}
func actor(c *gin.Context) User {
	v, ok := c.Get("actor")
	if !ok {
		problem(401, "请先登录")
	}
	return v.(User)
}
func admin(c *gin.Context) User {
	u := actor(c)
	if !u.Admin() {
		problem(403, "需要管理员权限")
	}
	return u
}
func super(c *gin.Context) User {
	u := actor(c)
	if u.Role != "SUPER_ADMIN" {
		problem(403, "需要超级管理员权限")
	}
	return u
}
func student(c *gin.Context) User {
	u := actor(c)
	if u.Role != "USER" {
		problem(403, "此操作需要普通用户账号")
	}
	return u
}
func validText(s string, min, max int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n >= min && n <= max
}
func validPassword(s string) bool { return len([]rune(s)) >= 6 && len(s) <= 72 }
func bearer(c *gin.Context) string {
	return strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
}
func New(s *Store, ch Chain, dir, password string) (*Server, error) {
	srv := &Server{S: s, Chain: ch, Dir: dir, limits: map[string]*bucket{}}
	users, e := records[User](s.DB, "user", "")
	if e != nil {
		return nil, e
	}
	if len(users) == 0 {
		if !validPassword(password) {
			return nil, errors.New("首次启动必须设置至少6个字符、最多72字节的 ADMIN_PASSWORD")
		}
		h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if e != nil {
			return nil, e
		}
		u := User{ID: uid(), Username: "admin", DisplayName: "平台管理员", Role: "SUPER_ADMIN", Status: "ACTIVE", PasswordHash: string(h), CreatedAt: now()}
		if e = put(s.DB, "user", u.ID, u); e != nil {
			return nil, e
		}
	}
	gin.SetMode(gin.ReleaseMode)
	if e = srv.repairClaimNotices(); e != nil {
		return nil, e
	}
	if e = srv.repairFeedback(); e != nil {
		return nil, e
	}
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	srv.Router = r
	r.Use(func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				switch e := p.(type) {
				case apiError:
					c.AbortWithStatusJSON(e.Code, gin.H{"error": e.Text})
				default:
					log.Printf("request failed: %T", p)
					c.AbortWithStatusJSON(500, gin.H{"error": "服务器暂时无法完成操作"})
				}
			}
		}()
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Cache-Control", "no-store")
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		c.Next()
	})
	api := r.Group("/api/v1")
	api.Use(srv.authenticate, srv.passwordGate, srv.limit, srv.idempotency)
	api.GET("/health", func(c *gin.Context) {
		e := ch.Health()
		code := 200
		if e != nil {
			code = 503
		}
		c.JSON(code, gin.H{"ok": e == nil, "mode": ch.Mode()})
	})
	srv.authRoutes(api)
	srv.itemRoutes(api)
	srv.managementRoutes(api)
	srv.communityRoutes(api)
	r.GET("/uploads/:id", srv.serveUpload)
	front := os.Getenv("FRONTEND_DIR")
	if front == "" {
		front = "frontend/dist"
	}
	r.Static("/assets", filepath.Join(front, "assets"))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/uploads/") || strings.Contains(filepath.Base(c.Request.URL.Path), ".") {
			c.JSON(404, gin.H{"error": "未找到资源"})
			return
		}
		// 入口文档始终重新读取；带哈希的历史资源由构建保留，兼容尚未关闭的旧页面。
		c.Header("Cache-Control", "no-store")
		if _, e := os.Stat(filepath.Join(front, "index.html")); e != nil {
			c.Data(200, "text/html; charset=utf-8", []byte("<h1>校园失物招领平台</h1><p>请构建 frontend 后刷新页面。</p>"))
			return
		}
		c.File(filepath.Join(front, "index.html"))
	})
	return srv, nil
}
func (s *Server) authenticate(c *gin.Context) {
	t := bearer(c)
	if t == "" {
		return
	}
	sess, e := get[Session](s.S.DB, "session", domain.Hash(t))
	if e != nil {
		return
	}
	exp, e := time.Parse(time.RFC3339Nano, sess.ExpiresAt)
	if e != nil || time.Now().After(exp) {
		return
	}
	u, e := get[User](s.S.DB, "user", sess.UserID)
	if e == nil && u.Status == "ACTIVE" {
		c.Set("actor", u)
	}
}

func (s *Server) passwordGate(c *gin.Context) {
	if v, ok := c.Get("actor"); ok && v.(User).ForcePassword {
		p := c.Request.URL.Path
		if p != "/api/v1/me/password" && p != "/api/v1/auth/logout" && !(p == "/api/v1/me" && c.Request.Method == "GET") {
			problem(403, "请先修改临时密码")
		}
	}
}
func (s *Server) limit(c *gin.Context) {
	if c.Request.Method == "GET" {
		return
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	key := c.ClientIP() + ":" + c.FullPath()
	if v, ok := c.Get("actor"); ok {
		key += ":" + v.(User).ID
	}
	now := time.Now()
	b := s.limits[key]
	if b == nil || now.Sub(b.At) > time.Minute {
		b = &bucket{At: now}
		s.limits[key] = b
	}
	b.N++
	max := 90
	if strings.HasPrefix(c.Request.URL.Path, "/api/v1/auth/") {
		max = 120
	}
	if c.FullPath() == "/api/v1/uploads" {
		max = 15
	}
	if b.N > max {
		problem(429, "操作过于频繁，请稍后重试")
	}
	if len(s.limits) > 5000 {
		for k, v := range s.limits {
			if now.Sub(v.At) > 2*time.Minute {
				delete(s.limits, k)
			}
		}
	}
}
func (s *Server) idempotency(c *gin.Context) {
	if c.Request.Method == "GET" || !strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		return
	}
	b, e := io.ReadAll(io.LimitReader(c.Request.Body, 2*1024*1024+1))
	check(e)
	require(len(b) <= 2*1024*1024, "请求过大")
	c.Request.Body = io.NopCloser(bytes.NewReader(b))
	hash := domain.Hash(c.Request.Method + ":" + c.Request.URL.Path + ":" + string(b))
	c.Set("requestHash", hash)
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		return
	}
	v, ok := c.Get("actor")
	if !ok {
		return
	}
	u := v.(User)
	var raw, oldhash string
	e = s.S.DB.QueryRow(`SELECT data,request_hash FROM operations WHERE user_id=? AND idem=?`, u.ID, key).Scan(&raw, &oldhash)
	if e == nil {
		if oldhash != hash {
			problem(409, "幂等键已用于其他请求")
		}
		var op Operation
		check(json.Unmarshal([]byte(raw), &op))
		c.AbortWithStatusJSON(202, op.Public())
	}
}
func audit(q querier, u User, action, target, reason string) error {
	id := uid()
	return put(q, "audit", id, Audit{ID: id, ActorID: u.ID, Action: action, TargetID: target, Reason: reason, At: now()})
}
func notice(q querier, id, user, item, title string) error {
	if user == "" {
		return nil
	}
	if _, e := get[Notice](q, "notice", id); e == nil {
		return nil
	}
	return put(q, "notice", id, Notice{ID: id, UserID: user, ItemID: item, Title: title, At: now()})
}
func revokeSessions(q querier, userID string) error {
	_, e := q.Exec(`DELETE FROM records WHERE kind='session' AND json_extract(data,'$.userId')=?`, userID)
	return e
}
func (s *Server) authRoutes(a *gin.RouterGroup) {
	a.POST("/auth/register", func(c *gin.Context) {
		q := body[struct{ Username, Password, DisplayName string }](c)
		q.Username = strings.ToLower(strings.TrimSpace(q.Username))
		require(regexp.MustCompile(`^[a-z0-9_]{3,32}$`).MatchString(q.Username), "用户名需为3至32位字母、数字或下划线")
		require(validPassword(q.Password), "密码至少6个字符，最多72字节")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		users := must(records[User](s.S.DB, "user", `AND json_extract(data,'$.username')=?`, q.Username))
		require(len(users) == 0, "用户名已存在")
		h := must(bcrypt.GenerateFromPassword([]byte(q.Password), bcrypt.DefaultCost))
		recovery := uid()
		u := User{ID: uid(), Username: q.Username, DisplayName: q.DisplayName, PasswordHash: string(h), RecoveryHash: domain.Hash(recovery), Role: "USER", Status: "ACTIVE", CreatedAt: now()}
		if !validText(u.DisplayName, 1, 30) {
			u.DisplayName = u.Username
		}
		check(put(s.S.DB, "user", u.ID, u))
		c.JSON(201, gin.H{"user": u.Public(), "recoveryCode": recovery})
	})
	a.POST("/auth/login", func(c *gin.Context) {
		s.Mu.Lock()
		defer s.Mu.Unlock()
		q := body[struct{ Username, Password string }](c)
		users := must(records[User](s.S.DB, "user", `AND json_extract(data,'$.username')=?`, strings.ToLower(strings.TrimSpace(q.Username))))
		if len(users) != 1 || users[0].Status != "ACTIVE" || bcrypt.CompareHashAndPassword([]byte(users[0].PasswordHash), []byte(q.Password)) != nil {
			problem(401, "用户名、密码错误或账号不可用")
		}
		token := uid() + uid()
		check(put(s.S.DB, "session", domain.Hash(token), Session{UserID: users[0].ID, ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)}))
		c.JSON(200, gin.H{"token": token, "user": users[0].Public(), "expiresIn": 86400})
	})
	a.POST("/auth/recover", func(c *gin.Context) {
		q := body[struct{ Username, RecoveryCode, Password string }](c)
		require(validPassword(q.Password), "密码至少6个字符，最多72字节")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		users := must(records[User](s.S.DB, "user", `AND json_extract(data,'$.username')=?`, strings.ToLower(strings.TrimSpace(q.Username))))
		if len(users) != 1 || users[0].Status != "ACTIVE" || users[0].RecoveryHash == "" || users[0].RecoveryHash != domain.Hash(q.RecoveryCode) {
			problem(403, "恢复信息不正确")
		}
		u := users[0]
		h := must(bcrypt.GenerateFromPassword([]byte(q.Password), bcrypt.DefaultCost))
		u.PasswordHash = string(h)
		next := uid()
		u.RecoveryHash = domain.Hash(next)
		u.ForcePassword = false
		check(s.S.Transaction(func(tx *sql.Tx) error {
			if e := put(tx, "user", u.ID, u); e != nil {
				return e
			}
			return revokeSessions(tx, u.ID)
		}))
		c.JSON(200, gin.H{"recoveryCode": next})
	})
	a.GET("/me", func(c *gin.Context) { c.JSON(200, actor(c).Public()) })
	a.POST("/auth/logout", func(c *gin.Context) {
		actor(c)
		_, e := s.S.DB.Exec(`DELETE FROM records WHERE kind='session' AND id=?`, domain.Hash(bearer(c)))
		check(e)
		c.JSON(200, gin.H{"ok": true})
	})
	a.PATCH("/me", func(c *gin.Context) {
		u := actor(c)
		q := body[struct{ DisplayName string }](c)
		require(validText(q.DisplayName, 1, 30), "显示名称需为1至30字")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		u = must(get[User](s.S.DB, "user", u.ID))
		u.DisplayName = strings.TrimSpace(q.DisplayName)
		check(put(s.S.DB, "user", u.ID, u))
		c.JSON(200, u.Public())
	})
	a.POST("/me/password", func(c *gin.Context) {
		u := actor(c)
		q := body[struct{ OldPassword, Password string }](c)
		require(validPassword(q.Password), "密码至少6个字符，最多72字节")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		u = must(get[User](s.S.DB, "user", u.ID))
		require(bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(q.OldPassword)) == nil, "原密码不正确")
		u.PasswordHash = string(must(bcrypt.GenerateFromPassword([]byte(q.Password), bcrypt.DefaultCost)))
		u.ForcePassword = false
		check(s.S.Transaction(func(tx *sql.Tx) error {
			if e := put(tx, "user", u.ID, u); e != nil {
				return e
			}
			return revokeSessions(tx, u.ID)
		}))
		c.JSON(200, gin.H{"ok": true, "message": "密码已修改，请重新登录"})
	})
}
func jsonValue(v any) string { b, _ := json.Marshal(v); return string(b) }
func findItem(q querier, id string) domain.Item {
	v, e := get[domain.Item](q, "item", id)
	if errors.Is(e, sql.ErrNoRows) {
		problem(404, "物品不存在")
	}
	check(e)
	return v
}
func fmtID(prefix, id string) string { return fmt.Sprintf("%s:%s", prefix, id) }
