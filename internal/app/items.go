package app

import (
	"campusreturn/internal/domain"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type itemInput struct {
	Content         domain.Content `json:"content"`
	Answer          string         `json:"answer"`
	Reason          string         `json:"reason"`
	ExpectedVersion uint64         `json:"expectedVersion"`
}
type actionInput struct {
	Action          string `json:"action"`
	ClaimID         string `json:"claimId"`
	Note            string `json:"note"`
	Code            string `json:"code"`
	ReportID        string `json:"reportId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

func (s *Server) content(q *itemInput, u User, owner string) {
	v := &q.Content
	v.Name = strings.TrimSpace(v.Name)
	v.Description = strings.TrimSpace(v.Description)
	v.Location = strings.TrimSpace(v.Location)
	v.Question = strings.TrimSpace(v.Question)
	q.Answer = strings.TrimSpace(q.Answer)
	require(validText(v.Name, 2, 80) && validText(v.Category, 1, 30) && validText(v.Description, 5, 2000) && validText(v.Location, 2, 100) && validText(v.Question, 2, 200), "请完整填写名称、分类、描述、地点和核验问题")
	t, e := time.Parse(time.RFC3339, v.FoundAt)
	require(e == nil, "拾到时间格式不正确")
	require(t.Before(time.Now().Add(5*time.Minute)), "拾到时间不能在未来")
	v.FoundAt = t.UTC().Format(time.RFC3339)
	if v.UploadID != "" {
		up, e := get[Upload](s.S.DB, "upload", v.UploadID)
		require(e == nil && (up.UserID == owner || (u.Admin() && up.UserID == u.ID)), "图片必须由登记人或当前编辑管理员上传")
		b, e := os.ReadFile(filepath.Join(s.Dir, "uploads", up.Path))
		require(e == nil && domain.Hash(string(b)) == up.Hash, "图片文件缺失或摘要不一致")
		v.ImageHash = up.Hash
	} else {
		v.ImageHash = ""
	}
}
func visible(it domain.Item, u User) bool {
	return it.Visibility == "PUBLIC" || u.Admin() || (u.ID != "" && (it.ReporterID == u.ID || it.OwnerID == u.ID))
}
func optionalUser(c *gin.Context) User {
	v, ok := c.Get("actor")
	if ok {
		return v.(User)
	}
	return User{}
}
func expected(it domain.Item, v uint64) {
	if v == 0 || it.Version != v {
		problem(409, "记录已变化，请刷新后再操作")
	}
}
func command(u User, it domain.Item, a string) domain.Command {
	return domain.Command{ID: uid(), Action: a, ItemID: it.ID, ActorID: u.ID, ExpectedVersion: it.Version}
}
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "12"))
	if p < 1 {
		p = 1
	}
	if n < 1 || n > 100 {
		n = 12
	}
	return p, n
}
func paginate[T any](c *gin.Context, values []T) {
	p, n := page(c)
	start := (p - 1) * n
	end := start + n
	if start > len(values) {
		start = len(values)
	}
	if end > len(values) {
		end = len(values)
	}
	c.JSON(200, gin.H{"items": values[start:end], "total": len(values), "page": p, "pageSize": n})
}
func (s *Server) itemRoutes(a *gin.RouterGroup) {
	a.GET("/items", s.listItems)
	a.GET("/items/:id", func(c *gin.Context) {
		it := findItem(s.S.DB, c.Param("id"))
		if !visible(it, optionalUser(c)) {
			problem(404, "物品不存在或已下架")
		}
		v := s.view(it)
		if u := optionalUser(c); u.ID != "" {
			v.MyClaimFeedback = claimFeedback(s.S.DB, it.ID, u.ID)
		}
		c.JSON(200, v)
	})
	a.GET("/items/:id/history", func(c *gin.Context) {
		it := findItem(s.S.DB, c.Param("id"))
		if !visible(it, optionalUser(c)) {
			problem(404, "物品不存在")
		}
		h := must(records[domain.Item](s.S.DB, "history", `AND json_extract(data,'$.id')=? ORDER BY json_extract(data,'$.version') DESC`, it.ID))
		out := []gin.H{}
		for _, v := range h {
			view := s.view(v)
			entry := gin.H{"state": v, "integrity": view.Integrity}
			// 已纠正内容的旧原文只供管理审计，公开追溯保留摘要和状态。
			if optionalUser(c).Admin() {
				entry["content"] = view.Content
			}
			out = append(out, entry)
		}
		c.JSON(200, out)
	})
	a.POST("/items", func(c *gin.Context) {
		u := student(c)
		q := body[itemInput](c)
		s.content(&q, u, u.ID)
		require(validText(q.Answer, 2, 500), "请设置2至500字参考答案")
		secret := Secret{Salt: uid(), Text: q.Answer}
		cmd := domain.Command{ID: uid(), Action: "CREATE", ActorID: u.ID, ItemID: "ITEM-" + uid()}
		cmd.Data = domain.Payload{ContentHash: domain.Digest(q.Content), AnswerHash: domain.Hash(secret.Salt + ":" + secret.Text), QuestionHash: domain.Hash(q.Content.Question), ImageHash: q.Content.ImageHash}
		s.queue(c, u, cmd, &q.Content, s.S.Seal(secret), "", "")
	})
	a.POST("/items/:id/claims", func(c *gin.Context) {
		u := student(c)
		q := body[struct {
			Evidence        string `json:"evidence"`
			ExpectedVersion uint64 `json:"expectedVersion"`
		}](c)
		require(validText(q.Evidence, 2, 2000), "认领证明需为2至2000字")
		it := findItem(s.S.DB, c.Param("id"))
		expected(it, q.ExpectedVersion)
		secret := Secret{Salt: uid(), Text: strings.TrimSpace(q.Evidence)}
		cmd := command(u, it, "CLAIM")
		cmd.Data.ClaimID = "CL-" + uid()
		cmd.Data.EvidenceHash = domain.Hash(secret.Salt + ":" + secret.Text)
		s.queue(c, u, cmd, nil, s.S.Seal(secret), "", "")
	})
	a.POST("/items/:id/claims/:claim/withdraw", func(c *gin.Context) {
		u := student(c)
		q := body[actionInput](c)
		it := findItem(s.S.DB, c.Param("id"))
		expected(it, q.ExpectedVersion)
		cmd := command(u, it, "WITHDRAW")
		cmd.Data.ClaimID = c.Param("claim")
		s.queue(c, u, cmd, nil, "", "本人撤回认领", "")
	})
	a.GET("/me/items", func(c *gin.Context) {
		u := actor(c)
		v := must(records[domain.Item](s.S.DB, "item", `AND json_extract(data,'$.reporterId')=? ORDER BY updated_at DESC`, u.ID))
		out := []ItemView{}
		for _, it := range v {
			out = append(out, s.view(it))
		}
		paginate(c, out)
	})
	a.GET("/me/claims", func(c *gin.Context) {
		u := actor(c)
		items := must(records[domain.Item](s.S.DB, "item", `ORDER BY updated_at DESC`))
		out := []gin.H{}
		for _, it := range items {
			if id := c.Query("itemId"); id != "" && id != it.ID {
				continue
			}
			for _, cl := range it.Claims {
				if cl.UserID == u.ID {
					sec, e := s.secret("claim:"+cl.ID, cl.EvidenceHash)
					out = append(out, gin.H{"item": s.view(it), "claim": cl, "myEvidence": sec.Text, "integrity": e == nil, "feedback": claimFeedbackFor(s.S.DB, it.ID, u.ID, cl.ID)})
				}
			}
		}
		s.listMyClaims(c, out)
	})
	a.GET("/operations/:id", func(c *gin.Context) {
		u := actor(c)
		o, e := s.operation(c.Param("id"))
		if e != nil {
			problem(404, "操作不存在")
		}
		if o.UserID != u.ID && !u.Admin() {
			problem(403, "无权查看此操作")
		}
		c.JSON(200, o.Public())
	})
	a.GET("/me/operations", func(c *gin.Context) {
		u := actor(c)
		rows := must(s.S.DB.Query(`SELECT data FROM operations WHERE user_id=? ORDER BY rowid DESC`, u.ID))
		defer rows.Close()
		out := []Operation{}
		for rows.Next() {
			var raw string
			check(rows.Scan(&raw))
			var o Operation
			check(json.Unmarshal([]byte(raw), &o))
			out = append(out, o.Public())
		}
		paginate(c, out)
	})
	a.POST("/uploads", s.upload)
	a.POST("/items/:id/handover-code", s.issueCode)
	a.POST("/items/:id/appointment", func(c *gin.Context) {
		u := actor(c)
		it := findItem(s.S.DB, c.Param("id"))
		require(it.Status == "CLAIMED" && (u.ID == it.OwnerID || u.Admin()), "只有已确权物主或管理员能安排交接")
		q := body[Appointment](c)
		require(validText(q.Location, 2, 100), "请填写交接地点")
		t, e := time.Parse(time.RFC3339, q.At)
		require(e == nil && t.After(time.Now()), "交接时间应晚于当前时间")
		q.ItemID = it.ID
		q.UserID = u.ID
		q.At = t.UTC().Format(time.RFC3339)
		q.Status = "PROPOSED"
		if u.Admin() {
			q.Status = "CONFIRMED"
		}
		s.Mu.Lock()
		defer s.Mu.Unlock()
		check(put(s.S.DB, "appointment", it.ID, q))
		title := "交接预约已提出"
		if q.Status == "CONFIRMED" {
			title = "交接预约已确认"
		}
		n := Notice{ID: "appointment:" + uid(), UserID: it.OwnerID, ItemID: it.ID, Title: title, Body: q.Location + " · " + q.At, Action: "APPOINTMENT", Result: q.Status, At: now(), Read: u.ID == it.OwnerID}
		check(put(s.S.DB, "notice", n.ID, n))
		if q.Status == "CONFIRMED" && it.ReporterID != it.OwnerID {
			n.ID = "appointment:" + uid()
			n.UserID = it.ReporterID
			n.Read = false
			check(put(s.S.DB, "notice", n.ID, n))
		}
		c.JSON(200, q)
	})
	a.GET("/items/:id/appointment", func(c *gin.Context) {
		u := actor(c)
		it := findItem(s.S.DB, c.Param("id"))
		require(u.Admin() || u.ID == it.OwnerID || u.ID == it.ReporterID, "无权查看预约")
		q, _ := get[Appointment](s.S.DB, "appointment", it.ID)
		c.JSON(200, q)
	})
}
func claimFeedbackFor(q querier, item, user, claim string) []ClaimFeedback {
	out := []ClaimFeedback{}
	for _, f := range claimFeedback(q, item, user) {
		if f.ClaimID == claim {
			out = append(out, f)
		}
	}
	return out
}
func (s *Server) listMyClaims(c *gin.Context, out []gin.H) {
	latest := func(v gin.H) string {
		fs := v["feedback"].([]ClaimFeedback)
		if len(fs) > 0 {
			return fs[0].At
		}
		return v["claim"].(domain.Claim).CreatedAt
	}
	sort.SliceStable(out, func(i, j int) bool { return latest(out[i]) > latest(out[j]) })
	filtered := []gin.H{}
	for _, v := range out {
		st := v["claim"].(domain.Claim).Status
		active := st == "PENDING" || st == "APPROVED"
		if c.Query("scope") == "active" && !active {
			continue
		}
		if c.Query("scope") == "ended" && active {
			continue
		}
		filtered = append(filtered, v)
	}
	if c.Query("grouped") != "true" {
		paginate(c, filtered)
		return
	}
	groups := []gin.H{}
	positions := map[string]int{}
	for _, v := range filtered {
		it := v["item"].(ItemView)
		i, ok := positions[it.ID]
		if !ok {
			i = len(groups)
			positions[it.ID] = i
			groups = append(groups, gin.H{"item": it, "latest": v, "attempts": []gin.H{}, "updatedAt": latest(v)})
		}
		g := groups[i]
		g["attempts"] = append(g["attempts"].([]gin.H), v)
	}
	paginate(c, groups)
}
func (s *Server) listItems(c *gin.Context) {
	base := ` FROM records r JOIN records cv ON cv.kind='content' AND cv.id=json_extract(r.data,'$.id')||':'||json_extract(r.data,'$.contentHash') WHERE r.kind='item' AND json_extract(r.data,'$.visibility')='PUBLIC'`
	args := []any{}
	for _, field := range []string{"status", "category"} {
		if v := c.Query(field); v != "" {
			expr := `json_extract(r.data,'$.status')`
			if field == "category" {
				expr = `json_extract(cv.data,'$.content.category')`
			}
			base += " AND " + expr + "=?"
			args = append(args, v)
		}
	}
	if v := strings.TrimSpace(c.Query("q")); v != "" {
		base += ` AND instr(lower(json_extract(cv.data,'$.content.name')||' '||json_extract(cv.data,'$.content.description')||' '||json_extract(cv.data,'$.content.location')),lower(?))>0`
		args = append(args, v)
	}
	if v := c.Query("location"); v != "" {
		base += ` AND instr(json_extract(cv.data,'$.content.location'),?)>0`
		args = append(args, v)
	}
	zone := time.FixedZone("Asia/Shanghai", 8*3600)
	for _, field := range []string{"from", "to"} {
		if v := c.Query(field); v != "" {
			d, e := time.ParseInLocation("2006-01-02", v, zone)
			require(e == nil, "日期格式不正确")
			operator := ">="
			if field == "to" {
				d = d.AddDate(0, 0, 1)
				operator = "<"
			}
			base += ` AND json_extract(cv.data,'$.content.foundAt')` + operator + `?`
			args = append(args, d.UTC().Format(time.RFC3339))
		}
	}
	order := `r.updated_at DESC`
	switch c.Query("sort") {
	case "oldest":
		order = `r.updated_at ASC`
	case "name":
		order = `json_extract(cv.data,'$.content.name') COLLATE NOCASE ASC`
	}
	var total int
	check(s.S.DB.QueryRow("SELECT count(*)"+base, args...).Scan(&total))
	p, n := page(c)
	queryArgs := append(append([]any{}, args...), n, (p-1)*n)
	rows := must(s.S.DB.Query("SELECT r.data"+base+" ORDER BY "+order+" LIMIT ? OFFSET ?", queryArgs...))
	items := []domain.Item{}
	for rows.Next() {
		var raw string
		check(rows.Scan(&raw))
		var it domain.Item
		check(json.Unmarshal([]byte(raw), &it))
		items = append(items, it)
	}
	check(rows.Err())
	rows.Close()
	out := []ItemView{}
	for _, it := range items {
		out = append(out, s.view(it))
	}
	c.JSON(200, gin.H{"items": out, "total": total, "page": p, "pageSize": n})
}
func (s *Server) upload(c *gin.Context) {
	u := actor(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6*1024*1024)
	f, header, e := c.Request.FormFile("file")
	require(e == nil, "请选择图片")
	defer f.Close()
	require(header.Size <= 5*1024*1024, "图片不能大于5MB")
	b := must(io.ReadAll(io.LimitReader(f, 5*1024*1024+1)))
	require(len(b) <= 5*1024*1024, "图片过大")
	mime := http.DetectContentType(b)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mime]
	require(ext != "", "仅支持 JPEG、PNG、WebP 图片")
	id := uid()
	check(os.MkdirAll(filepath.Join(s.Dir, "uploads"), 0700))
	check(os.WriteFile(filepath.Join(s.Dir, "uploads", id+ext), b, 0600))
	up := Upload{ID: id, UserID: u.ID, Hash: domain.Hash(string(b)), MIME: mime, Size: int64(len(b)), Path: id + ext}
	check(put(s.S.DB, "upload", id, up))
	up.Path = ""
	c.JSON(201, up)
}
func (s *Server) serveUpload(c *gin.Context) {
	s.authenticate(c)
	up, e := get[Upload](s.S.DB, "upload", c.Param("id"))
	if e != nil {
		problem(404, "图片不存在")
	}
	items := must(records[domain.Item](s.S.DB, "item", `AND json_extract(data,'$.visibility')='PUBLIC'`))
	allowed := false
	for _, it := range items {
		cv, e := get[ContentVersion](s.S.DB, "content", it.ID+":"+it.ContentHash)
		if e == nil && cv.Content.UploadID == up.ID {
			allowed = true
			break
		}
	}
	u := optionalUser(c)
	if !allowed && u.ID != up.UserID && !u.Admin() {
		problem(404, "图片不存在")
	}
	c.Header("Content-Type", up.MIME)
	c.File(filepath.Join(s.Dir, "uploads", up.Path))
}
func (s *Server) issueCode(c *gin.Context) {
	u := actor(c)
	it := findItem(s.S.DB, c.Param("id"))
	require(it.OwnerID == u.ID && it.Status == "CLAIMED", "仅已确权物主可以生成交接码")
	s.Mu.Lock()
	defer s.Mu.Unlock()
	n := must(rand.Int(rand.Reader, big.NewInt(1000000)))
	code := fmt.Sprintf("%06d", n.Int64())
	h := Handover{ItemID: it.ID, ClaimID: it.ClaimID, UserID: u.ID, Hash: domain.Hash(it.ID + ":" + code), ExpiresAt: time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339Nano)}
	check(put(s.S.DB, "handover", it.ID, h))
	c.JSON(200, gin.H{"code": code, "expiresAt": h.ExpiresAt, "payload": "campus-return:" + it.ID + ":" + it.ClaimID + ":" + code})
}
