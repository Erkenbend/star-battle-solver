# Entwicklung

Hintergrund, Regeln und Messungen stehen in [PLAN.md](../PLAN.md), die Entscheidungen in [architecture/decisions](../architecture/decisions/README.md) (englisch), die ursprüngliche Idee in [grober_plan.md](../grober_plan.md), der Aufbau des Ganzen und warum er so ist in [architecture/](../architecture/README.md) (englisch). Hier geht es ums Bauen, Testen und die Werkzeuge für die Entwicklung.

## Bauen und testen

Voraussetzung ist Go (Version siehe `go.mod`). Es gibt keine weiteren Abhängigkeiten.

```sh
go build -o starbattle ./cmd/starbattle
go test ./...            # dauert etwa zwei Minuten
go test -short ./...     # überspringt einige langsame Tests, etwa 25 Sekunden
go test -race ./...
STARBATTLE_ALL_HARD=1 go test ./...   # alle schweren Puzzles statt der Stichprobe
go test -short -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1   # Testabdeckung
```

Von den schweren Puzzles aus `testdata/hard` prüfen die Tests nur eine Stichprobe (`testdata/hard/sample.idx`, 29 Puzzles), die alle Größen, Regeln und Schwierigkeitsstufen abdeckt; die CI auf GitHub Actions ebenso. Mit `STARBATTLE_ALL_HARD=1` laufen alle Puzzles der Bibliothek durch (dauert geschätzt etwa zwölf Minuten, in der CI eine Dreiviertelstunde). Das lohnt sich nach Änderungen an Regeln, Suche oder Bewertung.

Go kann für jede Plattform von jeder Plattform aus bauen, z. B. für einen Mac mit Intel-Chip:

```sh
GOOS=darwin GOARCH=amd64 go build ./cmd/starbattle
```

Die Screenshots in [docs/screenshots](screenshots) nimmt `docs/screenshots/take.sh` neu auf (braucht Go, node ≥ 22 und Chromium).

Die Sequenzdiagramme in [architecture/concurrency.md](../architecture/concurrency.md) und den Abspieler `architecture/concurrency/player.html` (im Browser öffnen, Schritt für Schritt vor und zurück; der Workflow `pages.yml` veröffentlicht ihn unter <https://erkenbend.github.io/star-battle-solver/>) zeichnet `internal/concviz` aus kleinen Drehbüchern, deren Angaben über den Code (Funktionen, Kanäle, `select`, `close`, …) ein Test gegen den Quelltext prüft. Nach einer Änderung an Goroutinen oder Kanälen sagt `go test ./internal/concviz`, welche Angabe nicht mehr stimmt; nach dem Anpassen schreibt `go test ./internal/concviz -update` Diagramme, Abspieler und die Schrittlisten in `concurrency.md` neu.

### UI-Prüfung

```sh
internal/web/uicheck/run.sh              # alle Szenarien, etwa eine Minute
internal/web/uicheck/run.sh coop race    # nur einige
internal/web/uicheck/run.sh -update      # expected.jsonl neu schreiben
```

Spielt die Puzzle-Seite in einem Chromium ohne Fenster durch: allein lösen mit Hinweisen und Prüfen, Annahme, Lösungsweg, Anleitung mit Rundgang, Bibliothek, eine mit `html` gespeicherte Seite und Partys gegeneinander, miteinander und gegen die Uhr (Alice im Browser, Bob über die API). Partys und Anleitung laufen in Handygröße, die Übergänge einer Party (Lösungsweg, nächste Runde, nächstes Puzzle) auf langsamer Verbindung, während Bobs Verbindungen kommen und gehen. In jedem Szenario gelten als Fehler: ein Seitenwechsel, den die Seite selbst abbricht, und auf dem Handy ein Brett, das sich unter einem Tipp verschiebt. Nach jedem Schritt hält das Skript fest, was Seite und Server zeigen, und vergleicht es mit `internal/web/uicheck/expected.jsonl`; Abweichungen stehen in der Ausgabe. Nach einer gewollten Änderung der Oberfläche schreibt `-update` die Datei neu, ihr Diff gehört dann zum Commit. Braucht Go, node ≥ 22, curl und Chromium (`CHROME`, sonst `chromium`, `google-chrome` oder das von Playwright). Die CI führt die Prüfung als eigenen Job aus und legt bei Abweichungen die beobachtete Datei als Artefakt ab.

## Aufbau

| Verzeichnis | Inhalt |
|---|---|
| `cmd/starbattle` | Kommandozeile |
| `internal/puzzle` | Layout, Textformat, Einheiten (Zeilen, Spalten, Farben) |
| `internal/board` | Zustand des Bretts (frei, Stern, X), Widerspruchserkennung |
| `internal/solver` | Runner (`RunSync`, `RunConcurrent`), Suche mit Lookahead und Raten, Trace |
| `internal/solver/observers` | die logischen Regeln mit Erklärungen |
| `internal/bruteforce` | Referenz-Solver für Generator und Tests |
| `internal/generator` | Puzzles mit eindeutiger Lösung erzeugen, auch um ein gezeichnetes Muster herum |
| `internal/collection` | Format der Puzzle-Sammlungen `testdata/hard` und `designs`: Index `bibliothek.idx`, Dateinamen, Kopfzeilen (`# rating:`, `# name:`), ID |
| `internal/hardsearch` | Suche nach schweren Puzzles für `testdata/hard` (`starbattle hardsearch`) |
| `internal/render` | Ausgabe im Terminal |
| `internal/party` | Party-Modus: Partys, Runden, gegeneinander, miteinander, gegen die Uhr (ohne HTTP) |
| `internal/web` | HTML-Seite, Server, Bibliothek, Einstellungen, HTTP des Party-Modus |
| `internal/designer` | das Werkzeug `starbattle design` (Seite und Speichern) |
| `internal/concviz` | Diagramme und Abspieler in `architecture/concurrency` (Entwicklung) |
| `symbols` | Symbole, die man statt ★ wählen kann (`symbols.json` und Bilder, siehe [symbols/README.md](../symbols/README.md)) |
| `deploy` | Installation als systemd-Dienst auf dem Raspberry Pi (siehe [deploy/README.md](../deploy/README.md)) |
| `designs` | mit `starbattle design` entworfene Puzzles; zugleich die Motive der Bibliothek (Reihenfolge in `bibliothek.idx`, Name in der Zeile `# name:` jeder Datei) |
| `testdata/hard` | Puzzles, an denen die einfachen Regeln scheitern (Nachschub mit `starbattle hardsearch`); zugleich die Bibliothek im Browser (Reihenfolge in `bibliothek.idx`, Stichprobe für die Tests in `sample.idx`) |
| `docs` | diese Dokumentation und die Screenshots |
| `architecture` | Architektur für Entwickelnde (englisch), mit Vorschlägen zum Umbau |
| `.github/workflows` | Tests und Builds auf GitHub Actions (`ci.yml`), Abspieler auf GitHub Pages (`pages.yml`) |

## `design`: Puzzles entwerfen

```sh
starbattle design                       # http://localhost:6370/, speichert nach designs/
starbattle design -dir testdata/designs
```

Ein Werkzeug für die Entwicklung, bewusst nicht Teil von `serve`. Auf der Seite zeichnet man einen Teil des Puzzles: Farbe wählen und Felder bemalen, Rechtsklick oder Radierer macht sie wieder frei. Schon beim Zeichnen prüft der Server, ob das Muster in ein gültiges Puzzle passen kann:

- Die Teile jeder Farbe müssen sich über freie Felder verbinden lassen. Getrennt zeichnen ist erlaubt, der Generator verbindet sie.
- Eine ringsum eingeschlossene Farbe braucht mindestens so viele Felder wie jede Region (ab 10×10 zwei).
- Es muss eine Verteilung der Sterne geben, die zum Muster passt, also nicht z. B. eine Farbe, die zwei ganze Zeilen füllt.
- Sind alle Farben gezeichnet, dürfen die Sterne nicht auf mehrere Arten allein auf gezeichneten Feldern Platz finden. Gezeichnete Felder ändert der Generator nie, sonst hätte jedes fertige Puzzle mehrere Lösungen.

Danach berechnet „Vorschläge berechnen“ mehrere Vervollständigungen gleichzeitig, jede in einer eigenen Goroutine mit eigenem Seed (`generator.Complete`). Jeder Vorschlag erscheint, sobald er fertig ist, und lässt sich sofort speichern, während die anderen noch rechnen. Gezeichnete Felder behalten ihre Farbe, und gezeichnete Farben wachsen nur, wo es nötig ist, damit die Form erkennbar bleibt.

Mit „Streng: Farben nicht erweitern“ bleibt jede gezeichnete Farbe genau so, wie sie ist. Dann muss sie zusammenhängend gezeichnet sein, groß genug und mit Platz für ihren Stern, und die freien Felder füllen nur die Farben, die nicht gezeichnet sind: Jede getrennte freie Fläche braucht mindestens eine davon. Bekommt jede freie Fläche genau eine, steht das Puzzle schon fest, und die Seite prüft gleich, ob es eindeutig ist. Sonst lässt der strenge Modus dem Generator weniger Spielraum und scheitert öfter.

Zu jedem Vorschlag steht, wie weit die Regeln allein kommen. Den passenden auswählen, optional benennen und speichern: Er landet als `nNN_name.txt` (ohne Namen `nNN_d001.txt` usw.) im Verzeichnis, mit Seed und Muster als Kommentar, und wird unten an `bibliothek.idx` angehängt. Damit ist er nach dem nächsten Bauen ein Motiv der Bibliothek. Den Namen dort (`# name: Sad Smiley`, erste Zeile) bildet der Designer aus dem Dateinamen; ändern lässt er sich direkt in der Datei. Bestehende Dateien werden nie überschrieben, und derselbe Name in einer anderen Größe oder dasselbe Puzzle unter anderem Namen wird abgelehnt, weil die Bibliothek beides nicht laden würde. „Ausprobieren“ öffnet den Vorschlag als normale Spielseite.

Bis 14×14 dauert ein Vorschlag meist unter einer Sekunde, selten bis zwei Sekunden; 15×15 im Mittel gut eine Sekunde, manchmal über zehn. Sehr dichte oder verwinkelte Muster schafft der Generator nicht immer; dann hilft es, weniger zu zeichnen.

| Option | Bedeutung |
|---|---|
| `-addr` | Adresse (Standard `localhost:6370`) |
| `-dir` | Verzeichnis für gespeicherte Puzzles (Standard `designs`, wird bei Bedarf angelegt) |

Dazu die [Solver-Optionen](commands.md#solver-optionen).

## `hardsearch`: schwere Puzzles suchen

```sh
go build -o starbattle ./cmd/starbattle    # go build, nicht go run: nur dann steht der Commit im Kopf der Dateien
./starbattle hardsearch -n 13 -from 3001 -count 500 -min-lookahead 4 -o testdata/hard
```

So entsteht Nachschub für `testdata/hard` und damit für die Bibliothek. Der Befehl erzeugt die Puzzles der Seeds `-from` bis `-from + count − 1` genau wie `generate` und prüft jedes: Kommen die Basisregeln weiter? Und wenn nicht, alle Regeln? Wenn auch die nicht weiterkommen, wie viele Lookahead-Schritte (und ob Raten) braucht der Solver? Pro Seed eine Zeile, in Seed-Reihenfolge; Treffer sind mit `HIT` markiert:

```
n=10 seed=450 base=stuck all=solved rating=1 t=58ms
n=10 seed=451 base=solved t=30ms
n=10 seed=453 base=stuck all=stuck lookahead=4 guesses=0 rating=2 t=161ms  HIT → n10_s453.txt
```

Mit `-o` landet jeder Treffer als `nNN_sSEED.txt` im Verzeichnis, mit Befehl, Commit und Grund als Kommentar und darunter der Schwierigkeit (`# rating: 2 (4 lookahead steps, longest 5)`, siehe unten); bestehende Dateien werden nie überschrieben. Am Ende stehen die Zahlen der Suche, der Seed, bei dem die nächste Suche weitermacht (die bisherigen Bereiche stehen in [PLAN.md](../PLAN.md) und [Entscheidung 17](../architecture/decisions/17-library-over-100-sample-tests.md)), und die neuen Dateien. In die Bibliothek kommen sie erst, wenn man sie unten an `testdata/hard/bibliothek.idx` anhängt. Ctrl-C bricht nach den laufenden Seeds ab und nennt trotzdem, wo es weitergeht.

Treffer sind selten: Lookahead mit allen Regeln braucht bei 10×10 bis 12×12 etwa jedes 140. Puzzle, bei 13×13 jedes 55., bei 14×14 jedes 50., bei 15×15 jedes 40., unter 10×10 praktisch keins. Mindestens 4 Lookahead-Schritte hat bei 15×15 etwa jedes 65. Puzzle, bei 10×10 noch seltener (zuletzt keins in 2000 Seeds). Ein Seed dauert allein bis gut zehn Sekunden; weil die Suche alle Kerne gleichzeitig nutzt, zeigt sie Zeiten bis gut eine Minute. 2500 Seeds 15×15 brauchen auf 16 Kernen etwa drei Minuten.

| Option | Bedeutung |
|---|---|
| `-n` | Größe (Standard 10) |
| `-from` | erster Seed (Standard 1) |
| `-count` | Anzahl Seeds (Standard 100) |
| `-min-lookahead` | Treffer brauchen mit allen Regeln mindestens so viele Lookahead-Schritte (Standard 1); 0: Es reicht, dass die Basisregeln hängen bleiben |
| `-o` | Verzeichnis für Treffer (ohne: nur anzeigen) |
| `-workers` | Seeds gleichzeitig (Standard: Zahl der Kerne) |
| `-rate` | keine Suche: alle Puzzles in `-o` neu bewerten (siehe unten) |

### Schwierigkeit

Jedes schwere Puzzle hat eine Bewertung von 1 bis 3 Sternen, die die Bibliothek zeigt (`hardsearch.Rating`). Sie ergibt sich aus dem Lösungsweg des Solvers mit allen Regeln, Lookahead und Raten, also dem, den die Bibliothek nach dem Lösen zeigt:

| Sterne | Bedingung |
|---|---|
| ★★★ | Raten, mindestens 10 Lookahead-Schritte oder ein Gedankenexperiment mit mindestens 7 Schritten |
| ★★☆ | sonst: mindestens ein Lookahead-Schritt oder die Regel `Pigeonhole` |
| ★☆☆ | sonst: Die Regeln allein lösen es |

Die Bewertung steht in der Datei (Zeile `# rating:`), damit die Bibliothek beim Start nichts rechnen muss. Ändern sich Regeln oder Suche, schreibt `./starbattle hardsearch -rate -o testdata/hard` alle Zeilen neu (dauert ein paar Minuten); unveränderte Dateien bleiben unberührt. `TestLibraryRatings` vergleicht die Zeilen mit der heutigen Bewertung, für die Stichprobe bzw. mit `STARBATTLE_ALL_HARD=1` für alle. Motive aus `designs` werden nicht bewertet.
