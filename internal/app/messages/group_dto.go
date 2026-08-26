package messages

type GroupSummaryDTO struct {
	ID        Uint64String `json:"id"`
	GroupName string       `json:"groupName"`
}

type CreateGroupRequestDTO struct {
	Name string `json:"name" binding:"required"`
}

type UpdateUserGroupRequestDTO struct {
	GroupID   uint64 `json:"groupId"`
	GroupName string `json:"groupName"`
}
