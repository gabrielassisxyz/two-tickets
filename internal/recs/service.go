package recs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// ForYouWatchTitle names one title for a watchlist membership question.
type ForYouWatchTitle struct {
	MediaType string
	TMDBID    int
}

// ForYouWatchlist answers which of titles are on the watchlist of the
// account ctx carries. *library.Service satisfies it through the adapter
// the wiring builds; nil means nothing is on a watchlist.
type ForYouWatchlist interface {
	OnCallerWatchlist(ctx context.Context, titles []ForYouWatchTitle) (map[ForYouWatchTitle]bool, error)
}

// forYouTMDB is the part of *tmdb.Client the computation uses, so tests can
// substitute a fake server rather than a fake client.
type forYouTMDB interface {
	GetMovieRecommendations(ctx context.Context, id string, params map[string]string) (*tmdb.DiscoverMoviesResponse, error)
	GetTVRecommendations(ctx context.Context, id string, params map[string]string) (*tmdb.DiscoverTVResponse, error)
	GetAllTrending(ctx context.Context, timeWindow string, params map[string]string) (*tmdb.TrendingResponse, error)
}

// Service computes "For you" and serves its two routes. Reads go through
// the newest completed run; a read whose watermark moved recomputes first,
// and a failed computation keeps serving the previous run.
type Service struct {
	db        *store.Database
	tmdb      forYouTMDB
	cards     *hydrate.Hydrator
	watchlist ForYouWatchlist
	logger    *slog.Logger
	now       func() time.Time
	mu        sync.Mutex
}

// NewService builds the recommender. A nil logger discards; a nil now means
// time.Now. A nil watchlist marks nothing.
func NewService(db *store.Database, client forYouTMDB, cards *hydrate.Hydrator, watchlist ForYouWatchlist, logger *slog.Logger, now func() time.Time) *Service {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, tmdb: client, cards: cards, watchlist: watchlist, logger: logger, now: now}
}

// Register adds the recommendations routes to mux.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/recommendations", s.handleRecommendations)
	mux.HandleFunc("POST /api/v1/recommendations/exposures", s.handleExposures)
}

// forYouItem is one recommended title: the hydrated card plus what the
// carousel needs, the seed titles behind it and its identity in the run.
type forYouItem struct {
	hydrate.Card
	Reasons     []string `json:"reasons"`
	Rank        int64    `json:"rank"`
	RunID       int64    `json:"runId"`
	OnWatchlist bool     `json:"onWatchlist"`
}

// forYouPage is the GET answer: a Seerr-shaped page of recommendations,
// flagged personal when seeds produced it and popular when trending did.
type forYouPage struct {
	Page         int          `json:"page"`
	TotalPages   int          `json:"totalPages"`
	TotalResults int          `json:"totalResults"`
	Personal     bool         `json:"personal"`
	RunID        int64        `json:"runId"`
	Results      []forYouItem `json:"results"`
}

func (s *Service) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}
	filter, page, ok := forYouParsePageQuery(w, r)
	if !ok {
		return
	}
	runID, err := s.forYouFresh(r.Context(), user.ID)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "recommendations failed", "error", err)
		respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	stored, err := ReadLatest(r.Context(), s.db.Read, user.ID, filter, forYouPageSize, (page-1)*forYouPageSize)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "recommendations read failed", "error", err)
		respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	if stored.RunID == 0 {
		respond.JSON(w, forYouPage{Page: page, Results: []forYouItem{}})
		return
	}
	if stored.RunID != runID {
		s.logger.WarnContext(r.Context(), "recommendations served a run the freshness check did not pick", "served", stored.RunID, "fresh", runID)
	}
	respond.JSON(w, s.forYouAnswer(r.Context(), stored, page))
}

// forYouParsePageQuery reads mediaType and page. The filter happens on the
// server before pagination, so totals and pages stay right.
func forYouParsePageQuery(w http.ResponseWriter, r *http.Request) (filter string, page int, ok bool) {
	switch mediaType := r.URL.Query().Get("mediaType"); mediaType {
	case "", "all":
	case "movie", "tv":
		filter = mediaType
	default:
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "mediaType must be all, movie or tv")
		return "", 0, false
	}
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return filter, 1, true
	}
	number, err := strconv.Atoi(raw)
	if err != nil || number < 1 {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "page must be a positive integer")
		return "", 0, false
	}
	return filter, number, true
}

// forYouFresh returns the run the read serves: the newest completed run,
// recomputing first when the ratings or watches watermark moved. A failed
// computation keeps serving the previous run; only with no run at all does
// it fail.
func (s *Service) forYouFresh(ctx context.Context, userID int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	moved, err := WatermarkMoved(ctx, s.db.Read, userID)
	if err != nil {
		return 0, err
	}
	if !moved {
		page, err := ReadLatest(ctx, s.db.Read, userID, "", 1, 0)
		if err != nil {
			return 0, err
		}
		return page.RunID, nil
	}
	runID, err := s.forYouCompute(ctx, userID)
	if err == nil {
		return runID, nil
	}
	s.logger.WarnContext(ctx, "recommendations computation failed, serving the previous run", "error", err)
	page, readErr := ReadLatest(ctx, s.db.Read, userID, "", 1, 0)
	if readErr != nil {
		return 0, errors.Join(err, readErr)
	}
	if page.RunID == 0 {
		return 0, err
	}
	return page.RunID, nil
}

// forYouPersonal reports whether a run is personal rather than cold-start,
// by the algorithm value it stored.
func (s *Service) forYouPersonal(ctx context.Context, runID int64) bool {
	var algorithm string
	if err := s.db.Read.QueryRowContext(ctx, `SELECT algorithm FROM rec_runs WHERE id = ?`, runID).Scan(&algorithm); err != nil {
		return false
	}
	return algorithm == forYouAlgorithm
}

// forYouAnswer hydrates one stored page into the GET shape.
func (s *Service) forYouAnswer(ctx context.Context, stored Page, page int) forYouPage {
	answer := forYouPage{
		Page:         page,
		TotalPages:   (stored.Total + forYouPageSize - 1) / forYouPageSize,
		TotalResults: stored.Total,
		Personal:     s.forYouPersonal(ctx, stored.RunID),
		RunID:        stored.RunID,
		Results:      []forYouItem{},
	}
	titles := make([]hydrate.Title, 0, len(stored.Results))
	for _, result := range stored.Results {
		titles = append(titles, hydrate.Title{MediaType: result.MediaType, TMDBID: int(result.TmdbID)})
	}
	cards := s.cards.Cards(ctx, titles)
	watched := map[ForYouWatchTitle]bool{}
	if s.watchlist != nil && len(titles) > 0 {
		asking := make([]ForYouWatchTitle, 0, len(titles))
		for _, title := range titles {
			asking = append(asking, ForYouWatchTitle{MediaType: title.MediaType, TMDBID: title.TMDBID})
		}
		// A library fault never fails the page: the titles simply show
		// without their watchlist mark.
		if found, err := s.watchlist.OnCallerWatchlist(ctx, asking); err == nil {
			watched = found
		} else {
			s.logger.WarnContext(ctx, "recommendations watchlist lookup failed", "error", err)
		}
	}
	for i, result := range stored.Results {
		reasons := result.Reasons
		if reasons == nil {
			reasons = []string{}
		}
		answer.Results = append(answer.Results, forYouItem{
			Card:        cards[i],
			Reasons:     reasons,
			Rank:        result.Rank,
			RunID:       stored.RunID,
			OnWatchlist: watched[ForYouWatchTitle{MediaType: result.MediaType, TMDBID: int(result.TmdbID)}],
		})
	}
	return answer
}

// forYouCompute builds one run from the user's current ratings and watches
// and publishes it. Any error leaves the new run unpublished, so the newest
// completed run stays the one readers see.
func (s *Service) forYouCompute(ctx context.Context, userID int64) (int64, error) {
	ratings, excluded, err := s.forYouLoadRatings(ctx, userID)
	if err != nil {
		return 0, err
	}
	positive, negative := forYouSelectSeeds(ratings)
	var results []Result
	algorithm := forYouAlgorithm
	if len(positive) < forYouPersonalFloor {
		items, err := s.forYouTrending(ctx)
		if err != nil {
			return 0, err
		}
		results = forYouColdRank(items, excluded)
		algorithm = forYouAlgorithmCold
	} else {
		results, err = s.forYouPersonalResults(ctx, positive, negative, excluded)
		if err != nil {
			return 0, err
		}
	}
	watermark, err := Watermark(ctx, s.db.Read, userID)
	if err != nil {
		return 0, err
	}
	runID, err := CreateRun(ctx, s.db.Write, userID, algorithm, watermark, s.now().Unix())
	if err != nil {
		return 0, err
	}
	if err := AddResults(ctx, s.db.Write, runID, results); err != nil {
		return 0, fmt.Errorf("add recommendations results: %w", err)
	}
	if err := PublishRun(ctx, s.db.Write, runID, s.now().Unix()); err != nil {
		return 0, err
	}
	return runID, nil
}

// forYouPersonalResults fetches every seed's recommendations at most
// forYouFetchFanOut at a time, then ranks with hydrated seed titles as
// explanations.
func (s *Service) forYouPersonalResults(ctx context.Context, positive, negative []forYouSeed, excluded map[forYouCandidateKey]bool) ([]Result, error) {
	seeds := append(append([]forYouSeed{}, positive...), negative...)
	fetched := make([]forYouSeedRecs, len(seeds))
	slots := make(chan struct{}, forYouFetchFanOut)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	for i, seed := range seeds {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			items, err := s.forYouFetchSeed(ctx, seed)
			if err != nil {
				once.Do(func() { firstErr = err })
				return
			}
			fetched[i] = forYouSeedRecs{seed: seed, items: items}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	var positiveRecs, negativeRecs []forYouSeedRecs
	for _, recs := range fetched {
		if recs.seed.weight > 0 {
			positiveRecs = append(positiveRecs, recs)
		} else {
			negativeRecs = append(negativeRecs, recs)
		}
	}
	accumulated := forYouAccumulate(positiveRecs, negativeRecs)
	titles := s.forYouSeedTitles(ctx, forYouTopSeeds(accumulated, excluded))
	return forYouFinalize(accumulated, excluded, titles), nil
}

// forYouFetchSeed fetches one seed's recommendations, page 1, through the
// TMDB client, whose cache keeps them seven days.
func (s *Service) forYouFetchSeed(ctx context.Context, seed forYouSeed) ([]forYouRecItem, error) {
	params := map[string]string{"language": tmdb.DefaultLanguage, "page": "1"}
	id := strconv.FormatInt(seed.tmdbID, 10)
	switch seed.mediaType {
	case "movie":
		data, err := s.tmdb.GetMovieRecommendations(ctx, id, params)
		if err != nil {
			return nil, fmt.Errorf("recommendations for movie %d: %w", seed.tmdbID, err)
		}
		items := make([]forYouRecItem, 0, len(data.Results))
		for _, movie := range data.Results {
			items = append(items, forYouRecItem{mediaType: "movie", tmdbID: int64(movie.ID), adult: movie.Adult, voteAverage: movie.VoteAverage, voteCount: movie.VoteCount})
		}
		return items, nil
	case "tv":
		data, err := s.tmdb.GetTVRecommendations(ctx, id, params)
		if err != nil {
			return nil, fmt.Errorf("recommendations for series %d: %w", seed.tmdbID, err)
		}
		items := make([]forYouRecItem, 0, len(data.Results))
		for _, show := range data.Results {
			items = append(items, forYouRecItem{mediaType: "tv", tmdbID: int64(show.ID), adult: show.Adult, voteAverage: show.VoteAverage, voteCount: show.VoteCount})
		}
		return items, nil
	default:
		return nil, fmt.Errorf("recommendations for unknown media type %q", seed.mediaType)
	}
}

// forYouTrending fetches the week's trending list, the cold-start source.
func (s *Service) forYouTrending(ctx context.Context) ([]forYouRecItem, error) {
	params := map[string]string{"page": "1", "language": tmdb.DefaultLanguage, "region": tmdb.DefaultRegion}
	data, err := s.tmdb.GetAllTrending(ctx, "week", params)
	if err != nil {
		return nil, fmt.Errorf("trending of the week: %w", err)
	}
	items := make([]forYouRecItem, 0, len(data.Results))
	for _, item := range data.Results {
		switch {
		case item.Movie != nil:
			movie := item.Movie
			items = append(items, forYouRecItem{mediaType: "movie", tmdbID: int64(movie.ID), adult: movie.Adult, voteAverage: movie.VoteAverage, voteCount: movie.VoteCount})
		case item.TV != nil:
			show := item.TV
			items = append(items, forYouRecItem{mediaType: "tv", tmdbID: int64(show.ID), adult: show.Adult, voteAverage: show.VoteAverage, voteCount: show.VoteCount})
		}
	}
	return items, nil
}

// forYouSeedTitles hydrates the explanation seeds into display titles. A
// title TMDB cannot answer has no entry, so its reason is dropped rather
// than shown as a placeholder.
func (s *Service) forYouSeedTitles(ctx context.Context, seeds []forYouSeedKey) map[forYouSeedKey]string {
	titles := map[forYouSeedKey]string{}
	if len(seeds) == 0 {
		return titles
	}
	asking := make([]hydrate.Title, 0, len(seeds))
	for _, seed := range seeds {
		asking = append(asking, hydrate.Title{MediaType: seed.mediaType, TMDBID: int(seed.tmdbID)})
	}
	for i, card := range s.cards.Cards(ctx, asking) {
		if !card.Available {
			continue
		}
		titles[seeds[i]] = card.DisplayTitle()
	}
	return titles
}

// forYouLoadRatings reads the user's ratings for seeding and the watched-or-
// rated titles for exclusion: any watch or rating row hides its title,
// whatever the scope.
func (s *Service) forYouLoadRatings(ctx context.Context, userID int64) ([]forYouRating, map[forYouCandidateKey]bool, error) {
	rows, err := s.db.Read.QueryContext(ctx, `SELECT media_type, tmdb_id, scope, level, updated_at FROM ratings WHERE user_id = ?`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("load ratings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ratings []forYouRating
	excluded := map[forYouCandidateKey]bool{}
	for rows.Next() {
		var rating forYouRating
		var level sql.NullString
		var scope sql.NullString
		var tmdbID int64
		if err := rows.Scan(&rating.mediaType, &tmdbID, &scope, &level, &rating.updatedAt); err != nil {
			return nil, nil, err
		}
		rating.tmdbID = tmdbID
		if scope.Valid {
			rating.scope = scope.String
		}
		if level.Valid {
			rating.level = level.String
			ratings = append(ratings, rating)
		}
		excluded[forYouCandidateKey{mediaType: rating.mediaType, tmdbID: rating.tmdbID}] = true
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	watches, err := s.db.Read.QueryContext(ctx, `SELECT media_type, tmdb_id FROM watches WHERE user_id = ?`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("load watches: %w", err)
	}
	defer func() { _ = watches.Close() }()
	for watches.Next() {
		var mediaType string
		var tmdbID int64
		if err := watches.Scan(&mediaType, &tmdbID); err != nil {
			return nil, nil, err
		}
		excluded[forYouCandidateKey{mediaType: mediaType, tmdbID: tmdbID}] = true
	}
	if err := watches.Err(); err != nil {
		return nil, nil, err
	}
	return ratings, excluded, nil
}

// forYouExposureBody is the POST body: the run and the ranks shown. The user
// always comes from the session, never from the body.
type forYouExposureBody struct {
	RunID int64   `json:"runId"`
	Ranks []int64 `json:"ranks"`
}

// forYouExposureBodyLimitBytes fits a body of a few hundred ranks.
const forYouExposureBodyLimitBytes = 8 << 10

func (s *Service) handleExposures(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}
	var body forYouExposureBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, forYouExposureBodyLimitBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "body must be a JSON object of the documented fields: "+err.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "body must hold exactly one JSON object")
		return
	}
	if body.RunID <= 0 {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "runId must be a positive integer")
		return
	}
	seen := map[int64]bool{}
	var ranks []int64
	for _, rank := range body.Ranks {
		if rank <= 0 {
			respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "ranks must hold positive integers")
			return
		}
		if !seen[rank] {
			seen[rank] = true
			ranks = append(ranks, rank)
		}
	}
	// The run must exist, be completed, and belong to the session's user,
	// and a result must exist at every rank. Anything else answers 404 and
	// records nothing.
	var owner int64
	err := s.db.Read.QueryRowContext(r.Context(), `SELECT user_id FROM rec_runs WHERE id = ? AND completed_at IS NOT NULL`, body.RunID).Scan(&owner)
	if err != nil || owner != user.ID {
		respond.Error(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}
	rows, err := s.db.Read.QueryContext(r.Context(), `SELECT rank FROM rec_results WHERE run_id = ?`, body.RunID)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "exposures read failed", "error", err)
		respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	existing := map[int64]bool{}
	for rows.Next() {
		var rank int64
		if err := rows.Scan(&rank); err != nil {
			_ = rows.Close()
			s.logger.ErrorContext(r.Context(), "exposures read failed", "error", err)
			respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		existing[rank] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(r.Context(), "exposures read failed", "error", err)
		respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	for _, rank := range ranks {
		if !existing[rank] {
			respond.Error(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
	}
	now := s.now().Unix()
	for _, rank := range ranks {
		if err := RecordExposure(r.Context(), s.db.Write, body.RunID, rank, user.ID, now); err != nil {
			s.logger.ErrorContext(r.Context(), "exposure record failed", "error", err)
			respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
