package httpapi

import (
	"net/http"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
)

// userBody is Seerr's User as the frontend reads it from auth/me and from a
// login (src/hooks/useUser.ts at the commit pinned in web/UPSTREAM.md).
// Everything but the account's own fields is fixed: there is one kind of
// user here, with no requests and no permission bits.
type userBody struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	DisplayName string   `json:"displayName"`
	Avatar      string   `json:"avatar"`
	UserType    int      `json:"userType"`
	Permissions int      `json:"permissions"`
	Warnings    []string `json:"warnings"`
	// RequestCount stays 0: requests are not part of this app.
	RequestCount int              `json:"requestCount"`
	CreatedAt    string           `json:"createdAt"`
	UpdatedAt    string           `json:"updatedAt"`
	Settings     userSettingsBody `json:"settings"`
}

type userSettingsBody struct {
	Locale           string `json:"locale"`
	DiscoverRegion   string `json:"discoverRegion"`
	StreamingRegion  string `json:"streamingRegion"`
	OriginalLanguage string `json:"originalLanguage"`
	// NotificationTypes is always the empty object; the frontend iterates it.
	NotificationTypes map[string]int `json:"notificationTypes"`
}

const (
	// userTypeLocal is Seerr's UserType.LOCAL. The Plex value hides the
	// watchlist stars on every title.
	userTypeLocal = 2
	// userAvatarPath is served from web/public. It must not be empty: an
	// empty src breaks the image component on every page.
	userAvatarPath = "/os_icon.svg"
	// userTimestampLayout is how a JavaScript Date serialises, which is what
	// Seerr's own server sends.
	userTimestampLayout = "2006-01-02T15:04:05.000Z07:00"
)

func newUserBody(user auth.User) userBody {
	return userBody{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		Avatar:       userAvatarPath,
		UserType:     userTypeLocal,
		Permissions:  0,
		Warnings:     []string{},
		RequestCount: 0,
		CreatedAt:    user.CreatedAt.UTC().Format(userTimestampLayout),
		UpdatedAt:    user.UpdatedAt.UTC().Format(userTimestampLayout),
		Settings: userSettingsBody{
			Locale:            "pt-BR",
			DiscoverRegion:    "BR",
			StreamingRegion:   "BR",
			OriginalLanguage:  "",
			NotificationTypes: map[string]int{},
		},
	}
}

func writeUser(w http.ResponseWriter, user auth.User) {
	writeJSONBody(w, newUserBody(user))
}
