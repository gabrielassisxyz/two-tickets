package recs

import (
	"math"
	"testing"
)

// The ranked fixtures pin ids, order and scores at full precision. Every
// bound below is the literal the bead states, never the constant that
// implements it.
func TestForYouRankPinsTheExactOutput(t *testing.T) {
	seedOne := forYouSeed{mediaType: "movie", tmdbID: 1, weight: 2, updatedAt: 300}
	seedTwo := forYouSeed{mediaType: "movie", tmdbID: 2, weight: 1, updatedAt: 200}
	titles := map[forYouSeedKey]string{
		{mediaType: "movie", tmdbID: 1}: "Seed One",
		{mediaType: "movie", tmdbID: 2}: "Seed Two",
		{mediaType: "tv", tmdbID: 9}:    "Seed Nine",
	}
	positive := []forYouSeedRecs{
		{seed: seedOne, items: []forYouRecItem{
			{mediaType: "movie", tmdbID: 101, voteAverage: 7, voteCount: 100},
			{mediaType: "movie", tmdbID: 102, voteAverage: 6.5, voteCount: 0},
			{mediaType: "movie", tmdbID: 103, voteAverage: 8, voteCount: 500},
			{mediaType: "tv", tmdbID: 201, voteAverage: 5, voteCount: 50},
		}},
		{seed: seedTwo, items: []forYouRecItem{
			{mediaType: "movie", tmdbID: 101, voteAverage: 7, voteCount: 100},
			{mediaType: "movie", tmdbID: 104, voteAverage: 9, voteCount: 10000},
		}},
	}
	negative := []forYouSeedRecs{
		{seed: forYouSeed{mediaType: "tv", tmdbID: 9, weight: -1, updatedAt: 100}, items: []forYouRecItem{
			{mediaType: "movie", tmdbID: 102, voteAverage: 6.5, voteCount: 0},
			{mediaType: "movie", tmdbID: 105, voteAverage: 9, voteCount: 10000},
		}},
	}

	t.Run("positives only", func(t *testing.T) {
		results := forYouRank(positive, nil, map[forYouCandidateKey]bool{}, titles)
		wantOrder := []int64{101, 102, 103, 104, 201}
		if len(results) != len(wantOrder) {
			t.Fatalf("results = %+v, want %d rows", results, len(wantOrder))
		}
		for i, tmdbID := range wantOrder {
			if results[i].TmdbID != tmdbID {
				t.Fatalf("rank %d = %d, want %d (full order %+v)", i+1, results[i].TmdbID, tmdbID, results)
			}
			if results[i].Rank != int64(i+1) {
				t.Errorf("row %d rank = %d, want %d", tmdbID, results[i].Rank, i+1)
			}
		}
		// Hand-computed: 101 peaks at 3 * quality(7, 100), 104 trails at
		// 0.5 * quality(9, 10000).
		wantScores := map[int64]float64{
			101: 3 * (0.85 + (((7*100 + 6.5*200) / (100 + 200) / 10) * 0.30)),
			102: 1.5 * (0.85 + (((6.5*0 + 6.5*200) / (0 + 200) / 10) * 0.30)),
			103: 1.0 * (0.85 + (((8*500 + 6.5*200) / (500 + 200) / 10) * 0.30)),
			104: 0.5 * (0.85 + (((9*10000 + 6.5*200) / (10000 + 200) / 10) * 0.30)),
			201: 0.5 * (0.85 + (((5*50 + 6.5*200) / (50 + 200) / 10) * 0.30)),
		}
		for _, result := range results {
			if math.Abs(result.Score-wantScores[result.TmdbID]) > 1e-9 {
				t.Errorf("score of %d = %.17g, want %.17g", result.TmdbID, result.Score, wantScores[result.TmdbID])
			}
		}
		if len(results[0].Reasons) != 2 || results[0].Reasons[0] != "Seed One" || results[0].Reasons[1] != "Seed Two" {
			t.Errorf("reasons of 101 = %q, want both positive seeds", results[0].Reasons)
		}
	})

	t.Run("negatives lower only the positives candidates", func(t *testing.T) {
		results := forYouRank(positive, negative, map[forYouCandidateKey]bool{}, titles)
		for _, result := range results {
			if result.TmdbID == 105 {
				t.Fatalf("negative-only candidate 105 is ranked: %+v", results)
			}
		}
		// 102 loses 1.0 to the negative seed and falls behind 104.
		wantOrder := []int64{101, 103, 104, 102, 201}
		for i, tmdbID := range wantOrder {
			if results[i].TmdbID != tmdbID {
				t.Fatalf("rank %d = %d, want %d (full order %+v)", i+1, results[i].TmdbID, tmdbID, results)
			}
		}
		want102 := 0.5 * (0.85 + (((6.5*0 + 6.5*200) / (0 + 200) / 10) * 0.30))
		if math.Abs(results[3].Score-want102) > 1e-9 {
			t.Errorf("score of 102 = %.17g, want %.17g", results[3].Score, want102)
		}
		for _, reason := range results[3].Reasons {
			if reason == "Seed Nine" {
				t.Errorf("a negative seed explains 102: %q", results[3].Reasons)
			}
		}
	})

	t.Run("ties break by votes then media type then id", func(t *testing.T) {
		tied := []forYouSeedRecs{
			{seed: seedOne, items: []forYouRecItem{
				{mediaType: "movie", tmdbID: 301, voteAverage: 6.5, voteCount: 0},
				{mediaType: "movie", tmdbID: 302, voteAverage: 6.5, voteCount: 0},
				{mediaType: "tv", tmdbID: 300, voteAverage: 6.5, voteCount: 0},
			}},
		}
		results := forYouRank(tied, nil, map[forYouCandidateKey]bool{}, titles)
		wantOrder := []struct {
			mediaType string
			tmdbID    int64
		}{{"movie", 301}, {"movie", 302}, {"tv", 300}}
		for i, want := range wantOrder {
			if results[i].MediaType != want.mediaType || results[i].TmdbID != want.tmdbID {
				t.Fatalf("rank %d = %s/%d, want %s/%d", i+1, results[i].MediaType, results[i].TmdbID, want.mediaType, want.tmdbID)
			}
		}
		ordered := []Result{
			{MediaType: "tv", TmdbID: 7, Score: 1},
			{MediaType: "movie", TmdbID: 9, Score: 1},
			{MediaType: "movie", TmdbID: 8, Score: 1},
		}
		votes := map[forYouCandidateKey]int{
			{mediaType: "tv", tmdbID: 7}:    5,
			{mediaType: "movie", tmdbID: 9}: 5,
			{mediaType: "movie", tmdbID: 8}: 400,
		}
		forYouOrderResults(ordered, votes)
		if ordered[0].TmdbID != 8 || ordered[1].TmdbID != 9 || ordered[2].TmdbID != 7 {
			t.Fatalf("vote-count tiebreak = %+v, want 8, 9, 7", ordered)
		}
	})
}

func TestForYouQualityFactorIsPinned(t *testing.T) {
	cases := []struct {
		average float64
		count   int
		want    float64
	}{
		{6.5, 0, 0.85 + ((6.5 / 10) * 0.30)},
		{9, 10000, 0.85 + ((((9*10000 + 6.5*200) / (10000 + 200)) / 10) * 0.30)},
		{2, 10000, 0.85 + ((((2*10000 + 6.5*200) / (10000 + 200)) / 10) * 0.30)},
	}
	for _, tc := range cases {
		if got := forYouQualityFactor(tc.average, tc.count); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("quality(%v, %v) = %.17g, want %.17g", tc.average, tc.count, got, tc.want)
		}
	}
	// Invalid data counts as no votes at the mean too.
	for _, average := range []float64{math.NaN(), -1, 11} {
		if got := forYouQualityFactor(average, 100); math.Abs(got-(0.85+((6.5/10)*0.30))) > 1e-12 {
			t.Errorf("quality(%v, 100) = %.17g, want the mean value", average, got)
		}
	}
	if got := forYouQualityFactor(7, -5); math.Abs(got-(0.85+((6.5/10)*0.30))) > 1e-12 {
		t.Errorf("quality(7, -5) = %.17g, want the mean value", got)
	}
	// At the clamp boundaries the factor reaches the bound: the shrunk mean
	// tends to 0 and 10 with uncountable votes, so the bounds below hold
	// with room and the values above pin the limit.
	if got := forYouQualityFactor(10, 10000000000); got > 1.15 || math.Abs(got-1.15) > 1e-6 {
		t.Errorf("quality(10, 10000000000) = %.17g, want 1.15", got)
	}
	if got := forYouQualityFactor(0, 10000000000); got < 0.85 || math.Abs(got-0.85) > 1e-6 {
		t.Errorf("quality(0, 10000000000) = %.17g, want 0.85", got)
	}
}

func TestForYouSelectSeedsCapsAndFallback(t *testing.T) {
	var ratings []forYouRating
	for i := int64(1); i <= 41; i++ {
		ratings = append(ratings, forYouRating{mediaType: "movie", tmdbID: i, scope: "movie", level: "liked", updatedAt: i})
	}
	for i := int64(1); i <= 21; i++ {
		ratings = append(ratings, forYouRating{mediaType: "movie", tmdbID: 100 + i, scope: "movie", level: "hated", updatedAt: i})
	}
	// A show with only season ratings seeds from the highest one.
	ratings = append(ratings,
		forYouRating{mediaType: "tv", tmdbID: 50, scope: "season", level: "liked", updatedAt: 200},
		forYouRating{mediaType: "tv", tmdbID: 50, scope: "season", level: "loved", updatedAt: 100},
	)
	// A show rating wins over any season rating, and the middle of the
	// scale never seeds.
	ratings = append(ratings,
		forYouRating{mediaType: "tv", tmdbID: 70, scope: "show", level: "meh", updatedAt: 400},
		forYouRating{mediaType: "tv", tmdbID: 70, scope: "season", level: "loved", updatedAt: 400},
		forYouRating{mediaType: "movie", tmdbID: 80, scope: "movie", level: "fine", updatedAt: 400},
		forYouRating{mediaType: "movie", tmdbID: 81, scope: "movie", level: "good", updatedAt: 400},
	)
	positive, negative := forYouSelectSeeds(ratings)
	if len(positive) != 40 {
		t.Fatalf("positive seeds = %d, want the newest 40", len(positive))
	}
	for _, seed := range positive {
		if seed.mediaType == "movie" && (seed.tmdbID == 1 || seed.tmdbID == 2) {
			t.Errorf("one of the two oldest of 41 liked seeds is kept: %+v", positive)
		}
	}
	// The season-fallback show at updatedAt 100 is the newest seed of all.
	if positive[0].mediaType != "tv" || positive[0].tmdbID != 50 || positive[0].updatedAt != 100 {
		t.Errorf("newest positive seed = %+v, want the tv/50 fallback at 100", positive[0])
	}
	if len(negative) != 20 {
		t.Fatalf("negative seeds = %d, want the newest 20", len(negative))
	}
	for _, seed := range negative {
		if seed.tmdbID == 101 {
			t.Errorf("the oldest of 21 hated seeds is kept")
		}
	}
	var show *forYouSeed
	for i := range positive {
		if positive[i].mediaType == "tv" && positive[i].tmdbID == 50 {
			show = &positive[i]
		}
	}
	if show == nil {
		t.Fatal("the show with only season ratings is not a seed")
	}
	if show.weight != 2 || show.updatedAt != 100 {
		t.Errorf("season-fallback seed = %+v, want the loved row at 100", show)
	}
	for _, seed := range append(append([]forYouSeed{}, positive...), negative...) {
		if (seed.mediaType == "tv" && seed.tmdbID == 70) || seed.tmdbID == 80 || seed.tmdbID == 81 {
			t.Errorf("the middle of the scale seeds: %+v", seed)
		}
	}
}
