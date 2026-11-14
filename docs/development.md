# Entwicklung

Hintergrund, Regeln, Messungen und Entscheidungen stehen in [PLAN.md](../PLAN.md), die ursprüngliche Idee in [grober_plan.md](../grober_plan.md). Hier geht es ums Bauen, Testen und die Werkzeuge für die Entwicklung.

## Bauen und testen

Voraussetzung ist Go (Version siehe `go.mod`). Es gibt keine weiteren Abhängigkeiten.

```sh
go build -o starbattle ./cmd/starbattle
go test ./...            # dauert etwa 20 Sekunden
go test -short ./...     # überspringt einige langsame Tests
go test -race ./...
STARBATTLE_ALL_HARD=1 go test ./...   # alle schweren Puzzles statt der Stichprobe
go test -short -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1   # Testabdeckung
```

Von den schweren Puzzles aus `testdata/hard` prüfen die Tests nur eine Stichprobe (`testdata/hard/sample.idx`), die alle Größen und Regeln abdeckt. Mit `STARBATTLE_ALL_HARD=1` laufen alle Puzzles der Bibliothek durch (dauert etwa sechs Minuten). Die CI auf GitHub Actions prüft immer alle.

Go kann für jede Plattform von jeder Plattform aus bauen, z. B. für einen Mac mit Intel-Chip:

```sh
GOOS=darwin GOARCH=amd64 go build ./cmd/starbattle
```

Die Screenshots in [docs/screenshots](screenshots) nimmt `docs/screenshots/take.sh` neu auf (braucht Go, node ≥ 22 und Chromium).

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
| `internal/hardsearch` | Suche nach schweren Puzzles für `testdata/hard` (`starbattle hard`) |
| `internal/render` | Ausgabe im Terminal |
| `internal/web` | HTML-Seite, Server, Party-Modus, Bibliothek, Einstellungen, Designer |
| `symbols` | Symbole, die man statt ★ wählen kann (`symbols.json` und Bilder, siehe [symbols/README.md](../symbols/README.md)) |
| `deploy` | Installation als systemd-Dienst auf dem Raspberry Pi (siehe [deploy/README.md](../deploy/README.md)) |
| `designs` | mit `starbattle design` entworfene Puzzles; zugleich die Motive der Bibliothek (Reihenfolge in `bibliothek.idx`, Name in der Zeile `# name:` jeder Datei) |
| `testdata/hard` | Puzzles, an denen die einfachen Regeln scheitern (Nachschub mit `starbattle hard`); zugleich die Bibliothek im Browser (Reihenfolge in `bibliothek.idx`, Stichprobe für die Tests in `sample.idx`) |
| `docs` | diese Dokumentation und die Screenshots |
| `.github/workflows` | Tests und Builds auf GitHub Actions |

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

Bis 12×12 dauert ein Vorschlag meist unter einer Sekunde, 14×14 kann mehrere Minuten brauchen, besonders mit großen rechteckigen Blöcken. Sehr dichte oder verwinkelte Muster schafft der Generator nicht immer; dann hilft es, weniger zu zeichnen.

| Option | Bedeutung |
|---|---|
| `-addr` | Adresse (Standard `localhost:6370`) |
| `-dir` | Verzeichnis für gespeicherte Puzzles (Standard `designs`, wird bei Bedarf angelegt) |

Dazu die [Solver-Optionen](commands.md#solver-optionen).

## `hard`: schwere Puzzles suchen

```sh
go build -o starbattle ./cmd/starbattle    # go build, nicht go run: nur dann steht der Commit im Kopf der Dateien
./starbattle hard -n 13 -from 3001 -count 500 -min-lookahead 4 -o testdata/hard
```

So entsteht Nachschub für `testdata/hard` und damit für die Bibliothek. Der Befehl erzeugt die Puzzles der Seeds `-from` bis `-from + count − 1` genau wie `generate` und prüft jedes: Kommen die Basisregeln weiter? Und wenn nicht, alle Regeln? Wenn auch die nicht weiterkommen, wie viele Lookahead-Schritte (und ob Raten) braucht der Solver? Pro Seed eine Zeile, in Seed-Reihenfolge; Treffer sind mit `HIT` markiert:

```
n=10 seed=450 base=stuck all=solved t=58ms
n=10 seed=451 base=solved t=30ms
n=10 seed=453 base=stuck all=stuck lookahead=4 guesses=0 t=161ms  HIT → n10_s453.txt
```

Mit `-o` landet jeder Treffer als `nNN_sSEED.txt` im Verzeichnis, mit Befehl, Commit und Grund als Kommentar; bestehende Dateien werden nie überschrieben. Am Ende stehen die Zahlen der Suche, der Seed, bei dem die nächste Suche weitermacht (die bisherigen Bereiche stehen in [PLAN.md](../PLAN.md)), und die neuen Dateien. In die Bibliothek kommen sie erst, wenn man sie unten an `testdata/hard/bibliothek.idx` anhängt. Ctrl-C bricht nach den laufenden Seeds ab und nennt trotzdem, wo es weitergeht.

Treffer sind selten: Lookahead mit allen Regeln braucht bei 10×10 bis 12×12 etwa jedes 130. Puzzle, bei 13×13 jedes 70., bei 14×14 jedes 50., unter 10×10 praktisch keins. Ein 14×14-Seed dauert bis zu einigen Sekunden.

| Option | Bedeutung |
|---|---|
| `-n` | Größe (Standard 10) |
| `-from` | erster Seed (Standard 1) |
| `-count` | Anzahl Seeds (Standard 100) |
| `-min-lookahead` | Treffer brauchen mit allen Regeln mindestens so viele Lookahead-Schritte (Standard 1); 0: Es reicht, dass die Basisregeln hängen bleiben |
| `-o` | Verzeichnis für Treffer (ohne: nur anzeigen) |
| `-workers` | Seeds gleichzeitig (Standard: Zahl der Kerne) |
