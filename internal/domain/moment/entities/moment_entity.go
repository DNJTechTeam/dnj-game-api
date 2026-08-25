package entities

import "time"

type Origin string
type PublicationStatus string
type ModerationStatus string
type RewardStatus string

const (
	OriginFree          Origin            = "free"
	OriginChallenge     Origin            = "challenge"
	PublicationPrivate  PublicationStatus = "private"
	PublicationPublic   PublicationStatus = "public"
	ModerationApproved  ModerationStatus  = "approved"
	ModerationRejected  ModerationStatus  = "rejected"
	RewardNotApplicable RewardStatus      = "not_applicable"
	RewardAwarded       RewardStatus      = "awarded"
	RewardDenied        RewardStatus      = "denied"
	RewardReversed      RewardStatus      = "reversed"
)

type Moment struct {
	ID                  string
	UserID              uint64
	ParticipationID     *string
	ActivityID          *string
	MediaAssetID        string
	Origin              Origin
	PublicationStatus   PublicationStatus
	ModerationStatus    ModerationStatus
	RewardStatus        RewardStatus
	PointsAwarded       int
	CapturedAt          time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	AuthorName          string
	GroupID             *uint64
	ActivityName        *string
	PlaceName           *string
	AssetAvailable      bool
	AssetRetentionDueAt time.Time
	LikesCount          int
	LikedByCurrentUser  bool
	AuthorEligible      bool
}

type ModerationDecision struct {
	ID             string
	MomentID       string
	ActorUserID    uint64
	Action         string
	IdempotencyKey string
	CreatedAt      time.Time
}
type Cursor struct {
	CapturedAt time.Time
	ID         string
}
type Page struct {
	Items   []Moment
	HasNext bool
}
type ModerationPage struct {
	Items   []Moment
	HasNext bool
}
