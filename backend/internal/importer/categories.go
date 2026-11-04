package importer

import "github.com/trick77/reworm/internal/song"

// categoryRules decide the categories (plan step 7), one rule each, in deck/names.go order. They run on the
// dataset as written, overrides applied, so an override year counts.
var categoryRules = []struct {
	slug string
	in   func(song.Song) bool
}{
	// What a kid born 2010 or later grew up with. No upper bound: every newer song joins.
	{"gen-alpha", func(s song.Song) bool { return s.Year >= 2016 }},
}

// categories are the slugs of the rules s passes.
func categories(s song.Song) []string {
	var out []string
	for _, r := range categoryRules {
		if r.in(s) {
			out = append(out, r.slug)
		}
	}
	return out
}

// categorize sets every song's categories from the rules, replacing what a previous dataset had.
func categorize(rs []resolved) {
	for i := range rs {
		rs[i].Song.Categories = categories(rs[i].Song)
	}
}
