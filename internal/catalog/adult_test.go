package catalog

import (
	"encoding/json"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// Each adult fixture is a recorded list with one entry turned adult, and
// upstream keeps that entry, so its golden minus the adult positions is what
// WithoutAdult must return. Movie and series lists are filtered as typed
// slices, the way the recommendation and discover routes hold them.
func TestWithoutAdultRemovesExactlyTheAdultEntriesOfEachAdultFixture(t *testing.T) {
	cases := []struct {
		fixture, golden string
		filter          func(t *testing.T, fixture string) (kept any, before, after int)
	}{
		{"search/multi-adult", "search/search-multi-adult-entry", filterMixedFixture},
		{"trending/all-day-adult", "discover/discover-trending-adult-entry", filterMixedFixture},
		{"discover/movies-popular-adult", "discover/discover-movies-adult-entry", filterMovieFixture},
		{"movie/recommendations-adult", "title/movie-recommendations-adult-entry", filterMovieFixture},
		{"movie/similar-adult", "title/movie-similar-adult-entry", filterMovieFixture},
		{"tv/recommendations-adult", "title/tv-recommendations-adult-entry", filterTvFixture},
		{"tv/similar-adult", "title/tv-similar-adult-entry", filterTvFixture},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			adult := adultFixturePositions(t, c.fixture)
			if len(adult) == 0 {
				t.Fatalf("fixture %s holds no adult entry", c.fixture)
			}
			kept, before, after := c.filter(t, c.fixture)
			if after != before-len(adult) {
				t.Errorf("kept %d of %d entries, want %d removed", after, before, len(adult))
			}
			golden := mapperField(t, mapperGoldenBody(t, c.golden, c.fixture), "results").([]any)
			want := make([]any, 0, len(golden))
			for i, entry := range golden {
				if !adult[i] {
					want = append(want, entry)
				}
			}
			mapperRequireGoldenMatch(t, "WithoutAdult", kept, want)
		})
	}
}

// No fixture holds an adult person, so this one turns a recorded person and
// one title another person is known for adult.
func TestWithoutAdultDropsAdultPeopleAndAdultKnownForTitles(t *testing.T) {
	var page tmdb.SearchMultiResponse
	decodeMapperFixture(t, "search/multi-cooper", &page)
	var people []*tmdb.PersonBrief
	for _, item := range page.Results {
		if item.Person != nil && len(item.Person.KnownFor) > 1 {
			people = append(people, item.Person)
		}
	}
	if len(people) < 2 {
		t.Fatalf("the fixture holds %d people with two known-for titles, want 2", len(people))
	}
	adultPerson, otherPerson := people[0], people[1]
	if otherPerson.KnownFor[0].Movie == nil {
		t.Fatalf("person %d is first known for a non-movie", otherPerson.ID)
	}
	adultPerson.Adult = true
	otherPerson.KnownFor[0].Movie.Adult = true
	knownBefore := len(otherPerson.KnownFor)

	kept := WithoutAdult(MapSearchResults(page.Results))
	if len(kept) != len(page.Results)-1 {
		t.Errorf("kept %d of %d entries, want only the adult person removed", len(kept), len(page.Results))
	}
	for _, result := range kept {
		person, ok := result.(PersonResult)
		if !ok {
			continue
		}
		if person.ID == adultPerson.ID {
			t.Errorf("adult person %d kept", person.ID)
		}
		if person.ID == otherPerson.ID && len(person.KnownFor) != knownBefore-1 {
			t.Errorf("person %d keeps %d known-for titles, want %d", person.ID, len(person.KnownFor), knownBefore-1)
		}
	}
}

func TestWithoutAdultAnswersAnEmptyArrayForAnAllAdultList(t *testing.T) {
	kept := WithoutAdult([]MovieResult{{ID: 1, Adult: true}})
	if raw, _ := json.Marshal(kept); string(raw) != "[]" {
		t.Errorf("an all-adult list filters to %s, want []", raw)
	}
}

func adultFixturePositions(t *testing.T, fixture string) map[int]bool {
	t.Helper()
	var page struct {
		Results []struct {
			Adult bool `json:"adult"`
		} `json:"results"`
	}
	if err := json.Unmarshal(mapperFixtureBody(t, fixture), &page); err != nil {
		t.Fatal(err)
	}
	positions := map[int]bool{}
	for i, entry := range page.Results {
		if entry.Adult {
			positions[i] = true
		}
	}
	return positions
}

func filterMixedFixture(t *testing.T, fixture string) (any, int, int) {
	var page tmdb.TrendingResponse
	decodeMapperFixture(t, fixture, &page)
	mapped := MapSearchResults(page.Results)
	kept := WithoutAdult(mapped)
	return kept, len(mapped), len(kept)
}

func filterMovieFixture(t *testing.T, fixture string) (any, int, int) {
	mapped := mapMoviePageFixture(t, fixture).([]MovieResult)
	kept := WithoutAdult(mapped)
	return kept, len(mapped), len(kept)
}

func filterTvFixture(t *testing.T, fixture string) (any, int, int) {
	mapped := mapTvPageFixture(t, fixture).([]TvResult)
	kept := WithoutAdult(mapped)
	return kept, len(mapped), len(kept)
}
