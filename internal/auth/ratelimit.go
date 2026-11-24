package auth

import (
	"net/netip"
	"sync"
	"time"
)

const (
	// LoginRateWindow is the sliding window both login limits count over.
	LoginRateWindow = 15 * time.Minute
	// LoginAttemptsPerIdentifier bounds guesses at one account from one address.
	LoginAttemptsPerIdentifier = 10
	// LoginAttemptsPerAddress bounds one address across all identifiers, so
	// spreading guesses over many names does not multiply the budget.
	LoginAttemptsPerAddress = 30
)

// loginRateLimiter keeps, per client address and per (address, identifier),
// the times of the admitted attempts still inside the window. It lives in
// memory: a restart forgets it, which costs an attacker a restart they cannot
// trigger.
type loginRateLimiter struct {
	mu        sync.Mutex
	byPair    map[string][]time.Time
	byAddress map[string][]time.Time
	lastSweep time.Time
}

func newLoginRateLimiter() *loginRateLimiter {
	return &loginRateLimiter{byPair: map[string][]time.Time{}, byAddress: map[string][]time.Time{}}
}

// admit records an attempt and reports whether it may go ahead. A refused
// attempt is not recorded, so a client hammering at the limit does not keep
// pushing its own lockout further out. retryAfter is how long until the
// tightest limit that refused it frees a slot.
func (l *loginRateLimiter) admit(client netip.Addr, identifier string, now time.Time) (retryAfter time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepIfDue(now)

	addressKey := client.String()
	pairKey := addressKey + "\x00" + identifier
	addressHits := attemptsInsideWindow(l.byAddress[addressKey], now)
	pairHits := attemptsInsideWindow(l.byPair[pairKey], now)
	if len(addressHits) >= LoginAttemptsPerAddress {
		retryAfter = addressHits[0].Add(LoginRateWindow).Sub(now)
	}
	if len(pairHits) >= LoginAttemptsPerIdentifier {
		retryAfter = max(retryAfter, pairHits[0].Add(LoginRateWindow).Sub(now))
	}
	if retryAfter > 0 {
		return retryAfter, false
	}
	l.byAddress[addressKey] = append(addressHits, now)
	l.byPair[pairKey] = append(pairHits, now)
	return 0, true
}

// sweepIfDue drops every key with no attempt left inside the window, at most
// once per window, so the maps hold only what the last window saw.
func (l *loginRateLimiter) sweepIfDue(now time.Time) {
	if now.Sub(l.lastSweep) < LoginRateWindow {
		return
	}
	l.lastSweep = now
	for _, attempts := range []map[string][]time.Time{l.byAddress, l.byPair} {
		for key, hits := range attempts {
			if len(attemptsInsideWindow(hits, now)) == 0 {
				delete(attempts, key)
			}
		}
	}
}

// attemptsInsideWindow returns the suffix of hits (oldest first) that is
// younger than the window.
func attemptsInsideWindow(hits []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-LoginRateWindow)
	for i, hit := range hits {
		if hit.After(cutoff) {
			return hits[i:]
		}
	}
	return nil
}
