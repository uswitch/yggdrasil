// this lines of codes are copy and modified from contour project with Apache License 2.0

package envoy

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	v1 "k8s.io/api/networking/v1"
)

type Route struct {

	// PathMatchCondition specifies a MatchCondition to match on the request path.
	// Must not be nil.
	PathMatchCondition MatchCondition
}
type MatchCondition interface {
	fmt.Stringer
}

// PrefixMatchType represents different types of prefix matching alternatives.
type PrefixMatchType int

const (
	// PrefixMatchString represents a prefix match that functions like a
	// string prefix match, i.e. prefix /foo matches /foobar
	PrefixMatchString PrefixMatchType = iota
	// PrefixMatchSegment represents a prefix match that only matches full path
	// segments, i.e. prefix /foo matches /foo/bar but not /foobar
	PrefixMatchSegment
)

var prefixMatchTypeToName = map[PrefixMatchType]string{
	PrefixMatchString:  "string",
	PrefixMatchSegment: "segment",
}

// PrefixMatchCondition matches the start of a URL.
type PrefixMatchCondition struct {
	Prefix          string
	PrefixMatchType PrefixMatchType
}

func (ec *ExactMatchCondition) String() string {
	return "exact: " + ec.Path
}

// ExactMatchCondition matches the entire path of a URL.
type ExactMatchCondition struct {
	Path string
}

// RegexMatchCondition matches the URL by regular expression.
type RegexMatchCondition struct {
	Regex string
}

func httppaths(rule v1.IngressRule) []v1.HTTPIngressPath {
	if rule.IngressRuleValue.HTTP == nil {
		// rule.IngressRuleValue.HTTP value is optional.
		return nil
	}
	return rule.IngressRuleValue.HTTP.Paths
}

func stringOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func derefPathTypeOr(ptr *v1.PathType, def v1.PathType) v1.PathType {
	if ptr != nil {
		return *ptr
	}
	return def
}

func Pathtranslate(path string, pathtype v1.PathType) *Route {
	r := &Route{
		PathMatchCondition: nil,
	}
	switch pathtype {
	case v1.PathTypePrefix:
		prefixMatchType := PrefixMatchSegment
		// An "all paths" prefix should be treated as a generic string prefix
		// match.
		if path == "/" {
			prefixMatchType = PrefixMatchString
		} else {
			// Strip trailing slashes. Ensures /foo matches prefix /foo/
			path = strings.TrimRight(path, "/")
		}
		r.PathMatchCondition = &PrefixMatchCondition{Prefix: path, PrefixMatchType: prefixMatchType}
	case v1.PathTypeExact:
		r.PathMatchCondition = &ExactMatchCondition{Path: path}
	case v1.PathTypeImplementationSpecific:
		// If a path "looks like" a regex we give a regex path match.
		// Otherwise you get a string prefix match.
		if strings.ContainsAny(path, "^+*[]%") {
			// validate the regex
			if err := ValidateRegex(path); err != nil {
				return nil
			}
			r.PathMatchCondition = &RegexMatchCondition{Regex: path}
		} else {
			r.PathMatchCondition = &PrefixMatchCondition{Prefix: path, PrefixMatchType: PrefixMatchString}
		}
	}
	return r
}

// RouteMatch creates a *envoy_route_v3.RouteMatch for the supplied *dag.Route.
func RouteMatch(route *Route) *envoy_route_v3.RouteMatch {
	switch c := route.PathMatchCondition.(type) {
	case *RegexMatchCondition:
		return &envoy_route_v3.RouteMatch{
			PathSpecifier: &envoy_route_v3.RouteMatch_SafeRegex{
				// Add an anchor since we at the very least have a / as a string literal prefix.
				// Reduces regex program size so Envoy doesn't reject long prefix matches.
				SafeRegex: SafeRegexMatch("^" + c.Regex),
			},
		}
	case *PrefixMatchCondition:
		switch c.PrefixMatchType {
		case PrefixMatchSegment:
			return &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_PathSeparatedPrefix{
					PathSeparatedPrefix: c.Prefix,
				},
			}
		case PrefixMatchString:
			fallthrough
		default:
			return &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
					Prefix: c.Prefix,
				},
			}
		}
	case *ExactMatchCondition:
		return &envoy_route_v3.RouteMatch{
			PathSpecifier: &envoy_route_v3.RouteMatch_Path{
				Path: c.Path,
			},
		}
	default:
		return &envoy_route_v3.RouteMatch{}
	}
}

// ValidateRegex returns an error if the supplied
// RE2 regex syntax is invalid.
func ValidateRegex(regex string) error {
	_, err := regexp.Compile(regex)
	return err
}

func (rc *RegexMatchCondition) String() string {
	return "regex: " + rc.Regex
}

func (pc *PrefixMatchCondition) String() string {
	str := "prefix: " + pc.Prefix
	if typeStr, ok := prefixMatchTypeToName[pc.PrefixMatchType]; ok {
		str += " type: " + typeStr
	}
	return str
}

func stringTohash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
