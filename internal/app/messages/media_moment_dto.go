package messages

import "time"

type CreateUploadIntentRequestDTO struct {
	ContentType    string `json:"contentType"`
	Bytes          int64  `json:"bytes"`
	ChecksumSHA256 string `json:"checksumSha256"`
}
type UploadIntentResponseDTO struct {
	ID        string            `json:"id"`
	UploadURL string            `json:"uploadUrl"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

func (d UploadIntentResponseDTO) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID        string            `json:"id"`
		UploadURL string            `json:"uploadUrl"`
		Method    string            `json:"method"`
		Headers   map[string]string `json:"headers"`
		ExpiresAt time.Time         `json:"expiresAt"`
	}
	return marshalUTC(wire{d.ID, d.UploadURL, d.Method, d.Headers, d.ExpiresAt})
}

type MediaAssetResponseDTO struct {
	ID             string     `json:"id"`
	ContentType    string     `json:"contentType"`
	Bytes          int64      `json:"bytes"`
	State          string     `json:"state"`
	AvailableAt    *time.Time `json:"availableAt"`
	RetentionDueAt time.Time  `json:"retentionDueAt"`
}

type CreateMomentRequestDTO struct {
	MediaAssetID    string  `json:"mediaAssetId"`
	PublishConsent  bool    `json:"publishConsent"`
	ParticipationID *string `json:"participationId,omitempty"`
	ChallengeMode   bool    `json:"-"`
}

type CreateChallengeMomentRequestDTO struct {
	MediaAssetID   string `json:"mediaAssetId"`
	PublishConsent bool   `json:"publishConsent"`
}
type MomentResponseDTO struct {
	ID                 string        `json:"id"`
	Origin             string        `json:"origin"`
	ParticipationID    *string       `json:"participationId"`
	ImageURL           string        `json:"imageUrl"`
	ThumbnailURL       string        `json:"thumbnailUrl"`
	ShareImageURL      string        `json:"shareImageUrl"`
	ImageExpiresAt     *time.Time    `json:"imageExpiresAt"`
	PlaceName          *string       `json:"placeName"`
	AuthorName         string        `json:"authorName"`
	AuthorAvatarURL    *string       `json:"authorAvatarUrl,omitempty"`
	CapturedAt         time.Time     `json:"capturedAt"`
	PublicationStatus  string        `json:"publicationStatus"`
	ModerationStatus   string        `json:"moderationStatus"`
	PointsAwarded      int           `json:"pointsAwarded"`
	ModerationMessage  *string       `json:"moderationMessage,omitempty"`
	LikesCount         int           `json:"likesCount"`
	LikedByCurrentUser bool          `json:"likedByCurrentUser"`
	GroupID            *Uint64String `json:"groupId,omitempty"`
}

func (d MomentResponseDTO) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID                 string        `json:"id"`
		Origin             string        `json:"origin"`
		ParticipationID    *string       `json:"participationId"`
		ImageURL           string        `json:"imageUrl"`
		ThumbnailURL       string        `json:"thumbnailUrl"`
		ShareImageURL      string        `json:"shareImageUrl"`
		ImageExpiresAt     *time.Time    `json:"imageExpiresAt"`
		PlaceName          *string       `json:"placeName"`
		AuthorName         string        `json:"authorName"`
		AuthorAvatarURL    *string       `json:"authorAvatarUrl,omitempty"`
		CapturedAt         time.Time     `json:"capturedAt"`
		PublicationStatus  string        `json:"publicationStatus"`
		ModerationStatus   string        `json:"moderationStatus"`
		PointsAwarded      int           `json:"pointsAwarded"`
		ModerationMessage  *string       `json:"moderationMessage,omitempty"`
		LikesCount         int           `json:"likesCount"`
		LikedByCurrentUser bool          `json:"likedByCurrentUser"`
		GroupID            *Uint64String `json:"groupId,omitempty"`
	}
	return marshalUTC(
		wire{
			d.ID,
			d.Origin,
			d.ParticipationID,
			d.ImageURL,
			d.ThumbnailURL,
			d.ShareImageURL,
			d.ImageExpiresAt,
			d.PlaceName,
			d.AuthorName,
			d.AuthorAvatarURL,
			d.CapturedAt,
			d.PublicationStatus,
			d.ModerationStatus,
			d.PointsAwarded,
			d.ModerationMessage,
			d.LikesCount,
			d.LikedByCurrentUser,
			d.GroupID,
		},
	)
}

type MomentPageResponseDTO struct {
	Items      []MomentResponseDTO `json:"items"`
	NextCursor *string             `json:"nextCursor"`
}
type LikeResponseDTO struct {
	MomentID   string `json:"momentId"`
	Liked      bool   `json:"liked"`
	LikesCount int    `json:"likesCount"`
}

type ModerationRequestDTO struct {
	Action string `json:"action"`
}
type ModerationActivitySummaryDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ModerationMomentResponseDTO struct {
	MomentID          string                        `json:"momentId"`
	ImageURL          string                        `json:"imageUrl"`
	ImageExpiresAt    *time.Time                    `json:"imageExpiresAt"`
	CapturedAt        time.Time                     `json:"capturedAt"`
	ParticipantName   string                        `json:"participantName"`
	Activity          *ModerationActivitySummaryDTO `json:"activity"`
	PointsAwarded     int                           `json:"pointsAwarded"`
	PublicationStatus string                        `json:"publicationStatus"`
	ModerationStatus  string                        `json:"moderationStatus"`
	RewardStatus      string                        `json:"rewardStatus"`
	PhotoStatus       string                        `json:"photoStatus"`
	AvailableActions  []string                      `json:"availableActions"`
}

func (d ModerationMomentResponseDTO) MarshalJSON() ([]byte, error) {
	type wire struct {
		MomentID          string                        `json:"momentId"`
		ImageURL          string                        `json:"imageUrl"`
		ImageExpiresAt    *time.Time                    `json:"imageExpiresAt"`
		CapturedAt        time.Time                     `json:"capturedAt"`
		ParticipantName   string                        `json:"participantName"`
		Activity          *ModerationActivitySummaryDTO `json:"activity"`
		PointsAwarded     int                           `json:"pointsAwarded"`
		PublicationStatus string                        `json:"publicationStatus"`
		ModerationStatus  string                        `json:"moderationStatus"`
		RewardStatus      string                        `json:"rewardStatus"`
		PhotoStatus       string                        `json:"photoStatus"`
		AvailableActions  []string                      `json:"availableActions"`
	}
	return marshalUTC(wire(d))
}

type ModerationPaginationDTO struct {
	CurrentPage Uint64String `json:"currentPage"`
	HasNextPage bool         `json:"hasNextPage"`
	Limit       int          `json:"limit"`
}
type ModerationPageResponseDTO struct {
	Data       []ModerationMomentResponseDTO `json:"data"`
	Pagination ModerationPaginationDTO       `json:"pagination"`
}
type ModerationResponseDTO struct {
	MomentID          string `json:"momentId"`
	Action            string `json:"action"`
	PublicationStatus string `json:"publicationStatus"`
	ModerationStatus  string `json:"moderationStatus"`
	RewardStatus      string `json:"rewardStatus"`
	PhotoStatus       string `json:"photoStatus"`
	PointsAwarded     int    `json:"pointsAwarded"`
}

func (d ModerationResponseDTO) MarshalJSON() ([]byte, error) {
	type wire struct {
		MomentID          string `json:"momentId"`
		Action            string `json:"action"`
		PublicationStatus string `json:"publicationStatus"`
		ModerationStatus  string `json:"moderationStatus"`
		RewardStatus      string `json:"rewardStatus"`
		PhotoStatus       string `json:"photoStatus"`
		PointsAwarded     int    `json:"pointsAwarded"`
	}
	return marshalUTC(wire(d))
}
