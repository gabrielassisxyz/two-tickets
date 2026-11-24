package library

// RatingScale names the seven-step scale every stored level belongs to. A
// later scale can convert old ratings because each row records this.
const RatingScale = "sentiment-7-v1"

// ratingLevelValues maps each level of RatingScale to the recommender's
// internal 1-10 value. The labels and anchors the UI shows live in its i18n
// files, never here.
var ratingLevelValues = map[string]int{
	"hated":    1,
	"disliked": 3,
	"meh":      5,
	"fine":     6,
	"good":     7,
	"liked":    8,
	"loved":    10,
}

// LevelValue returns the 1-10 value of a level, and false for a name that is
// not on the scale.
func LevelValue(level string) (int, bool) {
	value, ok := ratingLevelValues[level]
	return value, ok
}
