package catalog

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The query schemas of /discover/movies and /discover/tv, ported from
// MovieApiQuerySchema and TvApiQuerySchema in upstream's
// server/routes/discover.ts, and the parameters upstream's getDiscoverMovies
// and getDiscoverTv turn them into. Upstream coerces every field to a string
// and never rejects one, so bad input reaches TMDB and comes back as a 500;
// here every field is checked and bad input answers 400, the intentional
// normalisation the status matrix records.

const (
	discoverDateLayout = "2006-01-02"
	// discoverPastBound is what upstream sends for the missing end of a range
	// that only has an upper bound: TMDB needs both ends once one is set.
	discoverPastBound = "1900-01-01"
	// discoverFutureSpan is upstream's missing upper bound: 1.5 years past now.
	discoverFutureSpan = time.Duration(365*1.5*24) * time.Hour
	// discoverAllLanguages is the original-language value meaning no filter.
	discoverAllLanguages = "all"
)

var (
	discoverIDListPattern      = regexp.MustCompile(`^[1-9][0-9]*([,|][1-9][0-9]*)*$`)
	discoverCountPattern       = regexp.MustCompile(`^[0-9]+$`)
	discoverCountryPattern     = regexp.MustCompile(`^[A-Z]{2}$`)
	discoverStatusListPattern  = regexp.MustCompile(`^[0-5]([,|][0-5])*$`)
	discoverOriginalLangFormat = regexp.MustCompile(`^[a-z]{2}$`)
)

// discoverCommonSorts and the two lists after it are upstream's
// MovieSortOptionsIterable and TvSortOptionsIterable.
var discoverCommonSorts = []string{
	"popularity.desc", "popularity.asc",
	"vote_average.desc", "vote_average.asc",
	"vote_count.desc", "vote_count.asc",
}

var discoverMovieSorts = discoverSortSet(append([]string{
	"release_date.desc", "release_date.asc",
	"revenue.desc", "revenue.asc",
	"primary_release_date.desc", "primary_release_date.asc",
	"original_title.desc", "original_title.asc",
}, discoverCommonSorts...))

var discoverTVSorts = discoverSortSet(append([]string{
	"first_air_date.desc", "first_air_date.asc",
	"original_name.desc", "original_name.asc",
}, discoverCommonSorts...))

func discoverSortSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

// discoverFilter is one optional query field and the TMDB parameter it
// becomes. check returns the value to send, or false for invalid input.
type discoverFilter struct {
	query string
	param string
	check func(string) (string, bool)
}

// discoverMedia is what tells a movie discover from a series one.
type discoverMedia struct {
	path    string
	sorts   map[string]bool
	dateGte discoverFilter
	dateLte discoverFilter
	filters []discoverFilter
	fixed   map[string]string
}

var discoverMovieMedia = discoverMedia{
	path:    "/discover/movie",
	sorts:   discoverMovieSorts,
	dateGte: discoverFilter{"primaryReleaseDateGte", "primary_release_date.gte", discoverCheckDate},
	dateLte: discoverFilter{"primaryReleaseDateLte", "primary_release_date.lte", discoverCheckDate},
	filters: append([]discoverFilter{
		{"studio", "with_companies", discoverCheckIDList},
	}, discoverSharedFilters...),
	fixed: map[string]string{"include_adult": "false", "include_video": "true"},
}

var discoverTVMedia = discoverMedia{
	path:    "/discover/tv",
	sorts:   discoverTVSorts,
	dateGte: discoverFilter{"firstAirDateGte", "first_air_date.gte", discoverCheckDate},
	dateLte: discoverFilter{"firstAirDateLte", "first_air_date.lte", discoverCheckDate},
	filters: append([]discoverFilter{
		// Upstream passes Number(network), so only one network is possible.
		{"network", "with_networks", discoverCheckID},
		{"status", "with_status", discoverCheckPattern(discoverStatusListPattern)},
	}, discoverSharedFilters...),
	fixed: map[string]string{"include_null_first_air_dates": "false"},
}

// discoverSharedFilters are the fields both schemas send. certificationMode
// is not among them: the final upstream schemas omit it.
var discoverSharedFilters = []discoverFilter{
	{"genre", "with_genres", discoverCheckIDList},
	{"keywords", "with_keywords", discoverCheckIDList},
	{"excludeKeywords", "without_keywords", discoverCheckIDList},
	{"withRuntimeGte", "with_runtime.gte", discoverCheckPattern(discoverCountPattern)},
	{"withRuntimeLte", "with_runtime.lte", discoverCheckPattern(discoverCountPattern)},
	{"voteAverageGte", "vote_average.gte", discoverCheckScore},
	{"voteAverageLte", "vote_average.lte", discoverCheckScore},
	{"voteCountGte", "vote_count.gte", discoverCheckPattern(discoverCountPattern)},
	{"voteCountLte", "vote_count.lte", discoverCheckPattern(discoverCountPattern)},
	{"watchProviders", "with_watch_providers", discoverCheckIDList},
	{"watchRegion", "watch_region", discoverCheckPattern(discoverCountryPattern)},
	{"certification", "certification", discoverCheckCertification},
	{"certificationGte", "certification.gte", discoverCheckCertification},
	{"certificationLte", "certification.lte", discoverCheckCertification},
	{"certificationCountry", "certification_country", discoverCheckPattern(discoverCountryPattern)},
}

// discoverQueryValue reads one field the way zod's z.coerce.string() does:
// a repeated parameter arrives as an array and is joined with commas. An
// empty value counts as absent; TMDB reads an empty filter as no filter.
func discoverQueryValue(query url.Values, key string) (string, bool) {
	values := query[key]
	if len(values) == 0 {
		return "", false
	}
	joined := strings.Join(values, ",")
	return joined, joined != ""
}

// discoverParsePage reads the page every paginated route takes. The frontend
// always sends one; a request without it gets the first page.
func discoverParsePage(query url.Values) (string, bool) {
	raw, present := discoverQueryValue(query, "page")
	if !present {
		return "1", true
	}
	return discoverCheckID(raw)
}

// discoverBaseParams is what upstream's discover helpers send for every call
// a route makes on behalf of a user: the defaults of getDiscoverMovies or
// getDiscoverTv, the display locale and region, and the empty original
// language upstream's settings hold.
func discoverBaseParams(media discoverMedia, page string) map[string]string {
	params := map[string]string{
		"sort_by":                "popularity.desc",
		"page":                   page,
		"language":               tmdb.DefaultLanguage,
		"region":                 tmdb.DefaultRegion,
		"with_original_language": "",
	}
	for key, value := range media.fixed {
		params[key] = value
	}
	return params
}

// discoverParseQuery validates a /discover/movies or /discover/tv query and
// returns the TMDB parameters it becomes, or the reason it is refused.
//
// language has two meanings upstream. Here the display language is always
// pt-BR and the query's language is only ever the original-language filter.
func discoverParseQuery(media discoverMedia, query url.Values, now time.Time) (map[string]string, string) {
	page, ok := discoverParsePage(query)
	if !ok {
		return nil, "page must be a positive integer"
	}
	params := discoverBaseParams(media, page)
	if sort, present := discoverQueryValue(query, "sortBy"); present {
		if !media.sorts[sort] {
			return nil, "sortBy is not a sort order for this media type"
		}
		params["sort_by"] = sort
	}
	if problem := discoverApplyOriginalLanguage(params, query); problem != "" {
		return nil, problem
	}
	filters := append([]discoverFilter{media.dateGte, media.dateLte}, media.filters...)
	for _, filter := range filters {
		raw, present := discoverQueryValue(query, filter.query)
		if !present {
			continue
		}
		value, valid := filter.check(raw)
		if !valid {
			return nil, filter.query + " is not valid"
		}
		params[filter.param] = value
	}
	discoverFillDateBounds(params, media, now)
	return params, ""
}

func discoverApplyOriginalLanguage(params map[string]string, query url.Values) string {
	language, present := discoverQueryValue(query, "language")
	switch {
	case !present:
	case language == discoverAllLanguages:
		delete(params, "with_original_language")
	case discoverOriginalLangFormat.MatchString(language):
		params["with_original_language"] = language
	default:
		return "language must be an ISO 639-1 code or all"
	}
	return ""
}

// discoverFillDateBounds gives a one-sided date range its other end, exactly
// as upstream's TMDB client does.
func discoverFillDateBounds(params map[string]string, media discoverMedia, now time.Time) {
	_, hasGte := params[media.dateGte.param]
	_, hasLte := params[media.dateLte.param]
	switch {
	case hasGte && !hasLte:
		params[media.dateLte.param] = now.UTC().Add(discoverFutureSpan).Format(discoverDateLayout)
	case hasLte && !hasGte:
		params[media.dateGte.param] = discoverPastBound
	}
}

// discoverUpcomingParams is upstream's upcoming slider: a popularity-sorted
// discover from today, server clock, to the synthesised upper bound. Today
// is the server's local date, as upstream computes it.
func discoverUpcomingParams(media discoverMedia, page string, now time.Time) map[string]string {
	params := discoverBaseParams(media, page)
	params[media.dateGte.param] = now.Format(discoverDateLayout)
	discoverFillDateBounds(params, media, now)
	return params
}

func discoverCheckID(raw string) (string, bool) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 || strconv.Itoa(id) != raw {
		return "", false
	}
	return raw, true
}

func discoverCheckIDList(raw string) (string, bool) {
	return raw, discoverIDListPattern.MatchString(raw)
}

func discoverCheckPattern(pattern *regexp.Regexp) func(string) (string, bool) {
	return func(raw string) (string, bool) {
		return raw, pattern.MatchString(raw)
	}
}

// discoverCheckScore accepts a vote average, a number from 0 to 10.
func discoverCheckScore(raw string) (string, bool) {
	score, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(score) || score < 0 || score > 10 {
		return "", false
	}
	return raw, true
}

// discoverCheckCertification accepts any short printable rating; the labels
// differ per country ("L", "PG-13", "TV-MA", "MA 15+") and the separator
// between several is "|".
func discoverCheckCertification(raw string) (string, bool) {
	if len(raw) > 64 {
		return "", false
	}
	for _, r := range raw {
		if r < ' ' || r == 0x7f {
			return "", false
		}
	}
	return raw, true
}

// discoverCheckDate accepts an ISO date or an ISO timestamp and sends the
// date part, as upstream's new Date(x).toISOString().split('T')[0] does.
func discoverCheckDate(raw string) (string, bool) {
	if date, err := time.Parse(discoverDateLayout, raw); err == nil {
		return date.Format(discoverDateLayout), true
	}
	if stamp, err := time.Parse(time.RFC3339, raw); err == nil {
		return stamp.UTC().Format(discoverDateLayout), true
	}
	return "", false
}
