package models

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

var ValidRoles = []string{RoleUser, RoleAdmin}

func IsValidRole(role string) bool {
	for _, r := range ValidRoles {
		if r == role {
			return true
		}
	}
	return false
}

type User struct {
	Base
	Email        string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Role         string `gorm:"not null;default:user" json:"role"`
}

func (User) TableName() string { return "users" }
