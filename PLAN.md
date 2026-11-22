# Star Battle (Meowdoku) Solver – Plan und Stand

Variante: NxN-Brett mit N Farben, **1 Stern** pro Zeile, Spalte und Farbe; Sterne berühren sich nicht, auch nicht diagonal.
Ausgangsidee: `grober_plan.md` (nebenläufige Observer, die über Channels Markierungen M/X austauschen).
Fokus: der Solver-Algorithmus und **verständliche Erklärungen** der Lösungsschritte, nicht Geschwindigkeit.

## Kernideen (validiert)

- **Monotonie:** Zellen gehen nur von *frei* zu M oder X, nie zurück. Darum ist die Reihenfolge von Updates egal und Duplikate schaden nicht. Jeder Observer hat eine eigene Kopie des Bretts, ohne Locks.
- **Observer = reine Funktion** `Deduce(state) []Mark` plus `Done(state)`. Die Nebenläufigkeit steckt allein im Runner. Regeln sind so ohne Goroutinen testbar.
- **Zwei Runner mit derselben `Apply`-Logik** (ein M setzt automatisch X in Zeile, Spalte, Farbe und Nachbarschaft):
  - `RunSync`: einfache Referenz, ruft alle Observer reihum auf.
  - `RunConcurrent`: eine Goroutine pro Observer; unbegrenzte Mailboxen verhindern Deadlocks; das Ende wird über Versionsnummern erkannt („alle idle auf der aktuellen Version“).
  - Beide erreichen denselben Fixpunkt, solange die Regeln monoton sind (`TestRunnersAgree`).
- **Widerspruchserkennung** im Runner (`board.ContradictionError` mit den betroffenen Zellen), Grundlage für die Suche.

## Stand (umgesetzt)

| Paket | Inhalt |
|---|---|
| `internal/puzzle` | Layout, Textformat (Buchstaben bleiben erhalten, wenn es die ersten N sind), Einheiten |
| `internal/words` | deutsche Namen von Einheiten und Feldern („Zeilen 3 und 4“, „Farbe B“, „(2,3)“) für alle Erklärungen; `Namer` merkt sich, was ein Text nennt (`Refs`), damit die Seite es hervorhebt, ohne den Text zu lesen |
| `internal/board` | Zustand, `Set` mit Konflikterkennung, `Implied`, `Contradiction`, Parser für Testzustände; was aus dem Layout folgt (Einheiten, `Implied` je Feld), rechnet `New` einmal aus, Kopien teilen es ([Entscheidung 32](architecture/decisions/32-board-geometry.md)) |
| `internal/render` | ANSI-Farben; neue Felder in `[ ]`, Widerspruchszellen in `( )`, Pfeile am Rand |
| `internal/bruteforce` | Referenz-Solver (Backtracking über Zeilen), Test-Orakel |
| `internal/generator` | zufällige Lösung → Regionen wachsen → zweite Lösungen durch Verschieben von Regionsstücken zerstören (siehe [Entscheidung 8](architecture/decisions/08-generator-repair-by-pieces.md)) |
| `internal/solver` | `Mark`, `Observer`, `Apply`, `RunSync`, `RunConcurrent`, `Solve` (Lookahead + Backtracking), Trace mit Begründungen |
| `internal/solver/observers` | Regeln, deutsche Erklärungen, Soundness- und Vergleichstests |
| `internal/web` | Interaktive HTML-Seite (eine Datei, ohne externe Abhängigkeiten): Puzzle, Lösung und Trace als JSON, dazu `assets/` (HTML, CSS, JS); `Handler` für `serve`, mit Vorrat (`Pool`), dazu HTTP und Server-Sent Events des Party-Modus |
| `internal/party` | Party-Modus ohne HTTP: Partys, Runden, die drei Spielarten, Wertung (`party.Manager`) |
| `cmd/starbattle` | Befehle mit allen Optionen in `docs/commands.md`: `generate [-min-region]`, `show`, `html [-o] [-title]`, `serve [-addr]`, `solve [-runner] [-search] [-step] [-verbose] [-nocolor] [-partner-depth] [-dead-max] [-pigeonhole-max] [-non-contiguous]`, `design [-addr] [-dir]`, `hard [-n] [-from] [-count] [-min-lookahead] [-o] [-workers]` |
| `deploy` | Hosting auf dem Raspberry Pi: `deploy.sh [user@]host` (baut passend zu `uname -m`, installiert als systemd-Dienst), `starbattle.service`, Anleitung in `deploy/README.md` |
| `internal/hardsearch` | Suche nach schweren Puzzles (`starbattle hardsearch`): Seed für Seed erzeugen wie `generate`, dann Basisregeln → alle Regeln → Lookahead zählen; Treffer als `nNN_sSEED.txt` mit Befehl und Commit im Kopf |
| `testdata/hard` | 193 Puzzles mit N = 5 bis 15, an denen die Basisregeln scheitern (162 davon brauchen auch mit allen Regeln Lookahead), Nachschub mit `starbattle hardsearch` (nächste Seeds, siehe [Entscheidung 21](architecture/decisions/21-library-175.md) und [23](architecture/decisions/23-size-15.md): N = 10 bis 12 ab 4401/4401/4001, N = 13 ab 5001, N = 14 ab 3301, N = 15 ab 1083). Die Tests (auch die CI) nehmen nur die Stichprobe aus `sample.idx`, mit `STARBATTLE_ALL_HARD=1` alle. Eingebettet über `embed.go` (Paket `starbattle`), auch die **Bibliothek** im Web (`/bibliothek`, `internal/web/library.go`): Name „10-3“ = drittes 10×10 in `bibliothek.idx` (nur anhängen!), Links und Browser-Speicher über `collection.ID` (Hash der normalisierten Regionen; Format der Sammlungen in `internal/collection`), weil die Seeds mit dem heutigen Generator nicht mehr reproduzierbar sind. Lösen dort ohne Hinweise/Prüfen, Lösungsweg erst nach dem Lösen. Kein Vorrechnen: Der Solver startet beim ersten Aufruf eines Puzzles im Hintergrund, die Seite holt Lösung und Trace erst nach dem Lösen von Hand (`/bibliothek/weg?id=…`, wartet auf den Solver, solange Spinner) |

### Regeln
| Regel | Inhalt |
|---|---|
| `UnitFinish` | Zeile, Spalte oder Farbe mit genau einem freien Feld → Stern |
| `Confinement` (k, From→To) | Freie Felder von k offenen Einheiten liegen in genau k Einheiten eines anderen Typs → Rest dieser Einheiten X. Deckt „Line/Column“, „Color“ und die „Multiple“-Varianten aus dem groben Plan ab, in allen 6 Richtungen. Für k ≥ 2 über Zeilen/Spalten standardmäßig nur **Blöcke aufeinanderfolgender offener Linien** (dann k ≤ N−1), über Farben beliebige Teilmengen mit k ≤ N/2; `-non-contiguous` bzw. `Config.ConfinementAnySubset` erlaubt wieder beliebige Teilmengen |
| `Elimination` | Ein Stern auf c würde alle freien Felder einer Einheit streichen → c X. Ersetzt die Form-Regeln (2x1, L, Z …). Die Erklärung nennt den konkreten Grund (liegt in einer Einheit / berührt alle / gemischt) |
| `ForcedPartner` (Tiefe, Standard 1) | Ein Stern auf c ließe in einer Einheit nur ein Feld q übrig; c und q zusammen würden eine Einheit leeren → c X. Mit Tiefe > 1 auch Ketten aus mehreren erzwungenen Sternen |
| `DeadCells` (max. Felder, Standard 2) | Nach einem Stern auf c bleiben in einer Einheit nur 2 bis max. Felder, und jedes davon würde eine andere Einheit leeren → c X |
| `Pigeonhole` (max. k, Standard 2) | `Confinement` mit einer Annahme: Nach einem Stern auf c haben k offene Einheiten nur noch Platz in weniger als k Einheiten eines anderen Typs → c X. Zeilen/Spalten als Blöcke, Farben als beliebige Teilmengen, k ab 2 |

`ForcedPartner` und `DeadCells` sind die zwei häufigsten Muster aus den Annahmeketten der schweren Puzzles (zusammen gut drei Viertel aller Ketten). Konfiguration über `observers.Config` bzw. `solve -partner-depth N -dead-max N -pigeonhole-max N` (0 schaltet ab); `BaseConfig` = nur die ersten drei Regeln.

**Prioritätsstufen:** Jede Regel meldet über `Priority()` ihre Stufe (`solver.Prioritized`): 1 `UnitFinish` · 2 `Confinement` k=1 · 3 `Elimination` · 4 `Confinement` k≥2 · 5 `ForcedPartner` · 6 `DeadCells` · 7 `Pigeonhole`; Lookahead (100) und Raten (101) liegen darüber. Beide Runner wenden eine Regel nur an, wenn keine einfachere etwas findet:
- `RunSync` sortiert stabil nach Stufe und fängt nach jeder Änderung wieder vorne an.
- `RunConcurrent` sammelt Markierungen, die auf der aktuellen Version gefunden wurden, und wendet die der niedrigsten Stufe erst an, wenn alle Observer niedrigerer Stufen auf dieser Version idle sind (eine Nachricht pro Version; veraltete Nachrichten werden verworfen, die Absender rechnen nach dem Broadcast neu). Die Goroutinen rechnen weiter parallel.
- Innerhalb einer Stufe entscheidet die Reihenfolge aus `AllWith` (Confinement mit kleinerem k zuerst, dann Zeilen, Spalten, Farben).
- `Mark.Level`/`Step.Level` halten die Stufe im Trace fest. `TestPriorityOrder` prüft für jeden Schritt, dass vorher keine einfachere Regel gegriffen hätte.

Ergebnis:
- Basisregeln: ca. 96,5 % der generierten Puzzles (Stichprobe von 900 mit N = 4 bis 12).
- Mit `ForcedPartner` und `DeadCells`: 31 der 32 alten Puzzles aus `testdata/hard` ohne Annahmen; nur `n11_s095` braucht noch Lookahead. Dazu kommen `n14_s002`, `n14_s024` und `n14_s029` (12, 1 und 1 Lookahead-Schritte) und die zehn neuen Testdaten (2 bis 25 Lookahead-Schritte, siehe [Entscheidung 15](architecture/decisions/15-hard-testdata-for-all-rules.md)).
- Mit `Pigeonhole` zusätzlich: `n11_s095` braucht 11 statt 14 Lookahead-Schritte.

### Suche (`solver.Solve`)
1. Regeln bis zum Fixpunkt.
2. **Lookahead:** Für jedes freie Feld parallel „Stern hier“ annehmen und `RunSync` laufen lassen. Von den Widersprüchen wird der mit der **kürzesten Kette** übernommen (dann weniger gesetzte Felder, dann enge Einheiten zuerst). Das Feld wird X, danach geht es zurück zu 1. Die ganze Kette steht im Trace und wird mit `-verbose`/`-step` verschachtelt angezeigt.
3. **Raten:** Stern im engsten offenen Feld setzen und rekursiv lösen; scheitert das, ist das Feld bewiesenermaßen X, zurück zu 1.

Alle 45 schweren Puzzles werden allein mit Lookahead gelöst, Raten war nie nötig. Der Preis der kürzesten Ketten: 119 Annahmen mit Ø 2,6 Schritten statt 46 mit Ø 4,9 Schritten bei „erstes widerlegtes Feld“.

## Nächste Schritte

### Als Nächstes

Nichts Festes. Zuletzt erledigt: im Designer die Farben eines Vorschlags neu verteilen („Farben neu verteilen“, siehe [docs/development.md](docs/development.md#design-puzzles-entwerfen)).

### Erledigt / verworfen

Jede Entscheidung, auch verworfene Ideen, steht mit Begründung und Messungen in einer eigenen Datei unter [`architecture/decisions/`](architecture/decisions/README.md) (englisch); Neues kommt dort mit der nächsten Nummer dazu. Vor Änderungen an Regeln, Generator oder Suche dort nachlesen.

### Später

1. **Statistik der Regeln:** Wie oft greift welche Regel (aus dem Trace)? Welche werden kaum gebraucht (z. B. `Confinement` mit großem k)? Grundlage für Messungen wie bei den Prioritätsstufen und beim zusammenhängenden `Confinement`.
2. **Schwierigkeitsgrad für generierte Puzzles:** Für die Bibliothek gibt es ihn schon (siehe [Entscheidung 20](architecture/decisions/20-difficulty-rating.md)). Offen: `generate -difficulty` bzw. `-logic-only` und eine feinere Skala für gewöhnliche Puzzles, die alle unter ★ fielen.
3. **Ausgewogenere Regionen im Generator:** Inseln aus 1 bis 2 Feldern vermeiden (die Reparatur erzeugt sie gern, Ø kleinste Region ca. 1,1); ergibt kniffligere Puzzles. Ein erster Schritt ist `Options.MinRegionSize` (ab N = 10 Standard 2, siehe [Entscheidung 12](architecture/decisions/12-min-region-size-2.md); das verhindert Inseln aus einem Feld, aber nicht aus zwei). Nicht über ausgewogeneres Wachsen lösen, das kostet Eindeutigkeit (siehe [Entscheidung 8](architecture/decisions/08-generator-repair-by-pieces.md)), sondern als Nebenbedingung der Reparatur.
4. **Puzzles von Menschen einlesen** (abgetippt aus Zeitung oder App), um den Solver an gezielt gebauten Rätseln zu prüfen.
5. **Mittelweg beim Lookahead (optional):** Unter den kurzen Ketten die Annahme bevorzugen, nach der die Regeln am meisten selbst schaffen.

## Verifikation
- `go test ./...` und `go test -race ./...`
- `TestSoundness`: Jede Schlussfolgerung der Regeln stimmt mit der Brute-Force-Lösung überein.
- `TestRunnersAgree`: `RunSync` und `RunConcurrent` erreichen denselben Endzustand.
- `TestHardPuzzles`: Die Puzzles in `testdata/hard` werden gelöst und bleiben für die Basisregeln ohne Suche unlösbar. Standardmäßig, auch in der CI, nur die Stichprobe aus `sample.idx` (29 Puzzles, N = 5 bis 15, mit und ohne Lookahead, alle Regeln, 1 bis 3 Sterne), mit `STARBATTLE_ALL_HARD=1` alle. Das gilt auch für `TestPriorityOrder` und `TestJSLogic`. Messung: `observers` 59 → 21 s, `web` 18 → 2,6 s; `TestHardPuzzles` 35 → 5,4 s, `TestJSLogic` 17 → 1,8 s, `TestPriorityOrder` 12 → 3,9 s.
- Manuell: `go run ./cmd/starbattle generate -n 8 | go run ./cmd/starbattle solve -verbose -step`
