package app

import (
	"campusreturn/internal/domain"
	"database/sql"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

func (s *Server) managementRoutes(a *gin.RouterGroup) {
	a.GET("/admin/users", func(c *gin.Context) {
		super(c)
		users := must(records[User](s.S.DB, "user", `ORDER BY created_at DESC`))
		out := []User{}
		for _, u := range users {
			if q := strings.ToLower(c.Query("q")); q != "" && !strings.Contains(strings.ToLower(u.Username+" "+u.DisplayName), q) {
				continue
			}
			out = append(out, u.Public())
		}
		paginate(c, out)
	})
	a.PATCH("/admin/users/:id", func(c *gin.Context) {
		op := super(c)
		q := body[struct{ Role, Status, Reason string }](c)
		require(validText(q.Reason, 2, 300), "请填写管理原因")
		require(q.Role == "" || q.Role == "USER" || q.Role == "ADMIN" || q.Role == "SUPER_ADMIN", "角色无效")
		require(q.Status == "" || q.Status == "ACTIVE" || q.Status == "DISABLED", "账号状态无效")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		u, e := get[User](s.S.DB, "user", c.Param("id"))
		require(e == nil, "用户不存在")
		before := u
		if q.Role != "" {
			u.Role = q.Role
		}
		if q.Status != "" {
			u.Status = q.Status
		}
		if u.ID == op.ID && (u.Role != before.Role || u.Status != "ACTIVE") {
			problem(400, "不能修改自己的管理角色或禁用自己")
		}
		users := must(records[User](s.S.DB, "user", ""))
		n := 0
		for _, x := range users {
			if x.ID == u.ID {
				x = u
			}
			if x.Role == "SUPER_ADMIN" && x.Status == "ACTIVE" {
				n++
			}
		}
		require(n > 0, "必须保留至少一位有效超级管理员")
		check(s.S.Transaction(func(tx *sql.Tx) error {
			if e := put(tx, "user", u.ID, u); e != nil {
				return e
			}
			if e := revokeSessions(tx, u.ID); e != nil {
				return e
			}
			return audit(tx, op, "USER_UPDATE", u.ID, q.Reason)
		}))
		c.JSON(200, u.Public())
	})
	a.POST("/admin/users/:id/reset", func(c *gin.Context) {
		op := super(c)
		q := body[struct{ Reason string }](c)
		require(validText(q.Reason, 2, 300), "请输入重置原因")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		u := must(get[User](s.S.DB, "user", c.Param("id")))
		require(u.ID != op.ID, "请在个人中心修改自己的密码")
		temporary := "Tmp-" + uid()
		u.PasswordHash = string(must(bcrypt.GenerateFromPassword([]byte(temporary), bcrypt.DefaultCost)))
		u.RecoveryHash = ""
		u.ForcePassword = true
		check(s.S.Transaction(func(tx *sql.Tx) error {
			if e := put(tx, "user", u.ID, u); e != nil {
				return e
			}
			if e := revokeSessions(tx, u.ID); e != nil {
				return e
			}
			return audit(tx, op, "RESET_PASSWORD", u.ID, q.Reason)
		}))
		c.JSON(200, gin.H{"temporaryPassword": temporary, "message": "临时密码仅显示一次，用户登录后必须修改"})
	})
	a.GET("/admin/items", func(c *gin.Context) {
		admin(c)
		items := must(records[domain.Item](s.S.DB, "item", `ORDER BY updated_at DESC`))
		out := []ItemView{}
		for _, it := range items {
			if x := c.Query("status"); x != "" && it.Status != x {
				continue
			}
			if x := c.Query("visibility"); x != "" && it.Visibility != x {
				continue
			}
			v := s.view(it)
			if q := c.Query("q"); q != "" && !strings.Contains(v.Content.Name+v.Content.Location+it.ID, q) {
				continue
			}
			out = append(out, v)
		}
		paginate(c, out)
	})
	a.GET("/admin/items/:id/private", func(c *gin.Context) {
		u := admin(c)
		it := findItem(s.S.DB, c.Param("id"))
		secret, e := s.secret("item:"+it.ID, it.AnswerHash)
		claims := []gin.H{}
		for _, cl := range it.Claims {
			v, err := s.secret("claim:"+cl.ID, cl.EvidenceHash)
			claimant, _ := get[User](s.S.DB, "user", cl.UserID)
			claims = append(claims, gin.H{"claim": cl, "claimant": claimant.Public(), "evidence": v.Text, "integrity": err == nil, "feedback": claimFeedbackFor(s.S.DB, it.ID, cl.UserID, cl.ID)})
		}
		check(audit(s.S.DB, u, "READ_PRIVATE", it.ID, "管理员核验访问"))
		c.JSON(200, gin.H{"answer": secret.Text, "answerIntegrity": e == nil, "claims": claims})
	})
	a.PATCH("/admin/items/:id", func(c *gin.Context) {
		u := admin(c)
		q := body[itemInput](c)
		it := findItem(s.S.DB, c.Param("id"))
		expected(it, q.ExpectedVersion)
		require(validText(q.Reason, 2, 300), "编辑必须填写原因")
		s.content(&q, u, it.ReporterID)
		secret := must(s.secret("item:"+it.ID, it.AnswerHash))
		if q.Answer != "" && strings.TrimSpace(q.Answer) != secret.Text {
			require(validText(q.Answer, 2, 500), "答案长度不正确")
			secret = Secret{Salt: uid(), Text: strings.TrimSpace(q.Answer)}
		}
		cmd := command(u, it, "UPDATE")
		cmd.Data = domain.Payload{ContentHash: domain.Digest(q.Content), AnswerHash: domain.Hash(secret.Salt + ":" + secret.Text), QuestionHash: domain.Hash(q.Content.Question), ImageHash: q.Content.ImageHash, ReasonHash: domain.Hash(q.Reason)}
		s.queue(c, u, cmd, &q.Content, s.S.Seal(secret), q.Reason, "")
	})
	a.POST("/admin/items/:id/actions", s.adminAction)
	a.GET("/admin/audit", func(c *gin.Context) {
		admin(c)
		v := must(records[Audit](s.S.DB, "audit", `ORDER BY created_at DESC`))
		out := []Audit{}
		for _, x := range v {
			if q := c.Query("q"); q != "" && !strings.Contains(x.Action+" "+x.ActorID+" "+x.TargetID+" "+x.Reason, q) {
				continue
			}
			out = append(out, x)
		}
		paginate(c, out)
	})
	a.GET("/admin/operations", func(c *gin.Context) {
		admin(c)
		rows := must(s.S.DB.Query(`SELECT data FROM operations ORDER BY rowid DESC`))
		out := []Operation{}
		for rows.Next() {
			var raw string
			check(rows.Scan(&raw))
			var o Operation
			check(json.Unmarshal([]byte(raw), &o))
			if st := c.Query("status"); st != "" {
				if st == "ATTENTION" {
					if o.Status != "PENDING" && o.Status != "SUBMITTING" && o.Status != "FAILED" {
						continue
					}
				} else if st != o.Status {
					continue
				}
			}
			out = append(out, o.Public())
		}
		rows.Close()
		paginate(c, out)
	})
	a.GET("/stats", s.stats)
	a.GET("/admin/stats", func(c *gin.Context) { admin(c); s.stats(c) })
}
func (s *Server) adminAction(c *gin.Context) {
	u := admin(c)
	q := body[actionInput](c)
	require(validText(q.Note, 2, 500), "管理操作必须填写原因")
	allowed := map[string]bool{"APPROVE": true, "REJECT": true, "RETURN": true, "HIDE": true, "RESTORE": true, "CLOSE": true, "REOPEN": true, "REVOKE": true, "FLAG_FRAUD": true}
	require(allowed[q.Action], "无效管理操作")
	it := findItem(s.S.DB, c.Param("id"))
	expected(it, q.ExpectedVersion)
	if q.Action == "APPROVE" {
		_, e := s.secret("item:"+it.ID, it.AnswerHash)
		require(e == nil, "参考答案缺失或摘要不一致")
		valid := false
		for _, cl := range it.Claims {
			if cl.ID == q.ClaimID && cl.Status == "PENDING" {
				_, e = s.secret("claim:"+cl.ID, cl.EvidenceHash)
				valid = e == nil
			}
		}
		require(valid, "当前认领材料缺失或摘要不一致")
	}
	cmd := command(u, it, q.Action)
	cmd.Data = domain.Payload{ClaimID: q.ClaimID, ReasonHash: domain.Hash(q.Note)}
	if q.Action == "RETURN" {
		s.Mu.Lock()
		h, e := get[Handover](s.S.DB, "handover", it.ID)
		if e != nil {
			s.Mu.Unlock()
			problem(400, "请物主先生成交接码")
		}
		expiry, err := time.Parse(time.RFC3339Nano, h.ExpiresAt)
		if err != nil || h.Used || h.Attempts >= 5 || time.Now().After(expiry) || h.ClaimID != it.ClaimID {
			s.Mu.Unlock()
			problem(400, "交接码已失效，请物主重新生成")
		}
		h.Attempts++
		check(put(s.S.DB, "handover", it.ID, h))
		s.Mu.Unlock()
		require(h.Hash == domain.Hash(it.ID+":"+q.Code), "交接码不正确")
		require(q.ClaimID == it.ClaimID, "交接认领编号不匹配")
		cmd.Data.ReceiptHash = domain.Hash(it.ID + ":" + it.ClaimID + ":" + h.Hash + ":" + q.Note)
	}
	if q.ReportID != "" {
		r, e := get[Report](s.S.DB, "report", q.ReportID)
		require(e == nil && r.ItemID == it.ID && r.Status != "RESOLVED", "争议或举报不匹配")
	}
	s.queue(c, u, cmd, nil, "", q.Note, q.ReportID)
}
func (s *Server) stats(c *gin.Context) {
	items := must(records[domain.Item](s.S.DB, "item", ""))
	counts := map[string]int{"total": 0, "FOUND": 0, "CLAIM_PENDING": 0, "CLAIMED": 0, "RETURNED": 0, "CLOSED": 0, "hidden": 0}
	categories := map[string]int{}
	daily := map[string]int{}
	for _, it := range items {
		if !optionalUser(c).Admin() && it.Visibility != "PUBLIC" {
			continue
		}
		counts["total"]++
		counts[it.Status]++
		if it.Visibility == "HIDDEN" {
			counts["hidden"]++
		}
		v := s.view(it)
		categories[v.Content.Category]++
		day := it.CreatedAt
		if len(day) > 10 {
			day = day[:10]
		}
		daily[day]++
	}
	extra := gin.H{}
	if optionalUser(c).Admin() {
		for _, kind := range []string{"user", "report", "evaluation"} {
			values := must(records[map[string]any](s.S.DB, kind, ""))
			extra[kind+"Count"] = len(values)
		}
		reports := must(records[Report](s.S.DB, "report", `AND json_extract(data,'$.status')!='RESOLVED'`))
		extra["openReports"] = len(reports)
		var n int
		check(s.S.DB.QueryRow(`SELECT count(*) FROM operations WHERE json_extract(data,'$.status') IN ('PENDING','SUBMITTING','FAILED')`).Scan(&n))
		extra["abnormalOperations"] = n
	}
	c.JSON(200, gin.H{"counts": counts, "categories": categories, "daily": daily, "management": extra, "mode": s.Chain.Mode()})
}
