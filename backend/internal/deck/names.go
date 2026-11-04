package deck

import (
	"cmp"
	"strings"
)

// The import stores sets, categories and genres as slugs; these are their display names and list order (sets per
// plan D13, categories and genres as the mock lists them). A slug missing here is shown title-cased, after these.
var known = []struct{ id, name string }{
	{"essentials", "Essentials"},
	{"mega-mix", "Mega Mix"},
	{"rock-anthems", "Rock Anthems"},
	{"schlager-fest", "Schlager Fest"},
	{"sunshine", "Sunshine"},
	{"cheesy-classics", "Cheesy Classics"},
	{"party-starters", "Party Starters"},
	{"crowd-pleasers", "Crowd Pleasers"},
	{"screen-hits", "Screen Hits"},
	{"christmas-hits", "Christmas Hits"},
	{"radio-gold", "Radio Gold"},

	{"smash-hits", "Smash Hits"},
	{"number-ones", "Number ones"},
	{"evergreens", "Evergreens (pre-1950)"},
	{"christmas", "Christmas"},
	{"movie-tv", "Movie & TV"},
	{"summer", "Summer hits"},
	{"party", "Party classics"},
	{"swiss-charts", "Swiss charts"},
	{"eurovision", "Eurovision"},

	{"general", "General"},
	{"pop", "Pop"},
	{"rock", "Rock"},
	{"schlager", "Schlager & Deutschpop"},
	{"italo", "Italo-Schlager"},
	{"hip-hop", "Hip-Hop / Rap"},
	{"rnb-soul", "R&B / Soul"},
	{"dance", "Dance / Electronic"},
	{"disco-funk", "Disco & Funk"},
	{"indie", "Indie / Alternative"},
	{"metal", "Metal & Hard Rock"},
	{"country", "Country"},
	{"latin", "Latin"},
	{"reggae", "Reggae"},
	{"chanson", "French chanson"},
}

var rank = func() map[string]int {
	m := make(map[string]int, len(known))
	for i, k := range known {
		m[k.id] = i
	}
	return m
}()

// Name is the display name of a set, category or genre slug.
func Name(id string) string {
	if i, ok := rank[id]; ok {
		return known[i].name
	}
	words := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// order sorts known slugs in their list order, unknown ones after them by id.
func order(a, b string) int {
	ra, oka := rank[a]
	rb, okb := rank[b]
	switch {
	case oka && okb:
		return cmp.Compare(ra, rb)
	case oka:
		return -1
	case okb:
		return 1
	}
	return cmp.Compare(a, b)
}
