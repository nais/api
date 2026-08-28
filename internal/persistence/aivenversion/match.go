// Package aivenversion resolves the version string Aiven reports for a service to the
// GraphQL enum value that names it. OpenSearch and Valkey declare separate enums but
// answer the same question, so the rule lives here rather than in both.
package aivenversion

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Match picks the enum value that names the version Aiven reports.
//
// Enum names carry the version they stand for: "V3_6" is 3.6, and a bare "V2" is major
// 2 with no minor of its own. Matching never crosses a major, because a major bump is
// never a silent equivalence. Within one major:
//   - an exact minor match wins;
//   - where two or more minors are declared, a reported minor between two of them
//     resolves to the lower, and one outside their range to the closest;
//   - a bare VX catches reported minors below the VX_Y it upgrades to.
//
// Anything left over is an error, so the API never claims a version it cannot name.
func Match[T ~string, U ~[]T](reported string, declared []T, upgradesTo map[T]U) (T, error) {
	var zero T

	major, minor, err := parseReported(reported)
	if err != nil {
		return zero, err
	}

	var withMinor []version[T]
	var bare *version[T]
	for _, value := range declared {
		v, err := parseDeclared(value)
		if err != nil {
			return zero, err
		}
		if v.major != major {
			continue
		}
		if v.hasMinor {
			withMinor = append(withMinor, v)
		} else {
			bare = &v
		}
	}

	slices.SortFunc(withMinor, func(a, b version[T]) int { return a.minor - b.minor })

	for _, v := range withMinor {
		if v.minor == minor {
			return v.value, nil
		}
	}

	// A lone declared minor is a point, not a range, so there is nothing to interpolate
	// between or fall off the end of.
	if len(withMinor) >= 2 {
		lowest, highest := withMinor[0], withMinor[len(withMinor)-1]
		switch {
		case minor < lowest.minor:
			return lowest.value, nil
		case minor > highest.minor:
			return highest.value, nil
		}
		for i := len(withMinor) - 1; i >= 0; i-- {
			if withMinor[i].minor < minor {
				return withMinor[i].value, nil
			}
		}
	}

	if bare != nil {
		if boundary, ok := bareBoundary(*bare, upgradesTo); ok && minor < boundary {
			return bare.value, nil
		}
	}

	return zero, fmt.Errorf("unsupported Aiven version: %q", reported)
}

// MatchPin resolves the version pinned in a Kubernetes resource. A pin is written by this
// API rather than reported by Aiven, so it is matched against the declared versions
// directly instead of through Match. Older releases pinned a bare major such as "2", so
// both that shape and the current "2.19" occur in the wild.
func MatchPin[T ~string](pin string, declared []T, aivenString func(T) (string, error)) (T, error) {
	var zero T

	for _, value := range declared {
		s, err := aivenString(value)
		if err == nil && s == pin {
			return value, nil
		}
	}

	// A bare VX and the VX_Y sharing its Aiven version are one version under two names, so
	// the qualified spelling is what callers see.
	if bare := T("V" + pin); slices.Contains(declared, bare) {
		if want, err := aivenString(bare); err == nil {
			for _, value := range declared {
				if value == bare {
					continue
				}
				if v, err := parseDeclared(value); err != nil || !v.hasMinor {
					continue
				}
				if s, err := aivenString(value); err == nil && s == want {
					return value, nil
				}
			}
		}
		return bare, nil
	}

	return zero, fmt.Errorf("unsupported pinned version: %q", pin)
}

type version[T ~string] struct {
	value    T
	major    int
	minor    int
	hasMinor bool
}

// bareBoundary returns the minor below which a bare VX claims a reported version. VX
// has no minor itself, so the VX_Y it upgrades to supplies the boundary.
func bareBoundary[T ~string, U ~[]T](bare version[T], upgradesTo map[T]U) (int, bool) {
	boundary := -1
	for _, target := range upgradesTo[bare.value] {
		v, err := parseDeclared(target)
		if err != nil || !v.hasMinor || v.major != bare.major {
			continue
		}
		if boundary < 0 || v.minor < boundary {
			boundary = v.minor
		}
	}
	return boundary, boundary >= 0
}

func parseDeclared[T ~string](value T) (version[T], error) {
	majorPart, minorPart, hasMinor := strings.Cut(strings.TrimPrefix(string(value), "V"), "_")

	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return version[T]{}, fmt.Errorf("parsing major version from %q: %w", value, err)
	}

	v := version[T]{value: value, major: major, hasMinor: hasMinor}
	if hasMinor {
		if v.minor, err = strconv.Atoi(minorPart); err != nil {
			return version[T]{}, fmt.Errorf("parsing minor version from %q: %w", value, err)
		}
	}
	return v, nil
}

func parseReported(reported string) (int, int, error) {
	parts := strings.SplitN(reported, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unsupported Aiven version %q: no minor version", reported)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing major version from %q: %w", reported, err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing minor version from %q: %w", reported, err)
	}

	return major, minor, nil
}
