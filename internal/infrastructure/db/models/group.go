package models

type Group struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"not null;uniqueIndex"`
}

func (Group) TableName() string {
	return "groups"
}
