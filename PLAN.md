# Star Battle (Meowdoku) Solver – genauer Plan

## Kontext
Das Repo enthält bisher nur `grober_plan.md`: ein Solver in Go, bei dem mehrere nebenläufige **Observer** (Goroutinen) jeweils eine Regel beobachten und Markierungen (M = Stern, X = ausgeschlossen) über Channels austauschen. Ein Runner verteilt die Markierungen. Dazu kommen ein **Generator** für gültige, eindeutig lösbare Puzzles und eine einfache **Terminal-Darstellung** mit ANSI-Farben.
Variante: **1 Stern** pro Zeile, Spalte und Farbe, auf einem NxN-Brett mit N Farben. Sterne dürfen sich nicht berühren, auch nicht diagonal.

## Validierung der Idee

**Was trägt:**
- **Monotonie macht das Channel-Design sicher.** Der Zustand einer Zelle geht nur von *frei* zu M oder zu X und nie zurück. Dadurch ist die Reihenfolge der Nachrichten egal, und doppelte Nachrichten schaden nicht (idempotent). Jeder Observer kann eine eigene Kopie des Zustands führen, ohne zu locken. Die Variante „eigene Kopie, Updates per Channel“ ist die bessere: Sie ist frei von Data Races und isoliert testbar. Die Variante „Shared Board, read-only“ wäre ein Race, weil der Runner gleichzeitig schreibt.
- Die Regeln „Line/Column“, „Multiple Line/Column“, „Color“ und „Multiple Color“ sind korrekt. Es sind Pigeonhole-Argumente: Sind die freien Zellen von k Zeilen auf genau k Farben beschränkt, dann liegen die Sterne dieser Farben genau in diesen Zeilen, und alle anderen Zellen dieser Farben sind X. Umgekehrt gilt dasselbe mit Farben und Zeilen vertauscht.
- „Bei M verteilt der Runner X für Zeile, Spalte, Farbe und Nachbarschaft“ ist richtig und gehört zentral in den Runner.

**Lücken und Korrekturen:**
1. **„Finish“ fehlt für Zeile und Spalte.** Auch eine Zeile oder Spalte mit nur einer freien Zelle bekommt ein M. „Color Finish“ wird deshalb zu „Unit Finish“ für alle drei Einheitstypen.
2. **„Form“ verallgemeinern.** Statt Formen wie 2x1, L oder Z einzeln aufzuzählen, gibt es einen **Elimination-Observer**: Würde ein M auf Zelle c *alle* freien Zellen einer Einheit (Zeile, Spalte oder Farbe) ausschalten, wird c zu X. Das deckt alle Formen automatisch ab und ist leicht zu testen.
3. **Terminierung ist das schwierigste Problem.** Der Runner kann nicht einfach „Channel leer“ prüfen, weil ein Observer gerade rechnen könnte. Er braucht eine **Quiescence-Erkennung**: Jeder Observer meldet nach seiner Arbeit `Idle{seenVersion}`. Fertig oder festgefahren ist die Runde erst, wenn alle Observer idle gemeldet haben, und zwar auf der aktuellen Version, und keine Nachrichten mehr unterwegs sind.
4. **Deadlock-Gefahr.** Der Runner blockiert beim Senden an Observer A, während A beim Senden an den Runner blockiert. Lösung: Jeder Observer hat eine **unbegrenzte Inbox** (eine Goroutine mit Slice-Queue). Der Runner sendet also nie blockierend. Observer senden auf einen gemeinsamen, gepufferten Output-Channel, den der Runner in seinem `select` immer liest.
5. **„Observer exit wenn Einheit voll“:** Danach darf der Runner ihm nichts mehr schicken (Unsubscribe über eine `Done`-Nachricht). Sonst läuft die Inbox voll, oder ein Send auf einen geschlossenen Channel löst eine Panic aus.
6. **Widerspruchserkennung fehlt**, ist aber für das Backtracking nötig. Widersprüche sind: eine Einheit ohne freie Zelle und ohne M, zwei M in einer Einheit oder benachbart. Der Runner prüft das bei jedem Update.
7. **Backtracking mit Goroutinen ist einfach.** Wenn die Logik stagniert: Zelle mit den wenigsten Alternativen wählen, Zustand klonen, M setzen und eine neue Solver-Instanz starten. Bei Widerspruch wird die Zelle X. Es lohnt sich, das als eigenen „Hypothesis“-Schritt zu bauen und nicht als Observer.

## Architektur (Go 1.26, nur Standardbibliothek)

```
cmd/starbattle/main.go     CLI: generate | solve | show
internal/puzzle/           Puzzle (N, regions [][]int), Parsing/Format (Textformat: Buchstaben je Farbe)
internal/board/            State: cells []Cell{Free,Star,Cross}, Apply(Mark) bool, Clone, Contradiction(), Solved()
internal/render/           ANSI-Ausgabe: Hintergrund je Region, ★ / · / leer; Diff-Highlight für neue Marks
internal/bruteforce/       Referenz-Solver (Backtracking über Zeilen) – CountSolutions(p, limit)
internal/generator/        Puzzle-Generator
internal/solver/           Runner + Observer-Framework
internal/solver/observers/ ein File pro Regel + _test.go
```

### Kerntypen (`internal/solver`)
```go
type Mark struct { Row, Col int; Kind board.CellKind; Source string } // Source = Observer-Name (für Trace)
type Observer interface {
    Name() string
    // reine Funktion: gegeben Zustand → neue Marks. Macht Observer trivial testbar.
    Deduce(s *board.State) []Mark
    Done(s *board.State) bool
}
```
- **Die Logik ist rein, die Nebenläufigkeit liegt im Wrapper.** `runObserver(o, init, inbox <-chan []Mark, out chan<- Msg)` übernimmt die Inbox in die lokale Kopie (Leerlesen ohne Blockieren), ruft `Deduce` auf, bis nichts Neues mehr kommt, sendet die Marks und `Idle{version}` und wartet dann wieder. Jeder Observer lässt sich so ohne Goroutinen testen: Board-Zustand rein, erwartete Marks raus.
- **Observer-Instanzen:** Für maximale Parallelität eine Goroutine pro (Regel × Einheit), z. B. „RowSingleColor[r=3]“. Das passt zum Gedanken „ignoriert, was ihn nicht betrifft“ und zu „exit wenn voll“. Die Multiple-Regeln laufen als je eine Instanz über alle Teilmengen. Die Teilmengengröße ist begrenzt, Standardwert k ≤ N/2, damit die Zahl der Teilmengen klein bleibt.
- **Runner:** Er ist die einzige Instanz mit dem autoritativen State. Er übernimmt Marks (ein Duplikat wird ignoriert, und ein Konflikt zählt als Widerspruch). Bei einem M erzeugt er die abgeleiteten X selbst. Dann leitet er das Delta an alle aktiven Inboxes weiter und erhöht `version`. Er endet bei `Solved`, `Contradiction` oder Quiescence; im letzten Fall greift Hypothesis/Backtracking. Ein optionaler `Trace []Mark` zeichnet auf, welcher Observer was wann gefunden hat. Das ist für die Visualisierung und das Debugging wertvoll.

### Observer-Liste (MVP)
| Name | Regel |
|---|---|
| `UnitFinish` | Zeile, Spalte oder Farbe mit genau 1 freien Zelle und ohne M → M |
| `RowColor` / `ColColor` | Freie Zellen einer Zeile/Spalte haben alle eine Farbe → restliche Zellen dieser Farbe X |
| `ColorInLine` | Freie Zellen einer Farbe liegen in einer Zeile/Spalte → restliche Zellen dieser Zeile/Spalte X |
| `MultiLine` | k Zeilen (oder Spalten) enthalten zusammen genau k Farben → diese Farben außerhalb X |
| `MultiColor` | k Farben liegen zusammen in genau k Zeilen (oder Spalten) → diese Zeilen außerhalb X |
| `Elimination` | Ein M auf c würde eine Einheit leeren → c X (ersetzt „Form“) |

`RowColor` und `ColorInLine` sind die Fälle k=1 von `MultiLine` und `MultiColor`. Ich würde sie trotzdem als eigene, einfache Observer bauen, weil sie leichter zu lesen sind und gut als Lernbeispiel taugen.

## Generator (`internal/generator`)
1. **Zufällige Lösung:** Permutation `p` mit `|p[i]−p[i+1]| > 1`, per Backtracking mit gemischter Reihenfolge.
2. **Regionen wachsen lassen:** Jeder Stern ist der Keim einer Region. Danach wird zufällig per Flood-Fill erweitert, indem eine zufällige Grenzzelle einer zufälligen Region zugeschlagen wird. Das ergibt N zusammenhängende Regionen mit je genau einem Stern.
3. **Eindeutigkeit:** `bruteforce.CountSolutions(p, 2)` prüft auf genau eine Lösung. Bei mehreren Lösungen eine Zelle finden, in der eine zweite Lösung einen Stern hat, die keine Keimzelle ist, und sie einer Nachbarregion zuschlagen, solange die Zusammenhängigkeit erhalten bleibt. Dann erneut prüfen. Nach X Versuchen neu starten.
4. **Optional, Schwierigkeit:** Den eigenen Solver ohne Backtracking laufen lassen und ausweisen, ob ihn die Logik allein schafft und welche Observer nötig waren. Mit `--logic-only` werden nur Puzzles behalten, die ohne Raten lösbar sind.

Der Brute-Force-Solver ist zugleich das **Test-Orakel** für den Observer-Solver.

## Darstellung (`internal/render`)
- `Render(p, state) string`: Jede Zelle besteht aus 2 bis 3 Zeichen mit dem ANSI-256-Hintergrund ihrer Region (Palette mit ca. 12 unterscheidbaren Farben). Inhalt: `★` für M, `·` oder `x` für X, Leerzeichen für frei.
- `--step`: Nach jedem Delta wird das Brett neu ausgegeben, neue Marks sind hervorgehoben, und daneben steht der Name des Observers aus dem Trace.
- Textformat für Puzzles (Datei oder stdin), z. B. für 5x5:
  ```
  AABBB
  ACCDB
  ...
  ```

## CLI
- `starbattle generate -n 8 [-seed 42] [-logic-only] > p.txt`
- `starbattle solve [-step] [-no-guess] p.txt`
- `starbattle show p.txt`

## Umsetzungsreihenfolge
1. `go mod init`, `puzzle`, `board` und `render` (`show` funktioniert)
2. `bruteforce` und Tests mit handgebauten Puzzles
3. `generator` (`generate` funktioniert, Eindeutigkeit wird über `bruteforce` geprüft)
4. `solver`: zuerst ein synchroner Runner, der alle `Deduce` in einer Schleife aufruft (zeigt, dass die Regeln korrekt sind), danach der nebenläufige Runner mit Channels und Quiescence
5. Observer einzeln mit Tabellentests
6. Hypothesis/Backtracking
7. `-step`-Trace-Visualisierung

Der synchrone Runner aus Schritt 4 ist der Trick, um Logikfehler von Nebenläufigkeitsfehlern zu trennen. Beide Runner müssen auf denselben Puzzles dasselbe Ergebnis liefern.

## Verifikation
- `go test ./...`: Tabellentests pro Observer (Zustand rein, erwartete Marks raus). Für den Generator wird geprüft, dass die Regionen zusammenhängen und jeweils genau einen Stern der Lösung enthalten, und dass `CountSolutions == 1` gilt.
- `go test -race ./internal/solver/...`: Der nebenläufige Runner muss frei von Data Races sein.
- **Property-Test:** 200 generierte Puzzles (N = 5 bis 10). Der Observer-Solver mit Backtracking muss jedes Mal die Brute-Force-Lösung finden. Synchroner und nebenläufiger Runner liefern identische Endzustände.
- Manuell: `go run ./cmd/starbattle generate -n 8 | go run ./cmd/starbattle solve -step`
