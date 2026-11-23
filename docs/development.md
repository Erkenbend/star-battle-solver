# Entwicklung

Hintergrund, Regeln und Messungen stehen in [PLAN.md](../PLAN.md), die Entscheidungen in [architecture/decisions](../architecture/decisions/README.md) (englisch), die ursprüngliche Idee in [grober_plan.md](../grober_plan.md), der Aufbau des Ganzen und warum er so ist in [architecture/](../architecture/README.md) (englisch). Hier geht es ums Bauen, Testen und die Werkzeuge für die Entwicklung.

## Bauen und testen

Voraussetzung ist Go (Version siehe `go.mod`). Es gibt keine weiteren Abhängigkeiten.

```sh
go build -o starbattle ./cmd/starbattle
go build -ldflags "-X starbattle/internal/buildinfo.Time=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o starbattle ./cmd/starbattle   # mit Bauzeit für die Einstellungsseite, wie deploy.sh und die CI
go test ./...            # dauert etwa zwei Minuten
go test -short ./...     # überspringt einige langsame Tests, etwa 25 Sekunden
go test -race ./...
STARBATTLE_ALL_HARD=1 go test ./...   # alle schweren Puzzles statt der Stichprobe
go test -short -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1   # Testabdeckung
```

Go-Code ist mit `gofmt -s` formatiert; die CI prüft das. Der Hook in `.githooks` formatiert die Go-Dateien eines Commits vorher selbst (einmal je Klon einschalten mit `git config core.hooksPath .githooks`). Hat eine Datei außerdem Änderungen, die nicht im Commit sind, oder wird mit `git commit <Pfade>` committet, formatiert er nicht, sondern bricht ab und nennt sie.

Von den schweren Puzzles aus `testdata/hard` prüfen die Tests nur eine Stichprobe (`testdata/hard/sample.idx`, 29 Puzzles), die alle Größen, Regeln und Schwierigkeitsstufen abdeckt; die CI auf GitHub Actions ebenso. Mit `STARBATTLE_ALL_HARD=1` laufen alle Puzzles der Bibliothek durch (dauert geschätzt etwa zwölf Minuten, in der CI eine Dreiviertelstunde). Das lohnt sich nach Änderungen an Regeln, Suche oder Bewertung.

Die Lösungswege der Stichprobe, so wie die Seiten sie bekommen (Regel, Stufe, Begründung, was sie nennt (`refs`, das hebt die Seite hervor) und Markierungen jedes Schritts in ihrer Reihenfolge, die widerlegten Annahmen mit ihrem Widerspruch), liegen in `internal/web/testdata/traces`; `TestTraceGolden` vergleicht sie (mit `-short` ohne die Puzzles mit 3 Sternen). Eine Änderung an Regeln, Suche oder `board`, die keine Erklärung ändern soll, lässt die Dateien, wie sie sind. Soll sie Erklärungen ändern (neue Regel, andere Formulierung), schreibt `go test ./internal/web -run TestTraceGolden -update` sie neu, und ihr Diff gehört zum Commit.

Go kann für jede Plattform von jeder Plattform aus bauen, z. B. für einen Mac mit Intel-Chip:

```sh
GOOS=darwin GOARCH=amd64 go build ./cmd/starbattle
```

Die Screenshots in [docs/screenshots](screenshots) nimmt `docs/screenshots/take.sh` neu auf (braucht Go, node ≥ 22 und Chromium). Das Skript `shots.mjs` klickt Knöpfe über ihre ids an wie die UI-Prüfung; teilweise gelöste Bretter zeigen einen Stand, wie er beim Spielen entsteht: zu jedem Stern die Kreuze, die er nach sich zieht.

Die Sequenzdiagramme in [architecture/concurrency.md](../architecture/concurrency.md) und den Abspieler `architecture/concurrency/player.html` (im Browser öffnen, Schritt für Schritt vor und zurück; der Workflow `pages.yml` veröffentlicht ihn unter <https://erkenbend.github.io/star-battle-solver/>) zeichnet `internal/concviz` aus kleinen Drehbüchern, deren Angaben über den Code (Funktionen, Kanäle, `select`, `close`, …) ein Test gegen den Quelltext prüft. Nach einer Änderung an Goroutinen oder Kanälen sagt `go test ./internal/concviz`, welche Angabe nicht mehr stimmt; nach dem Anpassen schreibt `go test ./internal/concviz -update` Diagramme, Abspieler und die Schrittlisten in `concurrency.md` neu.

### UI-Prüfung

```sh
internal/web/uicheck/run.sh                   # alle Gruppen, etwa zwei Minuten
internal/web/uicheck/run.sh solo designer     # nur diese Gruppen
internal/web/uicheck/run.sh -update designer  # ihre Dateien in expected/ neu schreiben
```

Spielt die Seiten in einem Chromium ohne Fenster durch. Die Szenarien sind nach Seiten bzw. Wegen zwischen Seiten gruppiert, jede Gruppe in einer Datei in `internal/web/uicheck/groups` mit ihrer Datei in `expected/`; nach einer Änderung an einer Seite reicht ihre Gruppe, die CI führt alle aus. Die Szenarien einer Gruppe laufen immer zusammen und in ihrer Reihenfolge.

| Gruppe | Was sie durchspielt |
|---|---|
| `tutorial` | Anleitung mit Rundgang für einen neuen Besucher |
| `solo` | allein lösen mit Hinweisen und Prüfen, Annahme, Lösungsweg, „Ergebnis kopieren“, „Neues Puzzle“ mit einer oder mehreren Größen, eine mit `html` gespeicherte Seite (bis zum Lösen) |
| `library` | Bibliothek: ein schweres Puzzle (Lösungsweg erst nach dem Lösen, auch mit `#weg`) und ein Motiv, „Ergebnis kopieren“, das Gelöste nach dem Neuladen und in der Übersicht, das letzte ungelöste Motiv |
| `party` | Partys gegeneinander (auch mit Leben), miteinander (auch neu geladen) und gegen die Uhr (Alice im Browser, Bob über die API), „Link kopieren“ |
| `site` | der Weg zurück zum ungelösten Solo-Puzzle über „Solo“, das Ende von Bibliothek, Einstellungen und Party-Seite, GitHub-Link und Version in den Einstellungen, Installieren als App |
| `designer` | `starbattle design`: zeichnen und radieren, Prüfen beim Zeichnen, strenger Modus, Suche und Anhalten, Vorschläge ansehen, Farben neu verteilen, speichern und ausprobieren, Größe ändern und leeren |

Partys, Anleitung, das Lösen allein (ein zweites Mal), die Bibliothek (ein zweites Mal) und der Weg zurück zum Solo-Puzzle laufen in Handygröße, die Übergänge einer Party (Lösungsweg, nächste Runde, nächstes Puzzle) auf langsamer Verbindung, während Bobs Verbindungen kommen und gehen. Bibliothek, Einstellungen und Party-Seite werden in Handygröße ans Ende gescrollt: Ihr letzter Teil muss über der Leiste der Seitenbereiche enden. Ob Chromium jede Art von Seite als App installieren würde, prüft es selbst (Manifest und Icons), die mit `html` gespeicherte nicht. Den Designer startet `run.sh` mit `-dir designs` in einem temporären Verzeichnis. In jedem Szenario gelten als Fehler: ein Seitenwechsel, den die Seite selbst abbricht, ein Link oder Formular auf einer Puzzle-Seite, das außerhalb der Leiste der Seitenbereiche auf eine andere Seite führt (erlaubt ist nur der Weg zum nächsten Puzzle, `ALLOWED_WAYS` in `lib.mjs`), und auf dem Handy ein Brett, das sich unter einem Tipp verschiebt, im Designer ebenso die Zeichenfläche und das große Bild eines Vorschlags (auch am Desktop). Auf dem Handy hält jede Beobachtung außerdem fest, ob die Knopfleiste unter der Leiste der Seitenbereiche liegt und welche ihrer Wörter abgeschnitten sind („Aufgeb…“). Nach jedem Schritt hält das Skript fest, was Seite und Server zeigen, und vergleicht es mit `internal/web/uicheck/expected/<gruppe>.jsonl`; Abweichungen stehen in der Ausgabe. Nach einer gewollten Änderung der Oberfläche schreibt `-update` die Dateien der ausgeführten Gruppen neu, ihr Diff gehört dann zum Commit. Braucht Go, node ≥ 22, curl und Chromium (`CHROME`, sonst `chromium`, `google-chrome` oder das von Playwright). Die CI führt die Prüfung als eigenen Job aus und legt bei Abweichungen die beobachteten Dateien als Artefakt ab.

## Aufbau

| Verzeichnis | Inhalt |
|---|---|
| `cmd/starbattle` | Kommandozeile |
| `internal/puzzle` | Layout, Textformat, Einheiten (Zeilen, Spalten, Farben) |
| `internal/words` | deutsche Namen von Einheiten und Feldern für die Erklärungen, und was ein Text nennt (zum Hervorheben auf der Seite) |
| `internal/board` | Zustand des Bretts (frei, Stern, X), Widerspruchserkennung |
| `internal/solver` | Runner (`RunSync`, `RunConcurrent`), Suche mit Lookahead und Raten, Trace |
| `internal/solver/observers` | die logischen Regeln mit Erklärungen |
| `internal/bruteforce` | Referenz-Solver für Generator und Tests |
| `internal/generator` | Puzzles mit eindeutiger Lösung erzeugen, auch um ein gezeichnetes Muster herum |
| `internal/collection` | Format der Puzzle-Sammlungen `testdata/hard` und `designs`: Index `bibliothek.idx`, Dateinamen, Kopfzeilen (`# rating:`, `# name:`), ID |
| `internal/hardsearch` | Suche nach schweren Puzzles für `testdata/hard` (`starbattle hardsearch`) |
| `internal/stress` | Lastszenarien gegen `serve` (`starbattle stress`) |
| `internal/render` | Ausgabe im Terminal |
| `internal/party` | Party-Modus: Partys, Runden, gegeneinander, miteinander, gegen die Uhr (ohne HTTP) |
| `internal/web` | HTML-Seite, Server, Bibliothek, Einstellungen, HTTP des Party-Modus |
| `internal/buildinfo` | Commit und Bauzeit des Programms (Einstellungsseite, Kopf der Dateien von `hardsearch`) |
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

Danach sucht „Vorschläge suchen“ auf allen Kernen bis auf einen: Jeder Kern vervollständigt das Muster immer wieder mit einem neuen Seed (`generator.Complete`, die Eindeutigkeit prüft dabei der Brute-Force-Solver), bis so viele verschiedene Puzzles gefunden sind, wie bei „Höchstens … Vorschläge“ eingestellt, spätestens nach 100.000 Versuchen des Generators oder bei „Anhalten“. Jeder Vorschlag erscheint, sobald er gefunden und vom Solver bewertet ist, und lässt sich sofort speichern, während die Suche weiterläuft. Neben der Zeit zählt die Seite mit, wie viele Puzzles angenommen (eindeutig lösbar und neu) und wie viele versucht wurden, und wie viele doppelt gefunden wurden. Gezeichnete Felder behalten ihre Farbe, und gezeichnete Farben wachsen nur, wo es nötig ist, damit die Form erkennbar bleibt.

Mit „Streng: Farben nicht erweitern“ bleibt jede gezeichnete Farbe genau so, wie sie ist. Dann muss sie zusammenhängend gezeichnet sein, groß genug und mit Platz für ihren Stern, und die freien Felder füllen nur die Farben, die nicht gezeichnet sind: Jede getrennte freie Fläche braucht mindestens eine davon. Bekommt jede freie Fläche genau eine, steht das Puzzle schon fest, und die Seite prüft gleich, ob es eindeutig ist. Sonst lässt der strenge Modus dem Generator weniger Spielraum und scheitert öfter.

Zu jedem Vorschlag steht, wie weit die Regeln allein kommen und wie viele Gedankenexperimente der Solver danach braucht; die schwersten stehen vorn. Den passenden auswählen, optional benennen und speichern: Er landet als `nNN_name.txt` (ohne Namen `nNN_d001.txt` usw.) im Verzeichnis, mit Seed und Muster als Kommentar, und wird unten an `bibliothek.idx` angehängt. Damit ist er nach dem nächsten Bauen ein Motiv der Bibliothek. Den Namen dort (`# name: Sad Smiley`, erste Zeile) bildet der Designer aus dem Dateinamen; ändern lässt er sich direkt in der Datei. Bestehende Dateien werden nie überschrieben, und derselbe Name in einer anderen Größe oder dasselbe Puzzle unter anderem Namen wird abgelehnt, weil die Bibliothek beides nicht laden würde. „Ausprobieren“ öffnet den Vorschlag als normale Spielseite (in einem Browser, der den Designer noch nicht kennt, wie bei `serve` erst die Anleitung).

Die Farben der Regionen vergibt der Generator; passen sie nicht zum Bild (etwa ein grünes Herz), verteilt „Farben neu verteilen“ sie neu. Dann werden alle Zonen grau und „Muster hervorheben“ geht aus. Unter dem Bild erscheint eine Leiste mit den Farben: Farbe anklicken, dann die Zone im Bild. Jede Farbe gibt es nur einmal; malt man sie auf eine andere Zone, wird ihre bisherige wieder grau, und ein Klick auf eine Zone in der gewählten Farbe macht sie ebenfalls grau. „Rest automatisch verteilen“ gibt jeder grauen Zone ihre ursprüngliche Farbe, wenn die noch frei ist, sonst eine freie (nach „Alles grau“ also wieder die Farben des Generators), „Alles grau“ fängt von vorn an, „Fertig“ schließt die Leiste. Speichern und Ausprobieren gehen erst, wenn jede Zone eine Farbe hat, und nehmen dann die neuen Farben; die Vorschau des Vorschlags zeigt sie auch. Die Datei enthält das Puzzle in den neuen Farben, das Muster im Kommentar bleibt, wie es gezeichnet war (das bekam der Generator), und eine Zeile `# colors swapped afterwards: ABCDEFGH -> CABDEFGH` nennt die Zuordnung. Ein anderer Vorschlag schließt die Leiste und verwirft eine halb fertige Färbung; eine fertige bleibt beim Vorschlag, bis zur nächsten Suche. Andere Farben machen kein neues Puzzle: Ist es schon gespeichert, lehnt der Designer es ab.

Bei den meisten Mustern sind die Vorschläge nach Bruchteilen einer Sekunde da, bei 14×14 nach wenigen Sekunden. Die 100.000 Versuche sind für Muster, die der Generator selten schafft; auf 16 Kernen reichen sie bei gezeichneten Mustern von 10×10 bis 14×14 für 20 Sekunden bis gut eine Minute (Messungen in [Entscheidung 29](../architecture/decisions/29-designer-searches-longer.md)). Sehr dichte oder verwinkelte Muster schafft der Generator nicht immer; dann bleibt „angenommen“ bei 0, und es hilft, weniger zu zeichnen.

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

## `stress`: den Server unter Last messen

```sh
./starbattle stress                                # alle Szenarien gegen diese Version, gut zwei Minuten
./starbattle stress -server /tmp/alt crawl flood   # dieselben gegen eine andere Version
./starbattle stress -serve-args=-gzip=false        # gegen serve mit diesen Optionen
./starbattle stress -url http://localhost:6369 -pid 1234 reload   # gegen einen laufenden Server
```

Spielt Lastszenarien gegen `serve` durch und misst, wie lange die Besucher warten und wie viel Rechenzeit und Speicher der Server braucht ([Entscheidung 33](../architecture/decisions/33-solve-budget.md)). Gedacht für Änderungen daran, wie der Server seine Arbeit einteilt (Budget für den Solver, vorbereitete Puzzles, Cache, Bibliothek): vorher und nachher messen, mit `-server` gegen ein Programm, das aus dem Commit davor gebaut ist (`git worktree add /tmp/alt HEAD~1 && go build -C /tmp/alt -o /tmp/alt/starbattle ./cmd/starbattle`, dann `-server /tmp/alt/starbattle`, danach `git worktree remove /tmp/alt`).

Jedes Szenario bekommt einen frischen Server (`serve -no-pregen`) mit `GOMAXPROCS=4`, wie ein Raspberry Pi 4; wo es `taskset` gibt, läuft er auf den ersten vier Kernen und die Last auf den übrigen. Rechenzeit und Speicher liest `stress` unter Linux aus `/proc`, sonst steht dort `n/a`. Die Last fragt wie ein Browser nach gzip; nur `pages` wählt selbst.

| Szenario | Was passiert |
|---|---|
| `tutorial` | 200-mal `/anleitung` hintereinander |
| `reload` | ein schweres Puzzle über `/play` zehnmal neu geladen, dann öffnen acht Spieler gleichzeitig ein anderes |
| `abandon` | zwölf Besucher fügen schwere Puzzles ein und gehen nach 300 ms wieder: Rechnet der Server danach weiter? |
| `flood` | zwölf Besucher fügen gleichzeitig schwere Puzzles ein und warten |
| `crawl` | ein Crawler öffnet alle schweren Puzzles der Bibliothek, während ein Spieler alle 300 ms ein neues 10×10 will und danach drei Motive geöffnet werden; dauert auf vier Kernen etwa eine Minute |
| `pages` | 16 Spieler laden 50-mal gleichzeitig eine Seite (wie eine Party in die nächste Runde), ohne und mit gzip: eine gelöste schwere Seite der Bibliothek, neue 8×8, die Anleitung; Rechenzeit und gesendete Bytes je Seite ([Entscheidung 34](../architecture/decisions/34-gzip.md)) |

| Option | Bedeutung |
|---|---|
| `-server` | Programm, das als Server läuft (Standard: dieses) |
| `-cores` | `GOMAXPROCS` des Servers und Kerne, auf die er gelegt wird (Standard 4) |
| `-addr` | Adresse des gestarteten Servers (Standard `localhost:6391`) |
| `-url` | statt einen zu starten: diesen laufenden Server nehmen |
| `-pid` | mit `-url`: sein Prozess, für Rechenzeit und Speicher |
| `-heavy` | Größe der schweren Puzzles für `/play` (und eine mehr; Standard 14) |
