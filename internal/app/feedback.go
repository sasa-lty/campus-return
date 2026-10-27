package app

import (
	"campusreturn/internal/domain"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"sort"
	"strings"
)

func operationFrom(q querier, id string) Operation {
	var raw string
	if q.QueryRow(`SELECT data FROM operations WHERE id=?`, id).Scan(&raw) != nil {
		return Operation{}
	}
	var o Operation
	_ = json.Unmarshal([]byte(raw), &o)
	return o
}

func verifiedReason(it domain.Item, o Operation) (string, bool) {
	if len(it.Events) == 0 {
		return "", false
	}
	e := it.Events[len(it.Events)-1]
	return o.Reason, o.Reason != "" && domain.Hash(o.Reason) == e.ReasonHash
}

func claimFeedback(q querier, item, user string) []ClaimFeedback {
	return must(records[ClaimFeedback](q, "claim_feedback", `AND json_extract(data,'$.itemId')=? AND json_extract(data,'$.userId')=? ORDER BY json_extract(data,'$.at') DESC, id DESC`, item, user))
}

// 使用相邻版本识别真正受影响的申请，包含择一通过、关闭取消等批量状态变化。
func projectClaimFeedback(q querier, it domain.Item, o Operation) error {
	if len(it.Events) == 0 {
		return nil
	}
	event := it.Events[len(it.Events)-1]
	previous, _ := get[domain.Item](q, "history", fmt.Sprintf("%s:%012d", it.ID, it.Version-1))
	states := map[string]string{}
	for _, cl := range previous.Claims {
		states[cl.ID] = cl.Status
	}
	reason, ok := verifiedReason(it, o)
	for _, cl := range it.Claims {
		if cl.Status == "PENDING" || states[cl.ID] == cl.Status {
			continue
		}
		f := ClaimFeedback{ID: event.OperationID + ":" + cl.ID, ItemID: it.ID, ClaimID: cl.ID, UserID: cl.UserID, Status: cl.Status, Action: event.Action, At: event.At, Reason: reason, ReasonAvailable: ok}
		if cl.Status == "SUPERSEDED" {
			f.Reason = "该物品已确认其他认领人，本次申请未被选中。"
			f.ReasonAvailable = true
		}
		if cl.Status == "WITHDRAWN" {
			f.Reason = "本人撤回认领申请。"
			f.ReasonAvailable = true
		}
		if !f.ReasonAvailable {
			f.Reason = "历史原因未留存"
		}
		if err := put(q, "claim_feedback", f.ID, f); err != nil {
			return err
		}
	}
	return nil
}

func businessNotices(q querier, it domain.Item, o Operation) map[string]Notice {
	out := map[string]Notice{}
	if len(it.Events) == 0 {
		return out
	}
	e := it.Events[len(it.Events)-1]
	cv, _ := get[ContentVersion](q, "content", it.ID+":"+it.ContentHash)
	reason, reasonOK := verifiedReason(it, o)
	add := func(user, title, body, result string, process, read bool, why string) {
		if user == "" {
			return
		}
		out[user] = Notice{ID: e.OperationID + ":" + user, UserID: user, ItemID: it.ID, Title: title, Body: body, ItemName: cv.Content.Name, Action: e.Action, Result: result, ClaimID: o.Command.Data.ClaimID, Reason: why, Process: process, Read: read, At: e.At}
	}
	selected := domain.Claim{}
	for _, cl := range it.Claims {
		if cl.ID == o.Command.Data.ClaimID {
			selected = cl
		}
	}
	actorUser, _ := get[User](q, "user", e.ActorID)
	name := actorUser.DisplayName
	if name == "" {
		name = "用户"
	}
	adminUsers := func() {
		users, _ := records[User](q, "user", `AND json_extract(data,'$.status')='ACTIVE' AND json_extract(data,'$.role') IN ('ADMIN','SUPER_ADMIN')`)
		for _, u := range users {
			add(u.ID, "有新的认领申请待审核", "请进入审核队列核对申请材料。", "PENDING", true, false, "")
		}
	}
	why := reason
	if !reasonOK {
		why = "历史原因未留存"
	}
	switch e.Action {
	case "CREATE":
		add(it.ReporterID, "拾物已登记", "物品已发布，可以接收失主认领。", "FOUND", true, true, "")
	case "CLAIM":
		add(it.ReporterID, "收到新的认领申请", name+"提交了申请，等待管理员核验。", "PENDING", true, false, "")
		add(e.ActorID, "认领申请已提交，等待管理员审核", "本次申请已进入审核队列，无需重复提交。", "PENDING", true, true, "")
		adminUsers()
	case "WITHDRAW":
		add(it.ReporterID, "一份认领申请已撤回", name+"撤回了自己的申请。", "WITHDRAWN", true, false, "")
		add(e.ActorID, "认领申请已撤回", "本次申请已停止审核。", "WITHDRAWN", true, true, "")
	case "REJECT":
		add(selected.UserID, "认领申请未通过审核", "请查看处理原因；物品仍开放时可补充材料重新申请。", "REJECTED", false, false, why)
		add(it.ReporterID, "一份认领申请未通过审核", "管理员已处理一份申请，物品当前状态可在详情查看。", "REJECTED", true, false, "")
	case "APPROVE":
		add(it.OwnerID, "认领申请已通过审核", "请安排线下交接，到场后出示交接码。", "APPROVED", false, false, why)
		add(it.ReporterID, "物品已确认认领人", "管理员已完成核验，等待与认领人办理交接。", "CLAIMED", false, false, "")
		for _, cl := range it.Claims {
			if cl.Status == "SUPERSEDED" {
				f, err := get[ClaimFeedback](q, "claim_feedback", e.OperationID+":"+cl.ID)
				if err == nil {
					add(cl.UserID, "本次认领未被选中", "该物品已确认其他认领人。", "SUPERSEDED", false, false, f.Reason)
				}
			}
		}
	case "RETURN":
		add(it.ReporterID, "交接完成，物品已归还", "你登记的物品已完成交接，可查看贡献记录与提交评价。", "RETURNED", false, false, "")
		add(it.OwnerID, "交接完成，物品已归还", "本次领取已完成，可评价交接体验。", "RETURNED", false, false, "")
	case "HIDE", "RESTORE", "CLOSE", "REOPEN", "UPDATE", "REVOKE", "FLAG_FRAUD":
		titles := map[string]string{"HIDE": "物品已下架", "RESTORE": "物品已恢复展示", "CLOSE": "物品已关闭认领", "REOPEN": "物品已重新开放", "UPDATE": "物品内容已更新", "REVOKE": "确权已撤销", "FLAG_FRAUD": "交接争议已处理，积分已核对"}
		add(it.ReporterID, titles[e.Action], "管理员已处理此物品，请查看本次说明。", it.Status, false, false, why)
		if it.OwnerID != "" {
			add(it.OwnerID, titles[e.Action], "与你相关的物品处理结果已更新。", it.Status, false, false, why)
		}
		for _, cl := range it.Claims {
			affected := cl.Status == "PENDING"
			if e.Action == "CLOSE" || e.Action == "REVOKE" {
				_, err := get[ClaimFeedback](q, "claim_feedback", e.OperationID+":"+cl.ID)
				affected = err == nil
			}
			if affected {
				add(cl.UserID, titles[e.Action], "与你相关的认领流程发生变化，请查看处理说明。", cl.Status, false, false, why)
			}
		}
	}
	return out
}

func writeBusinessNotices(q querier, it domain.Item, o Operation) error {
	for _, n := range businessNotices(q, it, o) {
		if _, e := get[Notice](q, "notice", n.ID); e == nil {
			continue
		}
		if e := put(q, "notice", n.ID, n); e != nil {
			return e
		}
	}
	return nil
}

// 仅补齐应用层展示资料；从每个历史版本重建，不用当前状态冒充旧消息。
func (s *Server) repairFeedback() error {
	if done, _ := get[bool](s.S.DB, "meta", "feedback-v1"); done {
		return nil
	}
	return s.S.Transaction(func(q *sql.Tx) error {
		histories, err := records[domain.Item](q, "history", `ORDER BY json_extract(data,'$.id'),json_extract(data,'$.version') ASC`)
		if err != nil {
			return err
		}
		for _, it := range histories {
			if len(it.Events) == 0 {
				continue
			}
			e := it.Events[len(it.Events)-1]
			o := operationFrom(q, e.OperationID)
			if err := projectClaimFeedback(q, it, o); err != nil {
				return err
			}
			allowed := businessNotices(q, it, o)
			ns, err := records[Notice](q, "notice", `AND json_extract(data,'$.itemId')=?`, it.ID)
			if err != nil {
				return err
			}
			for _, old := range ns {
				if old.ID != e.OperationID+":"+old.UserID {
					continue
				}
				next, ok := allowed[old.UserID]
				if !ok {
					old.Suppressed = true
					old.Read = true
					next = old
				} else {
					next.At = old.At
					next.Read = old.Read || next.Read
				}
				if err := put(q, "notice", next.ID, next); err != nil {
					return err
				}
			}
		}
		return put(q, "meta", "feedback-v1", true)
	})
}

type noticeGroup struct {
	ID       string   `json:"id"`
	ItemID   string   `json:"itemId"`
	ItemName string   `json:"itemName"`
	Title    string   `json:"title"`
	At       string   `json:"at"`
	Process  bool     `json:"process"`
	Unread   int      `json:"unread"`
	Entries  []Notice `json:"entries"`
}

func groupedNotices(v []Notice) []noticeGroup {
	out := []noticeGroup{}
	indexes := map[string]int{}
	for _, n := range v {
		key := n.ID
		if n.Process {
			key = "process:" + n.ItemID
		}
		index, ok := indexes[key]
		if !ok {
			index = len(out)
			indexes[key] = index
			out = append(out, noticeGroup{ID: key, ItemID: n.ItemID, ItemName: n.ItemName, Title: n.Title, At: n.At, Process: n.Process, Entries: []Notice{}})
		}
		g := &out[index]
		g.Entries = append(g.Entries, n)
		if !n.Read {
			g.Unread++
		}
		if n.At > g.At {
			g.At = n.At
			g.Title = n.Title
		}
		if g.Process {
			g.Title = "认领过程动态"
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At > out[j].At })
	return out
}

func (s *Server) feedbackRoutes(a *gin.RouterGroup) {
	a.GET("/notifications/:id", func(c *gin.Context) {
		u := actor(c)
		n := must(get[Notice](s.S.DB, "notice", c.Param("id")))
		if n.UserID != u.ID {
			problem(403, "无权查看消息")
		}
		c.JSON(200, n)
	})
	a.GET("/me/evaluations", func(c *gin.Context) {
		u := actor(c)
		v := must(records[Evaluation](s.S.DB, "evaluation", `AND json_extract(data,'$.userId')=? ORDER BY created_at DESC`, u.ID))
		if id := c.Query("itemId"); id != "" {
			filtered := []Evaluation{}
			for _, e := range v {
				if e.ItemID == id {
					filtered = append(filtered, e)
				}
			}
			v = filtered
		}
		paginate(c, s.evaluationViews(v, true))
	})
}

func (s *Server) evaluationViews(values []Evaluation, private bool) []gin.H {
	out := []gin.H{}
	for _, v := range values {
		u, _ := get[User](s.S.DB, "user", v.UserID)
		it, _ := get[domain.Item](s.S.DB, "item", v.ItemID)
		cv, _ := get[ContentVersion](s.S.DB, "content", it.ID+":"+it.ContentHash)
		role := "认领人"
		if v.UserID == it.ReporterID {
			role = "登记人"
		}
		row := gin.H{"id": v.ID, "itemId": v.ItemID, "itemName": cv.Content.Name, "displayName": u.DisplayName, "participantRole": role, "stars": v.Stars, "text": v.Text, "status": v.Status, "at": v.At, "mine": false}
		if private {
			row["history"] = must(records[Audit](s.S.DB, "audit", `AND json_extract(data,'$.targetId')=? AND json_extract(data,'$.action') LIKE 'EVALUATION_%' ORDER BY created_at DESC`, v.ID))
		}
		out = append(out, row)
	}
	return out
}

func reportHistory(q querier, id string) []Audit {
	return must(records[Audit](q, "audit", `AND json_extract(data,'$.targetId')=? AND json_extract(data,'$.action') LIKE 'REPORT_%' ORDER BY created_at DESC`, id))
}
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(strings.TrimSpace(needle)))
}
