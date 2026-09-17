package lambda

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

const (
	headerOrigin           = "Origin"
	headerRequestMethod    = "Access-Control-Request-Method"
	headerRequestHeaders   = "Access-Control-Request-Headers"
	headerAllowOrigin      = "Access-Control-Allow-Origin"
	headerAllowMethods     = "Access-Control-Allow-Methods"
	headerAllowHeaders     = "Access-Control-Allow-Headers"
	headerExposeHeaders    = "Access-Control-Expose-Headers"
	headerMaxAge           = "Access-Control-Max-Age"
	headerAllowCredentials = "Access-Control-Allow-Credentials"
	corsWildcard           = "*"
)

// isCORSPreflight reports whether r is a CORS preflight: an OPTIONS request
// that names its origin and the method it asks about.
func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get(headerOrigin) != "" && r.Header.Get(headerRequestMethod) != ""
}

// preflightAllowed reports whether the configured CORS policy admits the
// origin, method and request headers a preflight asks about.
func preflightAllowed(cors *FunctionURLCORS, r *http.Request) bool {
	if _, ok := allowedOrigin(cors, r.Header.Get(headerOrigin)); !ok {
		return false
	}

	if !corsListAllows(cors.AllowMethods, r.Header.Get(headerRequestMethod)) {
		return false
	}

	for _, name := range strings.Split(r.Header.Get(headerRequestHeaders), ",") {
		if name = strings.TrimSpace(name); name != "" && !corsListAllows(cors.AllowHeaders, name) {
			return false
		}
	}

	return true
}

// allowedOrigin returns the Access-Control-Allow-Origin value for origin: "*"
// when the policy allows any origin, the origin itself when it is listed.
func allowedOrigin(cors *FunctionURLCORS, origin string) (string, bool) {
	switch {
	case origin == "":
		return "", false
	case slices.Contains(cors.AllowOrigins, corsWildcard):
		return corsWildcard, true
	case slices.Contains(cors.AllowOrigins, origin):
		return origin, true
	}

	return "", false
}

// corsListAllows reports whether allowed lists value (case-insensitively) or
// the wildcard.
func corsListAllows(allowed []string, value string) bool {
	for _, item := range allowed {
		if item == corsWildcard || strings.EqualFold(item, value) {
			return true
		}
	}

	return false
}

// writeCORSPreflight answers an allowed preflight with the configured policy
// and an empty body; the function is not invoked.
func writeCORSPreflight(w http.ResponseWriter, cors *FunctionURLCORS, origin string) {
	h := w.Header()
	addCORSHeaders(h, cors, origin)
	setJoinedHeader(h, headerAllowMethods, cors.AllowMethods)
	setJoinedHeader(h, headerAllowHeaders, cors.AllowHeaders)

	if cors.MaxAge > 0 {
		h.Set(headerMaxAge, strconv.Itoa(cors.MaxAge))
	}

	w.WriteHeader(http.StatusOK)
}

// addCORSHeaders adds the response-side CORS headers for an allowed origin
// without replacing headers the function sets itself: AWS returns both.
func addCORSHeaders(h http.Header, cors *FunctionURLCORS, origin string) {
	value, ok := allowedOrigin(cors, origin)
	if !ok {
		return
	}

	h.Add(headerAllowOrigin, value)

	if value != corsWildcard {
		h.Add("Vary", headerOrigin)
	}

	if len(cors.ExposeHeaders) > 0 {
		h.Add(headerExposeHeaders, strings.Join(cors.ExposeHeaders, ","))
	}

	if cors.AllowCredentials {
		h.Add(headerAllowCredentials, "true")
	}
}

func setJoinedHeader(h http.Header, key string, values []string) {
	if len(values) > 0 {
		h.Set(key, strings.Join(values, ","))
	}
}
