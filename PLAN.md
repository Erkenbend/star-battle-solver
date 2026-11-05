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
| `internal/board` | Zustand, `Set` mit Konflikterkennung, `Implied`, `Contradiction`, Parser für Testzustände |
| `internal/render` | ANSI-Farben; neue Felder in `[ ]`, Widerspruchszellen in `( )`, Pfeile am Rand |
| `internal/bruteforce` | Referenz-Solver (Backtracking über Zeilen), Test-Orakel |
| `internal/generator` | zufällige Lösung → Regionen wachsen → zweite Lösungen durch Verschieben von Zellen zerstören |
| `internal/solver` | `Mark`, `Observer`, `Apply`, `RunSync`, `RunConcurrent`, `Solve` (Lookahead + Backtracking), Trace mit Begründungen |
| `internal/solver/observers` | Regeln, deutsche Erklärungen, Soundness- und Vergleichstests |
| `cmd/starbattle` | `generate`, `show`, `solve [-runner] [-search] [-step] [-verbose] [-nocolor]` |
| `testdata/hard` | 32 Puzzles, an denen die Regeln allein scheitern |

### Regeln
| Regel | Inhalt |
|---|---|
| `UnitFinish` | Zeile, Spalte oder Farbe mit genau einem freien Feld → Stern |
| `Confinement` (k, From→To) | Freie Felder von k offenen Einheiten liegen in genau k Einheiten eines anderen Typs → Rest dieser Einheiten X. Deckt „Line/Column“, „Color“ und die „Multiple“-Varianten aus dem groben Plan ab, in allen 6 Richtungen, k ≤ N/2 |
| `Elimination` | Ein Stern auf c würde alle freien Felder einer Einheit streichen → c X. Ersetzt die Form-Regeln (2x1, L, Z …). Die Erklärung nennt den konkreten Grund (liegt in einer Einheit / berührt alle / gemischt) |
| `ForcedPartner` (Tiefe, Standard 1) | Ein Stern auf c ließe in einer Einheit nur ein Feld q übrig; c und q zusammen würden eine Einheit leeren → c X. Mit Tiefe > 1 auch Ketten aus mehreren erzwungenen Sternen |
| `DeadCells` (max. Felder, Standard 2) | Nach einem Stern auf c bleiben in einer Einheit nur 2 bis max. Felder, und jedes davon würde eine andere Einheit leeren → c X |

`ForcedPartner` und `DeadCells` sind die zwei häufigsten Muster aus den Annahmeketten der schweren Puzzles (zusammen gut drei Viertel aller Ketten). Konfiguration über `observers.Config` bzw. `solve -partner-depth N -dead-max N` (0 schaltet ab); `BaseConfig` = nur die ersten drei Regeln.

`AllWith` liefert die Regeln von einfach nach anspruchsvoll sortiert (UnitFinish, Confinement k=1, Elimination, Confinement k≥2, ForcedPartner, DeadCells). `RunSync` behandelt die Liste als Priorität und fängt nach jeder Änderung wieder vorne an, damit die Erklärungen einfache Regeln bevorzugen. `RunConcurrent` kennt keine Priorität.

Ergebnis:
- Basisregeln: ca. 96,5 % der generierten Puzzles (Stichprobe von 900 mit N = 4 bis 12).
- Mit `ForcedPartner` und `DeadCells`: 31 der 32 Puzzles aus `testdata/hard` ohne Annahmen; nur `n11_s095` braucht noch Lookahead.

### Suche (`solver.Solve`)
1. Regeln bis zum Fixpunkt.
2. **Lookahead:** Für jedes freie Feld parallel „Stern hier“ annehmen und `RunSync` laufen lassen. Von den Widersprüchen wird der mit der **kürzesten Kette** übernommen (dann weniger gesetzte Felder, dann enge Einheiten zuerst). Das Feld wird X, danach geht es zurück zu 1. Die ganze Kette steht im Trace und wird mit `-verbose`/`-step` verschachtelt angezeigt.
3. **Raten:** Stern im engsten offenen Feld setzen und rekursiv lösen; scheitert das, ist das Feld bewiesenermaßen X, zurück zu 1.

Alle 32 schweren Puzzles werden allein mit Lookahead gelöst, Raten war nie nötig. Der Preis der kürzesten Ketten: 119 Annahmen mit Ø 2,6 Schritten statt 46 mit Ø 4,9 Schritten bei „erstes widerlegtes Feld“.

### Entscheidungen
- Erklärungen sind deutsch; Koordinaten (Zeile, Spalte), beginnend bei 0.
- Probeläufe im Lookahead immer mit `RunSync`, damit die gewählte Erklärung reproduzierbar ist.
- CLI und Tests verwenden dieselbe Seeding-Formel (`rand.NewPCG(seed, n)`); `generate -n 5 -seed 29` erzeugt `testdata/hard/n05_s029.txt`.

## Nächste Schritte

### Als Nächstes

1. **Prioritätsregeln für die Observer:** Die einfachste anwendbare Regel soll immer Vorrang haben, damit Erklärungen so menschlich wie möglich bleiben.
   - Stand: `RunSync` behandelt die Reihenfolge aus `AllWith` als Priorität (Neustart nach jeder Änderung). `RunConcurrent` kennt keine Priorität, dort hängt die Reihenfolge der Schritte vom Zufall ab.
   - **Explizite Prioritätsstufen** statt Reihenfolge in einer Liste, z. B. eine Methode `Priority() int` am Observer oder Stufen in `AllWith`:
     1. `UnitFinish` · 2. `Confinement` k=1 · 3. `Elimination` · 4. `Confinement` k≥2 · 5. `ForcedPartner` · 6. `DeadCells` · (7. Lookahead, 8. Raten im Solver).
   - **Stufen im nebenläufigen Runner:** Der Runner übernimmt Markierungen einer Stufe nur, wenn alle Observer niedrigerer Stufen auf der aktuellen Version idle sind. Vorgeschlagene Umsetzung: Nachrichten höherer Stufen puffern und erst nach Quiescence der niedrigeren Stufen anwenden. Die Goroutinen dürfen weiter parallel rechnen.
   - **Innerhalb einer Stufe:** die kleinere Schlussfolgerung bevorzugen, z. B. bei `Confinement` kleineres k und bei `ForcedPartner` geringere Tiefe. Prüfen, ob eine feste Reihenfolge der Einheiten (Zeilen vor Spalten vor Farben) die Erklärungen ruhiger macht.
   - Messen: Wie oft greifen die Regeln jeder Stufe vorher und nachher (Trace-Statistik, siehe Punkt 5)? `TestRunnersAgree` muss weiter gelten.

2. **`Confinement` auf nebeneinanderliegende Zeilen/Spalten ausrichten:** Menschen betrachten bei „k Zeilen haben nur noch k Farben“ fast immer benachbarte Zeilen oder Spalten, nicht beliebige Teilmengen. Erklärungen wie „Zeilen 1, 4 und 7 …“ sind schwer nachzuvollziehen.
   - Für k ≥ 2 mit From = Zeile/Spalte nur zusammenhängende Blöcke (r, r+1, …, r+k−1) betrachten. Für From = Farbe sind beliebige Teilmengen weiter sinnvoll; zu prüfen ist, ob dort „benachbarte Farben“ (Regionen, die sich berühren) menschlicher sind.
   - Konfigurierbar machen (`Config.ConfinementContiguous`), damit beide Varianten verglichen werden können.
   - Messen, was verloren geht: Wie viele Puzzles brauchen danach zusätzliche Annahmen bzw. `ForcedPartner`/`DeadCells`? Ein Nebeneffekt ist die kleinere Suche (N−k+1 Blöcke statt „N über k“ Teilmengen).
   - Zusammen mit der vorgemerkten Regel „benachbarte Zeilen/Spalten mit drei Spalten Platz“ (unten) ergibt sich eine Familie von Regeln für benachbarte Linien.

### Erledigt / verworfen

- ~~**Generator mit dem eigenen Solver statt Brute Force**~~ ausprobiert und verworfen: Brute Force ist beim Generieren schneller. Der Generator bleibt bei `bruteforce.Solve`.
- ~~**Neue Regeln aus wiederkehrenden Annahmen**~~ umgesetzt als `ForcedPartner` und `DeadCells`.

### Später

3. **Weitere Regeln (vorgemerkt):**
   - **Benachbarte Zeilen/Spalten:** Zwei benachbarte Zeilen haben zusammen nur noch freie Felder in drei nebeneinanderliegenden Spalten c, c+1, c+2 → die Sterne liegen in c und c+2, Spalte c+1 ist in beiden Zeilen X (bei nur zwei Spalten: Widerspruch). Klassische Technik; noch nicht gemessen, wie oft sie greift.
   - **Kreuzung von Zeile und Spalte:** Ein Observer für ein Paar aus Zeile r und Spalte c: Wenn beide ihren Stern in derselben Farbe F haben müssen (alle freien Felder der Zeile und alle freien Felder der Spalte sind F), dann liegt der Stern von F auf der Kreuzung (r, c). Ist (r, c) nicht frei oder nicht F, ist das ein Widerspruch. Bisher nur über eine Kette erreichbar (`Confinement` Zeile→Farbe, dann Spalte→Farbe, dann `UnitFinish`); als eigene Regel ergibt das eine einzige, direkte Erklärung. Zu prüfen: Priorität (vermutlich direkt nach `Confinement` k=1) und wie oft die Regel in den Traces greift.
   - **Schubfachprinzip mit Annahme:** Ein Stern auf c ließe für k Einheiten nur noch weniger als k Einheiten eines anderen Typs übrig (z. B. zwei Spalten nur noch in Farbe B) → X. Kam in den Ketten nur 3-mal vor.
4. **Neue Testdaten**, die auch für die neuen Regeln schwer sind (bisher nur `n11_s095`).
5. **Statistik der Regeln:** Wie oft greift welche Regel (aus dem Trace)? Welche werden kaum gebraucht (z. B. `Confinement` mit großem k)? Grundlage für die Messungen in Punkt 1 und 2.
6. **Schwierigkeitsgrad** aus den benötigten Techniken (Prioritätsstufen, Anzahl und Länge der Annahmen, Raten). Daraus `generate -difficulty` bzw. `-logic-only`.
7. **Ausgewogenere Regionen im Generator:** Kleine Regionen bevorzugen, Inseln aus 2 bis 3 Feldern vermeiden; ergibt kniffligere Puzzles.
8. **Puzzles von Menschen einlesen** (abgetippt aus Zeitung oder App), um den Solver an gezielt gebauten Rätseln zu prüfen.
9. **HTML-Darstellung, möglichst interaktiv:**
   - **Selbst lösen:** Felder per Klick als X oder Stern markieren, mit Rückgängig, Prüfung auf Regelverstöße (Hervorhebung wie die `( )`-Widerspruchszellen) und optional einem Hinweis-Knopf, der die nächste Regel samt Erklärung zeigt.
   - **Automatische Lösung anschauen:** Den Trace Schritt für Schritt abspielen (vor/zurück, Autoplay), neue Felder hervorheben und die deutsche Begründung dazu anzeigen; Annahmeketten aus dem Lookahead aufklappbar wie bei `-verbose`.
   - Umsetzung offen: z. B. `starbattle html` erzeugt eine eigenständige Datei mit Puzzle und Trace als JSON, oder ein kleiner lokaler Server (`starbattle serve`), der auch Hinweise live berechnet. Dafür braucht der Trace ein stabiles JSON-Format.
10. **Mittelweg beim Lookahead (optional):** Unter den kurzen Ketten die Annahme bevorzugen, nach der die Regeln am meisten selbst schaffen.
11. **Aufräumen:** README mit CLI-Befehlen, Branch `grundgeruest` nach `main` mergen.

## Verifikation
- `go test ./...` und `go test -race ./...`
- `TestSoundness`: Jede Schlussfolgerung der Regeln stimmt mit der Brute-Force-Lösung überein.
- `TestRunnersAgree`: `RunSync` und `RunConcurrent` erreichen denselben Endzustand.
- `TestHardPuzzles`: Alle Puzzles in `testdata/hard` werden gelöst und bleiben für die Basisregeln ohne Suche unlösbar.
- Manuell: `go run ./cmd/starbattle generate -n 8 | go run ./cmd/starbattle solve -verbose -step`
