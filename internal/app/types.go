package app

import "campusreturn/internal/domain"

type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	PasswordHash  string `json:"passwordHash,omitempty"`
	RecoveryHash  string `json:"recoveryHash,omitempty"`
	ForcePassword bool   `json:"forcePassword"`
	CreatedAt     string `json:"createdAt"`
}

func (u User) Public() User { u.PasswordHash = ""; u.RecoveryHash = ""; return u }
func (u User) Admin() bool  { return u.Role == "ADMIN" || u.Role == "SUPER_ADMIN" }

type Session struct {
	UserID    string `json:"userId"`
	ExpiresAt string `json:"expiresAt"`
}
type ContentVersion struct {
	ItemID    string         `json:"itemId"`
	Hash      string         `json:"hash"`
	Content   domain.Content `json:"content"`
	CreatedAt string         `json:"createdAt"`
}
type ItemView struct {
	domain.Item
	Content         domain.Content  `json:"content"`
	Integrity       bool            `json:"integrity"`
	ReporterName    string          `json:"reporterName"`
	OwnerName       string          `json:"ownerName"`
	MyClaimFeedback []ClaimFeedback `json:"myClaimFeedback,omitempty"`
}
type Secret struct {
	Salt string `json:"salt"`
	Text string `json:"text"`
}
type Upload struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	Hash   string `json:"hash"`
	MIME   string `json:"mime"`
	Path   string `json:"path,omitempty"`
	Size   int64  `json:"size"`
}
type Notice struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	ItemID     string `json:"itemId"`
	Title      string `json:"title"`
	Read       bool   `json:"read"`
	At         string `json:"at"`
	Action     string `json:"action,omitempty"`
	ItemName   string `json:"itemName,omitempty"`
	Body       string `json:"body,omitempty"`
	Reason     string `json:"reason,omitempty"`
	ClaimID    string `json:"claimId,omitempty"`
	RelatedID  string `json:"relatedId,omitempty"`
	Result     string `json:"result,omitempty"`
	Process    bool   `json:"process,omitempty"`
	Suppressed bool   `json:"suppressed,omitempty"`
}

// 处理结果是应用层投影，不改变链上状态；原因按认领归属控制读取。
type ClaimFeedback struct {
	ID              string `json:"id"`
	ItemID          string `json:"itemId"`
	ClaimID         string `json:"claimId"`
	UserID          string `json:"userId"`
	Status          string `json:"status"`
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	ReasonAvailable bool   `json:"reasonAvailable"`
	At              string `json:"at"`
}
type Audit struct {
	ID       string `json:"id"`
	ActorID  string `json:"actorId"`
	Action   string `json:"action"`
	TargetID string `json:"targetId"`
	Reason   string `json:"reason"`
	At       string `json:"at"`
}
type Report struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	ItemID     string `json:"itemId"`
	ClaimID    string `json:"claimId,omitempty"`
	Kind       string `json:"kind"`
	Reason     string `json:"reason"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	At         string `json:"at"`
}
type Evaluation struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	ItemID string `json:"itemId"`
	Stars  int    `json:"stars"`
	Text   string `json:"text"`
	Status string `json:"status"`
	At     string `json:"at"`
}
type Point struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	ItemID string `json:"itemId"`
	Pair   string `json:"pair"`
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
	At     string `json:"at"`
}
type Appointment struct {
	ItemID   string `json:"itemId"`
	UserID   string `json:"userId"`
	At       string `json:"at"`
	Location string `json:"location"`
	Status   string `json:"status"`
}
type Handover struct {
	ItemID    string `json:"itemId"`
	ClaimID   string `json:"claimId"`
	UserID    string `json:"userId"`
	Hash      string `json:"hash"`
	ExpiresAt string `json:"expiresAt"`
	Attempts  int    `json:"attempts"`
	Used      bool   `json:"used"`
}
type Operation struct {
	ID          string          `json:"id"`
	UserID      string          `json:"userId"`
	ItemID      string          `json:"itemId"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	TxID        string          `json:"txId,omitempty"`
	At          string          `json:"at"`
	UpdatedAt   string          `json:"updatedAt"`
	Command     domain.Command  `json:"command"`
	RequestHash string          `json:"requestHash"`
	Secret      string          `json:"secret,omitempty"`
	Content     *domain.Content `json:"content,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	ReportID    string          `json:"reportId,omitempty"`
	Endorsed    []byte          `json:"endorsed,omitempty"`
}

func (o Operation) Public() Operation { o.Secret = ""; o.Endorsed = nil; return o }
