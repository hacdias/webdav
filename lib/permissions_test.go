package lib

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleMatchesFolding(t *testing.T) {
	t.Parallel()

	rule := &Rule{Path: "/pub/"}

	// Exact comparison: on a case-sensitive file system "/PUB/" is a different
	// directory, which a rule granting "/pub/" must not reach.
	require.True(t, rule.Matches("/pub/x.txt", false))
	require.False(t, rule.Matches("/PUB/x.txt", false))
	require.False(t, rule.Matches("/Pub/x.txt", false))

	// Folded they name one directory, so the rule governs both.
	require.True(t, rule.Matches("/pub/x.txt", true))
	require.True(t, rule.Matches("/PUB/x.txt", true))
	require.True(t, rule.Matches("/Pub/x.txt", true))

	// A sibling merely starting with the same characters stays unaffected.
	require.False(t, rule.Matches("/public/x.txt", true))
}

func TestRuleMatchesNormalization(t *testing.T) {
	t.Parallel()

	const (
		nfc = "/café/"  // café, composed
		nfd = "/café/" // café, decomposed
	)

	rule := &Rule{Path: nfc}

	// A file system ignoring case treats these spellings as one file too, so
	// folding has to normalize or the rule is evaded by retyping it.
	require.True(t, rule.Matches(nfd+"flag.txt", true))
	require.False(t, rule.Matches(nfd+"flag.txt", false))
}

func TestRuleMatchesCollectionFolding(t *testing.T) {
	t.Parallel()

	rule := &Rule{Path: "/c/"}

	require.True(t, rule.matchesCollection("/c", false))
	require.False(t, rule.matchesCollection("/C", false))
	require.True(t, rule.matchesCollection("/C", true))

	// A rule without a trailing slash names a resource, not a collection.
	require.False(t, (&Rule{Path: "/c"}).matchesCollection("/c", false))
}

func TestParentCollection(t *testing.T) {
	t.Parallel()

	require.Equal(t, "/data/", parentCollection("/data/sub"))
	require.Equal(t, "/data/", parentCollection("/data/sub/"))
	require.Equal(t, "/data/sub/", parentCollection("/data/sub/leaf.txt"))
	require.Equal(t, "/", parentCollection("/pub"))
	require.Equal(t, "/", parentCollection("/"))
}

func TestFlipCase(t *testing.T) {
	t.Parallel()

	alt, ok := flipCase("/srv/dav")
	require.True(t, ok)
	require.Equal(t, "/srv/Dav", alt)

	alt, ok = flipCase("/srv/DAV")
	require.True(t, ok)
	require.Equal(t, "/srv/dAV", alt)

	// Only the first letter flips, so a name whose case mapping does not
	// round-trip is left alone rather than changed by more than its case.
	alt, ok = flipCase("/srv/ıstanbul")
	require.True(t, ok)
	require.Equal(t, "/srv/ıStanbul", alt)

	// Nothing to flip.
	_, ok = flipCase("/srv/001")
	require.False(t, ok)
}

func TestRuleMatchesRegexFolding(t *testing.T) {
	t.Parallel()

	rule := &Rule{Regex: regexp.MustCompile("^/secret/")}

	require.True(t, rule.Matches("/secret/flag.txt", false))
	require.False(t, rule.Matches("/SECRET/flag.txt", false))

	// Where the file system serves both spellings as one file, the rule has to
	// cover both or it denies nothing.
	require.True(t, rule.Matches("/SECRET/flag.txt", true))

	// A pattern written with upper case still relies on the path as written, so
	// folding never takes a match away.
	upper := &Rule{Regex: regexp.MustCompile("^/Secret/")}
	require.True(t, upper.Matches("/Secret/flag.txt", true))
	require.False(t, upper.Matches("/public/flag.txt", true))
}
