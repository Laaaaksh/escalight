package db

import "database/sql"

type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	IsAdmin      bool
	SlackUserID  string
	CreatedAt    string
}

type PushSubscription struct {
	ID        string
	UserID    string
	Endpoint  string
	P256dh    string
	Auth      string
	CreatedAt string
}

type EscalationPolicy struct {
	ID          string
	Name        string
	Description string
	Repeat      int
	CreatedAt   string
}

type EscalationStep struct {
	ID          string
	PolicyID    string
	StepOrder   int
	WaitMinutes int
	Targets     []EscalationStepTarget
}

type EscalationStepTarget struct {
	ID         string
	StepID     string
	TargetType string // "user" | "schedule"
	TargetID   string
	ViaEmail   bool
	ViaPush    bool
	ViaSlack   bool
	ViaDiscord bool
}

type Schedule struct {
	ID        string
	Name      string
	Timezone  string
	CreatedAt string
	Rotation  *ScheduleRotation
}

type ScheduleRotation struct {
	ID           string
	ScheduleID   string
	RotationType string // "daily" | "weekly"
	HandoffTime  string // "HH:MM"
	StartAt      string // RFC3339
	UserOrder    []string
}

type ScheduleOverride struct {
	ID         string
	ScheduleID string
	UserID     string
	StartAt    string
	EndAt      string
	CreatedAt  string
}

type Service struct {
	ID                 string
	Name               string
	EscalationPolicyID string
	WebhookKey         string
	CreatedAt          string
}

type Incident struct {
	ID               string
	ServiceID        string
	Title            string
	Description      string
	Source           string
	Fingerprint      string
	Status           string // "triggered" | "acknowledged" | "resolved"
	CurrentStep      int
	RepeatCount      int
	NextEscalationAt sql.NullString
	AcknowledgedBy   sql.NullString
	AcknowledgedAt   sql.NullString
	ResolvedBy       sql.NullString
	ResolvedAt       sql.NullString
	CreatedAt        string
}

type IncidentEvent struct {
	ID         string
	IncidentID string
	EventType  string
	Actor      string
	Detail     string
	CreatedAt  string
}

var ErrNotFound = sql.ErrNoRows
