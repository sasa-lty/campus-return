package app

import (
	"campusreturn/internal/domain"
	"github.com/gin-gonic/gin"
)

// 个人概览只汇总当前登录用户的业务，不返回其他人的申请材料。
func (s *Server) personalOverview(c *gin.Context) {
	u := actor(c)
	counts := map[string]int{"items": 0, "claims": 0, "pendingClaims": 0, "handovers": 0, "unread": 0, "openReports": 0, "pendingOperations": 0}
	items := must(records[domain.Item](s.S.DB, "item", `AND (json_extract(data,'$.reporterId')=? OR json_extract(data,'$.ownerId')=? OR EXISTS (SELECT 1 FROM json_each(json_extract(data,'$.claims')) cl WHERE json_extract(cl.value,'$.userId')=?)) ORDER BY updated_at DESC`, u.ID, u.ID, u.ID))
	type task struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Location string `json:"location"`
		Status   string `json:"status"`
	}
	handovers := []task{}
	for _, it := range items {
		if it.ReporterID == u.ID {
			counts["items"]++
		}
		for _, cl := range it.Claims {
			if cl.UserID == u.ID {
				counts["claims"]++
				if cl.Status == "PENDING" {
					counts["pendingClaims"]++
				}
			}
		}
		if it.Status == "CLAIMED" && it.OwnerID == u.ID {
			counts["handovers"]++
			if len(handovers) < 5 {
				v := s.view(it)
				handovers = append(handovers, task{it.ID, v.Content.Name, v.Content.Location, it.Status})
			}
		}
	}
	count := func(key, query string) {
		var n int
		check(s.S.DB.QueryRow(query, u.ID).Scan(&n))
		counts[key] = n
	}
	count("unread", `SELECT count(*) FROM records WHERE kind='notice' AND json_extract(data,'$.userId')=? AND json_extract(data,'$.read')=0 AND coalesce(json_extract(data,'$.suppressed'),0)=0`)
	count("openReports", `SELECT count(*) FROM records WHERE kind='report' AND json_extract(data,'$.userId')=? AND json_extract(data,'$.status')!='RESOLVED'`)
	count("pendingOperations", `SELECT count(*) FROM operations WHERE user_id=? AND json_extract(data,'$.status') IN ('QUEUED','SUBMITTING','PENDING')`)
	notices := must(records[Notice](s.S.DB, "notice", `AND json_extract(data,'$.userId')=? AND coalesce(json_extract(data,'$.suppressed'),0)=0 ORDER BY created_at DESC LIMIT 5`, u.ID))
	c.JSON(200, gin.H{"counts": counts, "handovers": handovers, "notices": notices, "updatedAt": now()})
}
