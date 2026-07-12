package user

import (
	"encoding/json"
	"time"
)

const (
	RoleDefault = 0
	RoleAdmin   = 1
)

type User struct {
	ID          int64     `json:"id"`
	Username    *string   `json:"username"`
	UserAccount *string   `json:"userAccount"`
	AvatarURL   *string   `json:"avatarUrl"`
	Gender      *int8     `json:"gender"`
	Password    string    `json:"-"`
	Phone       *string   `json:"phone"`
	Email       *string   `json:"email"`
	UserStatus  int       `json:"userStatus"`
	UserRole    int       `json:"userRole"`
	PlanetCode  *string   `json:"planetCode"`
	Tags        *string   `json:"tags"`
	CreateTime  time.Time `json:"createTime"`
}

func (u *User) ParseTags() []string {
	if u.Tags == nil || *u.Tags == "" {
		return nil
	}
	var tags []string
	_ = json.Unmarshal([]byte(*u.Tags), &tags)
	return tags
}

type RegisterParams struct {
	UserAccount   string `json:"userAccount"   binding:"required,min=4,max=256"`
	UserPassword  string `json:"userPassword"  binding:"required,min=8"`
	CheckPassword string `json:"checkPassword" binding:"required,min=8,eqfield=UserPassword"`
	PlanetCode    string `json:"planetCode"    binding:"required,max=5"`
}

type LoginParams struct {
	UserAccount  string `json:"userAccount"  binding:"required,min=4"`
	UserPassword string `json:"userPassword" binding:"required,min=8"`
}