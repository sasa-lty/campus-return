package ledger

import (
	"campusreturn/internal/domain"
	"encoding/hex"
	"errors"
	"fmt"
)

func validHash(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == 32 }

// Apply 是内存模式与链码共用的确定性规则。时间与交易编号由调用环境传入。
func Apply(old *domain.Item, c domain.Command, msp, txID, at string) (domain.Item, error) {
	var it domain.Item
	fail := func(s string) (domain.Item, error) { return it, errors.New(s) }
	expected := "Org1MSP"
	if domain.StudentAction(c.Action) {
		expected = "Org2MSP"
	}
	if msp != expected {
		return fail("调用组织不允许执行此操作")
	}
	if c.ID == "" || c.ActorID == "" || c.ItemID == "" {
		return fail("操作编号、物品编号和操作者不能为空")
	}
	if old != nil {
		it = *old
		it.Claims = append([]domain.Claim{}, old.Claims...)
		it.Events = append([]domain.Event{}, old.Events...)
	}
	if c.Action == "CREATE" {
		if old != nil {
			return fail("物品已存在")
		}
		if !validHash(c.Data.ContentHash) || !validHash(c.Data.AnswerHash) || !validHash(c.Data.QuestionHash) {
			return fail("内容与核验摘要无效")
		}
		it = domain.Item{ID: c.ItemID, ReporterID: c.ActorID, ContentHash: c.Data.ContentHash, AnswerHash: c.Data.AnswerHash, QuestionHash: c.Data.QuestionHash, ImageHash: c.Data.ImageHash, Status: "FOUND", Visibility: "PUBLIC", CreatedAt: at, Claims: []domain.Claim{}, Events: []domain.Event{}}
	} else {
		if old == nil {
			return fail("物品不存在")
		}
		if c.ExpectedVersion != it.Version {
			return fail("记录已更新，请刷新后重试")
		}
		claim := -1
		for i := range it.Claims {
			if it.Claims[i].ID == c.Data.ClaimID {
				claim = i
			}
		}
		switch c.Action {
		case "UPDATE":
			if it.Status != "FOUND" && it.Status != "CLAIM_PENDING" {
				return fail("当前状态不允许编辑")
			}
			if !validHash(c.Data.ContentHash) || !validHash(c.Data.AnswerHash) || !validHash(c.Data.QuestionHash) {
				return fail("摘要无效")
			}
			if it.Status == "CLAIM_PENDING" && (c.Data.AnswerHash != it.AnswerHash || c.Data.QuestionHash != it.QuestionHash) {
				return fail("已有认领时不能修改核验问题或答案")
			}
			it.ContentHash, it.AnswerHash, it.QuestionHash, it.ImageHash = c.Data.ContentHash, c.Data.AnswerHash, c.Data.QuestionHash, c.Data.ImageHash
		case "CLAIM":
			if it.Visibility != "PUBLIC" || (it.Status != "FOUND" && it.Status != "CLAIM_PENDING") {
				return fail("物品当前不接受认领")
			}
			if c.ActorID == it.ReporterID {
				return fail("登记人不能认领自己的物品")
			}
			if c.Data.ClaimID == "" || !validHash(c.Data.EvidenceHash) || claim >= 0 {
				return fail("认领编号或摘要无效")
			}
			n := 0
			for _, cl := range it.Claims {
				if cl.Status == "PENDING" {
					n++
					if cl.UserID == c.ActorID {
						return fail("已有待审核申请")
					}
				}
			}
			if n >= 20 {
				return fail("待审核申请已达上限")
			}
			it.Claims = append(it.Claims, domain.Claim{ID: c.Data.ClaimID, UserID: c.ActorID, EvidenceHash: c.Data.EvidenceHash, Status: "PENDING", CreatedAt: at})
			it.Status = "CLAIM_PENDING"
		case "APPROVE", "REJECT", "WITHDRAW":
			if it.Status != "CLAIM_PENDING" || claim < 0 || it.Claims[claim].Status != "PENDING" {
				return fail("该认领已不处于待审核状态")
			}
			if c.Action == "WITHDRAW" && it.Claims[claim].UserID != c.ActorID {
				return fail("只能撤回本人的申请")
			}
			if c.Action == "APPROVE" {
				it.OwnerID, it.ClaimID, it.Status = it.Claims[claim].UserID, it.Claims[claim].ID, "CLAIMED"
				for i := range it.Claims {
					if it.Claims[i].Status == "PENDING" {
						it.Claims[i].Status = "SUPERSEDED"
					}
				}
				it.Claims[claim].Status = "APPROVED"
			} else {
				it.Claims[claim].Status = c.Action + "ED"
				if c.Action == "REJECT" {
					it.Claims[claim].Status = "REJECTED"
				} else {
					it.Claims[claim].Status = "WITHDRAWN"
				}
				it.Status = "FOUND"
				for _, cl := range it.Claims {
					if cl.Status == "PENDING" {
						it.Status = "CLAIM_PENDING"
					}
				}
			}
		case "RETURN":
			if it.Status != "CLAIMED" || it.ClaimID != c.Data.ClaimID || !validHash(c.Data.ReceiptHash) {
				return fail("归还需绑定当前确权认领及交接凭证")
			}
			it.Status = "RETURNED"
			it.ReceiptHash = c.Data.ReceiptHash
			if claim >= 0 {
				it.Claims[claim].Status = "RETURNED"
			}
		case "REVOKE":
			if it.Status != "CLAIMED" || it.ClaimID != c.Data.ClaimID {
				return fail("只能撤销当前确权")
			}
			if claim >= 0 {
				it.Claims[claim].Status = "REVOKED"
			}
			it.Status, it.OwnerID, it.ClaimID = "FOUND", "", ""
		case "HIDE":
			it.Visibility = "HIDDEN"
		case "RESTORE":
			it.Visibility = "PUBLIC"
		case "CLOSE":
			if it.Status != "FOUND" && it.Status != "CLAIM_PENDING" {
				return fail("只能关闭未确权物品")
			}
			for i := range it.Claims {
				if it.Claims[i].Status == "PENDING" {
					it.Claims[i].Status = "CANCELLED"
				}
			}
			it.Status = "CLOSED"
		case "REOPEN":
			if it.Status != "CLOSED" {
				return fail("只有关闭记录可以重新开放")
			}
			it.Status = "FOUND"
		case "FLAG_FRAUD":
			if it.Status != "RETURNED" {
				return fail("只能对已归还交接追加纠正记录")
			}
		default:
			return fail(fmt.Sprintf("不支持的业务操作：%s", c.Action))
		}
		if !domain.StudentAction(c.Action) && !validHash(c.Data.ReasonHash) {
			return fail("管理操作必须记录原因摘要")
		}
	}
	it.Version++
	it.UpdatedAt = at
	it.Events = append(it.Events, domain.Event{Action: c.Action, ActorID: c.ActorID, OperationID: c.ID, TxID: txID, At: at, ReasonHash: c.Data.ReasonHash})
	return it, nil
}
