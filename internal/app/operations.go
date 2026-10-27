package app

import (
	"campusreturn/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Server) operation(id string) (Operation, error) {
	var raw string
	e := s.S.DB.QueryRow(`SELECT data FROM operations WHERE id=?`, id).Scan(&raw)
	var op Operation
	if e == nil {
		e = json.Unmarshal([]byte(raw), &op)
	}
	return op, e
}
func saveOperation(q querier, o Operation) error {
	o.UpdatedAt = now()
	_, e := q.Exec(`UPDATE operations SET data=? WHERE id=?`, jsonValue(o), o.ID)
	return e
}
func (s *Server) queue(c *gin.Context, u User, cmd domain.Command, content *domain.Content, secret, reason, reportID string) {
	key := c.GetHeader("Idempotency-Key")
	require(len(key) >= 8 && len(key) <= 128, "写入操作必须提供 Idempotency-Key")
	s.Mu.Lock()
	defer s.Mu.Unlock()
	hash, _ := c.Get("requestHash")
	requestHash, _ := hash.(string)
	var raw, oldHash string
	e := s.S.DB.QueryRow(`SELECT data,request_hash FROM operations WHERE user_id=? AND idem=?`, u.ID, key).Scan(&raw, &oldHash)
	if e == nil {
		if requestHash != oldHash {
			problem(409, "幂等键冲突")
		}
		var prior Operation
		check(json.Unmarshal([]byte(raw), &prior))
		c.JSON(202, prior.Public())
		return
	}
	op := Operation{ID: cmd.ID, UserID: u.ID, ItemID: cmd.ItemID, Status: "QUEUED", At: now(), UpdatedAt: now(), Command: cmd, RequestHash: requestHash, Content: content, Secret: secret, Reason: reason, ReportID: reportID}
	check(s.S.Transaction(func(tx *sql.Tx) error {
		if content != nil {
			cv := ContentVersion{ItemID: cmd.ItemID, Hash: cmd.Data.ContentHash, Content: *content, CreatedAt: now()}
			if e := put(tx, "content", cmd.ItemID+":"+cv.Hash, cv); e != nil {
				return e
			}
		}
		_, e := tx.Exec(`INSERT INTO operations VALUES(?,?,?,?,?)`, op.ID, u.ID, key, requestHash, jsonValue(op))
		return e
	}))
	c.JSON(202, op.Public())
}
func (s *Server) Run(ctx context.Context) { go s.worker(ctx); go s.events(ctx) }
func (s *Server) events(ctx context.Context) {
	for ctx.Err() == nil {
		checkpoint, _ := get[uint64](s.S.DB, "checkpoint", "fabric")
		e := s.Chain.Events(ctx, checkpoint, func(block uint64, r domain.Receipt) error {
			if r.Item.ID != "" {
				return s.applyReceipt(r)
			}
			return put(s.S.DB, "checkpoint", "fabric", block+1)
		})
		if ctx.Err() != nil {
			return
		}
		if e != nil {
			log.Print("ledger event stream reconnecting")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}
func (s *Server) worker(ctx context.Context) {
	tick := time.NewTicker(600 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		rows, e := s.S.DB.Query(`SELECT data FROM operations WHERE json_extract(data,'$.status') IN ('QUEUED','PENDING','SUBMITTING') ORDER BY rowid LIMIT 12`)
		if e != nil {
			continue
		}
		ops := []Operation{}
		for rows.Next() {
			var raw string
			if rows.Scan(&raw) == nil {
				var o Operation
				if json.Unmarshal([]byte(raw), &o) == nil {
					ops = append(ops, o)
				}
			}
		}
		rows.Close()
		for _, o := range ops {
			if ctx.Err() != nil {
				return
			}
			if r, e := s.Chain.Receipt(o.ID); e == nil {
				s.finish(o, r)
				continue
			}
			if o.Status != "QUEUED" && len(o.Endorsed) == 0 {
				// 背书结果必须先持久化才允许提交；缺少它说明尚未发送，可安全重新准备。
				o.Status = "QUEUED"
				o.TxID = ""
				_ = saveOperation(s.S.DB, o)
			}
			if o.Status == "QUEUED" && o.TxID == "" {
				u, err := get[User](s.S.DB, "user", o.UserID)
				allowed := err == nil && u.Status == "ACTIVE" && !u.ForcePassword
				if domain.StudentAction(o.Command.Action) {
					allowed = allowed && u.Role == "USER"
				} else {
					allowed = allowed && u.Admin()
				}
				if !allowed {
					o.Status = "FAILED"
					o.Error = "账号状态或角色已变化，请重新登录核对"
					_ = saveOperation(s.S.DB, o)
					continue
				}
			}
			o.Status = "SUBMITTING"
			_ = saveOperation(s.S.DB, o)
			r, e := s.Chain.Submit(o, func(txID string, endorsed []byte) error {
				o.TxID = txID
				if len(endorsed) > 0 {
					o.Endorsed = endorsed
				}
				return saveOperation(s.S.DB, o)
			})
			if e == nil {
				s.finish(o, r)
				continue
			}
			var uncertain interface{ Uncertain() bool }
			if errors.As(e, &uncertain) && uncertain.Uncertain() {
				o.Status = "PENDING"
				o.Error = "交易结果待确认，材料已保留"
			} else {
				o.Status = "FAILED"
				o.Error = e.Error()
			}
			_ = saveOperation(s.S.DB, o)
		}
	}
}
func (s *Server) finish(o Operation, r domain.Receipt) {
	if r.CommandHash != domain.Digest(o.Command) {
		o.Status = "FAILED"
		o.Error = "操作回执不匹配"
		_ = saveOperation(s.S.DB, o)
		return
	}
	if e := s.applyReceipt(r); e != nil {
		log.Print("local receipt persistence requires retry")
		return
	}
	o.Status = "SUCCEEDED"
	o.Error = ""
	if len(r.Item.Events) > 0 {
		o.TxID = r.Item.Events[len(r.Item.Events)-1].TxID
	}
	o.Endorsed = nil
	_ = saveOperation(s.S.DB, o)
}
func (s *Server) applyReceipt(r domain.Receipt) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	it := r.Item
	if len(it.Events) == 0 {
		return errors.New("missing event")
	}
	event := it.Events[len(it.Events)-1]
	o, _ := s.operation(event.OperationID)
	return s.S.Transaction(func(tx *sql.Tx) error {
		old, e := get[domain.Item](tx, "item", it.ID)
		if e != nil || old.Version < it.Version {
			if e = put(tx, "item", it.ID, it); e != nil {
				return e
			}
		}
		if _, e = get[bool](tx, "effect", event.OperationID); e == nil {
			return nil
		}
		if o.Secret != "" {
			key := "item:" + it.ID
			if o.Command.Action == "CLAIM" {
				key = "claim:" + o.Command.Data.ClaimID
			}
			if e = put(tx, "private", key, o.Secret); e != nil {
				return e
			}
		}
		if e = put(tx, "history", fmt.Sprintf("%s:%012d", it.ID, it.Version), it); e != nil {
			return e
		}
		if e = put(tx, "audit", event.OperationID, Audit{ID: event.OperationID, ActorID: event.ActorID, Action: event.Action, TargetID: it.ID, Reason: o.Reason, At: event.At}); e != nil {
			return e
		}
		if e = projectClaimFeedback(tx, it, o); e != nil {
			return e
		}
		if e = writeBusinessNotices(tx, it, o); e != nil {
			return e
		}
		if event.Action == "RETURN" {
			if e = s.award(tx, it, event.At); e != nil {
				return e
			}
			if h, e := get[Handover](tx, "handover", it.ID); e == nil {
				h.Used = true
				if e = put(tx, "handover", it.ID, h); e != nil {
					return e
				}
			}
		}
		if event.Action == "FLAG_FRAUD" {
			points, e := records[Point](tx, "point", `AND json_extract(data,'$.itemId')=?`, it.ID)
			if e != nil {
				return e
			}
			for _, p := range points {
				if p.Delta > 0 {
					p.ID = "reversal:" + p.ID
					p.Delta = -p.Delta
					p.Reason = "虚假交接纠正"
					p.At = event.At
					if _, e = get[Point](tx, "point", p.ID); e != nil {
						if e = put(tx, "point", p.ID, p); e != nil {
							return e
						}
					}
				}
			}
		}
		if o.ReportID != "" {
			report, e := get[Report](tx, "report", o.ReportID)
			if e == nil {
				report.Status = "RESOLVED"
				report.Resolution = o.Reason
				if err := put(tx, "audit", "report:"+event.OperationID, Audit{ID: "report:" + event.OperationID, ActorID: event.ActorID, Action: "REPORT_RESOLVED", TargetID: report.ID, Reason: o.Reason, At: event.At}); err != nil {
					return err
				}
				if e = put(tx, "report", report.ID, report); e != nil {
					return e
				}
				if e = notice(tx, "report:"+event.OperationID, report.UserID, it.ID, "举报或争议已处理："+o.Reason); e != nil {
					return e
				}
			}
		}
		return put(tx, "effect", event.OperationID, true)
	})
}
func (s *Server) award(q querier, it domain.Item, at string) error {
	pairIDs := []string{it.ReporterID, it.OwnerID}
	sort.Strings(pairIDs)
	pair := strings.Join(pairIDs, ":")
	t, e := time.Parse(time.RFC3339Nano, at)
	if e != nil {
		return e
	}
	pairs, e := records[Point](q, "point", `AND json_extract(data,'$.pair')=? AND json_extract(data,'$.delta')>0 AND json_extract(data,'$.at')>=?`, pair, t.Add(-7*24*time.Hour).UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	if len(pairs) > 0 {
		return nil
	}
	for _, v := range []struct {
		u     string
		delta int
	}{{it.ReporterID, 10}, {it.OwnerID, 2}} {
		local := t.In(time.FixedZone("Asia/Shanghai", 8*3600))
		dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).UTC().Format(time.RFC3339Nano)
		points, e := records[Point](q, "point", `AND json_extract(data,'$.userId')=? AND json_extract(data,'$.at')>=?`, v.u, dayStart)
		if e != nil {
			return e
		}
		used := 0
		for _, p := range points {
			if p.Delta > 0 {
				used += p.Delta
			}
		}
		delta := v.delta
		if used+delta > 20 {
			delta = 20 - used
		}
		if delta < 0 {
			delta = 0
		}
		p := Point{ID: it.ID + ":" + v.u, UserID: v.u, ItemID: it.ID, Pair: pair, Delta: delta, Reason: "有效归还贡献", At: at}
		if e = put(q, "point", p.ID, p); e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) view(it domain.Item) ItemView {
	cv, e := get[ContentVersion](s.S.DB, "content", it.ID+":"+it.ContentHash)
	reporter, _ := get[User](s.S.DB, "user", it.ReporterID)
	owner, _ := get[User](s.S.DB, "user", it.OwnerID)
	return ItemView{Item: it, Content: cv.Content, Integrity: e == nil && domain.Digest(cv.Content) == it.ContentHash, ReporterName: reporter.DisplayName, OwnerName: owner.DisplayName}
}
func (s *Server) secret(key, hash string) (Secret, error) {
	raw, e := get[string](s.S.DB, "private", key)
	if e != nil {
		return Secret{}, e
	}
	var v Secret
	if e = s.S.Unseal(raw, &v); e != nil {
		return v, e
	}
	if domain.Hash(v.Salt+":"+v.Text) != hash {
		return v, errors.New("私密材料摘要不一致")
	}
	return v, nil
}
