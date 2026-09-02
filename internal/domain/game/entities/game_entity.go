package entities

import (
	"time"

	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
)

type RunStatus string

const (
	RunStatusDraft     RunStatus = "draft"
	RunStatusActive    RunStatus = "active"
	RunStatusPaused    RunStatus = "paused"
	RunStatusResults   RunStatus = "results"
	RunStatusCompleted RunStatus = "completed"
	RunStatusCancelled RunStatus = "cancelled"
)

func (s RunStatus) IsOpen() bool {
	return s == RunStatusDraft || s == RunStatusActive || s == RunStatusPaused || s == RunStatusResults
}

type Result string

const (
	ResultFirst         Result = "first"
	ResultSecond        Result = "second"
	ResultThird         Result = "third"
	ResultParticipation Result = "participation"
)

type PointRules struct {
	First         int `json:"first"`
	Second        int `json:"second"`
	Third         int `json:"third"`
	Participation int `json:"participation"`
}

func DefaultPointRules() PointRules {
	return PointRules{First: 50, Second: 30, Third: 20, Participation: 10}
}

func (r PointRules) PointsFor(result Result) int {
	switch result {
	case ResultFirst:
		return r.First
	case ResultSecond:
		return r.Second
	case ResultThird:
		return r.Third
	default:
		return r.Participation
	}
}

type ActivityRun struct {
	ID           string
	ActivityID   string
	StartedBy    uint64
	Status       RunStatus
	PointRules   PointRules
	StartedAt    *time.Time
	EndedAt      *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Activity     *activityEntities.Activity
	Participants []RunParticipant
}

type RunParticipant struct {
	ID              string
	ActivityRunID   string
	UserID          uint64
	ParticipationID string
	Name            string
	CheckedInAt     time.Time
	Result          *Result
	PointsAwarded   int
	CreatedAt       time.Time
}

type ParticipationStatus string

const (
	ParticipationStatusActive    ParticipationStatus = "active"
	ParticipationStatusCompleted ParticipationStatus = "completed"
	ParticipationStatusCancelled ParticipationStatus = "cancelled"
)

type Participation struct {
	ID             string
	UserID         uint64
	ActivityID     string
	ActivityRunID  string
	QRCodeID       string
	CheckedInAt    time.Time
	Status         ParticipationStatus
	CanShareMoment bool
	CheckInPoints  int
	CreatedAt      time.Time
	ActivityName   string
	SpaceID        *string
	SpaceName      *string
}

type QRCodeStatus string

const (
	QRCodeStatusActive   QRCodeStatus = "active"
	QRCodeStatusDisabled QRCodeStatus = "disabled"
)

type QRCode struct {
	ID            string
	ActivityID    string
	ActivityRunID string
	AllowsMoment  bool
	TokenHash     string
	ExpiresAt     time.Time
	Status        QRCodeStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PointEntry struct {
	ID              string
	UserID          uint64
	ActivityID      string
	ActivityName    string
	ActivityRunID   *string
	ParticipationID *string
	MomentID        *string
	Origin          string
	Reason          string
	Delta           int
	CreatedAt       time.Time
}

type PointBalanceMismatch struct {
	UserID             uint64
	LedgerPoints       int64
	MaterializedPoints int64
}

type ManagerOperation struct {
	ID              string
	ActorUserID     uint64
	IdempotencyKey  string
	Operation       string
	ActivityID      string
	ActivityRunID   *string
	IntentHash      string
	ResultRef       *string
	ResultStatus    *string
	ResultStartedAt *time.Time
	ResultEndedAt   *time.Time
	ResultExpiresAt *time.Time
	HTTPStatus      int
	CreatedAt       time.Time
}

type IndividualRanking struct {
	UserID    uint64
	Name      string
	GroupName *string
	Points    int
	Position  uint64
}

type GroupRanking struct {
	GroupID  uint64
	Name     string
	Members  int
	Points   int
	Position uint64
}
