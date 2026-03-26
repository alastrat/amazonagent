package domain

import "time"

type TenantID string
type UserID string

type Tenant struct {
	ID        TenantID  `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Membership struct {
	TenantID TenantID `json:"tenant_id"`
	UserID   UserID   `json:"user_id"`
	Role     Role     `json:"role"`
}

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)
