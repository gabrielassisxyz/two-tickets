package store

import (
	"strings"
	"testing"
)

var librarySentimentLevels = []string{"hated", "disliked", "meh", "fine", "good", "liked", "loved"}

func TestLibraryRatingLevelIsOneOfTheSeven(t *testing.T) {
	db := newLibraryDatabase(t).Write
	for i, level := range librarySentimentLevels {
		row := libraryMovieRow("ratings").with("tmdb_id", int64(100+i)).with("level", level)
		libraryExpectAccepted(t, "level "+level, libraryInsert(db, "ratings", row))
	}
	for _, level := range []string{"ok", "Loved", ""} {
		libraryExpectRejected(t, "level "+level, libraryInsert(db, "ratings", libraryMovieRow("ratings").with("level", level)), libraryCheckFailed("ratings_level_known"))
	}
}

// With "=" instead of "IS", a NULL scale makes the comparison NULL, which a
// CHECK passes: the null case is the one that proves the operator.
func TestLibraryRatingLevelCarriesItsScale(t *testing.T) {
	db := newLibraryDatabase(t).Write
	want := libraryCheckFailed("ratings_level_has_scale")
	libraryExpectRejected(t, "level without a scale", libraryInsert(db, "ratings", libraryMovieRow("ratings").with("scale", nil)), want)
	libraryExpectRejected(t, "level on another scale", libraryInsert(db, "ratings", libraryMovieRow("ratings").with("scale", "sentiment-10-v1")), want)
	libraryExpectAccepted(t, "level on sentiment-7-v1", libraryInsert(db, "ratings", libraryMovieRow("ratings")))
}

func TestLibraryRatingScaleNeedsALevel(t *testing.T) {
	db := newLibraryDatabase(t).Write
	row := libraryMovieRow("ratings").with("level", nil).with("review", "Worth it.")
	libraryExpectRejected(t, "scale on a review-only row", libraryInsert(db, "ratings", row), libraryCheckFailed("ratings_scale_only_with_level"))
	libraryExpectAccepted(t, "review-only row", libraryInsert(db, "ratings", row.with("scale", nil)))
}

func TestLibraryRatingNeedsALevelOrAReview(t *testing.T) {
	db := newLibraryDatabase(t).Write
	empty := libraryMovieRow("ratings").with("level", nil).with("scale", nil).with("review", nil)
	libraryExpectRejected(t, "neither level nor review", libraryInsert(db, "ratings", empty), libraryCheckFailed("ratings_level_or_review"))
	libraryExpectAccepted(t, "level and review", libraryInsert(db, "ratings", libraryMovieRow("ratings").with("review", "Both.")))
}

// length() counts characters, so a review of 10,000 two-byte characters is at
// the limit, not twice over it.
func TestLibraryRatingReviewIsNonEmptyAndAtMostTenThousandCharacters(t *testing.T) {
	db := newLibraryDatabase(t).Write
	want := libraryCheckFailed("ratings_review_length")
	review := func(tmdbID int64, text string) libraryRow {
		return libraryMovieRow("ratings").with("tmdb_id", tmdbID).with("review", text)
	}
	libraryExpectRejected(t, "empty review", libraryInsert(db, "ratings", review(1, "")), want)
	libraryExpectRejected(t, "10,001 characters", libraryInsert(db, "ratings", review(2, strings.Repeat("a", 10_001))), want)
	libraryExpectAccepted(t, "10,000 characters", libraryInsert(db, "ratings", review(3, strings.Repeat("a", 10_000))))
	libraryExpectAccepted(t, "10,000 two-byte characters", libraryInsert(db, "ratings", review(4, strings.Repeat("é", 10_000))))
	libraryExpectAccepted(t, "one character", libraryInsert(db, "ratings", review(5, "!")))
}
