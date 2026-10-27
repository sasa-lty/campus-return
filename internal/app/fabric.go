package app

import (
	"campusreturn/backend/fabric"
	"campusreturn/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	common "github.com/hyperledger/fabric-protos-go-apiv2/common"
	peer "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/proto"
	"os"
	"strings"
)

type FabricChain struct {
	gateways  []*client.Gateway
	contracts []*client.Contract
	networks  []*client.Network
	clients   []*fabric.Client
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func NewFabricChain() (*FabricChain, error) {
	f := &FabricChain{}
	for i := 1; i <= 2; i++ {
		org := fmt.Sprintf("org%d.example.com", i)
		peerName := "peer0." + org
		address := env(fmt.Sprintf("FABRIC_ORG%d_ADDRESS", i), map[int]string{1: "localhost:17051", 2: "localhost:19051"}[i])
		// 连接、TLS 和组织签名沿用原项目；持久化交易恢复由本层扩展。
		cfg := fabric.ConfigForOrg(fmt.Sprintf("ORG%d", i), fmt.Sprintf("Org%d", i), org, fmt.Sprintf("Org%dMSP", i), address, peerName)
		cfg.Channel = env("CHANNEL_NAME", "returnchannel")
		cfg.Chaincode = env("CHAINCODE_NAME", "campusreturn")
		connection, e := fabric.New(cfg)
		if e != nil {
			f.Close()
			return nil, e
		}
		f.clients = append(f.clients, connection)
		gw := connection.Gateway
		f.gateways = append(f.gateways, gw)
		network := gw.GetNetwork(cfg.Channel)
		f.networks = append(f.networks, network)
		f.contracts = append(f.contracts, connection.Contract)
	}
	return f, nil
}
func (f *FabricChain) Mode() string { return "hyperledger-fabric" }
func (f *FabricChain) Close() {
	for _, c := range f.clients {
		c.Close()
	}
}
func (f *FabricChain) Health() error {
	for _, c := range f.contracts {
		if _, e := c.EvaluateTransaction("GetItems"); e != nil {
			return e
		}
	}
	return nil
}
func (f *FabricChain) Get(id string) (domain.Item, error) {
	var v domain.Item
	b, e := f.contracts[0].EvaluateTransaction("GetItem", id)
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (f *FabricChain) Items() ([]domain.Item, error) {
	v := []domain.Item{}
	b, e := f.contracts[0].EvaluateTransaction("GetItems")
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (f *FabricChain) Receipt(id string) (domain.Receipt, error) {
	var v domain.Receipt
	b, e := f.contracts[0].EvaluateTransaction("GetOperation", id)
	if e != nil {
		if strings.Contains(e.Error(), "operation not found") {
			return v, ErrNotFound
		}
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}

type uncertainError struct{ error }

func (e uncertainError) Uncertain() bool { return true }
func (f *FabricChain) Submit(o Operation, save func(string, []byte) error) (domain.Receipt, error) {
	i := 0
	if domain.StudentAction(o.Command.Action) {
		i = 1
	}
	var tx *client.Transaction
	var e error
	if len(o.Endorsed) > 0 {
		tx, e = f.gateways[i].NewTransaction(o.Endorsed)
		if e != nil {
			return domain.Receipt{}, e
		}
	} else {
		proposal, e := f.contracts[i].NewProposal("Execute", client.WithArguments(jsonValue(o.Command)))
		if e != nil {
			return domain.Receipt{}, e
		}
		if e = save(proposal.TransactionID(), nil); e != nil {
			return domain.Receipt{}, e
		}
		tx, e = proposal.Endorse()
		if e != nil {
			return domain.Receipt{}, fmt.Errorf("链码未接受请求：%w", e)
		}
		b, e := tx.Bytes()
		if e != nil {
			return domain.Receipt{}, e
		}
		if e = save(tx.TransactionID(), b); e != nil {
			return domain.Receipt{}, e
		}
	}
	commit, e := tx.Submit()
	if e != nil {
		if r, err := f.Receipt(o.ID); err == nil {
			return r, nil
		}
		return domain.Receipt{}, uncertainError{e}
	}
	status, e := commit.Status()
	if e != nil {
		return domain.Receipt{}, uncertainError{e}
	}
	if !status.Successful {
		return domain.Receipt{}, fmt.Errorf("交易验证未通过（状态码 %d），请刷新记录", status.Code)
	}
	var r domain.Receipt
	e = json.Unmarshal(tx.Result(), &r)
	if e != nil {
		return r, uncertainError{e}
	}
	return r, nil
}
func (f *FabricChain) Events(ctx context.Context, start uint64, fn func(uint64, domain.Receipt) error) error {
	blocks, e := f.networks[0].BlockEvents(ctx, client.WithStartBlock(start))
	if e != nil {
		return e
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case b, ok := <-blocks:
			if !ok {
				return errors.New("block stream disconnected")
			}
			if b.Data == nil || b.Metadata == nil {
				return errors.New("invalid block")
			}
			var flags []byte
			if len(b.Metadata.Metadata) > int(common.BlockMetadataIndex_TRANSACTIONS_FILTER) {
				flags = b.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER]
			}
			for index, raw := range b.Data.Data {
				if index >= len(flags) || flags[index] != 0 {
					continue
				}
				envelope := &common.Envelope{}
				if proto.Unmarshal(raw, envelope) != nil {
					continue
				}
				payload := &common.Payload{}
				if proto.Unmarshal(envelope.Payload, payload) != nil {
					continue
				}
				t := &peer.Transaction{}
				if proto.Unmarshal(payload.Data, t) != nil {
					continue
				}
				for _, action := range t.Actions {
					cap := &peer.ChaincodeActionPayload{}
					if proto.Unmarshal(action.Payload, cap) != nil || cap.Action == nil {
						continue
					}
					prp := &peer.ProposalResponsePayload{}
					if proto.Unmarshal(cap.Action.ProposalResponsePayload, prp) != nil {
						continue
					}
					ca := &peer.ChaincodeAction{}
					if proto.Unmarshal(prp.Extension, ca) != nil {
						continue
					}
					ev := &peer.ChaincodeEvent{}
					if proto.Unmarshal(ca.Events, ev) != nil || ev.ChaincodeId != env("CHAINCODE_NAME", "campusreturn") || ev.EventName != "BusinessCommitted" {
						continue
					}
					var receipt domain.Receipt
					if e = json.Unmarshal(ev.Payload, &receipt); e != nil {
						return e
					}
					if e = fn(b.Header.Number, receipt); e != nil {
						return e
					}
				}
			}
			if e = fn(b.Header.Number, domain.Receipt{}); e != nil {
				return e
			}
		}
	}
}
