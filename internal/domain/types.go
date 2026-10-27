package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// 账本类型只包含业务状态、匿名内部编号和摘要，不携带私密原文。
type Claim struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	EvidenceHash string `json:"evidenceHash"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
}
type Event struct {
	Action      string `json:"action"`
	ActorID     string `json:"actorId"`
	OperationID string `json:"operationId"`
	TxID        string `json:"txId"`
	At          string `json:"at"`
	ReasonHash  string `json:"reasonHash"`
}
type Item struct {
	ID           string  `json:"id"`
	ReporterID   string  `json:"reporterId"`
	ContentHash  string  `json:"contentHash"`
	AnswerHash   string  `json:"answerHash"`
	QuestionHash string  `json:"questionHash"`
	ImageHash    string  `json:"imageHash"`
	Status       string  `json:"status"`
	Visibility   string  `json:"visibility"`
	Version      uint64  `json:"version"`
	OwnerID      string  `json:"ownerId"`
	ClaimID      string  `json:"claimId"`
	ReceiptHash  string  `json:"receiptHash"`
	Claims       []Claim `json:"claims"`
	Events       []Event `json:"events"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}
type Payload struct {
	ContentHash  string `json:"contentHash,omitempty"`
	AnswerHash   string `json:"answerHash,omitempty"`
	QuestionHash string `json:"questionHash,omitempty"`
	ImageHash    string `json:"imageHash,omitempty"`
	ClaimID      string `json:"claimId,omitempty"`
	EvidenceHash string `json:"evidenceHash,omitempty"`
	ReasonHash   string `json:"reasonHash,omitempty"`
	ReceiptHash  string `json:"receiptHash,omitempty"`
}
type Command struct {
	ID              string  `json:"id"`
	Action          string  `json:"action"`
	ItemID          string  `json:"itemId"`
	ActorID         string  `json:"actorId"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	Data            Payload `json:"data"`
}
type Receipt struct {
	CommandHash string `json:"commandHash"`
	Item        Item   `json:"item"`
}

// Content 的字段顺序和时间格式构成 v1 摘要规范，历史版本保留相同结构。
type Content struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Location    string `json:"location"`
	FoundAt     string `json:"foundAt"`
	Question    string `json:"question"`
	UploadID    string `json:"uploadId,omitempty"`
	ImageHash   string `json:"imageHash,omitempty"`
}

func Hash(v string) string        { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func Digest(v any) string         { b, _ := json.Marshal(v); return Hash(string(b)) }
func StudentAction(a string) bool { return a == "CREATE" || a == "CLAIM" || a == "WITHDRAW" }
