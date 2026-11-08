# Star Battle Solver

Erzeugt und löst Star-Battle-Puzzles (auch „Meowdoku“ oder „Queens“) und erklärt jeden Lösungsschritt auf Deutsch.

**Regeln des Puzzles:** NxN-Brett mit N Farben. In jeder Zeile, jeder Spalte und jeder Farbe steht genau **ein Stern**. Sterne berühren sich nicht, auch nicht diagonal.

Der Solver arbeitet mit logischen Regeln, die als nebenläufige Observer laufen. Bleiben die Regeln stecken, sucht er mit Lookahead weiter und rät nur als letzten Ausweg. Hintergrund, Regeln und Messungen stehen in [PLAN.md](PLAN.md).

## Bauen und testen

Voraussetzung ist Go (Version siehe `go.mod`). Es gibt keine weiteren Abhängigkeiten.

```sh
go build -o starbattle ./cmd/starbattle
go test ./...            # dauert etwa eine Minute (schwere Testpuzzles)
go test -short ./...     # überspringt einige langsame Tests
go test -race ./...
```

Alle Beispiele unten lassen sich statt mit `starbattle` auch mit `go run ./cmd/starbattle` aufrufen.

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

## Befehle

Alle Befehle, die ein Puzzle lesen, nehmen eine Datei als Argument oder lesen ohne Datei von der Standardeingabe.

### `generate`: Puzzle erzeugen

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

Jedes erzeugte Puzzle hat genau eine Lösung. Bis N = 12 geht das in Sekundenbruchteilen, N = 14 dauert einige Sekunden.

### `show`: Puzzle anzeigen

```sh
starbattle show -solution p.txt
```

| Option | Bedeutung |
|---|---|
| `-solution` | zusätzlich die Lösung zeigen (per Brute Force; warnt, wenn sie nicht eindeutig ist) |
| `-nocolor` | Buchstaben statt Farben |

### `solve`: Puzzle lösen und erklären

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

Dazu kommen die **Solver-Optionen**, die auch `html` und `serve` verstehen:

| Option | Bedeutung |
|---|---|
| `-runner concurrent\|sync` | `concurrent`: eine Goroutine pro Observer (Standard bei `solve`); `sync`: alle reihum, mit reproduzierbarer Erklärung (Standard bei `html` und `serve`) |
| `-search full\|lookahead\|none` | was passiert, wenn die Regeln stecken bleiben: Lookahead und Raten (Standard), nur Lookahead, oder aufhören |
| `-partner-depth N` | Regel `ForcedPartner`: Länge der Ketten erzwungener Sterne (Standard 1, 0 = aus) |
| `-dead-max N` | Regel `DeadCells`: höchstens so viele verbleibende Felder einer Einheit (Standard 2, 0 = aus) |
| `-pigeonhole-max N` | Regel `Pigeonhole`: höchstens so viele Einheiten gleichzeitig (Standard 2, 0 = aus) |
| `-non-contiguous` | Regel `Confinement` mit k ≥ 2: beliebige Zeilen/Spalten statt nur benachbarter |

Am Ende steht das Ergebnis (`solved`, `stuck` oder ein Widerspruch) und, falls gesucht wurde, wie viele Lookahead-Schritte und geratene Sterne nötig waren.

### `html`: interaktive Seite als Datei

```sh
starbattle html -o p.html p.txt
```

Eine einzelne HTML-Datei ohne externe Abhängigkeiten: selbst lösen (mit Rückgängig, Prüfen, Hinweisen, Uhr) oder den Lösungsweg Schritt für Schritt abspielen.

| Option | Bedeutung |
|---|---|
| `-o Datei` | Ausgabedatei (Standard: stdout) |
| `-title T` | Seitentitel (Standard: aus dem Dateinamen) |

Dazu die Solver-Optionen wie bei `solve`.

### `serve`: im Browser spielen

```sh
starbattle serve                        # http://localhost:6369/
starbattle serve -addr :6369            # im ganzen Netz erreichbar
```

Dieselbe Seite wie bei `html`, dazu „Neues Puzzle“ (Größe 4 bis 14), eigene Puzzles einfügen und der Party-Modus unter `/party`: mehrere spielen dieselben Runden, wahlweise gegeneinander (jede Person auf ihrem eigenen Brett, Wertung nach Hinweisen und Zeit) oder miteinander (alle markieren auf demselben Brett, gemeinsame Zeit, ohne Hinweise; gleichzeitige Änderungen derselben Zelle entscheidet der Server: wer zuerst ankommt, gewinnt). Jedes Puzzle hat eine feste URL, z. B. `/?n=8&seed=42`, also dasselbe Puzzle wie `generate -n 8 -seed 42`.

| Option | Bedeutung |
|---|---|
| `-addr` | Adresse (Standard `localhost:6369`) |
| `-no-pregen` | kein vorbereitetes Puzzle je Größe im Hintergrund bereithalten |

Dazu die Solver-Optionen wie bei `solve`. Für den Betrieb auf einem Raspberry Pi siehe [deploy/README.md](deploy/README.md).

## Aufbau

| Verzeichnis | Inhalt |
|---|---|
| `cmd/starbattle` | Kommandozeile |
| `internal/puzzle` | Layout, Textformat, Einheiten (Zeilen, Spalten, Farben) |
| `internal/board` | Zustand des Bretts (frei, Stern, X), Widerspruchserkennung |
| `internal/solver` | Runner (`RunSync`, `RunConcurrent`), Suche mit Lookahead und Raten, Trace |
| `internal/solver/observers` | die logischen Regeln mit Erklärungen |
| `internal/bruteforce` | Referenz-Solver für Generator und Tests |
| `internal/generator` | Puzzles mit eindeutiger Lösung erzeugen |
| `internal/render` | Ausgabe im Terminal |
| `internal/web` | HTML-Seite, Server, Party-Modus |
| `deploy` | Installation als systemd-Dienst auf dem Raspberry Pi |
| `testdata/hard` | Puzzles, an denen die einfachen Regeln scheitern |
