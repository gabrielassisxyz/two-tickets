package tmdb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func decodeFixture(t *testing.T, name string, dest any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
}

func TestFixtureMovieDecodes(t *testing.T) {
	var movie MovieDetails
	decodeFixture(t, "movie_550.json", &movie)
	if movie.ID != 550 || movie.Title != "Central do Brasil" {
		t.Fatalf("unexpected movie: %+v", movie)
	}
	if len(movie.Genres) != 1 || movie.Genres[0].Name != "Drama" {
		t.Fatalf("unexpected genres: %+v", movie.Genres)
	}
	if movie.Runtime == nil || *movie.Runtime != 113 {
		t.Fatalf("unexpected runtime: %+v", movie.Runtime)
	}
}

func TestFixtureTVDecodes(t *testing.T) {
	var show TVDetails
	decodeFixture(t, "tv_1399.json", &show)
	if show.ID != 1399 || show.Name != "Bacurau Nights" {
		t.Fatalf("unexpected show: %+v", show)
	}
	if show.NumberOfSeasons != 1 || show.NumberOfEpisodes != 6 {
		t.Fatalf("unexpected counts: %+v", show)
	}
}

func TestFixtureSeasonDecodes(t *testing.T) {
	var season SeasonDetails
	decodeFixture(t, "season_1.json", &season)
	if season.SeasonNumber != 1 || len(season.Episodes) != 2 {
		t.Fatalf("unexpected season: %+v", season)
	}
	if season.Episodes[0].Name != "Arrival" || season.Episodes[1].EpisodeNumber != 2 {
		t.Fatalf("unexpected episodes: %+v", season.Episodes)
	}
}

func TestFixtureDiscoverDecodes(t *testing.T) {
	var discover DiscoverMoviesResponse
	decodeFixture(t, "discover_movie.json", &discover)
	if discover.Page != 1 || len(discover.Results) != 2 {
		t.Fatalf("unexpected discover: %+v", discover)
	}
	if discover.Results[0].ID != 550 || discover.TotalResults != 200 {
		t.Fatalf("unexpected discover content: %+v", discover)
	}
}

func TestFixtureSearchMultiDecodes(t *testing.T) {
	var multi SearchMultiResponse
	decodeFixture(t, "search_multi.json", &multi)
	if len(multi.Results) != 2 {
		t.Fatalf("unexpected multi: %+v", multi)
	}
	if multi.Results[0].MediaType != "movie" || multi.Results[1].MediaType != "tv" {
		t.Fatalf("unexpected media types: %+v", multi.Results)
	}
}

func TestFixtureGenresAndLanguagesDecode(t *testing.T) {
	var genres GenreList
	decodeFixture(t, "genres_movie.json", &genres)
	if len(genres.Genres) != 2 {
		t.Fatalf("unexpected genres: %+v", genres)
	}
	var languages []Language
	decodeFixture(t, "languages.json", &languages)
	if len(languages) != 2 || languages[0].ISO639_1 != "pt" {
		t.Fatalf("unexpected languages: %+v", languages)
	}
	var keyword KeywordDetails
	decodeFixture(t, "keyword_1234.json", &keyword)
	if keyword.ID != 1234 || keyword.Name != "neo-western" {
		t.Fatalf("unexpected keyword: %+v", keyword)
	}
}
