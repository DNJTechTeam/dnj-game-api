package messages

import "time"

// UpdateNotificationPreferencesRequestDTO intentionally omits moment
// moderation: that category can never be disabled by the client.
type UpdateNotificationPreferencesRequestDTO struct {
	PointsEnabled       *bool `json:"pointsEnabled"`
	AnnouncementEnabled *bool `json:"announcementEnabled"`
}

type NotificationPreferencesResponseDTO struct {
	MomentModerationEnabled bool      `json:"momentModerationEnabled"`
	PointsEnabled           bool      `json:"pointsEnabled"`
	AnnouncementEnabled     bool      `json:"announcementEnabled"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

type NotificationResponseDTO struct {
	ID         string     `json:"id"`
	Category   string     `json:"category"`
	State      string     `json:"state"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	SourceType string     `json:"sourceType"`
	SourceID   *string    `json:"sourceId,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	ReadAt     *time.Time `json:"readAt,omitempty"`
}

type NotificationListResponseDTO struct {
	Data        []NotificationResponseDTO `json:"data"`
	Pagination  Pagination                `json:"pagination"`
	UnreadCount Uint64String              `json:"unreadCount"`
}

type ListNotificationsFilterDTO struct {
	PaginationFilter
}

// AdminSendNotificationRequestDTO is the only free-content notification
// input. TargetUserIds is optional; when empty the announcement broadcasts
// to every eligible DEFAULT user who has not opted out.
type AdminSendNotificationRequestDTO struct {
	Title         string         `json:"title"`
	Body          string         `json:"body"`
	TargetUserIds []Uint64String `json:"targetUserIds,omitempty"`
}

// AdminSendNotificationResponseDTO exposes only the aggregate recipient
// count — never the individual recipients of a bulk administrative send.
type AdminSendNotificationResponseDTO struct {
	RecipientCount Uint64String `json:"recipientCount"`
}
