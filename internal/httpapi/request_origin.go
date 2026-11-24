package httpapi

import (
	"net/http"
	"net/netip"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
)

// peerIsTrustedProxy reports whether the TCP peer is inside one of the
// trusted networks. Only such a peer's forwarding headers are read: anyone
// else could write any address into them.
func peerIsTrustedProxy(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	addr := peer.Addr().Unmap()
	return addr, addrInPrefixes(addr, trusted)
}

func addrInPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// clientAddress is the address the login limits count against. From a
// trusted proxy it is the rightmost X-Forwarded-For entry outside the trusted
// networks: entries to its left were supplied by the browser and prove
// nothing. From anyone else it is the TCP peer, whatever the headers say.
func clientAddress(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer, viaProxy := peerIsTrustedProxy(r, trusted)
	if !viaProxy {
		return peer
	}
	entries := forwardedListEntries(r.Header.Values("X-Forwarded-For"))
	for i := len(entries) - 1; i >= 0; i-- {
		addr, ok := parseForwardedAddr(entries[i])
		if !ok {
			return peer
		}
		if !addrInPrefixes(addr, trusted) || i == 0 {
			return addr
		}
	}
	return peer
}

// requestIsHTTPS decides the cookie's Secure flag: TLS on this connection, or
// a trusted proxy saying the browser's connection was HTTPS.
func requestIsHTTPS(r *http.Request, trusted []netip.Prefix) bool {
	if r.TLS != nil {
		return true
	}
	if _, viaProxy := peerIsTrustedProxy(r, trusted); !viaProxy {
		return false
	}
	entries := forwardedListEntries(r.Header.Values("X-Forwarded-Proto"))
	return len(entries) > 0 && strings.EqualFold(entries[len(entries)-1], "https")
}

func forwardedListEntries(values []string) []string {
	var entries []string
	for _, v := range values {
		for _, item := range strings.Split(v, ",") {
			if item = strings.TrimSpace(item); item != "" {
				entries = append(entries, item)
			}
		}
	}
	return entries
}

func parseForwardedAddr(entry string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddr(entry); err == nil {
		return addr.Unmap(), true
	}
	if addrPort, err := netip.ParseAddrPort(entry); err == nil {
		return addrPort.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

// requireAllowedOrigin refuses any request but GET and HEAD whose Origin, or
// failing that Referer, is not one of the configured origins. A browser sends
// one of the two on every cross-site form post and fetch, so a page on another
// site cannot make a logged-in browser change anything here. A request with
// neither is refused too: nothing legitimate here omits both.
func requireAllowedOrigin(allowed []string, next http.Handler) http.Handler {
	allowedSet := make(map[string]bool, len(allowed))
	for _, origin := range allowed {
		allowedSet[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if !allowedSet[requestOrigin(r)] {
			writeStatusError(w, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requestOrigin is the normalised Origin, or the origin of the Referer when
// there is no Origin; "" when neither is usable. A present Origin decides
// alone, so a forged Referer cannot rescue a foreign one.
func requestOrigin(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		normalized, err := config.NormalizeOrigin(origin)
		if err != nil {
			return ""
		}
		return normalized
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		normalized, err := config.OriginOfURL(referer)
		if err != nil {
			return ""
		}
		return normalized
	}
	return ""
}
