package charts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// dePage is trimmed from "Liste der Nummer-eins-Hits in der Schweiz (2024)": singles, albums (ignored), year-end.
const dePage = `Die Liste …
== Singles ==
{{Nummer-eins-Hits
| Art = Singles
| Land = Schweiz
| Inhalt =
  {{Nummer-eins-Hits Zeile
  | Chartein = 2023-12-03
  | Titel = [[All I Want for Christmas Is You]]
  | Interpret = [[Mariah Carey]]
  | Info =
  }}
  {{Nummer-eins-Hits Zeile
  | Chartein = 2024-01-07
  | Titel = [[Wonderful Life (Hurts-Lied)|Wonderful Life]]
  | Interpret = [[6PM Records]] feat. [[Hurts]], [[Luciano (Rapper)|Luciano]] & [[Sira (Musikproduzent)|Sira]]
  }}
  {{Nummer-eins-Hits Zeile
  | Titel = Greedy<ref>{{Internetquelle |url=https://example.org |titel=x}}</ref>
  | Interpret = [[Tate McRae]]<!-- comment -->
  }}
  {{Nummer-eins-Hits Zeile
  | Titel = Greedy
  | Interpret = [[Tate McRae]]
  }}
}}
== Alben ==
{{Nummer-eins-Hits
| Art = Alben
| Inhalt =
  {{Nummer-eins-Hits Zeile
  | Titel = [[The Tortured Poets Department]]
  | Interpret = [[Taylor Swift]]
  }}
}}
== Jahreshitparaden ==
{{Nummer-eins-Hits
| Art = Jahr
| Inhalt =
 {{Jahreshitparaden Zeile
 | Position = 1
 | Single = [[Beautiful Things]]
 | InterpretS = [[Benson Boone]]
 | Album = [[The Tortured Poets Department]]
 | InterpretA = [[Taylor Swift]]
 }}
 {{Jahreshitparaden Zeile
 | Position = 2
 | Single = {{lang|es|Gata Only}}
 | InterpretS = [[Floyymenor]] &amp; [[Cris MJ]]
 }}
}}
== Weblinks ==
`

// billboardPage is trimmed from "Billboard Year-End Hot 100 singles of 2025".
const billboardPage = `{{Short description|Ranking of recorded music}}
{| class="wikitable sortable" style="text-align: center"
|+ List of songs on ''[[Billboard (magazine)|Billboard]]''{{'s}} 2025 Year-End Hot 100 chart<ref>{{cite web|url=x}}</ref>
|-
! scope="col" | {{Abbr|No.|Number}}
! scope="col" | Title
! scope="col" | Artist(s)
|-
| scope="row" | 1
| "[[Die with a Smile]]" || [[Lady Gaga]] and [[Bruno Mars]]
|-
| scope="row" | 2
| "[[Luther (song)|Luther]]" || [[Kendrick Lamar]] and [[SZA]]
|-
| scope="row" | 3
| "Messy"
| [[Lola Young]]
|-
| scope="row" | 8
| "[[I Had Some Help]]" || [[Post Malone]] featuring [[Morgan Wallen]]
|-
| scope="row" | 19
| "[[Saturn (SZA song)|Saturn]]" || rowspan="2"| [[SZA]]
|-
| scope="row" | 20
| "[[Snooze (SZA song)|Snooze]]"
|-
| scope="row" | 21
| "Alone"
|}
== See also ==
`

// ukPage is trimmed from "List of UK top-ten singles in 2025": the key and the per-artist table are no singles.
const ukPage = `==Background==
{| class="wikitable"
|-
| [[Alex Warren]]
| 4
| "[[Ordinary (Alex Warren song)|Ordinary]]"
| 1
|}
==Top-ten singles==
{| class="wikitable"
!Symbol
|-
| bgcolor="lightblue" |‡
|Single peaked in 2024 but still in chart in 2025.
|}
{| class="wikitable sortable" style="text-align:center;"
! Entered <br>(week ending)
! Single
|-
! colspan="9" |Singles in 2024
|-
| {{dts|format=dmy|2024|02|15}}
| 34
| align="left" bgcolor="lightblue"|"[[Beautiful Things (Benson Boone song)|Beautiful Things]]" ‡ <small>(#7)</small> {{ref label|Beautiful|HH|hh}}
| align="left" |[[Benson Boone]]
| 1
| {{dts|format=dmy|2024|03|28}}
| 2
|-
| rowspan="2" |{{dts|format=dmy|2024|12|05}}
| 3
| align="left"|"[[Lose Control (Teddy Swims song)|Lose Control]]"
| align="left" rowspan="2"|[[Teddy Swims]]
| 4
|-
| 2
| align="left"|"The Door"
| 8
|-
| 5
| align="left"|"[[Die with a Smile]]" ‡ <small>(#10)</small>
| align="left"| [[Lady Gaga]] & [[Bruno Mars]]
| 2
|-
| 6
| align="left" bgcolor="#ddffdd"|"[[Fairytale of New York]]" ♦ {{ref label|Fairytale|K|k}}
| align="left" |{{sort|Pogues|[[The Pogues]]}} <small> featuring [[Kirsty MacColl]] </small>
| 5
|}
==Entries by artist==
{| class="wikitable"
|-
| [[Doechii]]
| 2
| "[[Denial is a River]]"
| 9
|}
`

func TestParse_ukTopTen(t *testing.T) {
	got := Parse(UK, ukPage)
	want := []Entry{
		{Artist: "Benson Boone", Title: "Beautiful Things", Source: UK},
		{Artist: "Teddy Swims", Title: "Lose Control", Source: UK},
		{Artist: "Teddy Swims", Title: "The Door", Source: UK},
		{Artist: "Lady Gaga & Bruno Mars", Title: "Die with a Smile", Source: UK},
		{Artist: "The Pogues featuring Kirsty MacColl", Title: "Fairytale of New York", Source: UK},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Parse = %#v\nwant %#v", got, want)
	}
}

func TestParse_dePageSinglesAndYearEnd(t *testing.T) {
	got := Parse(CH, dePage)
	want := []Entry{
		{Artist: "Mariah Carey", Title: "All I Want for Christmas Is You", Source: CH},
		{Artist: "6PM Records feat. Hurts, Luciano & Sira", Title: "Wonderful Life", Source: CH},
		{Artist: "Tate McRae", Title: "Greedy", Source: CH},
		{Artist: "Benson Boone", Title: "Beautiful Things", Source: CH},
		{Artist: "Floyymenor & Cris MJ", Title: "Gata Only", Source: CH},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Parse = %#v\nwant %#v", got, want)
	}
}

func TestParse_billboardTable(t *testing.T) {
	got := Parse(Billboard, billboardPage)
	want := []Entry{
		{Artist: "Lady Gaga and Bruno Mars", Title: "Die with a Smile", Source: Billboard},
		{Artist: "Kendrick Lamar and SZA", Title: "Luther", Source: Billboard},
		{Artist: "Lola Young", Title: "Messy", Source: Billboard},
		{Artist: "Post Malone featuring Morgan Wallen", Title: "I Had Some Help", Source: Billboard},
		{Artist: "SZA", Title: "Saturn", Source: Billboard},
		{Artist: "SZA", Title: "Snooze", Source: Billboard},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Parse = %#v\nwant %#v", got, want)
	}
}

func TestPages_namesTheFourListsOfAYear(t *testing.T) {
	var titles []string
	for _, p := range Pages(2024) {
		titles = append(titles, p.Source+": "+p.Title)
	}
	want := []string{
		"ch: Liste der Nummer-eins-Hits in der Schweiz (2024)",
		"de: Liste der Nummer-eins-Hits in Deutschland (2024)",
		"at: Liste der Nummer-eins-Hits in Österreich (2024)",
		"billboard: Billboard Year-End Hot 100 singles of 2024",
		"uk: List of UK top-ten singles in 2024",
	}
	if !slices.Equal(titles, want) {
		t.Fatalf("Pages = %q", titles)
	}
}

func TestFetch_readsTheWikitextAndRefusesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("action") != "parse" || q.Get("prop") != "wikitext" || q.Get("formatversion") != "2" ||
			r.Header.Get("User-Agent") != "Reworm/test" {
			t.Errorf("query %v, user agent %q", q, r.Header.Get("User-Agent"))
		}
		switch q.Get("page") {
		case "Good":
			_ = json.NewEncoder(w).Encode(map[string]any{"parse": map[string]any{"wikitext": "text"}})
		case "Missing":
			_, _ = fmt.Fprint(w, `{"error":{"code":"missingtitle","info":"The page you specified doesn't exist."}}`)
		default:
			http.Error(w, "You are making too many requests to the API.", http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	got, err := Fetch(ctx, srv.Client(), "Reworm/test", srv.URL, "Good")
	if err != nil || got != "text" {
		t.Fatalf("Fetch = %q, %v", got, err)
	}
	if _, err := Fetch(ctx, srv.Client(), "Reworm/test", srv.URL, "Missing"); err == nil || !strings.Contains(err.Error(), "missingtitle") {
		t.Fatalf("missing page: %v", err)
	}
	if _, err := Fetch(ctx, srv.Client(), "Reworm/test", srv.URL, "Busy"); err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("rate limited: %v", err)
	}
}

func TestSnapshot_roundTripsAndDedupes(t *testing.T) {
	dir := t.TempDir()
	es := []Entry{
		{Artist: "Teddy Swims", Title: "Lose Control", Source: CH},
		{Artist: "Teddy Swims", Title: "Lose Control", Source: CH},
		{Artist: "Teddy Swims", Title: "Lose Control", Source: Billboard},
		{Artist: "Benson Boone", Title: "Beautiful Things", Source: DE},
	}
	if err := WriteYear(dir, 2024, es); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "2024.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "artist,title,year,source\nTeddy Swims,Lose Control,2024,ch\nTeddy Swims,Lose Control,2024,billboard\n" +
		"Benson Boone,Beautiful Things,2024,de\n"; string(b) != want {
		t.Fatalf("snapshot:\n%s", b)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0] != (Row{Entry: es[0], Year: 2024}) || rows[2].Source != DE {
		t.Fatalf("ReadAll = %#v", rows)
	}
	if rows, err := ReadAll(filepath.Join(dir, "none")); err != nil || rows != nil {
		t.Fatalf("no dir: %v, %v", rows, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2023.csv"), []byte("artist,title\nx,y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAll(dir); err == nil {
		t.Fatal("a broken snapshot must fail")
	}
}

func TestYears_parsesARange(t *testing.T) {
	for in, want := range map[string][]int{"2023-2025": {2023, 2024, 2025}, "2024": {2024}} {
		got, err := Years(in)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("Years(%q) = %v, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "2025-2023", "x", "1700-1800", "2023-"} {
		if _, err := Years(in); err == nil {
			t.Errorf("Years(%q) must fail", in)
		}
	}
}
