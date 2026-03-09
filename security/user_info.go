package security

import (
	"context"

	"github.com/guregu/null/v6"
)

// UserInfoLoader retrieves extended user information for the current session.
// Used to populate user profile data, preferences, or other session-specific details.
type UserInfoLoader interface {
	// LoadUserInfo retrieves detailed user information based on the Principal and parameters.
	LoadUserInfo(ctx context.Context, principal *Principal, params map[string]any) (*UserInfo, error)
}

type Gender string

const (
	GenderMale    Gender = "male"
	GenderFemale  Gender = "female"
	GenderUnknown Gender = "unknown"
)

type UserMenuType string

const (
	UserMenuTypeDirectory UserMenuType = "directory"
	UserMenuTypeMenu      UserMenuType = "menu"
	UserMenuTypeView      UserMenuType = "view"
	UserMenuTypeDashboard UserMenuType = "dashboard"
	UserMenuTypeReport    UserMenuType = "report"
)

type UserMenu struct {
	Type     UserMenuType   `json:"type"`
	Path     string         `json:"path"`
	Name     string         `json:"name"`
	Icon     null.String    `json:"icon"`
	Meta     map[string]any `json:"metadata,omitempty"`
	Children []UserMenu     `json:"children,omitempty"`
}

type UserInfo struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Gender     Gender      `json:"gender"`
	Avatar     null.String `json:"avatar"`
	PermTokens []string    `json:"permTokens"`
	Menus      []UserMenu  `json:"menus"`
	Details    any         `json:"details,omitempty"`
}
