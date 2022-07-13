package envoy

import (
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
)

// SafeRegexMatch returns a matcher.RegexMatcher for the supplied regex.
// SafeRegexMatch does not escape regex meta characters.
func SafeRegexMatch(regex string) *matcher.RegexMatcher {
	return &matcher.RegexMatcher{
		EngineType: &matcher.RegexMatcher_GoogleRe2{
			GoogleRe2: &matcher.RegexMatcher_GoogleRE2{},
		},
		Regex: regex,
	}
}
