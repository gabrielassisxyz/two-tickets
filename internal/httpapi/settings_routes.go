package httpapi

import "net/http"

const routePublicSettings = "/api/v1/settings/public"

// publicSettingsBody is Seerr's PublicSettingsResponse, field for field, at
// the upstream commit pinned in web/UPSTREAM.md. The frontend draws nothing
// until this answers, and it reads every field without a fallback, so a
// field left out here is a field the page reads as undefined.
type publicSettingsBody struct {
	JellyfinHost              string `json:"jellyfinHost"`
	JellyfinExternalHost      string `json:"jellyfinExternalHost"`
	JellyfinServerName        string `json:"jellyfinServerName"`
	JellyfinForgotPasswordURL string `json:"jellyfinForgotPasswordUrl"`
	Initialized               bool   `json:"initialized"`
	ApplicationTitle          string `json:"applicationTitle"`
	ApplicationURL            string `json:"applicationUrl"`
	HideAvailable             bool   `json:"hideAvailable"`
	HideBlocklisted           bool   `json:"hideBlocklisted"`
	HideRequested             bool   `json:"hideRequested"`
	LocalLogin                bool   `json:"localLogin"`
	MediaServerLogin          bool   `json:"mediaServerLogin"`
	Movie4kEnabled            bool   `json:"movie4kEnabled"`
	Series4kEnabled           bool   `json:"series4kEnabled"`
	DiscoverRegion            string `json:"discoverRegion"`
	StreamingRegion           string `json:"streamingRegion"`
	OriginalLanguage          string `json:"originalLanguage"`
	MediaServerType           int    `json:"mediaServerType"`
	PartialRequestsEnabled    bool   `json:"partialRequestsEnabled"`
	EnableSpecialEpisodes     bool   `json:"enableSpecialEpisodes"`
	CacheImages               bool   `json:"cacheImages"`
	VapidPublic               string `json:"vapidPublic"`
	EnablePushRegistration    bool   `json:"enablePushRegistration"`
	Locale                    string `json:"locale"`
	EmailEnabled              bool   `json:"emailEnabled"`
	NewPlexLogin              bool   `json:"newPlexLogin"`
	YoutubeURL                string `json:"youtubeUrl"`
	VersionCheck              bool   `json:"versionCheck"`
	PlexClientIdentifier      string `json:"plexClientIdentifier"`
}

// mediaServerTypeNotConfigured is Seerr's MediaServerType.NOT_CONFIGURED.
const mediaServerTypeNotConfigured = 4

// publicSettingsFixture sets every field explicitly. None is left to the
// frontend's own defaults in SettingsContext, which upstream may change at
// any time; testdata/contract/settings-public.json pins the same values.
var publicSettingsFixture = publicSettingsBody{
	// The Jellyfin fields are optional upstream and only read by the
	// Jellyfin login, which a NOT_CONFIGURED media server never shows.
	JellyfinHost:              "",
	JellyfinExternalHost:      "",
	JellyfinServerName:        "",
	JellyfinForgotPasswordURL: "",
	// Anything but true sends every page to /setup.
	Initialized:      true,
	ApplicationTitle: "Two Tickets",
	ApplicationURL:   "",
	HideAvailable:    false,
	HideBlocklisted:  false,
	HideRequested:    false,
	// Only the local username and password form.
	LocalLogin:       true,
	MediaServerLogin: false,
	Movie4kEnabled:   false,
	Series4kEnabled:  false,
	DiscoverRegion:   "BR",
	StreamingRegion:  "BR",
	OriginalLanguage: "",
	MediaServerType:  mediaServerTypeNotConfigured,
	// Requests are removed; the frontend's own default is true.
	PartialRequestsEnabled: false,
	// Season 0 is listed.
	EnableSpecialEpisodes: true,
	// Posters load straight from image.tmdb.org.
	CacheImages:            false,
	VapidPublic:            "",
	EnablePushRegistration: false,
	Locale:                 "pt-BR",
	EmailEnabled:           false,
	NewPlexLogin:           false,
	YoutubeURL:             "",
	VersionCheck:           false,
	PlexClientIdentifier:   "",
}

func registerSettingsRoutes(mux *http.ServeMux) {
	mux.Handle(routePublicSettings, allowRead(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, publicSettingsFixture)
	})))
}
