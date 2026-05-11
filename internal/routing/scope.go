package routing

import (
	"context"
	"strings"
)

const (
	// GinPathRouteContextKey stores the resolved path-route context in gin.Context.
	GinPathRouteContextKey = "cliproxy.path_route"
)

// PathRouteContext captures request-scoped channel-group routing derived from the URL path.
type PathRouteContext struct {
	RoutePath string
	Group     string
	Fallback  string
}

type pathRouteContextKey struct{}

// WithPathRouteContext returns a child context tagged with the resolved path-route scope.
func WithPathRouteContext(ctx context.Context, route *PathRouteContext) context.Context {
	if route == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cloned := *route
	return context.WithValue(ctx, pathRouteContextKey{}, &cloned)
}

// PathRouteContextFromContext extracts the resolved path-route scope from context.
func PathRouteContextFromContext(ctx context.Context) *PathRouteContext {
	if ctx == nil {
		return nil
	}
	raw := ctx.Value(pathRouteContextKey{})
	route, _ := raw.(*PathRouteContext)
	if route == nil {
		return nil
	}
	cloned := *route
	return &cloned
}

// NormalizeGroupName trims, lowercases, and canonicalizes channel group names.
func NormalizeGroupName(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.Trim(trimmed, "/")
	return trimmed
}

// NormalizeFallback canonicalizes fallback values. Empty defaults to "none".
func NormalizeFallback(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none":
		return "none"
	case "default":
		return "default"
	default:
		return "none"
	}
}

// ParseNormalizedSet splits a comma-separated string into a normalized set.
func ParseNormalizedSet(raw string, normalizer func(string) string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if normalizer != nil {
			value = normalizer(value)
		}
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
