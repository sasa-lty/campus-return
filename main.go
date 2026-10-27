package main

import (
	"campusreturn/internal/domain"
	"campusreturn/internal/ledger"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"log"
	"time"
)

type Contract struct{ contractapi.Contract }

func (s *Contract) GetItem(ctx contractapi.TransactionContextInterface, id string) (*domain.Item, error) {
	b, e := ctx.GetStub().GetState("item:" + id)
	if e != nil {
		return nil, e
	}
	if b == nil {
		return nil, fmt.Errorf("物品不存在")
	}
	var it domain.Item
	e = json.Unmarshal(b, &it)
	return &it, e
}
func (s *Contract) GetOperation(ctx contractapi.TransactionContextInterface, id string) (*domain.Receipt, error) {
	b, e := ctx.GetStub().GetState("op:" + id)
	if e != nil {
		return nil, e
	}
	if b == nil {
		return nil, fmt.Errorf("operation not found")
	}
	var v domain.Receipt
	e = json.Unmarshal(b, &v)
	return &v, e
}
func (s *Contract) GetItems(ctx contractapi.TransactionContextInterface) ([]*domain.Item, error) {
	it, e := ctx.GetStub().GetStateByRange("item:", "item;")
	if e != nil {
		return nil, e
	}
	defer it.Close()
	out := []*domain.Item{}
	for it.HasNext() {
		v, e := it.Next()
		if e != nil {
			return nil, e
		}
		var item domain.Item
		if e = json.Unmarshal(v.Value, &item); e != nil {
			return nil, e
		}
		out = append(out, &item)
	}
	return out, nil
}
func (s *Contract) Execute(ctx contractapi.TransactionContextInterface, raw string) (*domain.Receipt, error) {
	var c domain.Command
	if e := json.Unmarshal([]byte(raw), &c); e != nil {
		return nil, e
	}
	msp, e := ctx.GetClientIdentity().GetMSPID()
	if e != nil {
		return nil, e
	}
	want := "Org1MSP"
	if domain.StudentAction(c.Action) {
		want = "Org2MSP"
	}
	if msp != want {
		return nil, fmt.Errorf("组织权限不足")
	}
	prior, e := ctx.GetStub().GetState("op:" + c.ID)
	if e != nil {
		return nil, e
	}
	if prior != nil {
		var r domain.Receipt
		if e = json.Unmarshal(prior, &r); e != nil {
			return nil, e
		}
		if r.CommandHash != domain.Digest(c) {
			return nil, fmt.Errorf("操作编号已用于不同请求")
		}
		return &r, nil
	}
	var old *domain.Item
	b, e := ctx.GetStub().GetState("item:" + c.ItemID)
	if e != nil {
		return nil, e
	}
	if b != nil {
		old = &domain.Item{}
		if e = json.Unmarshal(b, old); e != nil {
			return nil, e
		}
	}
	stamp, e := ctx.GetStub().GetTxTimestamp()
	if e != nil {
		return nil, e
	}
	item, e := ledger.Apply(old, c, msp, ctx.GetStub().GetTxID(), time.Unix(stamp.Seconds, int64(stamp.Nanos)).UTC().Format(time.RFC3339Nano))
	if e != nil {
		return nil, e
	}
	r := domain.Receipt{CommandHash: domain.Digest(c), Item: item}
	b, _ = json.Marshal(item)
	if e = ctx.GetStub().PutState("item:"+item.ID, b); e != nil {
		return nil, e
	}
	b, _ = json.Marshal(r)
	if e = ctx.GetStub().PutState("op:"+c.ID, b); e != nil {
		return nil, e
	}
	e = ctx.GetStub().SetEvent("BusinessCommitted", b)
	return &r, e
}
func main() {
	cc, e := contractapi.NewChaincode(&Contract{})
	if e != nil {
		log.Fatal(e)
	}
	if e = cc.Start(); e != nil {
		log.Fatal(e)
	}
}
