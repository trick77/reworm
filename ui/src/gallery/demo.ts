// Demo data for the gallery: the mock's party state (D11 rules) and real Apple previews for a hand test of
// the audio engines.
import type { Player } from "../components/Players";

export type DemoSong = {
  title: string;
  artist: string;
  album: string;
  year: number;
  preview: string;
  art: string;
};

const AUDIO = "https://audio-ssl.itunes.apple.com/itunes-assets/";
const ART = "https://is1-ssl.mzstatic.com/image/thumb/";

export const SONGS: DemoSong[] = [
  {
    title: "Like a Prayer",
    artist: "Madonna",
    album: "Like a Prayer",
    year: 1989,
    preview:
      AUDIO +
      "AudioPreview211/v4/0b/23/82/0b238289-52e2-2107-5531-181836267ed2/mzaf_17722571351632165824.plus.aac.ep.m4a",
    art:
      ART +
      "Music114/v4/20/3c/f5/203cf53d-689e-528f-29d7-ba33758254aa/mzi.rotbotfl.jpg/300x300bb.jpg",
  },
  {
    title: "Dancing Queen",
    artist: "ABBA",
    album: "ABBA Gold",
    year: 1976,
    preview:
      AUDIO +
      "AudioPreview211/v4/9a/c1/a4/9ac1a457-6b2d-91f2-a74a-5e2ee7241d69/mzaf_13444108823547583965.plus.aac.ep.m4a",
    art:
      ART +
      "Music112/v4/dc/f7/90/dcf790c3-7c63-f7b3-e57b-5a8747a8ebbe/14UMGIM11148.rgb.jpg/300x300bb.jpg",
  },
  {
    title: "Superstition",
    artist: "Stevie Wonder",
    album: "Talking Book",
    year: 1972,
    preview:
      AUDIO +
      "AudioPreview211/v4/59/4b/88/594b880c-4744-0510-775c-73d8b06567e8/mzaf_14770892373808610282.plus.aac.ep.m4a",
    art:
      ART +
      "Music114/v4/62/61/61/626161c0-f4d7-e6ff-8586-768340ef278f/00602537002382.rgb.jpg/300x300bb.jpg",
  },
  {
    title: "Bohemian Rhapsody",
    artist: "Queen",
    album: "A Night at the Opera (2011 Remaster)",
    year: 1975,
    preview:
      AUDIO +
      "AudioPreview221/v4/44/10/de/4410de9b-4b10-c763-baf8-c4911c032248/mzaf_2164995177730538281.plus.aac.ep.m4a",
    art:
      ART +
      "Music221/v4/62/2e/97/622e97c4-d620-2ae1-2a47-a32e16a01492/602567988847.jpg/300x300bb.jpg",
  },
];

// The party round from the mock: round 7, first to 10, shown 1983, Like a Prayer (1989); Sarah takes it.
export const PARTY = {
  room: "K7Q4",
  round: 7,
  toWin: 10,
  shown: 1983,
  song: SONGS[0],
  winner: "Sarah",
  winTime: "3.4",
  gain: 1,
  me: "Mia",
};

export const PLAYERS: Player[] = [
  { name: "Jan", avatar: "vinyl", score: 6 },
  { name: "Mia", avatar: "mic", score: 8, out: true },
  { name: "Luca", avatar: "guitar", score: 5 },
  { name: "Sarah", avatar: "disco", score: 7 },
  { name: "Tom", avatar: "drum", score: 3, out: true },
  { name: "Lea", avatar: "note", score: 4 },
];
