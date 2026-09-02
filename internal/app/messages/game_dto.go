package messages

import "time"

type GameResponseDTO struct {
	ID           string                  `json:"id"`
	Space        *PublicSpaceResponseDTO `json:"space"`
	Slug         string                  `json:"slug"`
	Name         string                  `json:"name"`
	Description  *string                 `json:"description"`
	StartsAt     *time.Time              `json:"startsAt"`
	EndsAt       *time.Time              `json:"endsAt"`
	AllowsMoment bool                    `json:"allowsMoment"`
	State        *string                 `json:"state"`
}

type ListGamesFilterDTO struct{ PaginationFilter }

type RankingScope string

const (
	RankingScopeIndividual RankingScope = "individual"
	RankingScopeGroups     RankingScope = "groups"
)

type IndividualRankingResponseDTO struct {
	ID        Uint64String `json:"id"`
	Name      string       `json:"name"`
	GroupName *string      `json:"groupName"`
	Points    int          `json:"points"`
	Position  uint64       `json:"position"`
}

type GroupRankingResponseDTO struct {
	ID       Uint64String `json:"id"`
	Name     string       `json:"name"`
	Members  int          `json:"members"`
	Points   int          `json:"points"`
	Position uint64       `json:"position"`
}

type RankingResponseDTO struct {
	Data        any        `json:"data"`
	Pagination  Pagination `json:"pagination"`
	GeneratedAt time.Time  `json:"generatedAt"`
}

type PointEntryResponseDTO struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Points    int       `json:"points"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"createdAt"`
}

type GameCurrentResponseDTO struct {
	GroupID           *Uint64String `json:"groupId"`
	RankPosition      uint64        `json:"rankPosition"`
	GroupRankPosition *uint64       `json:"groupRankPosition"`
	Points            int           `json:"points"`
}

type GameOverviewResponseDTO struct {
	Individual   []IndividualRankingResponseDTO `json:"individual"`
	Groups       []GroupRankingResponseDTO      `json:"groups"`
	PointEntries []PointEntryResponseDTO        `json:"pointEntries"`
	Current      GameCurrentResponseDTO         `json:"current"`
}

type CreateRunRequestDTO struct {
	GameID string `json:"gameId"`
}

type CreateManagerGameRequestDTO struct {
	Name string `json:"name"`
}

type UpdateManagerGameRequestDTO struct {
	Name string `json:"name"`
}

type RunParticipantResponseDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CheckedInAt   time.Time `json:"checkedInAt"`
	Result        *string   `json:"result"`
	PointsAwarded int       `json:"pointsAwarded"`
}

type RunGameResponseDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ManagerRunResponseDTO struct {
	ID           string                      `json:"id"`
	Game         RunGameResponseDTO          `json:"game"`
	Status       string                      `json:"status"`
	StartedAt    *time.Time                  `json:"startedAt"`
	EndedAt      *time.Time                  `json:"endedAt"`
	Participants []RunParticipantResponseDTO `json:"participants"`
}

type ManagerGameResponseDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Points struct {
		First         int `json:"first"`
		Second        int `json:"second"`
		Third         int `json:"third"`
		Participation int `json:"participation"`
	} `json:"points"`
}

type ManagerDashboardRunResponseDTO struct {
	ID           string                      `json:"id"`
	GameID       string                      `json:"gameId"`
	GameName     string                      `json:"gameName"`
	Status       string                      `json:"status"`
	StartedAt    *time.Time                  `json:"startedAt"`
	EndedAt      *time.Time                  `json:"endedAt"`
	Participants []RunParticipantResponseDTO `json:"participants"`
}

type ManagerGameOverviewActionsDTO struct {
	Games []ManagerGameResponseDTO        `json:"games"`
	Run   *ManagerDashboardRunResponseDTO `json:"run"`
}

type ManagerSpaceItemResponseDTO struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	StartsAt    *time.Time `json:"startsAt"`
	StartedAt   *time.Time `json:"startedAt"`
	Status      string     `json:"status"`
	FlexMinutes int        `json:"flexMinutes"`
	SpaceName   string     `json:"spaceName,omitempty"`
}

type ManagerSpaceOverviewDTO struct {
	Current  *ManagerSpaceItemResponseDTO  `json:"current,omitempty"`
	Upcoming []ManagerSpaceItemResponseDTO `json:"upcoming"`
}

type ManagerGameOverviewResponseDTO struct {
	Scope   string                        `json:"scope"`
	Actions ManagerGameOverviewActionsDTO `json:"actions"`
	Space   *ManagerSpaceOverviewDTO      `json:"space,omitempty"`
}

type QRResponseDTO struct {
	RunID     string    `json:"runId"`
	QRID      string    `json:"qrId"`
	QRToken   string    `json:"qrToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type QRValidateRequestDTO struct {
	QRToken        string `json:"qrToken"`
	IdempotencyKey string `json:"-"`
}

type NamedGameReferenceDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ParticipationResponseDTO struct {
	ID             string                 `json:"id"`
	Activity       NamedGameReferenceDTO  `json:"activity"`
	Place          *NamedGameReferenceDTO `json:"place"`
	CheckedInAt    time.Time              `json:"checkedInAt"`
	Status         string                 `json:"status"`
	CanShareMoment bool                   `json:"canShareMoment"`
	CheckInPoints  int                    `json:"checkInPoints"`
	NewTotalPoints *int                   `json:"newTotalPoints,omitempty"`
}

type ParticipationEnvelopeDTO struct {
	Participation ParticipationResponseDTO `json:"participation"`
	Action        string                   `json:"action,omitempty"`
	PointsAwarded int                      `json:"pointsAwarded,omitempty"`
}

type ParticipantRunResponseDTO struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	GameName  string     `json:"gameName"`
	StartedAt *time.Time `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt"`
	Result    *string    `json:"result,omitempty"`
	Points    *int       `json:"points,omitempty"`
}

type ParticipantRunEnvelopeDTO struct {
	Run ParticipantRunResponseDTO `json:"run"`
}

type RunResultRequestDTO struct {
	ParticipantID string `json:"participantId"`
	Result        string `json:"result"`
}

type FinalizeRunResultsRequestDTO struct {
	Results []RunResultRequestDTO `json:"results"`
}
