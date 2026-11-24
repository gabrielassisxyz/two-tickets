// Package recs computes the "For you" list, version 1. The storage rules
// live in store.go; this file holds the algorithm the bead specifies: seed
// selection from ratings, candidate scoring from TMDB recommendations,
// quality adjustment, exclusions, ordering and explanations. service.go
// fetches through TMDB, writes runs and serves the two routes.
package recs

import (
	"sort"

	"github.com/gabrielassisxyz/two-tickets/internal/library"
)

// The bounds the bead states. Tests compare against these literals, never
// against the constants below, so a test still fails when a constant drifts.
const (
	forYouPositiveSeedCap = 40
	forYouNegativeSeedCap = 20
	forYouFetchFanOut     = 8
	forYouPersonalFloor   = 3
	forYouShrinkVotes     = 200
	forYouShrinkMean      = 6.5
	forYouQualityFloor    = 0.85
	forYouQualityCeil     = 1.15
)

// forYouAlgorithm is the algorithm value a personal run stores;
// forYouAlgorithmCold is what a cold-start run stores. The read path tells
// the two apart by it, so the response can flag popular rather than
// personal without a migration.
const (
	forYouAlgorithm     = "for-you-v1"
	forYouAlgorithmCold = "for-you-v1-cold"
)

// forYouPageSize is the one page size the recommendations route serves.
const forYouPageSize = 20

// forYouSeed is one rated title feeding the computation.
type forYouSeed struct {
	mediaType string
	tmdbID    int64
	weight    float64
	updatedAt int64
}

// forYouSeedKey identifies a seed across the accumulation.
type forYouSeedKey struct {
	mediaType string
	tmdbID    int64
}

func (s forYouSeed) key() forYouSeedKey {
	return forYouSeedKey{mediaType: s.mediaType, tmdbID: s.tmdbID}
}

// forYouRating is one stored rating row as the seed selection reads it.
type forYouRating struct {
	mediaType string
	tmdbID    int64
	scope     string
	level     string
	updatedAt int64
}

// forYouSelectSeeds splits ratings into positive and negative seeds. Movies
// seed from their own rating; a show seeds from its show rating, or, with
// no show rating, from the highest of its season ratings. The newest
// forYouPositiveSeedCap liked and loved seeds weigh 1 and 2; the newest
// forYouNegativeSeedCap hated and disliked seeds weigh -1. Anything else on
// the scale never becomes a seed.
func forYouSelectSeeds(ratings []forYouRating) (positive, negative []forYouSeed) {
	type titleKey struct {
		mediaType string
		tmdbID    int64
	}
	byTitle := map[titleKey][]forYouRating{}
	for _, rating := range ratings {
		key := titleKey{mediaType: rating.mediaType, tmdbID: rating.tmdbID}
		byTitle[key] = append(byTitle[key], rating)
	}
	for key, rows := range byTitle {
		var picked *forYouRating
		switch key.mediaType {
		case "movie":
			for i := range rows {
				if rows[i].scope != "movie" {
					continue
				}
				row := rows[i]
				picked = &row
			}
		case "tv":
			for i := range rows {
				if rows[i].scope == "show" {
					row := rows[i]
					picked = &row
				}
			}
			if picked == nil {
				for i := range rows {
					if rows[i].scope != "season" {
						continue
					}
					row := rows[i]
					if picked == nil || forYouLevelRank(row.level) > forYouLevelRank(picked.level) ||
						(forYouLevelRank(row.level) == forYouLevelRank(picked.level) && row.updatedAt > picked.updatedAt) {
						picked = &row
					}
				}
			}
		default:
			continue
		}
		if picked == nil {
			continue
		}
		seed := forYouSeed{mediaType: key.mediaType, tmdbID: key.tmdbID, updatedAt: picked.updatedAt}
		switch picked.level {
		case "loved":
			seed.weight = 2
			positive = append(positive, seed)
		case "liked":
			seed.weight = 1
			positive = append(positive, seed)
		case "hated", "disliked":
			seed.weight = -1
			negative = append(negative, seed)
		}
	}
	sort.Slice(positive, func(i, j int) bool { return positive[i].updatedAt > positive[j].updatedAt })
	sort.Slice(negative, func(i, j int) bool { return negative[i].updatedAt > negative[j].updatedAt })
	if len(positive) > forYouPositiveSeedCap {
		positive = positive[:forYouPositiveSeedCap]
	}
	if len(negative) > forYouNegativeSeedCap {
		negative = negative[:forYouNegativeSeedCap]
	}
	if positive == nil {
		positive = []forYouSeed{}
	}
	if negative == nil {
		negative = []forYouSeed{}
	}
	return positive, negative
}

// forYouLevelRank orders levels for the season fallback: a higher value
// wins, and only liked and loved (positive) or hated and disliked
// (negative) can seed at all, so the middle of the scale ranks below them.
func forYouLevelRank(level string) int {
	value, ok := library.LevelValue(level)
	if !ok {
		return -1
	}
	return value
}

// forYouRecItem is one TMDB recommendation or trending row as the ranking
// reads it.
type forYouRecItem struct {
	mediaType   string
	tmdbID      int64
	adult       bool
	voteAverage float64
	voteCount   int
}

// forYouSeedRecs is one seed's recommendations in TMDB order.
type forYouSeedRecs struct {
	seed  forYouSeed
	items []forYouRecItem
}

// forYouCandidateKey identifies a candidate across seeds.
type forYouCandidateKey struct {
	mediaType string
	tmdbID    int64
}

// forYouQualityFactor scales a candidate by its shrunk vote mean: the raw
// mean pulled towards forYouShrinkMean by forYouShrinkVotes phantom votes,
// mapped to forYouQualityFloor at 0 and forYouQualityCeil at 10, clamped at
// both ends. Missing or invalid vote data counts as no votes at the mean.
func forYouQualityFactor(voteAverage float64, voteCount int) float64 {
	if voteCount < 0 || voteAverage != voteAverage || voteAverage < 0 || voteAverage > 10 {
		voteCount = 0
		voteAverage = forYouShrinkMean
	}
	shrunk := (voteAverage*float64(voteCount) + forYouShrinkMean*forYouShrinkVotes) / (float64(voteCount) + forYouShrinkVotes)
	quality := forYouQualityFloor + (shrunk/10)*(forYouQualityCeil-forYouQualityFloor)
	if quality < forYouQualityFloor {
		return forYouQualityFloor
	}
	if quality > forYouQualityCeil {
		return forYouQualityCeil
	}
	return quality
}

// forYouRank accumulates every seed's contribution per candidate, applies
// the negative seeds only to candidates the positives already produced,
// drops excluded titles, and orders the rest: score descending, vote count
// descending, then media type and TMDB id ascending. Reasons name the three
// positive seeds with the largest contribution, resolved through titles.
func forYouRank(positive, negative []forYouSeedRecs, excluded map[forYouCandidateKey]bool, titles map[forYouSeedKey]string) []Result {
	return forYouFinalize(forYouAccumulate(positive, negative), excluded, titles)
}

// forYouAccumulate folds every seed's recommendations into per-candidate
// totals. A candidate at rank r of n carries weight times (1 - r/n) from
// its seed; negative seeds only touch candidates the positives produced.
func forYouAccumulate(positive, negative []forYouSeedRecs) map[forYouCandidateKey]*forYouAccumulation {
	candidates := map[forYouCandidateKey]*forYouAccumulation{}
	accumulate := func(recs forYouSeedRecs, negativesOnly bool) {
		n := len(recs.items)
		if n == 0 {
			return
		}
		for rank, item := range recs.items {
			if item.mediaType != "movie" && item.mediaType != "tv" {
				continue
			}
			if item.adult {
				continue
			}
			key := forYouCandidateKey{mediaType: item.mediaType, tmdbID: item.tmdbID}
			if negativesOnly && candidates[key] == nil {
				continue
			}
			contribution := recs.seed.weight * (1 - float64(rank)/float64(n))
			entry := candidates[key]
			if entry == nil {
				entry = &forYouAccumulation{item: item, contributions: map[forYouSeedKey]float64{}, voteCount: item.voteCount}
				candidates[key] = entry
			}
			entry.raw += contribution
			if recs.seed.weight > 0 {
				entry.contributions[recs.seed.key()] += contribution
			}
		}
	}
	for _, recs := range positive {
		accumulate(recs, false)
	}
	for _, recs := range negative {
		accumulate(recs, true)
	}
	return candidates
}

// forYouTopSeeds names the seeds whose titles an explanation needs: the
// three largest positive contributors of every kept candidate.
func forYouTopSeeds(candidates map[forYouCandidateKey]*forYouAccumulation, excluded map[forYouCandidateKey]bool) []forYouSeedKey {
	seen := map[forYouSeedKey]bool{}
	var seeds []forYouSeedKey
	for key, entry := range candidates {
		if excluded[key] {
			continue
		}
		ordered := forYouOrderedContributors(entry)
		for i := 0; i < len(ordered) && i < forYouReasonCount; i++ {
			if !seen[ordered[i]] {
				seen[ordered[i]] = true
				seeds = append(seeds, ordered[i])
			}
		}
	}
	return seeds
}

// forYouOrderedContributors sorts one candidate's positive seeds by
// contribution descending, breaking ties by media type then TMDB id.
func forYouOrderedContributors(entry *forYouAccumulation) []forYouSeedKey {
	ordered := make([]forYouSeedKey, 0, len(entry.contributions))
	for seed := range entry.contributions {
		ordered = append(ordered, seed)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if entry.contributions[ordered[i]] != entry.contributions[ordered[j]] {
			return entry.contributions[ordered[i]] > entry.contributions[ordered[j]]
		}
		if ordered[i].mediaType != ordered[j].mediaType {
			return ordered[i].mediaType < ordered[j].mediaType
		}
		return ordered[i].tmdbID < ordered[j].tmdbID
	})
	return ordered
}

// forYouFinalize drops excluded candidates, attaches scores and reasons,
// and orders the list, assigning ranks.
func forYouFinalize(candidates map[forYouCandidateKey]*forYouAccumulation, excluded map[forYouCandidateKey]bool, titles map[forYouSeedKey]string) []Result {
	results := make([]Result, 0, len(candidates))
	for key, entry := range candidates {
		if excluded[key] {
			continue
		}
		ordered := forYouOrderedContributors(entry)
		reasons := []string{}
		for _, seed := range ordered {
			if len(reasons) == forYouReasonCount {
				break
			}
			title, ok := titles[seed]
			if !ok || title == "" {
				continue
			}
			reasons = append(reasons, title)
		}
		results = append(results, Result{
			MediaType: key.mediaType,
			TmdbID:    key.tmdbID,
			Score:     entry.raw * forYouQualityFactor(entry.item.voteAverage, entry.item.voteCount),
			Reasons:   reasons,
		})
	}
	votes := make(map[forYouCandidateKey]int, len(candidates))
	for key, entry := range candidates {
		votes[key] = entry.voteCount
	}
	forYouOrderResults(results, votes)
	for i := range results {
		results[i].Rank = int64(i + 1)
	}
	return results
}

// forYouReasonCount is how many seeds an explanation names.
const forYouReasonCount = 3

// forYouAccumulation is one candidate's running total.
type forYouAccumulation struct {
	item          forYouRecItem
	raw           float64
	contributions map[forYouSeedKey]float64
	voteCount     int
}

// forYouOrderResults sorts score descending, then TMDB vote count
// descending, then media type and TMDB id ascending, so equal inputs always
// give the same list.
func forYouOrderResults(results []Result, votes map[forYouCandidateKey]int) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		ki := forYouCandidateKey{mediaType: results[i].MediaType, tmdbID: results[i].TmdbID}
		kj := forYouCandidateKey{mediaType: results[j].MediaType, tmdbID: results[j].TmdbID}
		if votes[ki] != votes[kj] {
			return votes[ki] > votes[kj]
		}
		if results[i].MediaType != results[j].MediaType {
			return results[i].MediaType < results[j].MediaType
		}
		return results[i].TmdbID < results[j].TmdbID
	})
}

// forYouColdRank orders the cold-start list: TMDB's trending of the week,
// minus everything watched or rated, keeping TMDB's order through a rank
// score. There are no seeds, so there are no reasons.
func forYouColdRank(items []forYouRecItem, excluded map[forYouCandidateKey]bool) []Result {
	kept := make([]forYouRecItem, 0, len(items))
	for _, item := range items {
		if item.mediaType != "movie" && item.mediaType != "tv" {
			continue
		}
		if item.adult {
			continue
		}
		if excluded[forYouCandidateKey{mediaType: item.mediaType, tmdbID: item.tmdbID}] {
			continue
		}
		kept = append(kept, item)
	}
	results := make([]Result, 0, len(kept))
	for i, item := range kept {
		results = append(results, Result{
			Rank:      int64(i + 1),
			MediaType: item.mediaType,
			TmdbID:    item.tmdbID,
			Score:     1 - float64(i)/float64(len(kept)),
			Reasons:   []string{},
		})
	}
	return results
}
