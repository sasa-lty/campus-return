package app

import (
	"campusreturn/internal/domain"
	"campusreturn/internal/ledger"
	"context"
	"database/sql"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("operation not found")

type Chain interface {
	Mode() string
	Get(string) (domain.Item, error)
	Items() ([]domain.Item, error)
	Receipt(string) (domain.Receipt, error)
	Submit(Operation, func(string, []byte) error) (domain.Receipt, error)
	Events(context.Context, uint64, func(uint64, domain.Receipt) error) error
	Health() error
	Close()
}
type DemoChain struct {
	S  *Store
	mu sync.Mutex
}

func (d *DemoChain) Mode() string  { return "demo-local" }
func (d *DemoChain) Close()        {}
func (d *DemoChain) Health() error { return nil }
func (d *DemoChain) Events(ctx context.Context, n uint64, fn func(uint64, domain.Receipt) error) error {
	<-ctx.Done()
	return ctx.Err()
}
func (d *DemoChain) Get(id string) (domain.Item, error) {
	return get[domain.Item](d.S.DB, "demo_item", id)
}
func (d *DemoChain) Items() ([]domain.Item, error) {
	return records[domain.Item](d.S.DB, "demo_item", "")
}
func (d *DemoChain) Receipt(id string) (domain.Receipt, error) {
	r, e := get[domain.Receipt](d.S.DB, "demo_receipt", id)
	if e != nil {
		return r, ErrNotFound
	}
	return r, nil
}
func (d *DemoChain) Submit(o Operation, save func(string, []byte) error) (domain.Receipt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r, e := d.Receipt(o.ID); e == nil {
		if r.CommandHash != domain.Digest(o.Command) {
			return r, errors.New("幂等操作冲突")
		}
		return r, nil
	}
	tx := "DEMO-" + o.ID
	if e := save(tx, nil); e != nil {
		return domain.Receipt{}, e
	}
	var old *domain.Item
	if it, e := d.Get(o.ItemID); e == nil {
		old = &it
	}
	msp := "Org1MSP"
	if domain.StudentAction(o.Command.Action) {
		msp = "Org2MSP"
	}
	item, e := ledger.Apply(old, o.Command, msp, tx, now())
	if e != nil {
		return domain.Receipt{}, e
	}
	r := domain.Receipt{CommandHash: domain.Digest(o.Command), Item: item}
	e = d.S.Transaction(func(q *sql.Tx) error {
		if e := put(q, "demo_item", item.ID, item); e != nil {
			return e
		}
		return put(q, "demo_receipt", o.ID, r)
	})
	return r, e
}
