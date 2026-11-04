# Import report

Written by `cmd/import` from `data/cards/*.csv`; rerun to refresh. The card year is the song's year (card sets
always win); MusicBrainz and Apple years are only compared. MusicBrainz: recordings first released by the year
after the card's, the earliest official one wins. Apple, per storefront ch → de → us: ISRC lookup (title and
artist must match), in ch also the MusicBrainz original album by name; the first original-album track wins,
else the first ISRC hit, else an artist + title search in ch and us. Only tracks with a preview count. Songs
without one stay in the dataset (no clip until an override). Last.fm, by the card's artist + title, the main
artist and the MusicBrainz credit: the most listened track gives the listeners and, by its top tags, the
genres (else the artist's top tags); Rock Anthems and Schlager Fest add their genre.

## Totals

| | count |
|---|---:|
| cards | 3036 |
| cards without a year (dropped) | 0 |
| songs | 2757 |
| card spellings merged into one recording | 27 |
| MusicBrainz matched | 2656 |
| MusicBrainz matched, no low-confidence note | 2115 |
| Apple track with preview | 2663 |
| no Apple preview | 94 |
| Apple track on the original album | 1853 |
| cover: Apple (original album) | 1853 |
| cover: Cover Art Archive | 627 |
| cover: none | 277 |
| Last.fm track found | 2718 |
| no genre | 195 |
| overrides applied | 0 |

## Sets

| set | cards | no year | songs | MusicBrainz | Apple preview |
|---|---:|---:|---:|---:|---:|
| cheesy-classics | 308 | 0 | 308 | 304 | 302 |
| christmas-hits | 154 | 0 | 154 | 147 | 151 |
| crowd-pleasers | 226 | 0 | 226 | 225 | 224 |
| essentials | 308 | 0 | 308 | 300 | 302 |
| mega-mix | 500 | 0 | 500 | 497 | 498 |
| party-starters | 308 | 0 | 308 | 305 | 305 |
| radio-gold | 154 | 0 | 154 | 151 | 150 |
| rock-anthems | 308 | 0 | 308 | 308 | 307 |
| schlager-fest | 308 | 0 | 307 | 282 | 290 |
| screen-hits | 154 | 0 | 154 | 113 | 106 |
| sunshine | 308 | 0 | 308 | 302 | 306 |

## Categories

| category | songs |
|---|---:|
| gen-alpha | 337 |

## Genres (Last.fm listeners of the songs Last.fm knows)

| genre | songs | on Last.fm | p10 | p50 |
|---|---:|---:|---:|---:|
| chanson | 4 | 4 | 9898 | 9972 |
| country | 51 | 50 | 831 | 185335 |
| dance | 396 | 395 | 14092 | 325703 |
| disco-funk | 171 | 171 | 18637 | 284259 |
| hip-hop | 143 | 142 | 25673 | 257856 |
| indie | 306 | 304 | 74823 | 971808 |
| italo | 5 | 5 | 69228 | 125341 |
| latin | 68 | 68 | 3181 | 187732 |
| metal | 210 | 210 | 97737 | 582831 |
| pop | 1167 | 1161 | 8575 | 301859 |
| reggae | 54 | 54 | 58338 | 241940 |
| rnb-soul | 337 | 336 | 32404 | 341092 |
| rock | 882 | 880 | 30753 | 568294 |
| schlager | 409 | 402 | 107 | 3350 |

## Year conflicts: MusicBrainz or Apple more than a year off the song's year (card, or override) (361)

| artist | title | year | MusicBrainz | Apple | Apple on original album |
|---|---|---:|---:|---:|---|
| 3 Doors Down | Kryptonite | 2000 | 1997 | 2000 | yes |
| Alabama 3 | Die Sopranos (Woke Up This Morning) | 1999 | 2000 | 1997 |  |
| Alan Silvestri | Forrest Gump (Forrest Gump Suite) | 1994 | 1995 | 2001 |  |
| Alcatraz | Crying At the Discoteque | 2000 | 2015 | 2007 |  |
| Alexandra Stan | Mr. Saxobeat | 2010 | 2011 | 2005 |  |
| America | The Last Unicorn | 1982 | 1983 | 1996 |  |
| Andreas Martin | Du bist alles (Maria, Maria) (You Want Love) | 1987 | 1987 | 1990 |  |
| Andy Borg | Arrivederci Claire | 1982 | 1982 | 1999 |  |
| Angelo Badalamenti | Twin Peaks | 1990 | 1994 | 2008 |  |
| Anthony Gonzalez, Gael Garcia Bernal | Coco - Lebendiger als das Leben! (Un Poco Loco) | 2017 | 2017 | 2011 |  |
| Atlantis | Lass mi bei Dir sein | 1987 | 1997 | 1997 | yes |
| Ava Max | So Am I | 2019 | 2019 | 2016 |  |
| B 3 | I.O.I.O. | 2002 | – | 1996 |  |
| Barenaked Ladies | The Big Bang Theory | 2007 | 2019 | 2019 |  |
| Bata Illic | Michaela | 1972 | 1972 | 1995 |  |
| Bay City Rollers | Saturday Night | 1976 | 1976 | 1974 |  |
| Beastie Boys | Fight For Your Right | 1986 | 1988 | 1986 |  |
| Bernard Herrmann | Psycho (Prelude) | 1960 | 2010 | 1960 |  |
| Bernhard Brink | Mit dir werden Wunder noch wahr | 1996 | 1995 | 1998 |  |
| Bill Haley & His Comets | Rock Around The Clock | 1954 | 1955 | 1948 |  |
| Bill Haley & His Comets | (We're Gonna) Rock Around The Clock | 1954 | 1954 | 1948 |  |
| Billy Mo | Ich kauf' mir lieber einen Tirolerhut | 1962 | 1987 | 2014 |  |
| Bing Crosby | I'll Be Home For Christmas | 1943 | 1943 | 1945 |  |
| Bing Crosby | White Christmas | 1941 | 1942 | 1947 |  |
| Black Stone Cherry | White Trash Millionaire | 2011 | 2001 | 2011 | yes |
| Bonnie Bianco | Stay | 1984 | 1987 | – |  |
| Brad Fiedel | Terminator 2 | 1991 | 1996 | – |  |
| Brian Hyland | Itsy Bitsy Teeny Weeny Yellow Polka Dot Bikini | 1960 | 1975 | 2003 |  |
| Bronski Beat | Why ? | 1984 | 2001 | 1984 | yes |
| Bruce Channel | Hey! Baby | 1961 | 1962 | 1959 |  |
| Bryan Adams | Christmas Time | 1985 | 1985 | 2007 |  |
| Bryan Ferry | Let's Stick Together | 1976 | 1976 | 1974 |  |
| C.C. Catch | Backseat of Your Cadillac | 1988 | 1989 | 1986 |  |
| Cheryl | Fight For This Love | 2009 | 2010 | 1995 |  |
| Chris Roberts | Du kannst nicht immer 17 sein | 1974 | 1974 | 1993 |  |
| Christian Anders | Geh nicht vorbei | 1968 | 1969 | 1970 |  |
| Christina Perri | The Twilight Saga: Breaking Dawn (A thousand years) | 2008 | – | 2011 |  |
| Chuck Berry | Johnny B. Goode | 1958 | 1958 | 1955 |  |
| Cliff Richard | Rote Lippen Soll Man Küssen | 1963 | 1963 | 1973 |  |
| Club Honolulu | Itsy Bitsy Teenie Weenie Honolulu Strand Bikini | 1960 | 2017 | 2014 |  |
| Colbie Caillat | Mistletoe | 2007 | 2007 | 2012 | yes |
| Conny Froboess | Zwei kleine Italiener | 1961 | 1962 | 1992 |  |
| Cornelia Froboess | Pack Die Badehose Ein | 1951 | – | 1992 |  |
| Costa Cordalis | Pan | 1980 | 1980 | 1976 |  |
| Creedence Clearwater Revival | Have You Ever Seen The Rain | 1970 | 1970 | 1968 |  |
| Cyndi Lauper | Early Christmas Morning | 1996 | 1996 | 1986 |  |
| Daddy DJ | Daddy DJ | 1999 | 2000 | 2005 |  |
| Dana | All Kinds of Everything | 1970 | 1970 | 2015 |  |
| Danny & The Juniors | At The Hop | 1957 | 1957 | 1983 |  |
| Danny Elfman | Batman | 1989 | 2010 | – |  |
| Danyel Gerard | Butterfly | 1971 | 1970 | 2006 |  |
| Danzel | Pump It Up! | 2004 | 2004 | 2024 |  |
| Dave Stewart (feat. Candy Dulfer) | Lily Was Here | 1989 | 1990 | 2008 |  |
| Demis Roussos | My Friend The Wind | 1973 | 1973 | 1983 |  |
| Demis Roussos | Schönes Madchen aus Arcadia | 1973 | 1973 | 1998 |  |
| Dennie Christian | Rosamunde | 1974 | 1975 | 1990 |  |
| Depeche Mode | Personal Jesus - Single Version | 1990 | 1989 | 1987 |  |
| Desireless | Voyage voyage | 1986 | 1986 | 1980 |  |
| Desmond Dekker | Israelites | 1968 | 1966 | 1968 | yes |
| Destiny's Child | Independent Women, Pt. 1 | 2000 | 2017 | 2000 |  |
| Diana King | Shy Guy | 1995 | 1995 | 1990 |  |
| Diana Ross | A Brand New Day | 1978 | 1978 | 2015 |  |
| Diddy | I'll Be Missing You (feat. Faith Evans, 112) | 1997 | 2020 | 1997 |  |
| Die Fantastischen Vier | Mit freundlichen Grüßen | 1999 | 2010 | 1999 |  |
| Die Fidelen Mölltaler | Gold, Platin und Diamant | 1984 | – | 2001 |  |
| Die Flippers | Die rote Sonne von Barbados | 1986 | 1986 | 1980 |  |
| Die Kolibris | Die Hände Zum Himmel | 1998 | 1998 | 2001 |  |
| Die Minstrels | Grüezi Wohl, Frau Stirnimaa | 1968 | 1969 | 2010 |  |
| Die Tahiti Tamoures | Wini-Wini | 1963 | – | 2003 |  |
| Die Toten Hosen | Altes Fieber | 2012 | 2012 | 1996 |  |
| Diether Krebs | Ich bin der Martin 'ne | 1992 | 1991 | 2004 |  |
| Dirk Busch | Du mußt deinen Weg alleine gehen | 1994 | – | 1999 |  |
| DJ BoBo | Pray | 1996 | 1996 | 2012 |  |
| DJ Zipfelmann | Schlittenfahren in Tirol (Der Schnittensong) | 2004 | 2004 | 2006 |  |
| DNA | Tom's Diner | 1987 | 1990 | 1987 |  |
| Döf | Codo (...düse im Sauseschritt) | 1983 | 1983 | 2008 |  |
| Domenico Modugno | Volare | 1958 | 1987 | 1957 |  |
| Dominik Hauser | Dallas | 1978 | – | 2011 |  |
| Dominik Hauser | L.A. Law - Law & Order | 1986 | – | 2010 |  |
| Dooley Wilson | Casablanca (As Time Goes By) | 1942 | 1943 | 2013 |  |
| Dorthe | Sind Sie der Graf von Luxemburg? | 1968 | 1988 | 1968 |  |
| Douglas J. Cuomo | Sex And The City | 1998 | 2002 | – |  |
| Drafi Deutscher | Heute male ich dein Bild, Cindy Lou (The Birds and the Bees) | 1965 | 1965 | 1987 |  |
| Drafi Deutscher | Marmor, Stein und Eisen bricht | 1965 | 1965 | 1987 |  |
| Dschinghis Khan | Dschinghis Khan | 1979 | 1979 | 1993 |  |
| Dschinghis Khan | Hadschi Halef Omar | 1979 | 1979 | 1993 |  |
| Edward Reekers | So schmeckt der Sommer | 1995 | 1995 | 2015 |  |
| Electric Callboy | We Got the Moves | 2021 | 2023 | 2022 | yes |
| EMF | Unbelievable | 1988 | 1988 | 1990 |  |
| Enrico Caruso | Rigoletto: "La Donna E' Mobile" | 1908 | 1903 | 1908 |  |
| Eric Carmen | Hungry Eyes | 1987 | 1987 | 1997 |  |
| Erik Silvester | Zucker im Kaffee | 1969 | 1978 | 2001 |  |
| Eurythmics, Annie Lennox, Dave Stewart | Sweet Dreams (Are Made Of This) | 1983 | 2024 | 1982 |  |
| Fancy | Flames Of Love | 1988 | 1988 | 1998 |  |
| Fats Domino | I'm Walkin' | 1957 | 1959 | 1959 |  |
| Felice Taylor | It May Be Winter Outside (But in My Heart It's Spring) | 1966 | 2003 | 2016 |  |
| Fettes Brot | An Tagen wie diesen | 2005 | 2005 | 2023 |  |
| Fettes Brot | Emanuela | 2005 | 2005 | 2023 |  |
| France Gall | Poupée de cire, poupée de son | 1965 | 1965 | 2002 |  |
| Francis Lai | Love Story | 1970 | 1975 | 1970 |  |
| Frank Farian | Sie war erst siebzehn (und neu in der Stadt) | 1979 | 1977 | 2009 |  |
| Frank Sinatra | Jingle Bells | 1948 | 1948 | 1946 |  |
| Frank Sinatra | Theme From New York, New York | 1980 | 1980 | 1977 |  |
| Frankie Valli & The Four Seasons | December, 1963 (Oh What a Night!) | 1975 | 1991 | 1975 |  |
| Fred Sonnenschein & seine Freunde | Ententanz (Ja wenn wir alle Englein wären) | 1981 | 1981 | 1994 |  |
| Freddy Breck | Bianca | 1973 | 1972 | 1975 |  |
| Freddy Breck | Rote Rosen | 1973 | 1973 | 1981 |  |
| Freddy Quinn | Aloha-Ahe | 1962 | – | 1987 |  |
| Freddy Quinn | Die Gitarre und das Meer | 1959 | 1959 | 1987 |  |
| Freddy Quinn | Heimweh (Dort wo die Blumen blüh'n) | 1956 | 1956 | 1987 |  |
| GABRIELLE | Bridget Jones - Schokolade zum Frühstück (Out Of Reach) | 2001 | 2001 | 1999 |  |
| Gaby Baginsky | Wer hat dir das Küssen beigebracht | 1994 | 1995 | 1999 |  |
| Gary Portnoy | Cheers (Where Everybody Knows Your Name) | 1982 | 1983 | 2004 |  |
| Gebrüder Blattschuss | Früh-Stück | 1979 | 1979 | 2001 |  |
| Gebrüder Blattschuss | Kreuzberger Nächte | 1978 | 1978 | 2009 |  |
| Geier Sturzflug | Die Pure Lust am Leben | 1984 | 2003 | 1999 |  |
| Gene Autry & The Cass County Boys | Frosty the Snowman | 1950 | 1950 | 2019 |  |
| George Baker | Una Paloma Blanca | 1975 | 1991 | 1993 |  |
| Georgie Fame & The Blue Flames | Yeh, Yeh | 1964 | 1964 | 1986 |  |
| Gerhard Wendland | Tanze mit mir in den Morgen | 1961 | 1961 | 1996 |  |
| G.G. Anderson | Nein heißt ja | 1981 | 2000 | 2000 | yes |
| G.G. Anderson | Sommernacht in Rom | 1985 | 1985 | 1993 |  |
| Gibson Brothers | Que Sera Mi Vida | 1979 | 1979 | 1981 |  |
| Gipsy Kings | Djobi, Djoba | 1987 | 1988 | 1982 |  |
| Gitte Haenning | Freu dich bloß nicht zu früh | 1980 | 1980 | 1991 |  |
| Gitte Haenning | Ich will alles | 1983 | 1984 | 1986 |  |
| Gitte Hænning | Ich will 'nen Cowboy als Mann | 1963 | 1988 | 2004 |  |
| Gitti Und Erica | Heidi | 1977 | – | 2014 |  |
| Glenn Frey | The Heat Is On | 1984 | 1984 | 1995 |  |
| Gölä, Trauffer | Büetzer Buebe | 2019 | 2009 | 2019 | yes |
| Gottlieb Wendehals | Polonase Blankenese | 1981 | 1981 | 2000 |  |
| Gunter Gabriel | Hey Boss, ich brauch mehr Geld | 1974 | 1974 | 1994 |  |
| Hannes Kröger | Der blonde Hans | 1988 | 1988 | 2009 |  |
| Harry Nilsson | Without You | 1971 | 1991 | 1971 |  |
| Heino | Blau blüht der Enzian | 1972 | 1972 | 1986 |  |
| Heino | Die Schwarze Barbara | 1975 | 1975 | 2016 |  |
| Heino | Einsam unter weißen Zweigen | 1986 | 1969 | 1969 |  |
| Heino | Ja, ja, die Katja, die hat ja | 1980 | 1981 | 1987 |  |
| Heintje | Mama | 1968 | 1967 | 2004 |  |
| Heintje | O Tannenbaum | 1968 | 1968 | 2012 |  |
| Helloween | Dr. Stein | 1989 | 1988 | 1985 |  |
| Henry Mancini | Der Rosarote Panther | 1963 | 1995 | – |  |
| Henry Valentino & Uschi | Im Wagen vor mir | 1977 | 1977 | 2006 |  |
| Hot Chocolate | No Doubt About It | 1980 | 1979 | 1977 |  |
| Howard Carpendale | Hello Again | 1984 | 1984 | 1972 |  |
| Howard Carpendale | Tür an Tür mit Alice | 1977 | 1977 | 1972 |  |
| Howard Shore | Der Herr der Ringe (The Shire) | 2001 | – | 2005 |  |
| In Extremo | Herr Winter stammt vom Kaukasus | 2020 | 2013 | 2013 | yes |
| Inner Circle | Bad Boys | 1995 | 1994 | 1987 |  |
| Irene Cara | Flashdance...What a Feeling | 1983 | 2007 | 1983 |  |
| James Brown & The Famous Flames | I Got You (I Feel Good) | 1965 | 1965 | 1988 | yes |
| James Brown | Get Up I Feel Like Being Like A Sex Machine | 1970 | 1995 | 1970 |  |
| James Brown | Get Up I Feel Like Being Like A Sex Machine, Pts. 1 & 2 | 1970 | 2018 | 1970 |  |
| James Horner | Willow | 1988 | 2015 | – |  |
| James Last | Petersburger Schlittenfahrt | 1966 | 1967 | 1998 |  |
| Jerry Goldsmith | Star Trek | 1979 | 1997 | – |  |
| Jerry Lee Lewis | Whole Lotta Shakin' Going On | 1957 | 1969 | 1957 |  |
| Jimi Hendrix | All Along the Watchtower | 1968 | 1971 | 1968 |  |
| Jimi Hendrix | Hey Joe | 1966 | 1971 | – |  |
| Jimi Jamison | Baywatch (I'm Always Here) | 1989 | – | 2011 |  |
| Jive Bunny and the Mastermixers | Swing The Mood | 1989 | 1989 | 2000 |  |
| Joe Cocker | Summer In the City | 1994 | 1994 | 1981 |  |
| Joe Cocker | You Can Leave Your Hat On | 1986 | 1986 | 1972 |  |
| John Barry | Jenseits von Afrika (I Had A Farm) | 1985 | – | 1995 |  |
| John Carpenter | Halloween - Die Nacht des Grauens | 1978 | – | 2016 |  |
| John E. Davis | Beverly Hills 90210 | 1990 | 2000 | – |  |
| John Kincade | Dreams Are Ten A Penny | 1972 | 1991 | 2003 |  |
| John Williams | Harry Potter und der Stein der Weisen (Prologue) | 2001 | 1989 | 1998 |  |
| John Williams | Jäger des verlorenen Schatzes (Indiana Jones) | 1981 | – | 1995 |  |
| John Williams | Jurassic Park | 1993 | 1995 | – |  |
| John Williams | Superman | 1978 | 1995 | 1978 |  |
| Johnny Cash | A Boy Named Sue | 1969 | 1969 | 1954 |  |
| Jona Lewie | Stop the Cavalry | 1980 | 1980 | 1982 | yes |
| Jonathan Wolff | Seinfeld | 1989 | 1996 | – |  |
| Joseito Fernandez | Guajira Guantanamera | 1929 | 1940 | 1995 |  |
| Juliane Werding | Am Tag als Conny Kramer starb | 1972 | 1972 | 1992 |  |
| Jürgen Drews | Wieder Alles im Griff | 1999 | 1999 | 2017 |  |
| Jürgen Marcus | Ein Festival der Liebe | 1973 | 1973 | 1975 |  |
| Justin Wellington | Iko Iko (My Bestie) (feat. Small Jam) | 2019 | 2019 | 2017 |  |
| Karel Gott | Die Biene Maja | 1975 | 1977 | 1977 |  |
| Karl Dall | Millionen Frauen lieben mich | 1987 | 1987 | 1992 |  |
| Karl Jenkins | Adiemus | 1995 | 2002 | 2017 |  |
| Kastelruther Spatzen | Ein Leben lang | 2000 | 2000 | 1991 |  |
| Kate Bush | Wuthering Heights | 1978 | 1978 | 2022 |  |
| Katja Ebstein | Wunder gibt es immer wieder | 1970 | 1970 | 1994 |  |
| Katrina & The Waves | Walking On Sunshine | 1983 | 1983 | 1985 |  |
| KC & The Sunshine Band | Shake Your Booty | 1976 | 1998 | 1976 |  |
| Klaus & Klaus | Der Eiermann | 1988 | 1988 | 1995 |  |
| Klostertaler, Kastelruther Spatzen, Nockis | Hal-Le-Lu-Ja | 1997 | – | 1994 |  |
| K.Maro | Femme Like U | 2004 | 2008 | 2004 |  |
| Kristine W | Feel What You Want | 1994 | 1994 | 1996 |  |
| Las Ketchup | The Ketchup Song | 2002 | 2002 | 1995 |  |
| Lea Salonga, Brad Kane | Aladdin (A Whole New World) | 2004 | 1996 | 1992 |  |
| Leonard Bernstein | West Side Story (America) | 1961 | 1993 | 1962 |  |
| Leony | Faded Love | 2021 | 2021 | 2023 | yes |
| Lil' Louis | French Kiss | 1989 | 1989 | 1996 |  |
| Lill-Babs | Wo finde ich den Mann? | 1961 | 2018 | 2013 |  |
| Liquido | Narcotic - Radio Edit | 1998 | 1998 | 1996 |  |
| Little Eva | The Locomotion | 1962 | 1973 | 1962 |  |
| Little Mix | One I've Been Missing | 2019 | 2017 | 2019 | yes |
| Little Richard | Freedom Blues | 1970 | 1970 | 1975 |  |
| Lizzo | Good as Hell | 2019 | 2020 | 2016 |  |
| Loona | Bailando | 1996 | 1995 | 2018 |  |
| Los Del Rio | Macarena | 1993 | 1993 | 1995 |  |
| Louis Prima | Buona sera | 1950 | 1956 | 1956 | yes |
| Luis Bacalov, Rocky Roberts | Django Unchained (Django) | 2012 | 1985 | 1966 |  |
| Luv' | You're The Greatest Lover | 1978 | 1978 | 1998 |  |
| M People | Moving on Up | 1999 | 1998 | 1993 |  |
| MadHouse | Like a Prayer | 2002 | 2000 | 2002 |  |
| Manic Street Preachers | If You Tolerate This Your Children Will Be Next | 1998 | 1998 | 1988 |  |
| Manuel & Pony | Das Lied von Manuel | 1979 | 1979 | 2014 |  |
| Manuela | Schuld war nur der Bossa Nova | 1963 | 1963 | 1992 |  |
| Margot Eskens | ... Denn es ist Weihnachtszeit | 1956 | – | 2008 |  |
| Marianne Rosenberg | Aba Heidschi Bumbeidschi | 1977 | 2013 | 2013 | yes |
| Marianne Rosenberg | Er gehört zu mir | 1975 | 1975 | 1988 |  |
| Marianne Rosenberg | Ich bin wie du | 1975 | 1975 | 1988 |  |
| Marianne Rosenberg | Marleen | 1976 | 1976 | 1988 |  |
| Marilyn Monroe | Blondinen bevorzugt (Diamonds Are a Girl's Best Friend) | 1953 | 1953 | 1950 |  |
| Mark Forster | Jingle Bells, Schlittenfahrt im Schnee | 2023 | 2015 | 2015 | yes |
| Mark Medlock | Summer Love | 2008 | 2008 | 2004 |  |
| Martha Reeves & The Vandellas | Heatwave | 1963 | 1980 | 2003 |  |
| Matthias Reim | Im Himmel geht es weiter | 1994 | 1994 | 2001 |  |
| Max Giesinger | 80 Millionen | 2016 | 2016 | 2012 |  |
| Max Steiner | Vom Winde verweht | 1939 | 2009 | 1995 |  |
| MGK | Bloody Valentine | 2020 | 2025 | 2020 | yes |
| Michael Bublé | The More You Give (The More You'll Have) | 2015 | 2015 | 2011 |  |
| Michael Holm | Mendocino | 1969 | 1969 | 1973 |  |
| Michael Jackson | Ben | 1987 | 1986 | 1972 |  |
| Michael Morgan | Es Schiesst Ein Blitz In Meinen Kopf | 1996 | 1996 | 2010 |  |
| Michael Wendler | Prinzessin | 2006 | 2006 | 2004 |  |
| Mickie Krause | Zehn Nackte Friseusen | 1999 | 1999 | 2006 |  |
| MIKA | Relax, Take It Easy | 2006 | 2006 | 2000 |  |
| Mike Krüger | Bodo mit dem Bagger | 1983 | 1983 | 1995 |  |
| Mike Krüger | Der Nippel | 1980 | 1980 | 1995 |  |
| Milli Vanilli | Girl I'm Gonna Miss You | 1988 | 1989 | 2023 |  |
| Mireille Mathieu | Akropolis Adieu | 1971 | 1971 | 1992 |  |
| Mireille Mathieu | Hinter den Kulissen von Paris | 1969 | 1969 | 1992 |  |
| Moloko | Sing It Back - Boris Musical Mix - Edit | 1999 | 1998 | 1995 |  |
| Monty Python | Das Leben des Brian (Always Look On The Bright Side Of Life) | 1979 | 1980 | 1989 |  |
| Mr. President | Jojo Action | 1997 | 1997 | 1995 |  |
| Nana Darkman | Lonely | 1997 | 2021 | 1997 |  |
| Nana Mouskouri | Ich Steh An Deiner Krippe Hier | 1989 | 1976 | 1964 |  |
| Nat King Cole Trio | The Christmas Song (Merry Christmas To You) | 1946 | 1949 | 1990 |  |
| Nena | Irgendwie, irgendwo, irgendwann | 1984 | 1983 | 1982 |  |
| Nick Straker | Walk In The Park | 1979 | 1996 | – |  |
| Nicky Jam & Enrique Iglesias | El Perdón | 2015 | 2015 | 2017 | yes |
| Nicola Piovani | Das Leben ist schön (La vita è bella) | 1997 | 1997 | 2011 |  |
| Nicole | Ein bisschen Frieden | 1982 | 1982 | 2006 |  |
| Nicole | Weihnacht zuhause | 1984 | 2000 | 1993 |  |
| Nik P. | Gloria II | 2008 | 2007 | 2006 |  |
| Nina Hagen | Du hast den Farbfilm vergessen | 1974 | 1974 | 1991 |  |
| Nina Simone | Ain't Got No / I Got Life | 1968 | 1997 | 1968 |  |
| Nino de Angelo | Jenseits von Eden | 1983 | 1983 | 1989 |  |
| Noah-Levi | Ach, bittrer Winter | 2023 | 2015 | 2015 | yes |
| Nockis | Du warst der geilste Fehler meines Lebens | 2014 | 2020 | 2020 | yes |
| Ofra Haza | Im Nin'alu | 1988 | 1987 | 1984 |  |
| Oliver Haidt | Lebenslänglich Du | 1996 | 1996 | 1998 |  |
| OneRepublic | Run | 2013 | 2021 | 2021 | yes |
| Otto Waalkes | Wir haben Grund zum Feiern | 2008 | 1995 | 2018 |  |
| Patrick Lindner | Halleluja - Auf das Leben | 2003 | – | 2001 |  |
| Patrick Lindner | Weihnachten bin ich daheim | 2003 | – | 1999 |  |
| Paul Anka | Zwei Mädchen aus Germany | 1964 | 1964 | 2001 |  |
| Peggy March | Mit 17 hat man noch Träume | 1965 | 1965 | 1998 |  |
| Perry Como & The Fontane Sisters | It's Beginning to Look a Lot Like Christmas | 1951 | 1951 | 2004 |  |
| Peter Alexander | Schneemann | 1978 | 1978 | 2002 |  |
| Peter Cornelius | Du entschuldige i kenn di | 1980 | 2015 | 2015 | yes |
| Peter Kent | It's a Real Good Feeling | 1979 | 1979 | 2000 |  |
| Peter Kraus | Sugar-Baby | 1958 | 1958 | 1989 | yes |
| Peter Maffay | Brief an den Weihnachtsmann | 1978 | 2007 | 2007 |  |
| Peter Maffay | Du | 1970 | 1970 | 1981 |  |
| Phil Collins | A Groovy Kind of Love | 1988 | 1990 | 1988 |  |
| Pink Floyd | Another Brick In The Wall, Pt. 2 | 1979 | 1999 | 1979 |  |
| Plüsch | Heimweh | 2007 | 2008 | 2002 |  |
| Rainhard Fendrich | Macho Macho | 1988 | 1988 | 1993 |  |
| Reinhard Mey | Es ist Weihnachtstag | 1985 | 1986 | 1981 |  |
| Rex Gildo | Fiesta Mexicana | 1972 | 1972 | 1997 |  |
| Rise Against | Savior | 2008 | 2008 | 2014 |  |
| Robert D. | Matrix (Clubbed to Death) | 1999 | – | 2011 |  |
| Robert Folk | Police Academy | 1984 | 1993 | – |  |
| Roland Kaiser | Dich zu lieben | 1981 | 1981 | 1983 |  |
| Roland Kaiser | Sieben Fässer Wein | 1977 | 1977 | 1983 |  |
| Rolfe Kent | Dexter | 2006 | 2015 | – |  |
| Roxette | It Must Have Been Love | 1990 | 1990 | 1987 |  |
| Roy Black | Wünsche und Gedanken | 1997 | 1968 | 1968 |  |
| Rozalla | Everybody's Free (To Feel Good) | 1991 | 1991 | 1987 |  |
| Rudi Carrell | Goethe war gut | 1978 | 1978 | 2003 |  |
| Rudi Carrell | Wann wird's mal wieder richtig Sommer | 1975 | 1975 | 2003 |  |
| Run–D.M.C. | It's Like That | 1998 | 1997 | 1983 |  |
| S Club | Don't Stop Movin' | 2001 | 2004 | 2001 |  |
| Sabrina | Boys - Summertime Love | 1987 | 1986 | 1985 |  |
| Sailor | La Cumbia | 1991 | 1991 | 1978 |  |
| Sam Cooke | (What A) Wonderful World | 1960 | 1995 | 1960 |  |
| Sarah-Jane | Einmal hin, einmal her | 2005 | 2005 | 2013 |  |
| Shaggy | Mr Boombastic | 1995 | 2007 | – |  |
| Shakespears Sister | Hello - Turn Your Radio On | 1992 | 1992 | 1989 |  |
| Shakin' Stevens | You Drive Me Crazy | 1980 | 1980 | 1978 |  |
| Shawn Phillips | A Christmas Song | 1999 | 1999 | 1972 |  |
| Sheppard | Die Young | 2019 | 2019 | 2021 | yes |
| Sick Puppies | You're Going Down | 2011 | 2011 | 2009 |  |
| Sido | Weihnachtssong | 2003 | 2003 | 2006 |  |
| Simone | Viel zu oft | 2005 | 2006 | 2000 |  |
| Siw Malmkvist | Liebeskummer Lohnt Sich Nicht | 1964 | 1964 | 2011 |  |
| Smokie | Lay Back in the Arms of Someone | 1977 | 1977 | 1994 | yes |
| Smokie | Living Next Door to Alice | 1976 | 1976 | 1994 | yes |
| Sonique | It Feels So Good | 2000 | 1999 | 1998 | yes |
| Spider Murphy Gang | So a Nacht | 1987 | 1987 | 1997 |  |
| Status Quo | What You're Proposing | 1980 | 1977 | 1980 |  |
| Steam | Na Na Hey Hey Kiss Him Goodbye | 1969 | 1969 | 2003 |  |
| Stefanie Hertel | Erst du, dann du | 2004 | 2004 | 2002 |  |
| Stefanie Hertel | Ich hab den Weihnachtsmann gesehen | 2008 | 2008 | 2000 |  |
| Stevie B | Because I Love You | 1990 | 1990 | 1988 |  |
| Stu Phillips | Knight Rider | 1982 | 1989 | – |  |
| Teddy Swims | Lose Control | 2023 | 2023 | 2005 |  |
| Terry Jacks | Seasons in the Sun | 1973 | 1973 | 2013 |  |
| The Animals | House of the Rising Sun | 1964 | 1970 | 1963 |  |
| The B-52's | Love Shack | 1989 | 1989 | 1981 |  |
| The Bangles | Eternal Flame | 1988 | 1997 | 1988 |  |
| The Beach Boys | Barbara Ann | 1961 | 1965 | 1965 |  |
| The Beatles | I Want To Hold Your Hand | 1963 | 1963 | 2014 |  |
| The Beatles | Yellow Submarine | 1968 | 1969 | 1966 |  |
| The Beloved | Sweet Harmony | 1993 | 1992 | 1989 |  |
| The Blues Brothers | Everybody Needs Somebody to Love | 1980 | 2020 | 1990 |  |
| The Buggles | Video Killed The Radio Star - Single Version | 1979 | 1979 | 1974 |  |
| The Edwin Hawkins Singers | Oh Happy Day | 1968 | 1973 | 1968 |  |
| The Everly Brothers | ('Til) I Kissed You | 1959 | 1959 | 1964 |  |
| The Goo Goo Dolls | Iris | 1998 | 2006 | 2026 |  |
| The Hollies | He Ain't Heavy He's My Brother | 1969 | 1969 | 1964 |  |
| The Jackson 5 | ABC | 2022 | 1970 | 1970 | yes |
| The Jackson 5 | Santa Claus Is Coming To Town | 1970 | 1987 | 1970 |  |
| The Rolling Stones | Jumpin' Jack Flash | 1968 | 1968 | 1966 |  |
| The Teens | Give Me More | 1980 | 1980 | 2000 |  |
| The Underdog Project | Summer Jam 2003 | 2003 | 2003 | 2000 |  |
| Tobee | Helikopter 117 (Mach' den Hub Hub Hub) | 2019 | 2018 | 2017 |  |
| Tom Scott | Starsky & Hutch (Gotcha) | 1975 | 1987 | 1980 |  |
| Tony Bennett | Winter Wonderland | 1968 | 1968 | 2019 |  |
| Tony Marshall | Auf der Straße nach Süden | 1978 | 1978 | 1990 |  |
| Toploader | Dancing in the Moonlight | 2000 | 2000 | 1972 |  |
| Truck Stop | Erwin der dicke Schneemann (Rudolph The Rednosed Reindeer) | 2015 | 2015 | 1996 |  |
| Trude Herr | Weil ich so sexy bin | 1961 | 1961 | 1998 |  |
| U2 | With Or Without You | 1985 | 1987 | 1987 |  |
| Udo Jürgens | Ach Papi, geh' doch heuer nicht auf die Weihnachtsfeier | 2003 | 2003 | 2001 |  |
| Udo Jürgens | Ich war noch niemals in New York | 1982 | 1982 | 1979 |  |
| Udo Jürgens | Siebzehn Jahr, blondes Haar | 1965 | 1965 | 2024 |  |
| Udo Lindenberg | Ein Herz kann man nicht reparieren | 1991 | 2004 | 1971 |  |
| Unheilig | Still still still | 2009 | 2002 | 2002 | yes |
| Ute Freudenberg | Willkommen im Leben | 2012 | 2012 | 1994 |  |
| Vangelis | Die Stunde des Siegers | 1981 | 2009 | – |  |
| Vaughn Monroe | Let It Snow! Let It Snow! Let It Snow! | 1945 | 1945 | 1949 |  |
| Vicky Leandros | Après toi | 1972 | 1972 | 2012 |  |
| Village People | In the Navy | 1978 | 1979 | 1998 |  |
| Wencke Myhre | Beiß nicht gleich in jeden Apfel | 1966 | 1966 | 1999 |  |
| Wendy Carlos | Shining | 1980 | 1996 | – |  |
| Westernhagen | Freiheit | 1990 | 1987 | 1987 | yes |
| Wet Wet Wet | Love Is All Around | 1994 | 1994 | 1986 |  |
| Willy Millowitsch | Schnaps, das war sein letztes Wort | 1960 | 1960 | 1999 |  |
| Wind | Lass die Sonne in dein Herz | 1987 | 1987 | 1996 |  |
| Wizzard | I Wish It Could Be Christmas Everyday | 1973 | 1985 | 1973 |  |
| Wolfgang Fierek | Resi, i hol di mit mei'm Traktor ab | 1985 | 1986 | 2007 |  |
| Wolfgang Petry | Wahnsinn | 1983 | 1983 | 1994 |  |
| Wolfgang Ziegler | Ich küss dir die Angst von deiner Haut | 2003 | 2003 | 1999 |  |
| Wolfram | Trödler Abraham | 1970 | – | 2010 |  |

## Card years that differ between sets (earliest kept) (14)

| artist | title | years |
|---|---|---|
| Akon | Lonely | 2004, 2005 |
| Alexandra Stan | Mr. Saxobeat | 2010, 2011 |
| Bee Gees | Night Fever | 1977, 1978 |
| Black Eyed Peas | Let's Get It Started | 2003, 2004 |
| Britney Spears | ...Baby One More Time | 1998, 1999 |
| Bryan Adams | Heaven | 1984, 1985 |
| Clowns & Helden | Ich liebe dich | 1986, 1987 |
| Connie Francis | Schöner fremder Mann | 1961, 1964 |
| Creedence Clearwater Revival | Have You Ever Seen The Rain | 1970, 1971 |
| Guns N' Roses | Sweet Child O' Mine | 1987, 1988 |
| Mariah Carey | Without You | 1993, 1994 |
| MIKA | Relax, Take It Easy | 2006, 2007 |
| Red Hot Chili Peppers | Under the Bridge | 1991, 1992 |
| The Guess Who | American Woman | 1970, 1971 |

## Cards merged into another card's recording (27)

| artist | title | kept as artist | kept as title |
|---|---|---|---|
| A Touch Of Class | Around the World (La La La La La) - Radio Version | A Touch Of Class | Around the World (La La La La La) |
| Alexandra Stan | Mr. Saxobeat - Radio Edit | Alexandra Stan | Mr. Saxobeat |
| Bee Gees | Saturday Night Fever (Night Fever) | Bee Gees | Night Fever |
| Beyoncé | Crazy In Love (feat. JAYZ) | Beyoncé | Crazy In Love (feat. Jay-Z) |
| Bill Medley | (I've Had) The Time of My Life | Bill Medley, Jennifer Warnes | (I've Had) The Time of My Life |
| Bill Withers | Ain't No Sunshine - Single Version | Bill Withers | Ain't No Sunshine |
| Black Eyed Peas | Let's Get It Started - Spike Mix | Black Eyed Peas | Let's Get It Started |
| Crystal Waters | Gypsy Woman (She's Homeless) (La Da Dee La Da Da) | Crystal Waters | Gypsy Woman (She's Homeless) |
| Elton John | Don't Go Breaking My Heart | Elton John & Kiki Dee | Don't Go Breaking My Heart |
| Gene Vincent | Be Bop a Lula | Gene Vincent & His Blue Caps | Be-Bop-A-Lula |
| Harold Faltermeyer | Beverly Hills Cop (Axel F) | Harold Faltermeyer | Axel F |
| J Balvin | Mi Gente - Original Mix | J Balvin | Mi Gente |
| Jamiroquai | Cosmic Girl - Remastered 2013 | Jamiroquai | Cosmic Girl |
| Joe Cocker | Up Where We Belong | Joe Cocker, Jennifer Warnes | Ein Offizier und ein Gentleman (Up Where We Belong) |
| John Lennon | Imagine - Remastered 2010 | John Lennon | Imagine |
| John Travolta | You're The One That I Want - From Grease | John Travolta | You're The One That I Want |
| Lady Gaga | Shallow | Lady Gaga, Bradley Cooper | Shallow |
| Las Ketchup | The Ketchup Song (Aserejé) | Las Ketchup | The Ketchup Song |
| Led Zeppelin | Stairway to Heaven - Remaster | Led Zeppelin | Stairway to Heaven |
| Phil Collins | In The Air Tonight - 2015 Remastered | Phil Collins | In The Air Tonight |
| Queen | We Will Rock You - Remastered 2011 | Queen | We Will Rock You |
| Roxette | Pretty Woman (It Must Have Been Love) | Roxette | It Must Have Been Love |
| Shakira | Hips Don't Lie (feat. Wyclef Jean) | Shakira (feat. Wyclef Jean) | Hips Don't Lie |
| The Beach Boys | Surfin' U.S.A. (Mono) | The Beach Boys | Surfin' U.S.A. |
| The Clash | Should I Stay or Should I Go - Remastered | The Clash | Should I Stay or Should I Go |
| The Rembrandts | I'll Be There for You (Theme from "Friends') | The Rembrandts | I'll Be There for You |
| Whitney Houston | I Will Always Love You | Whitney Houston | Bodyguard (I Will Always Love You) |

## Low-confidence matches (582)

| artist | title | why |
|---|---|---|
| A. R. Rahman | Slumdog Millionär (Jai Ho) | MusicBrainz matched as A. R. Rahman – Jai Ho |
| A Touch Of Class | Around the World (La La La La La) | MusicBrainz matched as A Touch Of Class – Around the World; Apple track found by search (A Touch of Class – Around the World (La La La La La) [Radio Version]) |
| Adele | James Bond 007 - Skyfall | MusicBrainz matched as Adele – Skyfall |
| Aerosmith | Armageddon (I Don't Want to Miss a Thing) | MusicBrainz matched as Aerosmith – I Don't Want to Miss a Thing |
| Alabama 3 | Die Sopranos (Woke Up This Morning) | MusicBrainz matched as Alabama 3 – Woke Up This Morning |
| Alan Silvestri | Forrest Gump (Forrest Gump Suite) | MusicBrainz matched as Alan Silvestri – Forrest Gump; Apple track found by search (Alan Silvestri – Forrest Gump Suite) |
| Alanis Morissette | You Oughta Know - 2015 Remaster | MusicBrainz matched as Alanis Morissette – You Oughta Know |
| Alcatraz | Crying At the Discoteque | MusicBrainz has it only from 2015, after the card year |
| Alice Merton | No Roots - Live from Spotify Berlin | MusicBrainz matched as Alice Merton – No Roots |
| Alisha | Deine braunen Augen | Apple track found by search (Alisha – Deine braunen Augen) |
| All Time Low | Monsters (feat. blackbear) | MusicBrainz matched as All Time Low – Monsters |
| Almklausi, Specktakel | Mama Laudaaa | MusicBrainz matched as Almklausi – Mama Laudaaa |
| Alphaville | Big in Japan - Single Version; 2019 Remaster | MusicBrainz matched as Alphaville – Big in Japan |
| Andrea Jürgens | Wir tanzen Lambada | Apple track found by search (Andrea Jürgens – Wir tanzen Lambada (Version 2017)) |
| Andreas Martin | Du bist alles (Maria, Maria) (You Want Love) | Apple track found by search (Andreas Martin – Du bist alles) |
| Angela Lansbury | Die Schöne und das Biest - Beauty and the Beast | MusicBrainz matched as Angela Lansbury – Beauty and the Beast |
| Angelo Badalamenti | Twin Peaks | MusicBrainz has it only from 1994, after the card year; Apple track found by search (Angelo Badalamenti – Twin Peaks (A-Version)) |
| Anna-Carina Woitschack, Stefan Mross | Blaue Lagune | MusicBrainz matched as Anna-Carina Woitschack – Blaue Lagune |
| Anna-Maria Zimmermann, Olaf Henning | Freundschaftsring | Apple track found by search (Anna-Maria Zimmermann – Freundschaftsring (Duett mit Olaf Henning) [Party Mix]) |
| Annie Lennox | Bram Stoker's Dracula (Love Song for a Vampire) | MusicBrainz matched as Annie Lennox – Love Song for a Vampire |
| Anthony Gonzalez, Gael Garcia Bernal | Coco - Lebendiger als das Leben! (Un Poco Loco) | MusicBrainz matched as Anthony Gonzalez – Un Poco Loco |
| Ashley Park | Emily in Paris (Mon Soleil) | MusicBrainz matched as Ashley Park – Mon Soleil; Apple track found by search (Ashley Park – Mon Soleil (from "Emily in Paris" Soundtrack)) |
| Atlantis | Lass mi bei Dir sein | MusicBrainz has it only from 1997, after the card year |
| Audrey Hepburn | Frühstück bei Tiffany (Moon River) | MusicBrainz matched as Audrey Hepburn – Moon River |
| Avicii, Sandro Cavazza | Without You (feat. Sandro Cavazza) | MusicBrainz matched as Avicii – Without You |
| Avicii | Levels - Radio Edit | MusicBrainz matched as Avicii – Levels |
| AYLIVA | Sie weiß (feat. Mero) | MusicBrainz matched as AYLIVA – Sie weiß |
| B 3 | I.O.I.O. | Apple track found by search (B 3 – I.O.I.O. (Radio Edit)) |
| Barbra Streisand | So wie wir waren - The Way We Were | MusicBrainz matched as Barbra Streisand – The Way We Were |
| Barenaked Ladies | The Big Bang Theory | MusicBrainz has it only from 2019, after the card year |
| Bata Illic | Bella Isola | Apple track found by search (Bata Illic – Bella Isola (Radio Edit)) |
| Bata Illic | Michaela | Apple track found by search (Bata Illic – Michaela) |
| Bay City Rollers | I Only Wanna Be with You | Apple track found by search (Bay City Rollers – I Only Wanna Be With You) |
| Bay City Rollers | Saturday Night | Apple track found by search (Bay City Rollers – Saturday Night) |
| BB Jürgen | Heute fahrt die 18 bis nach Istanbul | Apple track found by search (BB Jürgen – Heute fährt die 18 bis nach Istanbul) |
| Beagle Music | Like Ice in the Sunshine | Apple track found by search (Beagle Music – Like Ice In the Sunshine) |
| Beastie Boys | Fight For Your Right | MusicBrainz has it only from 1988, after the card year; Apple track found by search (Beastie Boys – Fight for Your Right) |
| Becky G | Arranca (feat. Omega) | MusicBrainz matched as Becky G – Arranca |
| Berlin | Top Gun (Take My Breath Away) | MusicBrainz matched as Berlin – Take My Breath Away |
| Bernard Herrmann | Psycho (Prelude) | MusicBrainz has it only from 2010, after the card year; Apple track found by search (Bernard Herrmann – Prelude (Psycho Original Motion Picture Soundtrack)) |
| Bernhard Brink | Mit dir werden Wunder noch wahr | Apple track found by search (Bernhard Brink – Mit Dir Werden Wunder Noch Wahr) |
| Betty Everett | It's In The Kiss (The Shoop Shoop Song) | MusicBrainz matched as Betty Everett – The Shoop Shoop Song; Apple track found by search (Betty Everett – The Shoop Shoop Song (It's In His Kiss)) |
| Beyoncé | Crazy In Love (feat. Jay-Z) | MusicBrainz matched as Beyoncé – Crazy In Love |
| Beyoncé | Dreamgirls (Listen) | MusicBrainz matched as Beyoncé – Listen |
| Bill Conti | Rocky (Gonna Fly Now) | MusicBrainz matched as Bill Conti – Gonna Fly Now |
| Bill Haley & His Comets | See You Later Alligator | Apple track found by search (Bill Haley and His Comets – See You Later Alligator) |
| Bill Haley & His Comets | (We're Gonna) Rock Around The Clock | Apple track found by search (Bill Haley and His Comets – (We're Gonna) Rock Around the Clock) |
| Bill Medley, Jennifer Warnes | (I've Had) The Time of My Life | MusicBrainz matched as Bill Medley – (I've Had) The Time of My Life |
| Bill Ramsey | Zuckerpuppe (Aus der Bauchtanz-Truppe) | Apple track found by search (Bill Ramsey – Zuckerpuppe (Aus der Bauchtanzgruppe)) |
| Billy Mack | Tatsächlich Liebe (Christmas Is All Around) | MusicBrainz matched as Billy Mack – Christmas Is All Around |
| Billy Mo | Ich kauf' mir lieber einen Tirolerhut | MusicBrainz has it only from 1987, after the card year; Apple track found by search (Billy Mo – Ich kauf' mir lieber einen Tirolerhut) |
| Bing Crosby | I'll Be Home For Christmas | Apple track found by search (Bing Crosby – I'll Be Home For Christmas (If Only In My Dreams)) |
| Blind Guardian | Mirror Mirror - Remastered 2007 | MusicBrainz matched as Blind Guardian – Mirror Mirror |
| blink-182 | First Date | Apple track found by search (blink-182 – First Date) |
| Blondie | One Way Or Another - Remastered 2001 | MusicBrainz matched as Blondie – One Way Or Another |
| Blur | Parklife - 2012 Remaster | MusicBrainz matched as Blur – Parklife |
| Bnd | Here I Go Again | Apple track found by search (Bnd – Here I Go Again (Extended Version)) |
| Bob Dylan | Hurricane | Apple track found by search (Bob Dylan – Hurricane) |
| B.o.B (feat. Hayley Williams) | Airplanes | Apple track found by search (B.o.B – Airplanes (feat. Hayley Williams of Paramore)) |
| Bob Marley & The Wailers | Get Up, Stand Up | Apple track found by search (Bob Marley & The Wailers – Get Up, Stand Up) |
| Bob Sinclar | Love Generation | Apple track found by search (Bob Sinclar – Love Generation (feat. Gary Pine) [Radio Edit]) |
| Bobby Lewis | Tossin' And Turnin' | Apple track found by search (Bobby Lewis – Tossin' and Turnin' (Stereo Version)) |
| Bonnie Bianco | Stay | MusicBrainz has it only from 1987, after the card year |
| Booker T. & the M.G.'s | Green Onions | MusicBrainz matched as Booker T. – Green Onions |
| Brad Fiedel | Terminator 2 | MusicBrainz has it only from 1996, after the card year |
| Brian Hyland | Itsy Bitsy Teeny Weeny Yellow Polka Dot Bikini | MusicBrainz has it only from 1975, after the card year; Apple track found by search (Brian Hyland – Itsy Bitsy Teeny Weeny Yellow Polka Dot Bikini) |
| Bronski Beat | Why ? | MusicBrainz has it only from 2001, after the card year; Apple track found by search (Bronski Beat – Why? (12" Version)) |
| Bryan Adams | Robin Hood - König der Diebe ((Everything I Do) I Do It For You) | MusicBrainz matched as Bryan Adams – (Everything I Do) I Do It For You |
| Bryan Adams | Spirit - Der wilde Mustang (Here I Am) | MusicBrainz matched as Bryan Adams – Here I Am |
| Buck Owens | Santa Looked A Lot Like Daddy (Daddy Looked A Lot Like Him) | MusicBrainz matched as Buck Owens – Santa Looked A Lot Like Daddy |
| Buddy | Ab in den Süden | Apple track found by search (Buddy – Ab in den Süden (Neuaufnahme)) |
| Burt Bacharach | Zwei Banditen (Raindrops Keep Fallin' On My Head) | MusicBrainz matched as Burt Bacharach – Raindrops Keep Fallin' On My Head |
| Bush | Machinehead - Remastered | MusicBrainz matched as Bush – Machinehead |
| C & C Music Factory | Gonna Make You Sweat (Everybody Dance Now) (feat. Freedom Williams) | MusicBrainz matched as C & C Music Factory – Gonna Make You Sweat |
| Calvin Harris (with Ellie Goulding) | Miracle | MusicBrainz matched as Calvin Harris – Miracle |
| Calvin Harris | By Your Side (feat. Tom Grennan) | MusicBrainz matched as Calvin Harris – By Your Side |
| Calvin Harris | Feels (feat. Pharrell Williams, Katy Perry & Big Sean) | MusicBrainz matched as Calvin Harris – Feels |
| Calvin Harris | Promises (with Sam Smith) | MusicBrainz matched as Calvin Harris – Promises |
| Camila Cabello | Bam Bam (feat. Ed Sheeran) | MusicBrainz matched as Camila Cabello – Bam Bam |
| Camila Cabello | Havana (feat. Young Thug) | MusicBrainz matched as Camila Cabello – Havana |
| Captain Hollywood Project | Flying High - Radio Mix | MusicBrainz matched as Captain Hollywood Project – Flying High |
| Carly Simon | Die Waffen der Frauen (Let The River Run) | MusicBrainz matched as Carly Simon – Let The River Run; Apple track found by search (Carly Simon – Let the River Run) |
| Carrière | Mein Ein und mein Alles | Apple track found by search (Carriére – Mein Ein und mein Alles) |
| Cassandra Steen ft. Adel Tawil | Stadt | MusicBrainz matched as Cassandra Steen – Stadt |
| Caterina Valente, Silvio Francesco | Itsy Bitsy Teenie Weenie Honolulu Strand Bikini | Apple track found by search (Silvio Francesco & Caterina Valente – Itsy Bitsy Teenie Weenie Honolulu Strand Bikini) |
| Catherine Zeta-Jones | Chicago (Cell Block Tango) | MusicBrainz matched as Catherine Zeta-Jones – Cell Block Tango |
| C.C. Catch | Backseat of Your Cadillac | Apple track found by search (C.C. Catch – Backseat of Your Cadillac (Single Version)) |
| CeeLo Green | All I Need Is Love (feat. Disney's the Muppets) | Apple track found by search (CeeLo Green – All I Need Is Love (feat. The Muppets)) |
| Chad Kroeger, Josey Scott | Spider-Man (Hero) | MusicBrainz matched as Chad Kroeger – Hero |
| Charles Brown | Please Come Home For Christmas | Apple track found by search (Charles Brown – Please Come Home for Christmas) |
| Cheryl | Fight For This Love | Apple track found by search (Cheryl – Fight for This Love (Moto Blanco Radio Edit)) |
| Christian Anders | Geh nicht vorbei | Apple track found by search (Christian Anders – Geh' nicht vorbei (Französisch)) |
| Christian Lais | Atemlos | Apple track found by search (Christian Lais – Atemlos (Radio Edit)) |
| Christina Perri | The Twilight Saga: Breaking Dawn (A thousand years) | Apple track found by search (Christina Perri – A Thousand Years) |
| Chuck Berry | Run Rudolph Run | Apple track found by search (Chuck Berry – Run Rudolph Run) |
| Cindy & Bert | Immer wieder sonntags | Apple track found by search (Cindy & Bert – Immer wieder Sonntags) |
| Claudia Jung, Richard Clayderman | Je T'aime Mon Amour | MusicBrainz matched as Claudia Jung – Je T'aime Mon Amour |
| Clean Bandit (feat. 24kGoldn) | Tick Tock | MusicBrainz matched as Clean Bandit – Tick Tock |
| Clean Bandit | Rather Be (feat. Jess Glynne) | MusicBrainz matched as Clean Bandit – Rather Be |
| Clean Bandit | Rockabye (feat. Sean Paul & Anne-Marie) | MusicBrainz matched as Clean Bandit – Rockabye |
| Clean Bandit | Solo (feat. Demi Lovato) | MusicBrainz matched as Clean Bandit – Solo |
| Cliff Edwards | Pinocchio (When You Wish Upon a Star) | MusicBrainz matched as Cliff Edwards – When You Wish Upon a Star |
| Clint Mansell | Requiem for a Dream (Lux Aeterna) | MusicBrainz matched as Clint Mansell – Lux Aeterna |
| Club Honolulu | Itsy Bitsy Teenie Weenie Honolulu Strand Bikini | MusicBrainz has it only from 2017, after the card year; Apple track found by search (Club Honolulu – Itsy Bitsy Teenie Weenie Honolulu Strand Bikini) |
| Colör | Su Mädche wie mir - Zwei Kerle wie wir | MusicBrainz matched as Colör – Su Mädche wie mir |
| Common, John Legend | Selma (Glory) | MusicBrainz matched as Common – Glory |
| Conkarah | Banana - feat. Shaggy | MusicBrainz matched as Conkarah – Banana |
| Coolio feat. L.V. | Gangsta's Paradise | Apple track found by search (Coolio – Gangsta's Paradise) |
| Cordalis | Königin der Nacht | Apple track found by search (Cordalis – Königin der Nacht) |
| Corey Taylor | CMFT Must Be Stopped (feat. Tech N9ne and Kid Bookie) | MusicBrainz matched as Corey Taylor – CMFT Must Be Stopped |
| Cornelia Froboess | Pack Die Badehose Ein | Apple track found by search (Cornelia Froboess – Pack die Badehose ein) |
| Costa Cordalis | Pan | Apple track found by search (Costa Cordalis – Pan) |
| Craig David | Rise & Fall (feat. Sting) | MusicBrainz matched as Craig David – Rise & Fall |
| Daft Punk | Get Lucky (feat. Pharrell Williams and Nile Rodgers) | MusicBrainz matched as Daft Punk – Get Lucky |
| Dana | All Kinds of Everything | Apple track found by search (Dana – All Kinds of Everything) |
| Dana | It's Gonna Be a Cold, Cold Christmas | Apple track found by search (Dana – It's Gonna Be a Cold, Cold Christmas) |
| Danny Elfman | Batman | MusicBrainz has it only from 2010, after the card year |
| Das Bo | Türlich, Türlich (Sicher, Dicker) | Apple track found by search (Das Bo – Türlich, Türlich (Sicher, Dicker)) |
| Dave Stewart (feat. Candy Dulfer) | Lily Was Here | Apple track found by search (Dave Stewart – Lily Was Here) |
| David Bowie & Pat Metheny Group | This Is Not America | MusicBrainz matched as David Bowie – This Is Not America |
| David Bowie | Heroes - 2017 Remaster | MusicBrainz matched as David Bowie – Heroes |
| David Guetta | Hey Mama (feat. Nicki Minaj, Bebe Rexha & Afrojack) - Afrojack Remix | MusicBrainz matched as David Guetta – Hey Mama |
| David Guetta | Lovers on the Sun (feat. Sam Martin) | Apple track found by search (David Guetta – Lovers on the Sun (feat. Sam Martin)) |
| David Guetta | When Love Takes Over (feat. Kelly Rowland) | Apple track found by search (David Guetta – When Love Takes Over (feat. Kelly Rowland)) |
| De Electronica's | Dance Little Birdie | Apple track found by search (De Electronica's – Dance Little Birdie) |
| Demi Lovato | Die Eiskönigin - Völlig unverfroren (Let It Go) | MusicBrainz matched as Demi Lovato – Let It Go |
| Demis Roussos | Schönes Madchen aus Arcadia | Apple track found by search (Demis Roussos – Schönes Mädchen aus Arcadia) |
| Dennie Christian | Rosamunde | Apple track found by search (Dennie Christian – Rosamunde) |
| Depeche Mode | Personal Jesus - Single Version | MusicBrainz matched as Depeche Mode – Personal Jesus |
| Desmond Dekker | Israelites | MusicBrainz artist matched loosely (Desmond Dekker & the Aces) |
| Destiny's Child | Independent Women, Pt. 1 | MusicBrainz has it only from 2017, after the card year; Apple track found by search (Destiny's Child – Independent Women, Pt. 1 (Victor Calderone Drum Dub Mix)) |
| Diana Ross | A Brand New Day | Apple track found by search (Diana Ross – A Brand New Day (Everybody Rejoice)) |
| Diddy | I'll Be Missing You (feat. Faith Evans, 112) | MusicBrainz has it only from 2020, after the card year; Apple track found by search (Diddy – I'll Be Missing You (feat. Faith Evans & 112)) |
| Dido | Thank You | Apple track found by search (Dido – Thank You) |
| Die Atzen | Das geht ab | Apple track found by search (Die Atzen – Das geht ab (Wir holen die Meisterschaft)) |
| Die Fantastischen Vier | Mit freundlichen Grüßen | MusicBrainz has it only from 2010, after the card year; Apple track found by search (Die Fantastischen Vier – Mit freundlichen Grüßen) |
| Die Fidelen Mölltaler | Gold, Platin und Diamant | Apple track found by search (Die Fidelen Mölltaler – Gold, Platin und Diamant) |
| Die Tahiti Tamoures | Wini-Wini | Apple track found by search (Die Tahiti Tamoures – Wini-Wini) |
| Die Wildecker Herzbuben | Herzilein | Apple track found by search (Die Wildecker Herzbuben – Herzilein) |
| Dieter Hallervorden, Helga Feddersen | Du, die Wanne ist voll | MusicBrainz matched as Dieter Hallervorden – Du, die Wanne ist voll |
| Dieter Thomas Kuhn & Band | Night Fever | Apple track found by search (Dieter Thomas Kuhn & Band – Night Fever) |
| Dion | The Wanderer | Apple track found by search (Dion – The Wanderer) |
| Dirk Busch | Du mußt deinen Weg alleine gehen | Apple track found by search (Dirk Busch – Du mußt deinen Weg alleine gehen) |
| DJ Antoine | Welcome to St. Tropez - DJ Antoine vs. Mad Mark Radio Edit | MusicBrainz matched as DJ Antoine – Welcome to St. Tropez |
| DJ Herzbeat, Roger Whittaker | Albany (feat. Roger Whittaker) | MusicBrainz matched as DJ Herzbeat – Albany |
| DJ Herzbeat, Sarah Engels | Weekend | MusicBrainz matched as DJ Herzbeat – Weekend |
| DJ Ötzi | Anton aus Tirol | Apple track found by search (DJ Ötzi – Anton aus Tirol (English Version)) |
| DJ Snake | Loco Contigo (feat. J. Balvin & Tyga) | MusicBrainz matched as DJ Snake – Loco Contigo |
| DJ Zipfelmann | Schlittenfahren in Tirol (Der Schnittensong) | MusicBrainz matched as DJ Zipfelmann – Schlittenfahren in Tirol; Apple track found by search (DJ Zipfelmann – Schlittenfahren in Tirol (Der Schnittensong)) |
| DNA | Tom's Diner | MusicBrainz has it only from 1990, after the card year |
| Döf | Codo (...düse im Sauseschritt) | MusicBrainz matched as Döf – Codo |
| Domenico Modugno | Volare | MusicBrainz has it only from 1987, after the card year; Apple track found by search (Domenico Modugno – Volare (Nel blu dipinto di blu)) |
| Dominik Hauser | Dallas | Apple track found by search (Dominik Hauser – Dallas (Theme from the TV Series)) |
| Dominik Hauser | L.A. Law - Law & Order | Apple track found by search (Dominik Hauser – L.A. Law (Main Theme From the Television Series)) |
| Don Omar | Danza Kuduro | Apple track found by search (Don Omar – Danza Kuduro (feat. Lucenzo)) |
| Donny Hathaway | This Christmas | Apple track found by search (Donny Hathaway – This Christmas) |
| Dooley Wilson | Casablanca (As Time Goes By) | MusicBrainz matched as Dooley Wilson – As Time Goes By |
| Doris Day | A Guy Is a Guy | Apple track found by search (Doris Day – A Guy Is a Guy) |
| Dorthe | Sind Sie der Graf von Luxemburg? | MusicBrainz has it only from 1988, after the card year; Apple track found by search (Dorthe – Sind Sie der Graf von Luxemburg?) |
| Douglas J. Cuomo | Sex And The City | MusicBrainz has it only from 2002, after the card year |
| Drafi Deutscher | Heute male ich dein Bild, Cindy Lou (The Birds and the Bees) | MusicBrainz matched as Drafi Deutscher – Heute male ich dein Bild, Cindy Lou; Apple track found by search (Drafi Deutscher – Heute male ich dein Bild, Cindy Lou (The Birds and the Bees)) |
| Drafi Deutscher | Marmor, Stein und Eisen bricht | MusicBrainz artist matched loosely (Drafi Deutscher and His Magics) |
| Eartha Kitt & Henri René and His Orchestra | Santa Baby | MusicBrainz matched as Eartha Kitt – Santa Baby; Apple track found by search (Eartha Kitt – Santa Baby) |
| Ed Sheeran & Justin Bieber | I Don't Care (with Justin Bieber) | MusicBrainz matched as Ed Sheeran & Justin Bieber – I Don't Care |
| Ed Sheeran | Merry Christmas | Apple track found by search (Ed Sheeran & Elton John – Merry Christmas) |
| Electric Callboy | We Got the Moves | MusicBrainz has it only from 2023, after the card year |
| Ella Endlich | Küss mich, Halt mich, Lieb mich (Drei Haselnüsse für Aschenbrödel) | MusicBrainz matched as Ella Endlich – Küss mich, Halt mich, Lieb mich; Apple track found by search (Ella Endlich – Küss mich, halt mich, lieb mich) |
| Eminem | 8 Mile (Lose Yourself) | MusicBrainz matched as Eminem – 8 Mile |
| Enigma | Sadeness - Pt. 1 / Radio Edit | MusicBrainz matched as Enigma – Sadeness; Apple track found by search (Enigma – Sadeness (Part I) [Meditation Mix]) |
| Ennio Morricone | Zwei glorreiche Halunken - The Good, The Bad And The Ugly | Apple track found by search (Ennio Morricone – The Good, The Bad and the Ugly (From "The Good, the Bad and the Ugly")) |
| Enrico Caruso | Rigoletto: "La Donna E' Mobile" | Apple track found by search (Enrico Caruso – Rigoletto: La donna è mobile (1908 Recording)) |
| Enrique Iglesias, Whitney Houston | Could I Have This Kiss Forever | MusicBrainz matched as Enrique Iglesias – Could I Have This Kiss Forever; Apple track found by search (Whitney Houston – Could I Have This Kiss Forever (with Enrique Iglesias) [Metro Mix]) |
| Erasure | Always - 2009 Remaster | MusicBrainz matched as Erasure – Always |
| Eric Clapton | Phenomenon (Change the World) | MusicBrainz matched as Eric Clapton – Change the World |
| Erik Silvester | Zucker im Kaffee | MusicBrainz has it only from 1978, after the card year; Apple track found by search (Erik Silvester – Zucker Im Kaffee) |
| Eros Ramazzotti | Cose Della Vita (with Tina Turner) | MusicBrainz matched as Eros Ramazzotti – Cose Della Vita |
| Eternal | I Wanna Be the Only One (feat. Bebe Winans) | MusicBrainz matched as Eternal – I Wanna Be the Only One |
| Eurythmics, Annie Lennox, Dave Stewart | Sweet Dreams (Are Made Of This) | MusicBrainz has it only from 2024, after the card year; Apple track found by search (Eurythmics – Sweet Dreams (Are Made of This)) |
| Fantasy | Flaschenpost | Apple track found by search (Fantasy – Flaschenpost) |
| Fats Domino | Blueberry Hill | Apple track found by search (Fats Domino – Blueberry Hill) |
| Fats Domino | I'm Walkin' | MusicBrainz has it only from 1959, after the card year |
| Felice Taylor | It May Be Winter Outside (But in My Heart It's Spring) | MusicBrainz has it only from 2003, after the card year; Apple track found by search (Felice Taylor – It May Be Winter Outside (But in My Heart It's Spring)) |
| Felix Jaehn | Ain't Nobody (Loves Me Better) (feat. Jasmine Thompson) | MusicBrainz matched as Felix Jaehn – Ain't Nobody |
| Felix Jaehn | Book Of Love (feat. Polina) | MusicBrainz matched as Felix Jaehn – Book Of Love |
| First Class | Beach Baby | Apple track found by search (First Class – Beach Baby) |
| Flo Rida | Low (feat. T-Pain) | MusicBrainz matched as Flo Rida – Low |
| Fools Garden | Lemon Tree | Apple track found by search (Fool's Garden – Lemon Tree (Studio Version 2009)) |
| Francis Lai | Love Story | MusicBrainz has it only from 1975, after the card year; Apple track found by search (Francis Lai – Love Story) |
| Frank Farian | Sie war erst siebzehn (und neu in der Stadt) | Apple track found by search (Frank Farian – Sie war erst siebzehn (und neu in der Stadt)) |
| Frank Lars | Ich kann nicht ohne Dich - Fox Mix | MusicBrainz matched as Frank Lars – Ich kann nicht ohne Dich; Apple track found by search (Frank Lars – Ich kann nicht ohne Dich (Fox Mix)) |
| Frankie Valli & The Four Seasons | December, 1963 (Oh What a Night!) | MusicBrainz has it only from 1991, after the card year |
| Fred Sonnenschein & seine Freunde | Ententanz (Ja wenn wir alle Englein wären) | MusicBrainz matched as Fred Sonnenschein & seine Freunde – Ja wenn wir alle Englein wären |
| Freddy Breck | Tanz mit mir durch die Nacht | Apple track found by search (Freddy Breck – Tanz mit mir durch die Nacht) |
| Freddy Quinn | Aloha-Ahe | Apple track found by search (Freddy Quinn – Aloha-Ahe) |
| Freddy Quinn | Heimweh (Dort wo die Blumen blüh'n) | MusicBrainz matched as Freddy Quinn – Heimweh |
| Funkstar De Luxe | Sun Is Shining | Apple track found by search (Funkstar De Luxe – Sun Is Shining (De Luxe Edit)) |
| fun. | We Are Young (feat. Janelle Monáe) | MusicBrainz matched as fun. – We Are Young |
| GABRIELLE | Bridget Jones - Schokolade zum Frühstück (Out Of Reach) | MusicBrainz matched as GABRIELLE – Out Of Reach |
| Gaby Baginsky | Wer hat dir das Küssen beigebracht | Apple track found by search (Gaby Baginsky – Wer hat dir das Küssen beigebracht) |
| Gary Portnoy | Cheers (Where Everybody Knows Your Name) | MusicBrainz matched as Gary Portnoy – Where Everybody Knows Your Name; Apple track found by search (Gary Portnoy – Where Everybody Knows Your Name (Cheers Theme) [2025 Remaster]) |
| Gebrüder Blattschuss | Früh-Stück | Apple track found by search (Gebrüder Blattschuss – Früh-Stück) |
| Geier Sturzflug | Die Pure Lust am Leben | MusicBrainz has it only from 2003, after the card year |
| Gene Autry & The Cass County Boys | Frosty the Snowman | MusicBrainz matched as Gene Autry – Frosty the Snowman |
| Gene Kelly | Du sollst mein Glücksstern sein (Singin' In The Rain) | MusicBrainz matched as Gene Kelly – Singin' In The Rain; Apple track found by search (Gene Kelly – Singin' In the Rain) |
| George Baker | Una Paloma Blanca | MusicBrainz has it only from 1991, after the card year; Apple track found by search (George Baker – Una Paloma Blanca) |
| George Harrison | My Sweet Lord (2014 Remaster) | MusicBrainz matched as George Harrison – My Sweet Lord |
| George Michael, Aretha Franklin | I Knew You Were Waiting (For Me) | MusicBrainz matched as George Michael – I Knew You Were Waiting (For Me) |
| Georgie Fame & The Blue Flames | Yeh, Yeh | Apple track found by search (Georgie Fame & The Blue Flames – Yeh, Yeh) |
| G.G. Anderson | Nein heißt ja | MusicBrainz has it only from 2000, after the card year |
| Gianna Nannini & Edoardo Bennato | Un'Estate Italiana | MusicBrainz matched as Gianna Nannini – Un'Estate Italiana |
| Gipsy Kings | Baila Me | Apple track found by search (Gipsy Kings – Baila Me) |
| Gipsy Kings | Djobi, Djoba | Apple track found by search (Gipsy Kings – Djobi Djoba) |
| Gipsy Kings | Volare (Nel Blu di Pinto di Blu) | MusicBrainz matched as Gipsy Kings – Volare |
| Gitte Haenning | Ich will alles | Apple track found by search (Gitte Hænning – Ich will alles) |
| Gitte Hænning | Ich will 'nen Cowboy als Mann | MusicBrainz has it only from 1988, after the card year; Apple track found by search (Gitte Hænning – Ich will 'nen Cowboy als Mann (Remastered 2004)) |
| Gitti Und Erica | Heidi | Apple track found by search (Gitti & Erica – Heidi) |
| Glen Hansard, Markéta Irglová | Once (Falling Slowly) | MusicBrainz matched as Glen Hansard – Once |
| Glenn Miller | In the Mood | Apple track found by search (Glenn Miller and His Orchestra – In the Mood) |
| Gölä, Trauffer | Büetzer Buebe | MusicBrainz matched as Gölä – Büetzer Buebe |
| Gotye | Somebody That I Used to Know (feat. Kimbra) | MusicBrainz matched as Gotye – Somebody That I Used to Know |
| Gunter Gabriel, Linda Feller | Wenn ich nur ein Trucker wär | MusicBrainz matched as Gunter Gabriel – Wenn ich nur ein Trucker wär; Apple track found by search (Gunter Gabriel & Linda Feller – Wenn ich nur ein Trucker wär) |
| Gwen Stefani | You Make It Feel Like Christmas (feat. Blake Shelton) | MusicBrainz matched as Gwen Stefani – You Make It Feel Like Christmas |
| H-Blockx | Risin' High - Def Core Grind Version | MusicBrainz matched as H-Blockx – Risin' High |
| Harry Belafonte | Day-O (The Banana Boat Song) | MusicBrainz matched as Harry Belafonte – Day-O |
| Harry Nilsson | Without You | MusicBrainz has it only from 1991, after the card year; Apple track found by search (Harry Nilsson – Without You) |
| Harry Simeone Chorale | The Little Drummer Boy | Apple track found by search (Harry Simeone Chorale – The Little Drummer Boy) |
| Heintje | Mama | Apple track found by search (Heintje – Mama) |
| Heintje | O Tannenbaum | Apple track found by search (Heintje – O Tannenbaum) |
| Helen Shapiro | Walkin' Back to Happiness | Apple track found by search (Helen Shapiro – Walkin' Back to Happiness (2003 Remastered Version)) |
| Helge Schneider | Katzeklo | Apple track found by search (Helge Schneider – Katzeklo) |
| Henry Mancini | Der Rosarote Panther | MusicBrainz has it only from 1995, after the card year |
| Hermes House Band | Country Roads (Dance Radio Version) | Apple track found by search (Hermes House Band – Country Roads (Radio Dance Version)) |
| High School Musical Cast | High School Musical (We're All In This Together) | MusicBrainz matched as High School Musical Cast – We're All In This Together |
| Howard Shore | Der Herr der Ringe (The Shire) | Apple track found by search (Howard Shore – The Shire) |
| Ian Dury | Hit Me with Your Rhythm Stick | MusicBrainz artist matched loosely (Ian Dury & the Blockheads) |
| Ich + Ich | So soll es bleiben - Single Edit | MusicBrainz matched as Ich + Ich – So soll es bleiben |
| Icona Pop, Charli XCX | I Love It (feat. Charli XCX) | MusicBrainz matched as Icona Pop – I Love It |
| Imagine Dragons | Enemy (with JID) | MusicBrainz matched as Imagine Dragons – Enemy |
| Inner Circle | Bad Boys | Apple track found by search (Inner Circle – Bad Boys) |
| Irene Cara | Fame - Der Weg zum Ruhm | MusicBrainz matched as Irene Cara – Fame |
| Irene Cara | Flashdance...What a Feeling | MusicBrainz has it only from 2007, after the card year; Apple track found by search (Irene Cara – Flashdance...What a Feeling) |
| James Brown | Get Up I Feel Like Being Like A Sex Machine | MusicBrainz has it only from 1995, after the card year; Apple track found by search (James Brown – Get Up I Feel Like Being Like A Sex Machine (Pts. 1 & 2)) |
| James Brown | Get Up I Feel Like Being Like A Sex Machine, Pts. 1 & 2 | MusicBrainz has it only from 2018, after the card year; Apple track found by search (James Brown – Get Up I Feel Like Being Like A Sex Machine, Pts. 1 & 2 (feat. The Original J.B.s)) |
| James Horner | Braveheart (For The Love Of A Princess) | MusicBrainz matched as James Horner – For The Love Of A Princess |
| James Horner | Willow | MusicBrainz has it only from 2015, after the card year |
| James Last | Petersburger Schlittenfahrt | Apple track found by search (James Last – Petersburger Schlittenfahrt) |
| Jamiroquai | Godzilla (Deeper Underground) | MusicBrainz matched as Jamiroquai – Deeper Underground |
| Jason Derulo | Talk Dirty (feat. 2 Chainz) | MusicBrainz matched as Jason Derulo – Talk Dirty |
| JAY-Z, Linkin Park | Numb / Encore | MusicBrainz matched as JAY-Z – Numb / Encore |
| Jennifer Lopez (feat. Jadakiss & Styles P.) | Jenny from the Block | Apple track found by search (Jennifer Lopez – Jenny from the Block (Seismic Crew's Latin Disco Trip)) |
| Jerry Goldsmith | Star Trek | MusicBrainz has it only from 1997, after the card year |
| Jerry Lee Lewis | Whole Lotta Shakin' Going On | MusicBrainz has it only from 1969, after the card year; Apple track found by search (Jerry Lee Lewis – Whole Lotta Shakin’ Going On) |
| Jimi Hendrix | All Along the Watchtower | MusicBrainz has it only from 1971, after the card year; Apple track found by search (Jimi Hendrix – All Along the Watchtower) |
| Jimi Hendrix | Hey Joe | MusicBrainz has it only from 1971, after the card year |
| Jimi Jamison | Baywatch (I'm Always Here) | Apple track found by search (Jimi Jamison – I'm Always Here (Baywatch Theme)) |
| Jimmy Cliff | You Can Get It If You Really Want | Apple track found by search (Jimmy Cliff – You Can Get It If You Really Want) |
| Jive Bunny and the Mastermixers | Swing The Mood | Apple track found by search (Jive Bunny & The Mastermixers – Swing The Mood) |
| Joe Cocker, Jennifer Warnes | Ein Offizier und ein Gentleman (Up Where We Belong) | MusicBrainz matched as Joe Cocker – Up Where We Belong |
| Joel Corry | Head & Heart (feat. MNEK) | MusicBrainz matched as Joel Corry – Head & Heart |
| Johannes Oerding | Ich komm nach Haus (Wenn das Jahr zu Ende geht) | Apple track found by search (Johannes Oerding – Ich komm nach Haus (Wenn das Jahr zu Ende geht) (aus "Sing meinen Song - die Weihnachtsparty, Vol. 6")) |
| John Barry | Jenseits von Afrika (I Had A Farm) | Apple track found by search (John Barry – Jenseits von Afrika (Flying Over Africa)) |
| John Carpenter | Halloween - Die Nacht des Grauens | Apple track found by search (John Carpenter – Halloween) |
| John E. Davis | Beverly Hills 90210 | MusicBrainz has it only from 2000, after the card year |
| John Kincade | Dreams Are Ten A Penny | MusicBrainz has it only from 1991, after the card year |
| John Travolta, Olivia Newton-John | Grease (Summer Nights) | MusicBrainz matched as John Travolta, Olivia Newton-John – Summer Nights |
| John Williams | Harry Potter und der Stein der Weisen (Prologue) | MusicBrainz matched as John Williams – Prologue |
| John Williams | Jäger des verlorenen Schatzes (Indiana Jones) | Apple track found by search (John Williams – Jäger des verlorenen Schatzes (March and Love Theme)) |
| John Williams | Jurassic Park | MusicBrainz has it only from 1995, after the card year |
| John Williams | Kevin - Allein zu Haus (Somewhere in My Memory) | MusicBrainz matched as John Williams – Somewhere in My Memory |
| John Williams | Superman | MusicBrainz has it only from 1995, after the card year; Apple track found by search (John Williams – Superman - Main Theme (From “Superman: The Movie”)) |
| Jonathan Wolff | Seinfeld | MusicBrainz has it only from 1996, after the card year |
| Jordin Sparks (feat. Chris Brown) | No Air | Apple track found by search (Jordin Sparks – No Air (Duet With Chris Brown)) |
| José Feliciano,Ewan McGregor,Jacek Koman | Moulin Rouge (El Tango De Roxanne) | MusicBrainz matched as José Feliciano – El Tango De Roxanne |
| José González | Das erstaunliche Leben des Walter Mitty (Stay Alive) | MusicBrainz matched as José González – Stay Alive |
| Joseito Fernandez | Guajira Guantanamera | MusicBrainz has it only from 1940, after the card year; Apple track found by search (Joseito Fernandez – Guajira Guantanamera) |
| Judy Garland | Der Zauberer von Oz (Over The Rainbow) | MusicBrainz matched as Judy Garland – Over The Rainbow |
| Juju | Vermissen (feat. Henning May) | MusicBrainz matched as Juju – Vermissen |
| Julie Andrews, Dick Van Dyke | Marry Poppins (Supercalifragilisticexpialidocious) | MusicBrainz matched as Julie Andrews, Dick Van Dyke – Supercalifragilisticexpialidocious |
| Julio Iglesias & Willie Nelson | To All the Girls I've Loved Before | Apple track found by search (Julio Iglesias – To All the Girls I've Loved Before) |
| Jürgen Drews | Wieder Alles im Griff | Apple track found by search (Jürgen Drews – Wieder alles im Griff (Version 2017)) |
| Jürgen Marcus | Ein Festival der Liebe | Apple track found by search (Jürgen Marcus – Ein Festival der Liebe) |
| Jürgen von der Lippe | Guten Morgen, liebe Sorgen | Apple track found by search (Jürgen von der Lippe – Guten Morgen, liebe Sorgen (Single)) |
| Justin Timberlake | SexyBack (feat. Timbaland) | MusicBrainz matched as Justin Timberlake – SexyBack |
| Justin Timberlake | What Goes Around...Comes Around | Apple track found by search (Justin Timberlake – What Goes Around...Comes Around (Interlude)) |
| Justin Wellington | Iko Iko (My Bestie) (feat. Small Jam) | MusicBrainz matched as Justin Wellington – Iko Iko |
| Kanye West, Jamie Foxx | Gold Digger | MusicBrainz matched as Kanye West – Gold Digger |
| Karat | Über sieben Brücken musst du gehn | Apple track found by search (Karat – Über sieben Brücken musst du gehn) |
| Karel Gott, Darinka | Fang das Licht | MusicBrainz matched as Karel Gott – Fang das Licht |
| Karel Gott | Die Biene Maja | MusicBrainz has it only from 1977, after the card year |
| Karen O, Ezra Koenig | Her (The Moon Song) | MusicBrainz matched as Karen O – The Moon Song |
| Karl Dall | Millionen Frauen lieben mich | Apple track found by search (Karl Dall – Millionen Frauen lieben mich) |
| Karl Jenkins | Adiemus | MusicBrainz has it only from 2002, after the card year |
| Katrina & The Waves | Walking On Sunshine | Apple track found by search (Katrina and the Waves – Walking On Sunshine) |
| KC & The Sunshine Band | Shake Your Booty | MusicBrainz has it only from 1998, after the card year; Apple track found by search (KC and The Sunshine Band – Shake Your Booty) |
| Keala Settle | Greatest Showman (This Is Me) | MusicBrainz matched as Keala Settle – This Is Me |
| Kendrick Lamar, SZA | Black Panther (All The Stars) | MusicBrainz matched as Kendrick Lamar – All The Stars |
| Klaus & Klaus | An Der Nordseeküste | Apple track found by search (Klaus & Klaus – An der Nordseeküste) |
| Klaus & Klaus | Der Eiermann | Apple track found by search (Klaus & Klaus – Der Eiermann) |
| Klaus Badelt | Fluch der Karibik (He's a Pirate) | MusicBrainz matched as Klaus Badelt – He's a Pirate |
| Klaus Lage | Faust auf Faust (Schimanski) | Apple track found by search (Klaus Lage – Faust auf Faust (Schimanski)) |
| Klingande | Jubel - Radio Edit | MusicBrainz matched as Klingande – Jubel |
| Klostertaler, Kastelruther Spatzen, Nockis | Hal-Le-Lu-Ja | Apple track found by search (Klostertaler, Kastelruther Spatzen & Nockis – Hal-le-lu-ja) |
| Klostertaler | Ciao D'Amore | Apple track found by search (Klostertaler – Ciao D'amore) |
| K.Maro | Femme Like U | MusicBrainz has it only from 2008, after the card year; Apple track found by search (K.Maro – Femme Like U) |
| Kraftwerk | Taschenrechner - 2009 Remaster | MusicBrainz matched as Kraftwerk – Taschenrechner |
| Kristina Bach | ...es kribbelt und es prickelt... | Apple track found by search (Kristina Bach – ... es kribbelt und es prickelt...) |
| Kristine W | Feel What You Want | Apple track found by search (Kristine W – Feel What You Want (Our Tribe Vocal Mix)) |
| Kygo | It Ain't Me (with Selena Gomez) | MusicBrainz matched as Kygo – It Ain't Me |
| Kylie Minogue | Every Day's Like Christmas | Apple track found by search (Kylie Minogue – Every Day's Like Christmas) |
| Lady A | Need You Now | Apple track found by search (Lady A – Need You Now) |
| Lady Gaga, Bradley Cooper | Shallow | MusicBrainz matched as Lady Gaga – Shallow |
| Lady Gaga | A Star Is Born (Always Remember Us This Way) | MusicBrainz matched as Lady Gaga – Always Remember Us This Way |
| Lea Salonga, Brad Kane | Aladdin (A Whole New World) | MusicBrainz matched as Lea Salonga – Aladdin (A Whole New World); Apple track found by search (Lea Salonga & Brad Kane – A Whole New World (From "Aladdin")) |
| LeAnn Rimes | Coyote Ugly (Can't Fight The Moonlight) | MusicBrainz matched as LeAnn Rimes – Can't Fight The Moonlight |
| Led Zeppelin | Immigrant Song - Remaster | MusicBrainz matched as Led Zeppelin – Immigrant Song |
| Lenny Kravitz | It Ain't Over 'Til It's Over | Apple track found by search (Lenny Kravitz – It Ain't Over 'Til It's Over) |
| Leona Lewis | Avatar - Aufbruch nach Pandora (I See You) | MusicBrainz matched as Leona Lewis – I See You |
| Leonard Bernstein | West Side Story (America) | MusicBrainz has it only from 1993, after the card year; Apple track found by search (Leonard Bernstein – America) |
| Les Paul & Mary Ford | Vaya Con Dios | Apple track found by search (Les Paul & Mary Ford – Vaya Con Dios) |
| Liane | Ich lass' nur noch Sonne in mein Herz | Apple track found by search (Liane – Ich lass' nur noch Sonne in mein Herz) |
| Lill-Babs | Wo finde ich den Mann? | MusicBrainz has it only from 2018, after the card year; Apple track found by search (Lill Babs – Wo finde ich den Mann?) |
| Linda Hesse | Knutschen...ich kann nichts dafür | Apple track found by search (Linda Hesse – Knutschen...ich kann nichts dafür) |
| Linda Hesse | Weihnachten zu zweit | Apple track found by search (Linda Hesse – Weihnachten zu zweit) |
| Linkin Park | Transformers (What I've Done) | MusicBrainz matched as Linkin Park – What I've Done |
| Liquido | Narcotic - Radio Edit | MusicBrainz matched as Liquido – Narcotic |
| Lisa Stansfield | All Around the World - Remastered | MusicBrainz matched as Lisa Stansfield – All Around the World |
| Little Eva | The Locomotion | MusicBrainz has it only from 1973, after the card year; Apple track found by search (Little Eva – The Locomotion) |
| Liza Minnelli | Cabaret | Apple track found by search (Liza Minnelli – Cabaret) |
| Lizzo | Good as Hell | Apple track found by search (Lizzo – Good As Hell) |
| Louis Prima | Buona sera | MusicBrainz has it only from 1956, after the card year |
| Luis Bacalov, Rocky Roberts | Django Unchained (Django) | MusicBrainz matched as Luis Bacalov – Django |
| Luis Fonsi & Demo Lovato | Échame La Culpa | MusicBrainz matched as Luis Fonsi – Échame La Culpa |
| Macklemore & Ryan Lewis | Can't Hold Us (feat. Ray Dalton) | MusicBrainz matched as Macklemore & Ryan Lewis – Can't Hold Us |
| Macklemore & Ryan Lewis | Thrift Shop (feat. Wanz) | MusicBrainz matched as Macklemore & Ryan Lewis – Thrift Shop |
| Madcon (with Ray Dalton) | Don't Worry | MusicBrainz matched as Madcon – Don't Worry |
| MadHouse | Like a Prayer | Apple track found by search (Mad'House – Like a Prayer (Radio Edit)) |
| Major Lazer & DJ Snake | Lean On (feat. MØ) | MusicBrainz matched as Major Lazer & DJ Snake – Lean On; Apple track found by search (Major Lazer – Lean On (feat. MØ & DJ Snake)) |
| Måneskin | GOSSIP (feat. Tom Morello) | MusicBrainz matched as Måneskin – GOSSIP |
| Manuela Fellner | 7 Tage hat die Woche | Apple track found by search (Manuela Fellner – 7 Tage hat die Woche (Album-Version)) |
| Margaret Whiting | Baby, It's Cold Outside | Apple track found by search (Margaret Whiting – Baby, it's cold outside) |
| Margot Eskens | ... Denn es ist Weihnachtszeit | Apple track found by search (Margot Eskens – ... Denn es ist Weihnachtszeit) |
| Marianne Rosenberg | Aba Heidschi Bumbeidschi | MusicBrainz has it only from 2013, after the card year; Apple track found by search (Marianne Rosenberg – Aba Heidschi Bumbeidschi) |
| Marilyn Monroe | Blondinen bevorzugt (Diamonds Are a Girl's Best Friend) | MusicBrainz matched as Marilyn Monroe – Diamonds Are a Girl's Best Friend; Apple track found by search (Marilyn Monroe – Diamonds Are a Girl's Best Friend) |
| Mark Forster, LEA | Drei Uhr Nachts | Apple track found by search (Mark Forster & LEA – Drei Uhr Nachts) |
| Mark Forster | Au Revoir (feat. Sido) | MusicBrainz matched as Mark Forster – Au Revoir |
| Mark Morrison | Return of the Mack | Apple track found by search (Mark Morrison – Return of the Mack) |
| Mark 'Oh | Let This Party Never End | Apple track found by search (Mark 'Oh – Let This Party Never End) |
| Mark Ronson, Amy Winehouse | Valerie (feat. Amy Winehouse) | MusicBrainz matched as Mark Ronson – Valerie |
| Mark Ronson | Nothing Breaks Like a Heart (feat. Miley Cyrus) | MusicBrainz matched as Mark Ronson – Nothing Breaks Like a Heart |
| Mark Ronson | Uptown Funk (feat. Bruno Mars) | MusicBrainz matched as Mark Ronson – Uptown Funk |
| Markus Becker, Bierkapitän | Bierkapitän | MusicBrainz matched as Markus Becker – Bierkapitän |
| Maroon 5, Christina Aguilera | Moves Like Jagger | Apple track found by search (Maroon 5 – Moves Like Jagger (feat. Christina Aguilera) [Studio Recording From The Voice Performance]) |
| Marshmello (with Jonas Brothers) | Leave Before You Love Me | Apple track found by search (Marshmello & Jonas Brothers – Leave Before You Love Me) |
| Martha Reeves & The Vandellas | Dancing In The Street | Apple track found by search (Martha Reeves & The Vandellas – Dancing In The Street (Single Version)) |
| Martha Reeves & The Vandellas | Heatwave | MusicBrainz has it only from 1980, after the card year |
| Martin Garrix | High On Life (feat. Bonn) | MusicBrainz matched as Martin Garrix – High On Life |
| Martin Garrix | We Are The People (feat. Bono & The Edge) | MusicBrainz matched as Martin Garrix – We Are The People |
| Matchbox Twenty | Push | Apple track found by search (Matchbox Twenty – Push) |
| Matthias Reim | Ich Will dich immer noch | Apple track found by search (Matthias Reim – Ich will dich immer noch) |
| Matthias Reim | Im Himmel geht es weiter | Apple track found by search (Matthias Reim – Im Himmel geht es weiter (Remastered)) |
| Maurice Williams | Stay | MusicBrainz artist matched loosely (Maurice Williams and the Zodiacs) |
| Max Giesinger | Irgendwann ist jetzt (Karte falsch!) | MusicBrainz matched as Max Giesinger – Irgendwann ist jetzt |
| Max Mutzke | Can't Wait Until Tonight - Radio Version | MusicBrainz matched as Max Mutzke – Can't Wait Until Tonight; Apple track found by search (Max Mutzke – Can't Wait Until Tonight (Radio Version)) |
| Max Steiner | Vom Winde verweht | MusicBrainz has it only from 2009, after the card year; Apple track found by search (Max Steiner – Vom Winde Verweht (Tara's Theme)) |
| MGK | Bloody Valentine | MusicBrainz has it only from 2025, after the card year |
| Michael Jackson | Ben | Apple track found by search (Michael Jackson – Ben (Single Version)) |
| Michael McDonald | Sweet Freedom | Apple track found by search (Michael McDonald – Sweet Freedom) |
| Michael Morgan | Es Schiesst Ein Blitz In Meinen Kopf | Apple track found by search (Michael Morgan – Es schiesst ein Blitz in meinen Kopf) |
| Michael Nyman | Das Piano (The Heart Asks Pleasure First) | MusicBrainz matched as Michael Nyman – The Heart Asks Pleasure First |
| Michael Wendler | Prinzessin | Apple track found by search (Michael Wendler – Prinzessin) |
| Michael Wendler | Sie liebt den DJ - 2004 | MusicBrainz matched as Michael Wendler – Sie liebt den DJ |
| Michel Teló | Nossa Nossa (Ai Se Eu Te Pego) | MusicBrainz matched as Michel Teló – Ai Se Eu Te Pego |
| Mickie Krause | Zehn Nackte Friseusen | Apple track found by search (Mickie Krause – Zehn nackte Friseusen) |
| Miguel Ríos | Song of Joy - Himno a la alegría | MusicBrainz matched as Miguel Ríos – Himno a la alegría; Apple track found by search (Miguel Ríos – Song of Joy (Himno a la alegría)) |
| Mike + The Mechanics | Over My Shoulder | Apple track found by search (Mike + The Mechanics – Over My Shoulder (Remastered 2014)) |
| Mike + The Mechanics | Word Of Mouth | Apple track found by search (Mike + The Mechanics – Word Of Mouth) |
| Mike Krüger | Bodo mit dem Bagger | Apple track found by search (Mike Krüger – Bodo mit dem Bagger (Remastered 2023)) |
| Mike Krüger | Das Trampolin - Live | MusicBrainz matched as Mike Krüger – Das Trampolin |
| Mike Krüger | Der Nippel | Apple track found by search (Mike Krüger – Der Nippel (Remastered 2023)) |
| Modjo | Lady - Hear Me Tonight | MusicBrainz matched as Modjo – Lady |
| Moloko | Sing It Back - Boris Musical Mix - Edit | MusicBrainz matched as Moloko – Sing It Back |
| Monique | Sieben Tage, Sieben Nächte | Apple track found by search (Mo'Nique – Sieben Tage, sieben Nächte) |
| Monty Python | Das Leben des Brian (Always Look On The Bright Side Of Life) | MusicBrainz matched as Monty Python – Always Look On The Bright Side Of Life; Apple track found by search (Monty Python – Always Look On the Bright Side of Life) |
| Münchner Zwietracht, Rudolph Moshammer | Moos Hamma | MusicBrainz matched as Münchner Zwietracht – Moos Hamma |
| MusikApostel | Ich bin wer ich sein will (feat. Allessa) | MusicBrainz matched as MusikApostel – Ich bin wer ich sein will |
| Nana Darkman | Lonely | MusicBrainz has it only from 2021, after the card year; Apple track found by search (Nana Darkman – Lonely (feat. Alexandra Prince) [2021 Remastered Version]) |
| Nana Mouskouri | Ich Steh An Deiner Krippe Hier | Apple track found by search (Nana Mouskouri – Ich Steh an Deiner Krippe Hier) |
| Nancy Sinatra & Lee Hazlewood | Summer Wine | MusicBrainz matched as Nancy Sinatra – Summer Wine; Apple track found by search (Nancy Sinatra – Summer Wine) |
| Nat King Cole Trio | The Christmas Song (Merry Christmas To You) | MusicBrainz has it only from 1949, after the card year |
| Nathan Evans | Wellerman - Sea Shanty | Apple track found by search (Nathan Evans – Wellerman (Sea Shanty)) |
| Neil Diamond | Sweet Caroline | Apple track found by search (Neil Diamond – Sweet Caroline (Single Version)) |
| N.E.R.D | She Wants To Move | Apple track found by search (N.E.R.D – She Wants to Move (Basement Jaxx Main Mix)) |
| Nick Straker | Walk In The Park | MusicBrainz has it only from 1996, after the card year |
| Nicola Piovani | Das Leben ist schön (La vita è bella) | MusicBrainz matched as Nicola Piovani – La vita è bella; Apple track found by search (Nicola Piovani – La vita e' bella) |
| Nicole | Dann küss mich doch | Apple track found by search (Nicole – Dann küss mich doch) |
| Nicole | Weihnacht zuhause | MusicBrainz has it only from 2000, after the card year; Apple track found by search (Nicole – Weihnacht zuhause) |
| NIC | Ich will dich für immer - Disco Fox | MusicBrainz matched as NIC – Ich will dich für immer |
| Nik P. | Gloria II | Apple track found by search (Nik P. – Gloria II - Lebenslust & Leidenschaft) |
| Nina Simone | Ain't Got No / I Got Life | MusicBrainz has it only from 1997, after the card year; Apple track found by search (Nina Simone – Ain't Got No - I Got Life) |
| Nino de Angelo | Engel | Apple track found by search (Nino de Angelo – Engel) |
| Nino Rota | Der Pate (The Godfather Waltz) | MusicBrainz matched as Nino Rota – The Godfather Waltz |
| No Angels | Daylight in Your Eyes (Radio Version) | MusicBrainz matched as No Angels – Daylight in Your Eyes; Apple track found by search (No Angels – Daylight in Your Eyes (Big Band)) |
| No Mercy | Where Do You Go | Apple track found by search (No Mercy – Where Do You Go) |
| Nockis | Du warst der geilste Fehler meines Lebens | MusicBrainz has it only from 2020, after the card year |
| Ofenbach | Wasted Love (feat. Lagique) | MusicBrainz matched as Ofenbach – Wasted Love |
| Ofra Haza | Im Nin'alu | Apple track found by search (Ofra Haza – Im Nin'Alu (English Mix Edit)) |
| Olaf Henning | Das Spiel ist aus (Game Over) | Apple track found by search (Olaf Henning – Das Spiel ist aus (Maxi Version)) |
| Olaf Henning | Ich bin nicht mehr dein Clown (... die Manege ist leer) | MusicBrainz matched as Olaf Henning – Ich bin nicht mehr dein Clown; Apple track found by search (Olaf Henning – Ich bin nicht mehr Dein Clown) |
| Olaf Henning | Ich mal für dich den Himmel blau | Apple track found by search (Olaf Henning – Ich mal für dich den Himmel blau) |
| Oli. P | Flugzeuge im Bauch - Radio Mix | MusicBrainz matched as Oli. P – Flugzeuge im Bauch |
| Oliver Haidt | Lebenslänglich Du | Apple track found by search (Oliver Haidt – Lebenslänglich Du) |
| Oliver Onions | Flying Through the Air | Apple track found by search (Oliver Onions – Flying Through the Air) |
| Oliver Onions | Santamaria | Apple track found by search (Oliver Onions – Santamaria) |
| Olivia Newton-John & Electric Light Orchestra | Xanadu | MusicBrainz matched as Olivia Newton-John – Xanadu |
| Olivia Newton-John | Physical - Remastered 2021 | MusicBrainz matched as Olivia Newton-John – Physical |
| Olly Murs | Troublemaker (feat. Flo Rida) | MusicBrainz matched as Olly Murs – Troublemaker |
| OMI (feat. Felix Jaehn) | Cheerleader | MusicBrainz matched as OMI – Cheerleader |
| OneRepublic | Run | MusicBrainz has it only from 2021, after the card year |
| Otto Waalkes | Wir haben Grund zum Feiern | Apple track found by search (Otto Waalkes – Wir haben Grund zum Feiern) |
| Patrick Hernandez | Born to Be Alive | Apple track found by search (Patrick Hernandez – Born to Be Alive (Extended Version)) |
| Patrick Lindner | Halleluja - Auf das Leben | Apple track found by search (Patrick Lindner – Halleluja - Auf das Leben) |
| Patrick Lindner | Weihnachten bin ich daheim | Apple track found by search (Patrick Lindner – Weihnachten bin ich daheim) |
| Patti Smith | Because the Night | Apple track found by search (Patti Smith – Because the Night) |
| Paul Anka | Zwei Mädchen aus Germany | Apple track found by search (Paul Anka – Zwei Mädchen aus Germany) |
| Paul Kuhn | Es gibt kein Bier auf Hawaii | Apple track found by search (Paul Kuhn – Es gibt kein Bier auf Hawaii) |
| Pérez Prado | Patricia | MusicBrainz artist matched loosely (Pérez Prado and His Orchestra) |
| Perry Como & The Fontane Sisters | It's Beginning to Look a Lot Like Christmas | MusicBrainz artist matched loosely (Perry Como & The Fontane Sisters with Mitchell Ayres and His Orchestra) |
| Peter Cornelius | Du entschuldige i kenn di | MusicBrainz has it only from 2015, after the card year |
| Peter Fox | Zukunft Pink (feat. Inéz) | MusicBrainz matched as Peter Fox – Zukunft Pink |
| Peter Kent | It's a Real Good Feeling | Apple track found by search (Peter Kent – It's A Real Good Feeling) |
| Peter Maffay | Brief an den Weihnachtsmann | MusicBrainz has it only from 2007, after the card year; Apple track found by search (Peter Maffay – Brief an den Weihnachtsmann) |
| Peter Maffay | Über sieben Brücken musst du geh'n - Digitally Remastered | MusicBrainz matched as Peter Maffay – Über sieben Brücken musst du geh'n |
| Phil Collins | A Groovy Kind of Love | MusicBrainz has it only from 1990, after the card year; Apple track found by search (Phil Collins – A Groovy Kind of Love) |
| Phil Harris, Bruce Reitherman | Das Dschungelbuch (The Bare Necessities) | MusicBrainz matched as Phil Harris – The Bare Necessities |
| Pink Floyd | Another Brick In The Wall, Pt. 2 | MusicBrainz has it only from 1999, after the card year; Apple track found by search (Pink Floyd – Another Brick In the Wall, Pt. 2) |
| Pink Floyd | Money - 2023 Remaster | MusicBrainz matched as Pink Floyd – Money |
| Pitbull | Give Me Everything (feat. Nayer) | MusicBrainz matched as Pitbull – Give Me Everything |
| Plüsch | Heimweh | Apple track found by search (Plüsch – Heimweh) |
| Pras (feat. Ol' Dirty Bastard & Mýa) | Ghetto Supastar (That is What You Are) | MusicBrainz matched as Pras – Ghetto Supastar (That is What You Are) |
| Pras | Ghetto Supastar (That is What You Are) (feat. Ol' Dirty Bastard & Mýa) | MusicBrainz matched as Pras – Ghetto Supastar |
| Prince Ital Joe | United (feat. Marky Mark) | MusicBrainz matched as Prince Ital Joe – United |
| Prince | The Most Beautiful Girl In the World | Apple track found by search (Prince – The Most Beautiful Girl in the World) |
| PS Alex | Eine Frau die mich nach Hause trägt | Apple track found by search (PS Alex – Eine Frau, die mich nach Hause trägt (Stadl-Star Mix)) |
| Pulp | Common People - Full Length Version | MusicBrainz matched as Pulp – Common People |
| Purple Disco Machine & Kung | Substitution | MusicBrainz matched as Purple Disco Machine – Substitution |
| Queen, David Bowie | Under Pressure | MusicBrainz matched as Queen – Under Pressure |
| Queen | A Winter's Tale - 2011 Remaster | MusicBrainz matched as Queen – A Winter's Tale |
| Queen | Highlander - Es kann nur einen geben (Princes Of The Universe) | MusicBrainz matched as Queen – Princes Of The Universe |
| Queen | I Want To Break Free | Apple track found by search (Queen – I Want to Break Free) |
| Ray Charles | Georgia on My Mind | Apple track found by search (Ray Charles and His Orchestra – Georgia On My Mind) |
| Ray Charles | Hit the Road Jack | Apple track found by search (Ray Charles – Hit the Road Jack) |
| Rednex | The Spirit of the Hawk | Apple track found by search (Rednex – The Spirit of the Hawk) |
| Rex Gildo | Silberglocken läuten | Apple track found by search (Rex Gildo – Silberglocken läuten) |
| Ricky Martin | She Bangs - English Version | MusicBrainz matched as Ricky Martin – She Bangs |
| Ricky Martin | The Cup of Life (La Copa De La Vida) | MusicBrainz matched as Ricky Martin – The Cup of Life |
| Riton | Friday (feat. Mufasa & Hypeman) | MusicBrainz matched as Riton – Friday |
| Robert D. | Matrix (Clubbed to Death) | Apple track found by search (Robert D – Clubbed to Death (From "the Matrix")) |
| Robert Folk | Police Academy | MusicBrainz has it only from 1993, after the card year |
| Robin Beck | First Time | Apple track found by search (Robin Beck – First Time) |
| Robin Schulz,Lilly Wood and The Prick | Prayer in C | MusicBrainz matched as Robin Schulz – Prayer in C; Apple track found by search (Lilly Wood & The Prick & Robin Schulz – Prayer in C (Robin Schulz Radio Edit)) |
| Robin Schulz | In Your Eyes (feat. Alida) | MusicBrainz matched as Robin Schulz – In Your Eyes |
| Robin Thicke | When I Get You Alone | MusicBrainz knows no release date for it; Apple track found by search (Robin Thicke – When I Get You Alone) |
| Rod Stewart | Baby Jane - 2008 Remaster | MusicBrainz matched as Rod Stewart – Baby Jane |
| Rodrigo Amarante | Narcos (Tuyo) | MusicBrainz matched as Rodrigo Amarante – Tuyo |
| Rolf Zuckowski | In der Weihnachtsbäckerei | MusicBrainz artist matched loosely (Rolf Zuckowski und seine Freunde) |
| Rolfe Kent | Dexter | MusicBrainz has it only from 2015, after the card year |
| Ronan Keating | Notting Hill (When You Say Nothing At All) | MusicBrainz matched as Ronan Keating – When You Say Nothing At All |
| Roy Black | Wünsche und Gedanken | Apple track found by search (Roy Black – Wünsche und Gedanken) |
| Rudi Carrell | Goethe war gut | Apple track found by search (Rudi Carrell – Goethe war gut) |
| Rudimental | Waiting All Night (feat. Ella Eyre) | MusicBrainz matched as Rudimental – Waiting All Night |
| Rufus, Chaka Khan | Ain't Nobody | MusicBrainz matched as Rufus – Ain't Nobody |
| Ryan Gosling, Emma Stone | La La Land (City Of Stars) | MusicBrainz matched as Ryan Gosling, Emma Stone – City Of Stars; Apple track found by search (Ryan Gosling – City of Stars) |
| Ryan Gosling | Barbie (I'm Just Ken) | MusicBrainz matched as Ryan Gosling – I'm Just Ken |
| S Club | Bring It All Back | Apple track found by search (S Club – Bring It All Back) |
| S Club | Don't Stop Movin' | MusicBrainz has it only from 2004, after the card year |
| Sabrina | Boys - Summertime Love | MusicBrainz matched as Sabrina – Boys |
| Sade | Smooth Operator - Single Version | MusicBrainz matched as Sade – Smooth Operator |
| Sam Cooke | (What A) Wonderful World | MusicBrainz has it only from 1995, after the card year |
| Sam Cooke | (What A) Wonderful World - Mono | Apple track found by search (Sam Cooke – (What A) Wonderful World (Mono)) |
| Sam Feldt | Post Malone (feat. RANI) | Apple track found by search (Sam Feldt – Post Malone (feat. RANI)) |
| Sam Smith (feat. Kim Petras) | Unholy | Apple track found by search (Sam Smith & Kim Petras – Unholy) |
| Sananda Maitreya | Dance Little Sister | Apple track found by search (Sananda Maitreya – Dance Little Sister (Remastered)) |
| Sandra | Don't Be Aggressive - Radio Edit | MusicBrainz matched as Sandra – Don't Be Aggressive |
| Sandra | Maria Magdalena | Apple track found by search (Sandra – Maria Magdalena (Single Verison)) |
| Santana | Corazon Espinado (feat. Mana) | MusicBrainz matched as Santana – Corazon Espinado |
| Santana | Smooth (feat. Rob Thomas) | MusicBrainz matched as Santana – Smooth |
| Sarah Connor | Living To Love You - Single Version | MusicBrainz matched as Sarah Connor – Living To Love You |
| Sarah-Jane | Einmal hin, einmal her | Apple track found by search (Sarah-Jane – Einmal Hin, Einmal Her) |
| Schnappi, Joy | Schnappi, das kleine Krokodil | Apple track found by search (Schnappi & Joy – Schnappi, das kleine Krokodil) |
| Shaggy, Rik Rok | It Wasn't Me | MusicBrainz matched as Shaggy – It Wasn't Me |
| Shaggy | Mr Boombastic | MusicBrainz has it only from 2007, after the card year |
| Shakespears Sister | Hello - Turn Your Radio On | MusicBrainz matched as Shakespears Sister – Hello |
| Shakira | La Tortura (feat. Alejandro Sanz) | MusicBrainz matched as Shakira – La Tortura |
| Shawn Mendes, Camila Cabello | Señorita | MusicBrainz matched as Shawn Mendes – Señorita |
| Simone | Solang wir lieben | Apple track found by search (Simone – Solang wir lieben) |
| Small Faces | Itchycoo Park | Apple track found by search (Small Faces – Itchycoo Park (Stereo Version) [2013 Remaster]) |
| Smash Mouth | All Star | Apple track found by search (Smash Mouth – All Star) |
| Söhne Mannheims | Und wenn ein Lied - Radio Edit | MusicBrainz matched as Söhne Mannheims – Und wenn ein Lied |
| Spandau Ballet | True - 2003 Remaster | MusicBrainz matched as Spandau Ballet – True |
| Starship | Mannequin (Nothing's Gonna Stop Us Now) | MusicBrainz matched as Starship – Nothing's Gonna Stop Us Now |
| Stefan Waggershausen, Alice | Zu nah am Feuer | MusicBrainz matched as Stefan Waggershausen – Zu nah am Feuer |
| Stefan Waggershausen, Viktor Lazlo | Das erste Mal tat's noch weh (Les histoires d'amour) | MusicBrainz matched as Stefan Waggershausen – Das erste Mal tat's noch weh (Les histoires d'amour) |
| Stefanie Hertel | Erst du, dann du | Apple track found by search (Stefanie Hertel – Erst du, dann du) |
| Stephan Remmler | Alles hat ein Ende, nur die Wurst hat zwei (Krause und Ruth) | MusicBrainz matched as Stephan Remmler – Alles hat ein Ende, nur die Wurst hat zwei |
| Stereoact, Kerstin Ott | Die Immer Lacht | MusicBrainz matched as Stereoact – Die Immer Lacht |
| Steve Harley | Make Me Smile (Come up and See Me) | MusicBrainz artist matched loosely (Steve Harley & Cockney Rebel) |
| Stevie Wonder | Superstition - Single Version | MusicBrainz matched as Stevie Wonder – Superstition |
| Stu Phillips | Knight Rider | MusicBrainz has it only from 1989, after the card year |
| Taio Cruz, Flo Rida | Hangover | MusicBrainz matched as Taio Cruz – Hangover |
| The Animals | House of the Rising Sun | MusicBrainz has it only from 1970, after the card year; Apple track found by search (The Animals – House of the Rising Sun) |
| The Bangles | Eternal Flame | MusicBrainz has it only from 1997, after the card year; Apple track found by search (The Bangles – Eternal Flame) |
| The Bangles | Manic Monday | Apple track found by search (The Bangles – Manic Monday) |
| The Beach Boys | Barbara Ann | MusicBrainz has it only from 1965, after the card year |
| The Beach Boys | Cocktail (Kokomo) | MusicBrainz matched as The Beach Boys – Kokomo |
| The Beach Boys | I Get Around (Mono) | MusicBrainz matched as The Beach Boys – I Get Around |
| The Blues Brothers | Everybody Needs Somebody to Love | MusicBrainz has it only from 2020, after the card year |
| The Buggles | Video Killed The Radio Star - Single Version | MusicBrainz matched as The Buggles – Video Killed The Radio Star; Apple track found by search (The Buggles – Video Killed the Radio Star (Single Version)) |
| The Chordettes | Lollipop | Apple track found by search (The Chordettes – Lollipop) |
| The Communards (with Sarah Jane Morris) | Don't Leave Me This Way | MusicBrainz matched as The Communards – Don't Leave Me This Way |
| The Crystals | Da Doo Ron Ron | Apple track found by search (The Crystals – Da Doo Ron Ron (When He Walked Me Home)) |
| The Drums | Lets Go Surfing | Apple track found by search (The Drums – Let's Go Surfing) |
| The Edwin Hawkins Singers | Oh Happy Day | MusicBrainz has it only from 1973, after the card year; Apple track found by search (The Edwin Hawkins Singers – Oh Happy Day) |
| The Emotions | Best of My Love | Apple track found by search (The Emotions – Best of My Love) |
| The Everly Brothers | All I Have to Do Is Dream | Apple track found by search (The Everly Brothers – All I Have to Do Is Dream (Single Version)) |
| The Goo Goo Dolls | Iris | MusicBrainz has it only from 2006, after the card year |
| The Isley Brothers | Twist And Shout | Apple track found by search (The Isley Brothers – Twist and Shout) |
| The Jackson 5 | Santa Claus Is Coming To Town | MusicBrainz has it only from 1987, after the card year |
| The Kelly Family | I Can't Help Myself (I Love You I Want You) | Apple track found by search (The Kelly Family – I Can't Help Myself (I Love You I Want You)) |
| The New Seekers | Never Ending Song Of Love | Apple track found by search (The New Seekers – Never Ending Song of Love) |
| The New Seekers | You Won't Find Another Fool Like Me | Apple track found by search (The New Seekers – You Won't Find Another Fool Like Me) |
| The Offspring | Self Esteem | Apple track found by search (The Offspring – Self Esteem) |
| The Osmonds | Goin' Home | Apple track found by search (The Osmonds – Goin' Home) |
| The Pogues | Fairytale of New York (feat. Kirsty MacColl) | MusicBrainz matched as The Pogues – Fairytale of New York |
| The Rolling Stones | Brown Sugar | Apple track found by search (The Rolling Stones – Brown Sugar) |
| The Rolling Stones | Sympathy For The Devil - 50th Anniversary Edition | MusicBrainz matched as The Rolling Stones – Sympathy For The Devil |
| The Script | Hall of Fame (feat. will.i.am) | MusicBrainz matched as The Script – Hall of Fame |
| The Supremes | Stop! In The Name Of Love | Apple track found by search (The Supremes – Stop! In the Name of Love (Single Version / Mono)) |
| The Teens | Give Me More | Apple track found by search (The Teens – Give Me More) |
| The Temptations | Papa Was A Rollin' Stone - Single Version | MusicBrainz matched as The Temptations – Papa Was A Rollin' Stone |
| The Underdog Project | Summer Jam 2003 | Apple track found by search (The Underdog Project – Summer Jam 2003 (DJ F.R.A.N.K's Summermix Extended)) |
| Thirty Seconds To Mars | The Kill (Bury Me) | Apple track found by search (Thirty Seconds to Mars – The Kill (Bury Me)) |
| Thomas Anders, Florian Silbereisen | Alles wird gut | Apple track found by search (Thomas Anders & Florian Silbereisen – Alles wird gut) |
| Tina Turner | James Bond 007 - GoldenEye | MusicBrainz matched as Tina Turner – GoldenEye |
| Tina Turner | Mad Max - Jenseits der Donnerkuppel (We Don't Need Another Hero) | MusicBrainz matched as Tina Turner – We Don't Need Another Hero |
| Tina Turner | What's Love Got to Do with It - 2015 Remaster | MusicBrainz matched as Tina Turner – What's Love Got to Do with It |
| Tom Astor | Irgendwie Wird's Schon Geh'n | Apple track found by search (Tom Astor – Irgendwie wird's schon geh'n) |
| Tom Jones | Was gibt's Neues, Pussy? (What's New Pussycat?) | MusicBrainz matched as Tom Jones – What's New Pussycat? |
| Tom Scott | Starsky & Hutch (Gotcha) | MusicBrainz has it only from 1987, after the card year; Apple track found by search (Tom Scott – Gotcha (Theme from "Starsky & Hutch")) |
| Toni Braxton | Christmas In Jamaica (feat. Shaggy) | MusicBrainz matched as Toni Braxton – Christmas In Jamaica |
| Tony Marshall | Auf der Straße nach Süden | Apple track found by search (Tony Marshall – Auf der Straße nach Süden) |
| Torfrock | Beinhart | Apple track found by search (Torfrock – Beinhart) |
| Trenkwalder | Verliab di | Apple track found by search (Trenkwalder – Verliab di) |
| Trio Rio | New York-Rio-Tokyo | Apple track found by search (Trio Rio – New York-Rio-Tokyo) |
| Truck Stop | Emmylou, Emmylou | Apple track found by search (Truck Stop – Emmylou, Emmylou) |
| Truck Stop | Erwin der dicke Schneemann (Rudolph The Rednosed Reindeer) | MusicBrainz matched as Truck Stop – Erwin der dicke Schneemann |
| Trude Herr | Weil ich so sexy bin | Apple track found by search (Trude Herr – Weil ich so sexy bin) |
| T.S. Monk | Bon bon vie - Gimme the Good Life | MusicBrainz matched as T.S. Monk – Bon bon vie |
| Twenty One Pilots | Suicide Squad (Heathens) | MusicBrainz matched as Twenty One Pilots – Heathens |
| U2 | With Or Without You | MusicBrainz has it only from 1987, after the card year |
| Udo Lindenberg & Das Panik-Orchester | Sonderzug nach Pankow | MusicBrainz matched as Udo Lindenberg – Sonderzug nach Pankow |
| Udo Lindenberg | Ein Herz kann man nicht reparieren | MusicBrainz has it only from 2004, after the card year; Apple track found by search (Udo Lindenberg – Ein Herz kann man nicht reparieren) |
| Uriah Heep | Lady In Black | Apple track found by search (Uriah Heep – Lady in Black (2016 - Remaster)) |
| Usher (feat. Pitbull) | DJ Got Us Fallin' In Love | Apple track found by search (USHER – DJ Got Us Fallin' In Love (feat. Pitbull)) |
| USHER | Yeah! (feat. Lil Jon & Ludacris) | MusicBrainz matched as USHER – Yeah! |
| Valeries Garten | Nächstes Mal am Ende der Welt | Apple track found by search (Valerie's Garten – Nächstes Mal am Ende der Welt) |
| Vanessa Carlton | A Thousand Miles | Apple track found by search (Vanessa Carlton – A Thousand Miles (Interlude)) |
| Vangelis | Die Stunde des Siegers | MusicBrainz has it only from 2009, after the card year |
| Vaughn Monroe | Let It Snow! Let It Snow! Let It Snow! | Apple track found by search (Vaughn Monroe – Let It Snow, Let It Snow, Let It Snow) |
| Vikinger | Das Kann Nur Liebe Sein (En Vän Som Du) | MusicBrainz matched as Vikinger – Das Kann Nur Liebe Sein |
| Vincent Gross | Ouzo | Apple track found by search (Vincent Gross – Ouzo) |
| voXXclub | Was geht hier ab | Apple track found by search (voXXclub – Was geht hier ab) |
| Wencke Myhre | Beiß nicht gleich in jeden Apfel | Apple track found by search (Wencke Myhre – Beiß nicht gleich in jeden Apfel) |
| Wendy Carlos | Shining | MusicBrainz has it only from 1996, after the card year |
| Whitney Houston | Bodyguard (I Will Always Love You) | MusicBrainz matched as Whitney Houston – I Will Always Love You |
| Willy Millowitsch | Schnaps, das war sein letztes Wort | Apple track found by search (Willy Millowitsch – Schnaps, das war sein letztes Wort) |
| Wincent Weiss, Johannes Oerding | Die guten Zeiten | MusicBrainz matched as Wincent Weiss – Die guten Zeiten |
| Wind | Radio Zuckerhut | Apple track found by search (Wind – Radio Zuckerhut) |
| Wizzard | I Wish It Could Be Christmas Everyday | MusicBrainz has it only from 1985, after the card year |
| Wolfgang Ziegler | Ich küss dir die Angst von deiner Haut | Apple track found by search (Wolfgang Ziegler – Ich küss dir die Angst von deiner Haut) |
| Wolfram | Trödler Abraham | Apple track found by search (Wolfram – Trödler Abraham) |
| Youssou N'Dour | 7 Seconds (feat. Neneh Cherry) | MusicBrainz matched as Youssou N'Dour – 7 Seconds |
| Yusuf / Cat Stevens | Father And Son | MusicBrainz matched as Yusuf – Father And Son |

## Not found on MusicBrainz (101)

| artist | title |
|---|---|
| Alisha | Deine braunen Augen |
| Andrew Lloyd Webber, Gerard Butler, Emmy Rossum | Das Phantom der Opera |
| Axwell Ingrosso | More Than You Know |
| Axwell/Ingrosso | Sun Is Shining |
| B 3 | I.O.I.O. |
| Bata Illic | Bella Isola |
| Beagle Music | Like Ice in the Sunshine |
| Bear McCreary | The Walking Dead |
| Bob Sinclair | World Hold on (Children of the Sky) |
| Caterina Valente, Silvio Francesco | Itsy Bitsy Teenie Weenie Honolulu Strand Bikini |
| CeeLo Green | All I Need Is Love (feat. Disney's the Muppets) |
| Christian Möllmann | Es ist geil ein Arschloch zu sein |
| Christina Perri | The Twilight Saga: Breaking Dawn (A thousand years) |
| Christopher Cross | Arthur (Best That You Can Do) |
| Chuck Lorre, Lee Aronsohn | Two and a Half Men (Manly Men) |
| Cornelia Froboess | Pack Die Badehose Ein |
| Danny Elfman | Wednesday |
| David Rose,Daniel Caine Orchestra | Unsere kleine Farm |
| De Electronica's | Dance Little Birdie |
| Die Amigos | Du bist wie Feuer im Vulkan |
| Die Fidelen Mölltaler | Gold, Platin und Diamant |
| Die Tahiti Tamoures | Wini-Wini |
| Dieter Thomas Kuhn, Michael Holm | Barfuß im Regen |
| Dirk Busch | Du mußt deinen Weg alleine gehen |
| DJ Jazzy Jeff & Will Smith | Der Prinz von Bel-Air |
| Dominik Hause & Mike Post | Das A-Team |
| Dominik Hauser | Dallas |
| Dominik Hauser | L.A. Law - Law & Order |
| Doris Day, Frank DeVol | Der Mann, der zuviel wusste (Que Sera Sera) |
| EAV (Erste Allgemeine Verunsicherung) | 300 PS (... Auto) |
| EAV (Erste Allgemeine Verunsicherung) | An der Copacabana |
| EAV (Erste Allgemeine Verunsicherung) | Ba-Ba Banküberfall |
| EAV (Erste Allgemeine Verunsicherung) | Ding Dong |
| EAV (Erste Allgemeine Verunsicherung) | Küss' die Hand schöne Frau |
| Encanto Cast | Encanto (We Don't Talk About Bruno) |
| Ennio Morricone | Zwei glorreiche Halunken - The Good, The Bad And The Ugly |
| Éric Serra, Inva Mula | Das fünfte Element (The Diva Dance) |
| Freddy Quinn | Aloha-Ahe |
| Gabriel Mann | Modern Family |
| Gato Barbieri | Der letzte Tango in Paris |
| Georges Delerue | Platoon |
| Gitti Und Erica | Heidi |
| Hans Zimmer | The Crown |
| Hey Beautiful | How I Met Your Mother |
| Howard Shore | Der Herr der Ringe (The Shire) |
| Imany,Filatov & Karas | Don't Be So Shy |
| Jack Elliot | Drei Engel für Charlie |
| Jay Ferguson | The Office |
| J.Bon Jovi | I Wish Everyday Could Be Like Christmas |
| Jessie J | Price Tag ft. B.o.B |
| Jimi Jamison | Baywatch (I'm Always Here) |
| John Addison | Mord ist ihr Hobby |
| John Barry | Jenseits von Afrika (I Had A Farm) |
| John Beal | Akte X - The X Files |
| John Carpenter | Halloween - Die Nacht des Grauens |
| John Lennon | Happy Xmas (War Is Over) - Remastered 2010 |
| John Williams | Der weiße Hai |
| John Williams | Jäger des verlorenen Schatzes (Indiana Jones) |
| John Williams | Krieg der Sterne (Star Wars) |
| Kelly Rowland | Dilemma (feat. Kelly Rowland) Nelly |
| Klaus Lage | Faust auf Faust (Schimanski) |
| Klostertaler, Kastelruther Spatzen, Nockis | Hal-Le-Lu-Ja |
| Linda Hesse | Knutschen...ich kann nichts dafür |
| Linda Hesse | Weihnachten zu zweit |
| Maite Kelly | Warum hast Du nicht nein gesagt Roland Kaiser |
| Malcolm Arnold | Die Brücke am Kwai (The Colonel Bogey March) |
| Manuela Fellner | 7 Tage hat die Woche |
| Marc Bradley | Falcon Crest |
| Margot Eskens | ... Denn es ist Weihnachtszeit |
| Matthias Reim | Verdammt lch lieb' dich |
| MC Miker & DJ Sven | Holiday Rap |
| Monique | Sieben Tage, Sieben Nächte |
| Münchner Freiheit | Ohne Dich (schlaf' ich heut Nacht nicht ein) |
| Nerf Herder | Buffy, im Bann der Dämonen - Buffy The Vampire Slayer |
| Nickerbocker & Biene | HALLO KLAUS (I wü zruck zu Dir) ZRUCK ZU DIR (Hallo Klaus) - Orginal '83 (weibliche Version) |
| Nik P., DJ Ötzi | Ein Stern, der Deinen Namen trägt |
| Olly Alexander (Years & Years) | King |
| OMD | Maid of Orleans - Live |
| Patrick Lindner | Halleluja - Auf das Leben |
| Patrick Lindner | Weihnachten bin ich daheim |
| Pink, Nate Ruess | Just Give Me a Reason (feat. Nate Ruess) |
| Randy Newman | Die Monster AG |
| Ray Evans, Jerry Livingston | Bonanza |
| Richard Hall, Moby | Die Bourne Identität (Extreme Ways) |
| Richard Strauss | 2001 Odyssee im Weltraum (Also Spracht Zarathustra) |
| Robert D. | Matrix (Clubbed to Death) |
| Sam Cooke | (What A) Wonderful World - Mono |
| Sananda Maitreya | Dance Little Sister |
| Schnappi, Joy | Schnappi, das kleine Krokodil |
| Seiler und Speer | Ham kummst Single |
| Shakira | Waka Waka (This Time for Africa) ft. Freshlyground |
| Sherwood Schwartz, Frank Devol | Drei Mädchen und drei Jungen - The Brady Bunch |
| The Drums | Lets Go Surfing |
| The Snowths | Die Muppet Show (Mah Na Mah Na) |
| Thomas Anders, Florian Silbereisen | Alles wird gut |
| Trenkwalder | Verliab di |
| Valeries Garten | Nächstes Mal am Ende der Welt |
| Vicky Leandros | Theo, wirfahr'n nach Lodz |
| voXXclub | Was geht hier ab |
| Wencke Myhre | Ich Glou'an Den Weihnachtsmann |
| Wolfram | Trödler Abraham |

## No Apple track with a preview (94)

| artist | title |
|---|---|
| Alex Parker | Aber sonst geht's mir gut |
| Andrew Lloyd Webber, Gerard Butler, Emmy Rossum | Das Phantom der Opera |
| Asaf Avidan | One Day / Reckoning Song |
| Axwell Ingrosso | More Than You Know |
| Axwell/Ingrosso | Sun Is Shining |
| Bear McCreary | The Walking Dead |
| Bob Sinclair | World Hold on (Children of the Sky) |
| Bonnie Bianco | Stay |
| Brad Fiedel | Terminator 2 |
| Christian Möllmann | Es ist geil ein Arschloch zu sein |
| Christopher Cross | Arthur (Best That You Can Do) |
| Chuck Lorre, Lee Aronsohn | Two and a Half Men (Manly Men) |
| Common, John Legend | Selma (Glory) |
| Danny Elfman | Batman |
| Danny Elfman | Wednesday |
| David Rose,Daniel Caine Orchestra | Unsere kleine Farm |
| De Randfichten | Lebt denn der alte Holzmichl noch? |
| Die Amigos | Du bist wie Feuer im Vulkan |
| Dieter Thomas Kuhn, Michael Holm | Barfuß im Regen |
| DJ Jazzy Jeff & Will Smith | Der Prinz von Bel-Air |
| Dominik Hause & Mike Post | Das A-Team |
| Doris Day, Frank DeVol | Der Mann, der zuviel wusste (Que Sera Sera) |
| Douglas J. Cuomo | Sex And The City |
| EAV (Erste Allgemeine Verunsicherung) | 300 PS (... Auto) |
| EAV (Erste Allgemeine Verunsicherung) | An der Copacabana |
| EAV (Erste Allgemeine Verunsicherung) | Ba-Ba Banküberfall |
| EAV (Erste Allgemeine Verunsicherung) | Ding Dong |
| EAV (Erste Allgemeine Verunsicherung) | Küss' die Hand schöne Frau |
| Encanto Cast | Encanto (We Don't Talk About Bruno) |
| Éric Serra, Inva Mula | Das fünfte Element (The Diva Dance) |
| Gabriel Mann | Modern Family |
| Gato Barbieri | Der letzte Tango in Paris |
| Georges Delerue | Platoon |
| Hans Zimmer | The Crown |
| Henry Mancini | Der Rosarote Panther |
| Hey Beautiful | How I Met Your Mother |
| High School Musical Cast | High School Musical (We're All In This Together) |
| Imany,Filatov & Karas | Don't Be So Shy |
| Ines Adler | Nimm einmal noch meine Hand |
| Isaac Hayes | Shaft |
| Jack Elliot | Drei Engel für Charlie |
| James Horner | Willow |
| Jay Ferguson | The Office |
| J.Bon Jovi | I Wish Everyday Could Be Like Christmas |
| Jerry Goldsmith | Star Trek |
| Jessie J | Price Tag ft. B.o.B |
| Jimi Hendrix | Hey Joe |
| John Addison | Mord ist ihr Hobby |
| John Beal | Akte X - The X Files |
| John E. Davis | Beverly Hills 90210 |
| John Lennon | Happy Xmas (War Is Over) - Remastered 2010 |
| John Williams | Der weiße Hai |
| John Williams | Jurassic Park |
| John Williams | Krieg der Sterne (Star Wars) |
| Jonathan Wolff | Seinfeld |
| Ke$ha | TiK ToK |
| Kelly Rowland | Dilemma (feat. Kelly Rowland) Nelly |
| Maite Kelly | Warum hast Du nicht nein gesagt Roland Kaiser |
| Malcolm Arnold | Die Brücke am Kwai (The Colonel Bogey March) |
| Marc Bradley | Falcon Crest |
| Matthias Reim | Verdammt lch lieb' dich |
| Max Raabe | Kein Schwein ruft mich an |
| MC Miker & DJ Sven | Holiday Rap |
| Münchner Freiheit | Ohne Dich (schlaf' ich heut Nacht nicht ein) |
| Nerf Herder | Buffy, im Bann der Dämonen - Buffy The Vampire Slayer |
| Nick Straker | Walk In The Park |
| Nickerbocker & Biene | HALLO KLAUS (I wü zruck zu Dir) ZRUCK ZU DIR (Hallo Klaus) - Orginal '83 (weibliche Version) |
| NIC | Ich will dich für immer - Disco Fox |
| Nik P., DJ Ötzi | Ein Stern, der Deinen Namen trägt |
| Ole ohne Kohle | Ich bin kein Model und kein Superstar |
| Oli. P | Flugzeuge im Bauch - Radio Mix |
| Olly Alexander (Years & Years) | King |
| OMD | Maid of Orleans - Live |
| Ph.D. | I Won't Let You Down |
| Pink, Nate Ruess | Just Give Me a Reason (feat. Nate Ruess) |
| Randy Newman | Die Monster AG |
| Ray Evans, Jerry Livingston | Bonanza |
| Richard Hall, Moby | Die Bourne Identität (Extreme Ways) |
| Richard Strauss | 2001 Odyssee im Weltraum (Also Spracht Zarathustra) |
| Robert Folk | Police Academy |
| Rolfe Kent | Dexter |
| Seiler und Speer | Ham kummst Single |
| Shaggy | Mr Boombastic |
| Shakira | Waka Waka (This Time for Africa) ft. Freshlyground |
| Sherwood Schwartz, Frank Devol | Drei Mädchen und drei Jungen - The Brady Bunch |
| Soulja Boy | Kiss Me Thru The Phone |
| Stu Phillips | Knight Rider |
| The Snowths | Die Muppet Show (Mah Na Mah Na) |
| Triggerfinger | I Follow Rivers |
| Vanessa Mai | Mein Herz schlägt Schlager |
| Vangelis | Die Stunde des Siegers |
| Vicky Leandros | Theo, wirfahr'n nach Lodz |
| Wencke Myhre | Ich Glou'an Den Weihnachtsmann |
| Wendy Carlos | Shining |

## Not on Last.fm (39)

| artist | title | genres (artist tags, sets, overrides) |
|---|---|---|
| Andrew Lloyd Webber, Gerard Butler, Emmy Rossum | Das Phantom der Opera |  |
| Christina Perri | The Twilight Saga: Breaking Dawn (A thousand years) | indie, pop |
| Chuck Lorre, Lee Aronsohn | Two and a Half Men (Manly Men) |  |
| David Rose,Daniel Caine Orchestra | Unsere kleine Farm |  |
| Dieter Thomas Kuhn, Michael Holm | Barfuß im Regen | schlager |
| DJ Jazzy Jeff & Will Smith | Der Prinz von Bel-Air | hip-hop |
| Dominik Hause & Mike Post | Das A-Team |  |
| Dominik Hauser | Dallas |  |
| Dominik Hauser | L.A. Law - Law & Order |  |
| Doris Day, Frank DeVol | Der Mann, der zuviel wusste (Que Sera Sera) |  |
| Encanto Cast | Encanto (We Don't Talk About Bruno) |  |
| Éric Serra, Inva Mula | Das fünfte Element (The Diva Dance) | dance |
| Gabriel Mann | Modern Family |  |
| Gato Barbieri | Der letzte Tango in Paris |  |
| Hey Beautiful | How I Met Your Mother |  |
| High School Musical Cast | High School Musical (We're All In This Together) | pop |
| Howard Shore | Der Herr der Ringe (The Shire) |  |
| Jack Elliot | Drei Engel für Charlie | country |
| John Barry | Jenseits von Afrika (I Had A Farm) |  |
| John Beal | Akte X - The X Files |  |
| John Williams | Jäger des verlorenen Schatzes (Indiana Jones) |  |
| John Williams | Krieg der Sterne (Star Wars) |  |
| Kelly Rowland | Dilemma (feat. Kelly Rowland) Nelly | pop, rnb-soul |
| Maite Kelly | Warum hast Du nicht nein gesagt Roland Kaiser | pop, rock, schlager |
| Malcolm Arnold | Die Brücke am Kwai (The Colonel Bogey March) |  |
| Manuela Fellner | 7 Tage hat die Woche | schlager |
| Marc Bradley | Falcon Crest |  |
| Matthias Reim | Verdammt lch lieb' dich | schlager |
| Nerf Herder | Buffy, im Bann der Dämonen - Buffy The Vampire Slayer | indie, rock |
| Nik P., DJ Ötzi | Ein Stern, der Deinen Namen trägt | schlager |
| Randy Newman | Die Monster AG | pop |
| Ray Evans, Jerry Livingston | Bonanza |  |
| Richard Hall, Moby | Die Bourne Identität (Extreme Ways) |  |
| Richard Strauss | 2001 Odyssee im Weltraum (Also Spracht Zarathustra) |  |
| Robert D. | Matrix (Clubbed to Death) |  |
| Sherwood Schwartz, Frank Devol | Drei Mädchen und drei Jungen - The Brady Bunch |  |
| The Snowths | Die Muppet Show (Mah Na Mah Na) |  |
| Vicky Leandros | Theo, wirfahr'n nach Lodz | pop, schlager |
| Wencke Myhre | Ich Glou'an Den Weihnachtsmann | schlager |
