package app

import (
	"campusreturn/internal/domain"
	"database/sql"
	"encoding/json"
	"strings"
)

// 修正旧版把“收到申请”广播给申请人的站内消息，保留原时间和已读状态。
// 仅修正应用通知，不修改账本业务或历史。
func (s *Server) repairClaimNotices() error {
	if done, _ := get[bool](s.S.DB, "meta", "claim-notices-v2"); done {
		return nil
	}
	return s.S.Transaction(func(tx *sql.Tx) error {
		notices, err := records[Notice](tx, "notice", `AND json_extract(data,'$.title')='收到新的认领申请'`)
		if err != nil {
			return err
		}
		for _, n := range notices {
			var raw string
			id := strings.TrimSuffix(n.ID, ":"+n.UserID)
			if err = tx.QueryRow(`SELECT data FROM operations WHERE id=?`, id).Scan(&raw); err != nil {
				continue
			}
			var op Operation
			if err = json.Unmarshal([]byte(raw), &op); err != nil {
				return err
			}
			if op.Command.Action != "CLAIM" {
				continue
			}
			if n.UserID == op.UserID {
				n.Title = "认领申请已提交，等待管理员审核"
				if err = put(tx, "notice", n.ID, n); err != nil {
					return err
				}
			} else if it, e := get[domain.Item](tx, "item", n.ItemID); e == nil && n.UserID != it.ReporterID {
				if _, err = tx.Exec(`DELETE FROM records WHERE kind='notice' AND id=?`, n.ID); err != nil {
					return err
				}
			}
		}
		return put(tx, "meta", "claim-notices-v2", true)
	})
}
