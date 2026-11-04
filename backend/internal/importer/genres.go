package importer

import (
	"slices"
	"strings"

	"github.com/trick77/reworm/internal/lastfm"
)

// genreTags maps Last.fm tags (tagKey: lower case, "-" and "_" as spaces) to the app's genres (deck/names.go).
// Language tags ("german", "italian") are no genre on their own: Udo Lindenberg's top artist tags are
// Deutschrock 100, german 81, rock 48 (2026-11-04). "general" is never stored: it is every song (deck.General).
var genreTags = map[string][]string{
	"pop": {"pop", "europop", "dance pop", "synthpop", "synth pop", "electropop", "teen pop", "pop rock", "new wave",
		"power pop", "soft pop", "boy band", "girl group", "swedish pop"},
	"rock": {"rock", "classic rock", "soft rock", "rock n roll", "rock and roll", "rockabilly", "deutschrock",
		"glam rock", "progressive rock", "blues rock", "punk", "punk rock", "southern rock", "folk rock", "grunge",
		"psychedelic rock", "art rock", "arena rock", "aor"},
	"schlager": {"schlager", "deutschpop", "deutsch pop", "german pop", "neue deutsche welle", "ndw", "volksmusik",
		"volkstumliche musik", "volkstümliche musik", "liedermacher", "austropop", "partyschlager", "discofox",
		"mundart", "swiss pop"},
	"italo": {"italo disco", "italo pop", "italian pop", "italo", "cantautori", "cantautore", "canzone italiana",
		"musica italiana", "italo schlager"},
	"hip-hop": {"hip hop", "hiphop", "rap", "gangsta rap", "trap", "deutschrap", "german hip hop", "deutscher hip hop",
		"east coast rap", "west coast rap", "conscious hip hop"},
	"rnb-soul": {"rnb", "r&b", "r and b", "soul", "motown", "neo soul", "rhythm and blues", "northern soul",
		"contemporary r&b", "classic soul", "gospel"},
	"dance": {"dance", "electronic", "house", "techno", "trance", "eurodance", "edm", "electro", "dubstep",
		"drum and bass", "electronica", "big beat", "deep house", "electro house", "hands up", "eurotrance"},
	"disco-funk": {"disco", "funk", "nu disco", "hi nrg", "eurodisco", "euro disco", "boogie"},
	"indie": {"indie", "indie rock", "indie pop", "alternative", "alternative rock", "britpop", "post punk", "shoegaze",
		"new rave", "garage rock", "emo", "pop punk"},
	"metal": {"metal", "heavy metal", "hard rock", "thrash metal", "nu metal", "glam metal", "power metal",
		"death metal", "black metal", "metalcore", "alternative metal", "speed metal", "hair metal", "symphonic metal",
		"neue deutsche härte"},
	"country": {"country", "country pop", "americana", "bluegrass", "outlaw country", "alt country", "country rock"},
	"latin": {"latin", "latin pop", "reggaeton", "salsa", "bachata", "cumbia", "bossa nova", "samba", "flamenco",
		"cuban", "merengue", "latin rock", "son cubano", "tango", "mambo", "cha cha cha"},
	"reggae": {"reggae", "ska", "dancehall", "roots reggae", "dub", "rocksteady"},
	"chanson": {"chanson", "chanson francaise", "chanson française", "french pop", "variete francaise",
		"variété française", "french chanson", "nouvelle chanson"},
}

// genreOf is genreTags by tag.
var genreOf = func() map[string]string {
	m := map[string]string{}
	for g, tags := range genreTags {
		for _, t := range tags {
			m[t] = g
		}
	}
	return m
}()

// setGenres are the genres a card set stands for: every song of it has the genre, whatever its tags say.
var setGenres = map[string]string{"rock-anthems": "rock", "schlager-fest": "schlager"}

const (
	// maxGenres bounds the genres taken from tags: past the third a song's tags are noise ("70s", "abba").
	maxGenres = 3
	// trackTags is how many of a track's tags count: Last.fm sends its top five, heaviest first, no weights.
	trackTags = 5
	// artistMinCount is the weight an artist tag needs (100 = the heaviest). Udo Lindenberg: Deutschrock 100,
	// german 81, rock 48, singer-songwriter 36, deutsch 14, … pop 3 (2026-11-04): pop is no genre of his.
	artistMinCount = 30
)

// tagKey normalizes a Last.fm tag for genreOf.
func tagKey(tag string) string {
	s := strings.ToLower(strings.NewReplacer("-", " ", "_", " ").Replace(tag))
	return strings.Join(strings.Fields(s), " ")
}

// fromTags are the genres tags name, in tag order, at most maxGenres.
func fromTags(tags []lastfm.Tag, minCount int) []string {
	var out []string
	for _, t := range tags {
		if t.Count < minCount {
			continue
		}
		if g, ok := genreOf[tagKey(t.Name)]; ok && !slices.Contains(out, g) {
			out = append(out, g)
			if len(out) == maxGenres {
				break
			}
		}
	}
	return out
}

// trackGenres are the genres a track's top tags name: its top trackTags only, unweighted.
func trackGenres(tags []lastfm.Tag) []string { return fromTags(tags[:min(len(tags), trackTags)], 0) }

// genres is a song's genre list: its sets' genres and its track's (trackGenres), else those of its artist's top tags.
func genres(sets, track []string, artist []lastfm.Tag) []string {
	out := track
	if len(out) == 0 {
		out = fromTags(artist, artistMinCount)
	}
	return withSetGenres(out, sets)
}

// withSetGenres is gs with the genres of the sets added, sorted.
func withSetGenres(gs, sets []string) []string {
	out := slices.Clone(gs)
	for _, s := range sets {
		if g, ok := setGenres[s]; ok && !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	slices.Sort(out)
	return out
}

// knownGenre says g is a genre the import writes.
func knownGenre(g string) bool {
	_, ok := genreTags[g]
	return ok
}
