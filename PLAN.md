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
| `internal/generator` | zufällige Lösung → Regionen wachsen → zweite Lösungen durch Verschieben von Regionsstücken zerstören (siehe „Erledigt“) |
| `internal/solver` | `Mark`, `Observer`, `Apply`, `RunSync`, `RunConcurrent`, `Solve` (Lookahead + Backtracking), Trace mit Begründungen |
| `internal/solver/observers` | Regeln, deutsche Erklärungen, Soundness- und Vergleichstests |
| `internal/web` | Interaktive HTML-Seite (eine Datei, ohne externe Abhängigkeiten): Puzzle, Lösung und Trace als JSON, dazu `assets/` (HTML, CSS, JS); `Handler` für `serve` |
| `cmd/starbattle` | `generate [-min-region]`, `show`, `html [-o] [-title]`, `serve [-addr]`, `solve [-runner] [-search] [-step] [-verbose] [-nocolor] [-partner-depth] [-dead-max] [-pigeonhole-max] [-non-contiguous]` |
| `deploy` | Hosting auf dem Raspberry Pi: `deploy.sh [user@]host` (baut passend zu `uname -m`, installiert als systemd-Dienst), `starbattle.service`, Anleitung in `deploy/README.md` |
| `testdata/hard` | 35 Puzzles, an denen die Basisregeln scheitern (4 davon brauchen auch mit allen Regeln Lookahead) |

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
- Mit `ForcedPartner` und `DeadCells`: 31 der 32 alten Puzzles aus `testdata/hard` ohne Annahmen; nur `n11_s095` braucht noch Lookahead. Dazu kommen `n14_s002`, `n14_s024` und `n14_s029` (12, 1 und 1 Lookahead-Schritte).
- Mit `Pigeonhole` zusätzlich: `n11_s095` braucht 11 statt 14 Lookahead-Schritte.

### Suche (`solver.Solve`)
1. Regeln bis zum Fixpunkt.
2. **Lookahead:** Für jedes freie Feld parallel „Stern hier“ annehmen und `RunSync` laufen lassen. Von den Widersprüchen wird der mit der **kürzesten Kette** übernommen (dann weniger gesetzte Felder, dann enge Einheiten zuerst). Das Feld wird X, danach geht es zurück zu 1. Die ganze Kette steht im Trace und wird mit `-verbose`/`-step` verschachtelt angezeigt.
3. **Raten:** Stern im engsten offenen Feld setzen und rekursiv lösen; scheitert das, ist das Feld bewiesenermaßen X, zurück zu 1.

Alle 35 schweren Puzzles werden allein mit Lookahead gelöst, Raten war nie nötig. Der Preis der kürzesten Ketten: 119 Annahmen mit Ø 2,6 Schritten statt 46 mit Ø 4,9 Schritten bei „erstes widerlegtes Feld“.

### Entscheidungen
- Erklärungen sind deutsch; Koordinaten (Zeile, Spalte), beginnend bei 0.
- Probeläufe im Lookahead immer mit `RunSync`, damit die gewählte Erklärung reproduzierbar ist.
- CLI und Tests verwenden dieselbe Seeding-Formel (`rand.NewPCG(seed, n)`). Die Dateien in `testdata/hard` stammen vom alten Generator (bis Commit `def8c05`, z. B. `generate -n 5 -seed 29` → `n05_s029.txt`); mit dem neuen Generator ergeben dieselben Seeds andere Puzzles. Die Dateien gelten als feste Testdaten. Ausnahme: die `n14_*` stammen von Commit `48ea102` (`generate -n 14 -seed 2` usw., `MinRegionSize` 2, Versuche noch nacheinander).

## Nächste Schritte

### Als Nächstes

_(leer: nächsten Punkt aus „Später“ auswählen)_

### Erledigt / verworfen

- ~~**Generator: Versuche parallel**~~ umgesetzt: `Options.Workers` (Standard 4, höchstens GOMAXPROCS) Versuche laufen gleichzeitig, begrenzt über einen gepufferten Kanal. Jeder Versuch zieht der Reihe nach einen eigenen Zufallsgenerator aus `rng`; es gewinnt der erfolgreiche Versuch mit dem **kleinsten Index**, spätere werden nicht mehr gestartet bzw. brechen in der Reparatur ab. Das Ergebnis hängt also nur vom Seed ab, nicht von der Zahl der Worker (`TestWorkersSameResult`); gegenüber vorher ergeben aber alle Seeds andere Puzzles.
   - Folge der Reproduzierbarkeit: Gelingt Versuch 0 (die Mehrheit), muss trotzdem auf ihn gewartet werden; Gewinn gibt es nur, wenn er scheitert. Dann laufen die nächsten Versuche schon parallel.
   - Messung (Wandzeit pro Puzzle, Puzzles nacheinander, Seeds 1000 ff., 3:40 min), 1 Worker → 4 Worker, Summe über alle Puzzles: N = 12 (30 Puzzles, Versuch 0 scheitert bei 8) 1,12×; N = 13 (20, 5) 1,10×; N = 14 (17, 7) 1,32×, Maximum 17,7 → 10,9 s; N = 15 (4, 1) 1,23×. Nur die Puzzles mit gescheitertem Versuch 0: 1,3× bis 1,5× (N = 15: 1,9×, ein Puzzle).
   - 16 Worker sind schlechter als 4 (N = 12 bis 14: 1,02× / 0,99× / 1,27×): Die zusätzlichen Versuche bremsen Versuch 0 um ca. 14 % (bei 4 Workern ca. 3 %), und mehr als 4 Versuche braucht kaum ein Puzzle.
   - Nebenbefund: Die Zeiten der Messung zur Mindestgröße (unten) liefen mit 14 Puzzles gleichzeitig und sind dadurch zu hoch; allein dauert N = 15 im Median ca. 5 s statt 22 s.
   - Offen: Ein gescheiterter Versuch läuft bis zum Abbruch nach 6·N Reparaturschritten und blockiert so den Gewinner dahinter. Früher abbrechen (z. B. wenn die Zahl der Alternativen nicht sinkt) würde gerade diese Fälle beschleunigen.

- ~~**Mindestgröße 2 für Regionen ab N = 10**~~ umgesetzt: `generator.DefaultMinRegionSize(n)`, verwendet von `generate` (überschreibbar mit `-min-region K`) und `serve`. Gleiche Seeds ergeben ab N = 10 also andere Puzzles als vorher. Messung mit je 50 Puzzles für N = 12 bis 15, Mindestgröße 1 → 2:
   - **Erzeugen:** Kaum mehr verworfene Layouts (Median 1 bis 2 Versuche, max. 5), aber die Reparatur dauert länger: Median 0,07 → 0,13 s (N = 12), 0,23 → 0,56 s (13), 1,1 → 2,8 s (14), 10 → 22 s (15); 90 %-Quantil bei N = 15 41 → 100 s. N = 15 ist also für `serve` zu langsam, egal mit welcher Mindestgröße.
   - **Regionen:** Mit Mindestgröße 1 haben 47 bis 49 von 50 Puzzles eine Ein-Feld-Region (geschenkter Stern). Mit 2 bleiben im Schnitt etwa 2 Regionen aus genau 2 Feldern (vorher 2,4 bis 3 Regionen mit ≤ 2 Feldern): Die Reparatur schrumpft Regionen bis an die Grenze, ausgewogener werden sie nicht.
   - **Schwierigkeit:** Lösungswege 20 bis 30 % länger (Ø Schritte 19,9 → 24,3 / 20,9 → 27,4 / 22,2 → 28,6 / 25,3 → 30,4). Schritte ab Stufe 5 pro Puzzle 0,44 → 0,72 / 0,10 → 1,10 / 0,16 → 0,92 / 0,64 → 0,98. Puzzles, deren schwerster Schritt höchstens Stufe 2 ist: 7 bis 9 → 1 bis 3 von 50. Basisregeln ohne Lookahead: 45 → 46, 48 → 43, 47 → 41, 45 → 43 von 50. Alle Regeln: bei N = 14 brauchen 3 von 50 Lookahead (→ neue Testdaten `n14_s002/024/029`), sonst 0 bzw. 1 von 50; zu wenige Fälle für eine Aussage.
   - Kosten der neuen Testdaten: `observers`-Tests 19 → 29 s, `web`-Tests 1,3 → 5,5 s.

- ~~**HTML-Darstellung**~~ umgesetzt als `starbattle html [-o page.html] [Solver-Optionen] puzzle.txt` (Standard-Runner `sync`, damit die Erklärung reproduzierbar ist).
   - **Selbst lösen:** Klick wechselt frei → ✕ → ★, Rechtsklick/Umschalt setzt direkt ★; Rückgängig/Wiederholen; Nachbarn eines Sterns optional automatisch gekreuzt; Regelverstöße live rot (zwei Sterne, Berührung, Einheit ohne Platz); „Fehler suchen“ vergleicht mit der Lösung; Fortschritt im `localStorage`.
   - **Hinweis:** erster Schritt des Traces, der auf dem Brett noch etwas Neues setzt. Alle Schritte davor liegen schon, also gelten seine Voraussetzungen (die Regeln sind monoton). Mit „Anwenden“ übernehmen; bei falschen Feldern zuerst „Fehler zeigen“.
   - **Lösungsweg:** Schritt für Schritt (Knöpfe, ←/→, Abspielen mit Tempo), Etikett pro Regel nach Stufe eingefärbt; Gedankengänge aus Annahmen eingerückt und mit abgespielt („Gedankenexperiment“, Widerspruchszellen rot), einzeln ausblendbar. Nicht in der Erklärung genannte Zeilen/Spalten/Farben werden abgedunkelt (die Seite liest „Zeilen 3 bis 5“, „Farben B und C“, „(2,3)“ aus dem Text).
   - **Ziehen** setzt viele ✕ auf einmal (auf einem ✕ begonnen: entfernt sie), Sterne bleiben unberührt, ein Zug = ein Rückgängig-Schritt; auch per Touch.
   - **Farben statt Buchstaben:** In allen Texten steht statt „Farbe B“ ein Farbfeld in der Farbe der Region; Zeigen (Touch: Antippen) hebt die Region auf dem Brett hervor. Feste Palette aus 12 abgestimmten Farben (mehr Regionen: berechnete Farbtöne), eigene Varianten für den dunklen Modus. Die Buchstaben in den Feldecken sind entfallen.
   - Hell/dunkel nach Systemeinstellung, Handy-Layout ohne horizontales Scrollen.
   - Tests: `internal/web` (Daten, Escaping, `logic_test.js` mit node: wer nur den Hinweisen folgt, löst alle schweren Puzzles ohne Fehler). Die Oberfläche selbst wurde mit jsdom und Chromium-Screenshots geprüft, diese Skripte liegen nicht im Repo.
   - **`starbattle serve [-addr localhost:8080] [Solver-Optionen]`:** dieselbe Seite vom Server, mit „Neues Puzzle“ (Größe 4 bis 14) und „Eigenes…“ (Puzzle als Text einfügen, vorbelegt mit dem aktuellen). Jedes Puzzle hat eine feste URL (`/?n=8&seed=123` wie `generate -seed`, `/play?p=AAB/…`), Neuladen behält also Puzzle und Fortschritt. Mehrdeutige Puzzles werden ohne Lösung gezeigt, unlösbare abgelehnt.
   - Offen: Hinweise bei Brettern, die vom Trace abweichen, sind nicht immer der einfachste mögliche Schritt (jetzt mit `serve` machbar: Solver auf dem aktuellen Brett starten); Stern per langem Druck auf dem Handy.
- ~~**Neue Regel `Pigeonhole`**~~ (Schubfachprinzip mit Annahme) umgesetzt. Die Suche nach Teilmengen/Blöcken teilt sie mit `Confinement` (`confinedSets`). Messung auf `testdata/hard`:
   - Mit allen Regeln greift sie 3-mal, alle in `n11_s095` (11 statt 14 Lookahead-Schritte); das Puzzle braucht weiter Lookahead. `ForcedPartner` und `DeadCells` haben Vorrang und decken fast alles ab.
   - Nur mit den Basisregeln ist sie stark: 19 von 32 ohne Lookahead (sonst 0), 47 statt 108 Lookahead-Schritte.
   - k ≤ 3 bringt nichts gegenüber k ≤ 2.
- ~~**Regeln „benachbarte Zeilen/Spalten mit drei Spalten Platz“ und „Kreuzung von Zeile und Spalte“**~~ verworfen: Beide finden nichts Neues, sie fassen nur Ketten zusammen (`Elimination` ×2 + `Confinement` k=2 bzw. `Confinement` k=1 + `UnitFinish`) und müssten dafür vor den einfachen Regeln stehen.
- ~~**Generator: weniger verworfene Layouts**~~ umgesetzt. Frisch gewachsene Layouts sind praktisch nie eindeutig (Median ca. 124 Lösungen bei N = 8, über 5000 bei N = 12); alles hängt an der Reparatur. Die alte Reparatur scheiterte meist, weil der Stern der Alternative ein Engpass seiner Region war (70 bis 87 % der Fehlschläge), und drehte sich im Kreis (55 bis 60 % der gesehenen Alternativen waren schon einmal zerstört).
   - **Stück statt Zelle:** Stern x einer Alternative in Region A wandert in eine Nachbarregion B, zusammen mit allem, was ohne x vom Lösungsstern von A abgeschnitten wäre (liegt x im Inneren, zusätzlich der kürzeste Weg zum Rand von A). A bleibt zusammenhängend, die Lösung gültig.
   - **Bewertung über bis zu 8 Alternativen pro Schritt:** Eine Alternative stirbt genau dann, wenn sie einen Stern im Stück hat; Punkte 10 je zerstörte, −20 je wiederbelebte früher zerstörte Alternative, −1 je bewegte Zelle.
   - **Abbruch nach 6·N Schritten** statt 4·N² (erfolgreiche Reparaturen brauchen höchstens ca. 4,5·N).
   - `Options.MinRegionSize` (verwirft Puzzles mit kleineren Regionen, Reparatur verkleinert nie darunter), `Options.Alternatives`.
   - Ergebnis, je 40 Puzzles: N = 10 in 0,09 s statt 1,6 s, N = 12 in 1,8 s statt 151 s. Mit `MinRegionSize` 3 etwa 3- bis 5-mal so viele Versuche. Logik allein löst etwa denselben Anteil wie vorher (135 bis 139 von 140 statt 137 bis 140).
   - Gegenprobe: ausgewogenes oder kompaktes Wachsen der Regionen verschlechtert die Eindeutigkeit deutlich (2- bis 3-mal so viele Lösungen bei N = 8). Krumme, ungleiche Regionen machen Lösungen eindeutig.

- ~~**`Confinement` auf nebeneinanderliegende Zeilen/Spalten**~~ umgesetzt, jetzt Standard (`-non-contiguous` schaltet zurück). Gelöste Linien innerhalb eines Blocks werden übersprungen. Erklärungen nennen Bereiche: „Zeilen 3 bis 5“. Messung über 900 generierte Puzzles (N = 4 bis 12, enthält die 32 schweren):
   - Alle Regeln: 899/900 ohne Lookahead, 14 Lookahead-Schritte, mit und ohne Blöcke gleich.
   - Nur Basisregeln: 867 statt 868 ohne Lookahead, 109 statt 108 Lookahead-Schritte.
   - Gewinn: Mit beliebigen Teilmengen nutzten 41 von 208 `Confinement`-Schritten (k ≥ 2, Zeilen/Spalten, Basisregeln, inkl. Annahmeketten) nicht benachbarte Linien, z. B. „Zeilen 1 und 6 …“.
   - Blöcke mit k > N/2 kamen nie zum Zug.
   - Offen: ob bei Farben „benachbarte Regionen“ menschlicher wären; mit der Regel „benachbarte Zeilen/Spalten mit drei Spalten Platz“ (unten) ergibt sich eine Familie von Regeln für benachbarte Linien.
- ~~**Prioritätsregeln für die Observer**~~ umgesetzt (siehe „Regeln“). Messung über 80 Puzzles (32 schwere + 48 generierte mit N = 5 bis 10), nur Regeln, Schritte pro Stufe L1 bis L6:
   - `RunSync`: 657 / 233 / 323 / 63 / 118 / 2, 0 Verletzungen.
   - `RunConcurrent` vorher: 657 / 122 / 475 / 353 / 656 / 642, **2013 Verletzungen**, d. h. `ForcedPartner` und `DeadCells` feuerten oft, obwohl eine einfache Regel griff.
   - `RunConcurrent` nachher: 657 / 229 / 327 / 61 / 112 / 2, 0 Verletzungen. Kosten: `TestHardPuzzles` 3,2 s statt 2,0 s.
   - Offen (klein): Innerhalb einer Stufe nimmt der nebenläufige Runner die wartende Nachricht mit dem kleinsten Index und wartet nicht auf langsamere Observer derselben Stufe. Bei Bedarf feinere Stufen (z. B. je k bei `Confinement`).
- ~~**Generator mit dem eigenen Solver statt Brute Force**~~ ausprobiert und verworfen: Brute Force ist beim Generieren schneller. Der Generator bleibt bei `bruteforce.Solve`.
- ~~**Neue Regeln aus wiederkehrenden Annahmen**~~ umgesetzt als `ForcedPartner` und `DeadCells`.

### Später

1. **Neue Testdaten**, die auch für die neuen Regeln schwer sind (bisher `n11_s095` und drei `n14_*`; mit Mindestgröße 2 bei N = 14 etwa jedes 17. Puzzle).
2. **Statistik der Regeln:** Wie oft greift welche Regel (aus dem Trace)? Welche werden kaum gebraucht (z. B. `Confinement` mit großem k)? Grundlage für Messungen wie bei den Prioritätsstufen und beim zusammenhängenden `Confinement`.
3. **Schwierigkeitsgrad** aus den benötigten Techniken (Prioritätsstufen, Anzahl und Länge der Annahmen, Raten). Daraus `generate -difficulty` bzw. `-logic-only`.
4. **Ausgewogenere Regionen im Generator:** Inseln aus 1 bis 2 Feldern vermeiden (die Reparatur erzeugt sie gern, Ø kleinste Region ca. 1,1); ergibt kniffligere Puzzles. Ein erster Schritt ist `Options.MinRegionSize` (ab N = 10 Standard 2, siehe „Erledigt“; das verhindert Inseln aus einem Feld, aber nicht aus zwei). Nicht über ausgewogeneres Wachsen lösen, das kostet Eindeutigkeit (siehe „Erledigt“), sondern als Nebenbedingung der Reparatur.
5. **Puzzles von Menschen einlesen** (abgetippt aus Zeitung oder App), um den Solver an gezielt gebauten Rätseln zu prüfen.
6. **Mittelweg beim Lookahead (optional):** Unter den kurzen Ketten die Annahme bevorzugen, nach der die Regeln am meisten selbst schaffen.
7. **Aufräumen:** README mit CLI-Befehlen.

## Verifikation
- `go test ./...` und `go test -race ./...`
- `TestSoundness`: Jede Schlussfolgerung der Regeln stimmt mit der Brute-Force-Lösung überein.
- `TestRunnersAgree`: `RunSync` und `RunConcurrent` erreichen denselben Endzustand.
- `TestHardPuzzles`: Alle Puzzles in `testdata/hard` werden gelöst und bleiben für die Basisregeln ohne Suche unlösbar.
- Manuell: `go run ./cmd/starbattle generate -n 8 | go run ./cmd/starbattle solve -verbose -step`
