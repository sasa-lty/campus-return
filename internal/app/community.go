package app

import (
	"campusreturn/internal/domain"
	"database/sql"
	"github.com/gin-gonic/gin"
	"sort"
	"strings"
)

func (s *Server) communityRoutes(a *gin.RouterGroup) {
	s.feedbackRoutes(a)
	a.GET("/me/overview", s.personalOverview)
	a.GET("/notifications", func(c *gin.Context) {
		u := actor(c)
		v := must(records[Notice](s.S.DB, "notice", `AND json_extract(data,'$.userId')=? ORDER BY created_at DESC`, u.ID))
		filtered := []Notice{}
		for _, n := range v {
			if !n.Suppressed {
				filtered = append(filtered, n)
			}
		}
		if c.Query("grouped") == "true" {
			paginate(c, groupedNotices(filtered))
		} else {
			paginate(c, filtered)
		}
	})
	a.PATCH("/notifications/:id", func(c *gin.Context) {
		u := actor(c)
		s.Mu.Lock()
		defer s.Mu.Unlock()
		n := must(get[Notice](s.S.DB, "notice", c.Param("id")))
		if n.UserID != u.ID {
			problem(403, "无权查看消息")
		}
		n.Read = true
		check(put(s.S.DB, "notice", n.ID, n))
		c.JSON(200, n)
	})
	a.POST("/notifications/read-all", func(c *gin.Context) {
		u := actor(c)
		s.Mu.Lock()
		defer s.Mu.Unlock()
		v := must(records[Notice](s.S.DB, "notice", `AND json_extract(data,'$.userId')=?`, u.ID))
		check(s.S.Transaction(func(q *sql.Tx) error {
			for _, n := range v {
				n.Read = true
				if e := put(q, "notice", n.ID, n); e != nil {
					return e
				}
			}
			return nil
		}))
		c.JSON(200, gin.H{"ok": true})
	})
	a.POST("/reports", func(c *gin.Context) {
		u := actor(c)
		q := body[Report](c)
		it := findItem(s.S.DB, q.ItemID)
		require(visible(it, u), "无权查看物品")
		require(q.Kind == "REPORT" || q.Kind == "DISPUTE", "请选择举报或交接争议")
		require(validText(q.Reason, 5, 1000), "请填写5至1000字说明")
		if q.Kind == "DISPUTE" {
			require(u.ID == it.ReporterID || u.ID == it.OwnerID, "交接争议仅限相关人员提出")
			require(it.Status == "CLAIMED" || it.Status == "RETURNED", "当前状态不适用交接争议")
			q.ClaimID = it.ClaimID
		}
		s.Mu.Lock()
		defer s.Mu.Unlock()
		prior := must(records[Report](s.S.DB, "report", `AND json_extract(data,'$.userId')=? AND json_extract(data,'$.itemId')=? AND json_extract(data,'$.kind')=? AND json_extract(data,'$.status')!='RESOLVED'`, u.ID, it.ID, q.Kind))
		require(len(prior) == 0, "已有未处理的同类反馈")
		q.ID = uid()
		q.UserID = u.ID
		q.Status = "OPEN"
		q.Resolution = ""
		q.At = now()
		check(put(s.S.DB, "report", q.ID, q))
		c.JSON(201, q)
	})
	a.GET("/me/reports", func(c *gin.Context) {
		u := actor(c)
		v := must(records[Report](s.S.DB, "report", `AND json_extract(data,'$.userId')=? ORDER BY created_at DESC`, u.ID))
		out := []gin.H{}
		for _, r := range v {
			out = append(out, gin.H{"id": r.ID, "itemId": r.ItemID, "kind": r.Kind, "status": r.Status, "reason": r.Reason, "resolution": r.Resolution, "at": r.At, "history": reportHistory(s.S.DB, r.ID)})
		}
		paginate(c, out)
	})
	a.GET("/admin/reports", func(c *gin.Context) {
		admin(c)
		where := `ORDER BY created_at DESC`
		args := []any{}
		if st := c.Query("status"); st != "" {
			where = `AND json_extract(data,'$.status')=? ` + where
			args = append(args, st)
		}
		v := must(records[Report](s.S.DB, "report", where, args...))
		paginate(c, v)
	})
	a.PATCH("/admin/reports/:id", func(c *gin.Context) {
		u := admin(c)
		q := body[struct{ Status, Resolution string }](c)
		require(q.Status == "IN_PROGRESS" || q.Status == "RESOLVED", "请选择受理或结案")
		require(validText(q.Resolution, 2, 1000), "请填写处理意见")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		r := must(get[Report](s.S.DB, "report", c.Param("id")))
		require(r.Status != "RESOLVED", "此反馈已结案")
		r.Status = q.Status
		r.Resolution = q.Resolution
		check(s.S.Transaction(func(tx *sql.Tx) error {
			if e := put(tx, "report", r.ID, r); e != nil {
				return e
			}
			n := Notice{ID: "report:" + uid(), UserID: r.UserID, ItemID: r.ItemID, Title: "举报与争议处理进度已更新", Body: r.Reason, Reason: q.Resolution, RelatedID: r.ID, Action: "REPORT_" + q.Status, Result: q.Status, At: now()}
			if e := put(tx, "notice", n.ID, n); e != nil {
				return e
			}
			return audit(tx, u, "REPORT_"+q.Status, r.ID, q.Resolution)
		}))
		c.JSON(200, r)
	})
	a.POST("/items/:id/evaluations", func(c *gin.Context) {
		u := actor(c)
		it := findItem(s.S.DB, c.Param("id"))
		require(it.Status == "RETURNED" && (u.ID == it.OwnerID || u.ID == it.ReporterID), "仅交接双方可在归还后评价")
		q := body[Evaluation](c)
		require(q.Stars >= 1 && q.Stars <= 5 && validText(q.Text, 2, 300), "请给出1至5星及2至300字评价")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		id := it.ID + ":" + u.ID
		if _, e := get[Evaluation](s.S.DB, "evaluation", id); e == nil {
			problem(409, "已经提交过评价")
		}
		q.ID = id
		q.UserID = u.ID
		q.ItemID = it.ID
		q.Status = "PUBLIC"
		q.At = now()
		check(put(s.S.DB, "evaluation", id, q))
		c.JSON(201, q)
	})
	a.GET("/items/:id/evaluations", func(c *gin.Context) {
		it := findItem(s.S.DB, c.Param("id"))
		require(visible(it, optionalUser(c)), "物品不可见")
		v := must(records[Evaluation](s.S.DB, "evaluation", `AND json_extract(data,'$.itemId')=? AND json_extract(data,'$.status')='PUBLIC' ORDER BY created_at DESC`, it.ID))
		out := s.evaluationViews(v, false)
		for i, e := range v {
			out[i]["mine"] = e.UserID == optionalUser(c).ID
		}
		c.JSON(200, out)
	})
	a.GET("/admin/evaluations", func(c *gin.Context) {
		admin(c)
		v := must(records[Evaluation](s.S.DB, "evaluation", `ORDER BY created_at DESC`))
		out := []Evaluation{}
		for _, e := range v {
			if st := c.Query("status"); st != "" && st != e.Status {
				continue
			}
			if !containsFold(e.Text+" "+e.ItemID, c.Query("q")) {
				continue
			}
			out = append(out, e)
		}
		paginate(c, s.evaluationViews(out, true))
	})
	a.PATCH("/admin/evaluations/:id", func(c *gin.Context) {
		u := admin(c)
		q := body[struct{ Status, Reason string }](c)
		require((q.Status == "PUBLIC" || q.Status == "HIDDEN") && validText(q.Reason, 2, 300), "请选择状态并填写原因")
		s.Mu.Lock()
		defer s.Mu.Unlock()
		v := must(get[Evaluation](s.S.DB, "evaluation", c.Param("id")))
		v.Status = q.Status
		check(s.S.Transaction(func(tx *sql.Tx) error {
			if e := put(tx, "evaluation", v.ID, v); e != nil {
				return e
			}
			if e := audit(tx, u, "EVALUATION_"+q.Status, v.ID, q.Reason); e != nil {
				return e
			}
			title := "你的交接评价已恢复展示"
			if q.Status == "HIDDEN" {
				title = "你的交接评价已隐藏"
			}
			n := Notice{ID: "evaluation:" + uid(), UserID: v.UserID, ItemID: v.ItemID, Title: title, Body: v.Text, Reason: q.Reason, Action: "EVALUATION_" + q.Status, RelatedID: v.ID, Result: q.Status, At: now()}
			return put(tx, "notice", n.ID, n)
		}))
		c.JSON(200, v)
	})
	a.GET("/me/points", func(c *gin.Context) {
		u := actor(c)
		v := must(records[Point](s.S.DB, "point", `AND json_extract(data,'$.userId')=? ORDER BY created_at DESC`, u.ID))
		score := 0
		for _, p := range v {
			score += p.Delta
		}
		level := 1
		for _, threshold := range []int{20, 50, 100} {
			if score >= threshold {
				level++
			}
		}
		items := must(records[domain.Item](s.S.DB, "item", `AND json_extract(data,'$.reporterId')=?`, u.ID))
		returned := 0
		for _, it := range items {
			if it.Status == "RETURNED" {
				returned++
			}
		}
		c.JSON(200, gin.H{"score": score, "level": level, "entries": v, "published": len(items), "returned": returned, "rules": "有效归还：登记人+10，认领人+2。同一对用户七天内仅首次计分，每人每天最多新增20分。等级阈值：0/20/50/100。评价不影响积分。"})
	})
	a.GET("/items/:id/similar", func(c *gin.Context) {
		it := findItem(s.S.DB, c.Param("id"))
		require(visible(it, optionalUser(c)), "物品不可见")
		base := s.view(it)
		type candidate struct {
			Item   ItemView `json:"item"`
			Score  int      `json:"score"`
			Reason string   `json:"reason"`
		}
		all := must(records[domain.Item](s.S.DB, "item", `AND json_extract(data,'$.visibility')='PUBLIC' AND json_extract(data,'$.id')!=?`, it.ID))
		out := []candidate{}
		for _, v := range all {
			cv := s.view(v)
			n := 0
			if cv.Content.Category == base.Content.Category {
				n += 3
			}
			if cv.Content.Location == base.Content.Location {
				n += 2
			}
			if strings.Contains(cv.Content.Name, base.Content.Name) || strings.Contains(base.Content.Name, cv.Content.Name) {
				n += 2
			}
			if n > 0 {
				out = append(out, candidate{cv, n, "根据分类、地点和名称相似度推荐"})
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
		if len(out) > 5 {
			out = out[:5]
		}
		c.JSON(200, out)
	})
}
