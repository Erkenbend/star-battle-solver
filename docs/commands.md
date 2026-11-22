# Befehle

Alles läuft über ein einziges Programm, `starbattle`. Wie man es bekommt, steht in der [README](../README.md#herunterladen); alle Beispiele lassen sich statt mit `starbattle` auch mit `go run ./cmd/starbattle` aufrufen.

| Befehl | wofür |
|---|---|
| [`generate`](#generate-puzzle-erzeugen) | Puzzle mit eindeutiger Lösung erzeugen |
| [`show`](#show-puzzle-anzeigen) | Puzzle (und Lösung) im Terminal zeigen |
| [`solve`](#solve-puzzle-lösen-und-erklären) | Puzzle lösen und jeden Schritt erklären |
| [`html`](#html-interaktive-seite-als-datei) | interaktive Seite als einzelne Datei |
| [`serve`](#serve-im-browser-spielen) | im Browser spielen, mit Bibliothek und Party-Modus |
| [`design`](development.md#design-puzzles-entwerfen), [`hardsearch`](development.md#hardsearch-schwere-puzzles-suchen) | Werkzeuge für die Entwicklung, siehe [development.md](development.md) |

Alle Befehle, die ein Puzzle lesen, nehmen eine Datei als Argument oder lesen ohne Datei von der Standardeingabe.

## Puzzle-Format

Eine Zeile pro Brettzeile, ein Buchstabe pro Feld; gleiche Buchstaben bilden eine Farbe (Region). Leerzeichen werden ignoriert, Zeilen mit `#` sind Kommentare. Möglich sind Bretter bis 52×52 (`A`–`Z`, `a`–`z`).

```
# seed 3
FFFFAA
FFFFFA
CFFAAA
DFFEAA
DFFEBB
DDDEBB
```

Koordinaten in Erklärungen sind (Zeile, Spalte), beginnend bei 0.

## `generate`: Puzzle erzeugen

```sh
starbattle generate -n 8 -seed 42 > p.txt
```

| Option | Bedeutung |
|---|---|
| `-n N` | Größe (Standard 8) |
| `-seed S` | Zufalls-Seed; gleicher Seed und gleiche Größe ergeben dasselbe Puzzle. 0 (Standard) = nach Uhrzeit |
| `-min-region K` | kleinste erlaubte Region (Standard: 2 ab N = 10, sonst 1) |
| `-show` | Puzzle und Lösung zusätzlich farbig auf stderr ausgeben |
| `-nocolor` | mit `-show`: Buchstaben statt Farben |

Jedes erzeugte Puzzle hat genau eine Lösung. Das dauert auch bei N = 15 nur Sekundenbruchteile.

## `show`: Puzzle anzeigen

```sh
starbattle show -solution p.txt
```

| Option | Bedeutung |
|---|---|
| `-solution` | zusätzlich die Lösung zeigen (per Brute Force; warnt, wenn sie nicht eindeutig ist) |
| `-nocolor` | Buchstaben statt Farben |

## `solve`: Puzzle lösen und erklären

```sh
starbattle solve -verbose p.txt                  # jeden Schritt in Worten
starbattle solve -step -runner sync p.txt        # Brett nach jedem Schritt
starbattle generate -n 10 | starbattle solve -verbose
```

| Option | Bedeutung |
|---|---|
| `-verbose` | jeden Schritt erklären |
| `-step` | Brett nach jedem Schritt zeigen (neue Felder in `[ ]`) |
| `-nocolor` | Buchstaben statt Farben |

Dazu kommen die [Solver-Optionen](#solver-optionen).

Am Ende steht das Ergebnis (`solved`, `stuck` oder ein Widerspruch) und, falls gesucht wurde, wie viele Lookahead-Schritte und geratene Sterne nötig waren.

## `html`: interaktive Seite als Datei

```sh
starbattle html -o p.html p.txt
```

Eine einzelne HTML-Datei ohne externe Abhängigkeiten: selbst lösen (mit Rückgängig, Prüfen, Hinweisen, Uhr) oder den Lösungsweg Schritt für Schritt abspielen. Eine Einstellungsseite gibt es darin nicht, die Schalter stehen unter „Mehr“; Symbole mit Bilddatei erscheinen als Stern (siehe [Spielen im Browser](playing.md#einstellungen)).

| Option | Bedeutung |
|---|---|
| `-o Datei` | Ausgabedatei (Standard: stdout) |
| `-title T` | Seitentitel (Standard: aus dem Dateinamen) |

Dazu die [Solver-Optionen](#solver-optionen).

## `serve`: im Browser spielen

```sh
starbattle serve                        # http://localhost:6369/
starbattle serve -addr :6369            # im ganzen Netz erreichbar
```

Dieselbe Seite wie bei `html`, dazu „Neues Puzzle“ unter „Mehr“ und nach dem Lösen (Größe 4 bis 15, auch zufällig aus mehreren), die Bibliothek (Motive und schwere Puzzles) unter `/bibliothek`, der Party-Modus unter `/party`, die Einstellungen unter `/einstellungen` und eine Anleitung unter `/anleitung`. Was es dort alles gibt, beschreibt [Spielen im Browser](playing.md).

Jedes Puzzle hat eine feste URL, z. B. `/?n=8&seed=42`, also dasselbe Puzzle wie `generate -n 8 -seed 42`.

Der Server löst höchstens so viele Puzzles gleichzeitig, wie er Kerne hat, einen weniger (auf einem Raspberry Pi 4 also drei). Was niemand gerade erwartet (vorbereitete Puzzles, die nächsten Puzzles einer Party, schwere Puzzles der Bibliothek) belegt davon nie alle, sodass eine Seite, die auf ihre Lösung wartet, immer sofort drankommt. Verlässt jemand eine Seite, bevor sie fertig ist, hört der Solver auf.

| Option | Bedeutung |
|---|---|
| `-addr` | Adresse (Standard `localhost:6369`) |
| `-no-pregen` | kein vorbereitetes Puzzle je Größe im Hintergrund bereithalten (betrifft die Bibliothek nicht, dort wird ohnehin nichts vorgerechnet) |

Dazu die [Solver-Optionen](#solver-optionen). Für den Betrieb auf einem Raspberry Pi siehe [deploy/README.md](../deploy/README.md).

## Solver-Optionen

Diese Optionen verstehen `solve`, `html`, `serve` und `design`:

| Option | Bedeutung |
|---|---|
| `-runner concurrent\|sync` | `concurrent`: eine Goroutine pro Observer (Standard bei `solve`); `sync`: alle reihum, mit reproduzierbarer Erklärung (Standard bei `html` und `serve`) |
| `-search full\|lookahead\|none` | was passiert, wenn die Regeln stecken bleiben: Lookahead und Raten (Standard), nur Lookahead, oder aufhören |
| `-partner-depth N` | Regel `ForcedPartner`: Länge der Ketten erzwungener Sterne (Standard 1, 0 = aus) |
| `-dead-max N` | Regel `DeadCells`: höchstens so viele verbleibende Felder einer Einheit (Standard 2, 0 = aus) |
| `-pigeonhole-max N` | Regel `Pigeonhole`: höchstens so viele Einheiten gleichzeitig (Standard 2, 0 = aus) |
| `-non-contiguous` | Regel `Confinement` mit k ≥ 2: beliebige Zeilen/Spalten statt nur benachbarter |
