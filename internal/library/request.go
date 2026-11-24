package library

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// libraryBodyLimitBytes fits the longest review the schema allows, 10000
// characters of up to four bytes each, with room for the other fields.
const libraryBodyLimitBytes = 64 << 10

// decodeLibraryBody reads one JSON object into dest. strict refuses unknown
// fields: a library body that carries anything else, a user id included, is
// rejected rather than silently ignored. The Seerr routes are lenient, as
// Seerr's own server is.
func decodeLibraryBody(w http.ResponseWriter, r *http.Request, dest any, strict bool) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, libraryBodyLimitBytes))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(dest); err != nil {
		libraryBadRequest(w, "body must be a JSON object of the documented fields: "+libraryDecodeProblem(err))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		libraryBadRequest(w, "body must hold exactly one JSON object")
		return false
	}
	return true
}

func libraryDecodeProblem(err error) string {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return "body too large"
	}
	return err.Error()
}

// libraryTitle is a title as the library stores it.
type libraryTitle struct {
	MediaType string
	TMDBID    int
}

func libraryMediaTypeKnown(mediaType string) bool {
	return mediaType == "movie" || mediaType == "tv"
}

// libraryTarget is what a watch or a rating applies to: a movie, a whole
// show, or one season. Season is set exactly when Scope is "season".
type libraryTarget struct {
	libraryTitle
	Scope  string
	Season *int
}

// libraryTargetBody is the part of a watch or rating body naming the target.
type libraryTargetBody struct {
	MediaType    string `json:"mediaType"`
	TMDBID       int    `json:"tmdbId"`
	Scope        string `json:"scope"`
	SeasonNumber *int   `json:"seasonNumber"`
}

// target validates the body against the rules the schema's CHECKs enforce,
// so a client mistake answers 400 instead of failing a constraint.
func (b libraryTargetBody) target() (libraryTarget, string) {
	title, problem := libraryTitleOf(b.MediaType, b.TMDBID)
	if problem != "" {
		return libraryTarget{}, problem
	}
	target := libraryTarget{libraryTitle: title, Scope: b.Scope}
	switch {
	case b.MediaType == "movie" && b.Scope != "movie":
		return libraryTarget{}, "a movie's scope must be movie"
	case b.MediaType == "tv" && b.Scope != "show" && b.Scope != "season":
		return libraryTarget{}, "a series' scope must be show or season"
	case b.Scope == "season" && b.SeasonNumber == nil:
		return libraryTarget{}, "scope season needs seasonNumber"
	case b.Scope != "season" && b.SeasonNumber != nil:
		return libraryTarget{}, "seasonNumber is only for scope season"
	case b.SeasonNumber != nil && *b.SeasonNumber < 0:
		return libraryTarget{}, "seasonNumber must not be negative"
	}
	target.Season = b.SeasonNumber
	return target, ""
}

// decodeLibraryTarget reads a body that is only a target.
func decodeLibraryTarget(w http.ResponseWriter, r *http.Request) (libraryTarget, bool) {
	var body libraryTargetBody
	if !decodeLibraryBody(w, r, &body, true) {
		return libraryTarget{}, false
	}
	target, problem := body.target()
	if problem != "" {
		libraryBadRequest(w, problem)
		return libraryTarget{}, false
	}
	return target, true
}

// libraryPathTitle reads {mediaType} and {tmdbId} from the path.
func libraryPathTitle(w http.ResponseWriter, r *http.Request) (libraryTitle, bool) {
	mediaType := r.PathValue("mediaType")
	if !libraryMediaTypeKnown(mediaType) {
		libraryBadRequest(w, "mediaType must be movie or tv")
		return libraryTitle{}, false
	}
	id, ok := libraryPositiveInt(r.PathValue("tmdbId"))
	if !ok {
		libraryBadRequest(w, "tmdbId must be a positive integer")
		return libraryTitle{}, false
	}
	return libraryTitle{MediaType: mediaType, TMDBID: int(id)}, true
}

func libraryPositiveInt(raw string) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 || value > 1<<31-1 {
		return 0, false
	}
	return value, true
}
