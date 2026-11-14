# Star Battle Solver

[![CI](https://github.com/Erkenbend/star-battle-solver/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Erkenbend/star-battle-solver/actions/workflows/ci.yml?query=branch%3Amain)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2FErkenbend%2Fstar-battle-solver%2Fbadges%2Fcoverage.json)](https://github.com/Erkenbend/star-battle-solver/actions/workflows/ci.yml?query=branch%3Amain)
[![Bibliothek](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2FErkenbend%2Fstar-battle-solver%2Fbadges%2Flibrary.json)](testdata/hard/bibliothek.idx)
[![Plattformen](https://img.shields.io/badge/Plattformen-Linux%20%C2%B7%20macOS%20%C2%B7%20Windows%20%C2%B7%20Raspberry%20Pi-informational)](#herunterladen)
[![Go](https://img.shields.io/github/go-mod/go-version/Erkenbend/star-battle-solver)](go.mod)
[![Abhängigkeiten: keine](https://img.shields.io/badge/Abh%C3%A4ngigkeiten-keine-brightgreen)](go.mod)
[![Lizenz: MIT](https://img.shields.io/github/license/Erkenbend/star-battle-solver?label=Lizenz)](LICENSE)
[![Vibe-coded mit Claude](https://img.shields.io/badge/vibe--coded-Claude%20Opus%205.5-D97757)](grober_plan.md)

Erzeugt und löst Star-Battle-Puzzles (auch „Meowdoku“ oder „Queens“) und erklärt jeden Lösungsschritt auf Deutsch.

![Lösungsweg im Browser: jeder Schritt mit Regel und Begründung](docs/screenshots/loesungsweg.png)

> **Hinweis:** Die gesamte Implementierung wurde von Claude Opus 5.5 geschrieben, das Projekt ist also auch ein **Vibe-Coding-Experiment**. Von Menschenhand stammt nur [grober_plan.md](grober_plan.md), die Idee ganz am Anfang. Alles andere, Code, Tests, Dokumentation und auch diese README, ist nach und nach in Chats mit Claude entstanden.

**Regeln des Puzzles:** NxN-Brett mit N Farben. In jeder Zeile, jeder Spalte und jeder Farbe steht genau **ein Stern**. Sterne berühren sich nicht, auch nicht diagonal.

## Worum es geht

**Kann ein Programm ein Star Battle so lösen, wie ein Mensch es tun würde?** Per Brute Force ist es in Millisekunden gelöst, aber das hilft niemandem, der selbst vor einem Puzzle sitzt und nicht weiterkommt. Dieser Solver geht deshalb Schritt für Schritt vor, mit Regeln, die man nachvollziehen kann, und einer Begründung für jedes Kreuz und jeden Stern.

- **Logische Regeln statt Suche:** letztes freies Feld, eingesperrte Farben, Schubfachprinzip … Einfache Regeln haben Vorrang, so wie man selbst zuerst nach dem Offensichtlichen sucht. Schon die drei Basisregeln lösen etwa 96,5 % der erzeugten Puzzles.
- **Gedankenexperiment statt Raten:** Stecken die Regeln fest, probiert der Solver „Was wäre, wenn hier ein Stern stünde?“ und nimmt den Widerspruch mit der kürzesten Kette. Echtes Raten gibt es nur als allerletzten Ausweg.
- **Erklärungen auf Deutsch,** im Terminal und auf einer interaktiven Seite zum Abspielen des Lösungswegs oder für Hinweise beim eigenen Lösen.
- **Eine Bibliothek der härtesten Fälle,** im Browser ohne jede Hilfe zu lösen. Den Lösungsweg des Solvers gibt es zum Vergleich erst danach.
- **Brute Force nur als Schiedsrichter:** Sie prüft, dass jedes erzeugte Puzzle genau eine Lösung hat und dass die Regeln nichts Falsches folgern.

Nebenbei ist es ein Go-Experiment: Die Regeln laufen als nebenläufige Observer, eine Goroutine pro Regel, die ihre Folgerungen über Channels austauschen. Regeln, Messungen und Entscheidungen stehen in [PLAN.md](PLAN.md).

| | |
|---|---|
| ![Gedankenexperiment](docs/screenshots/annahme.png) | ![Selbst lösen](docs/screenshots/spielen.png) |
| **Gedankenexperiment:** Bei schweren Puzzles zeigt der Solver die ganze Kette bis zum Widerspruch. | **Selbst lösen:** mit Rückgängig, Prüfen und Hinweisen, die den nächsten logischen Schritt zeigen. |
| ![Gegeneinander](docs/screenshots/party-runde.png) | ![Bibliothek](docs/screenshots/bibliothek.png) |
| **Zusammen spielen:** gegeneinander auf eigenen Brettern oder miteinander auf einem. | **Bibliothek:** besonders schwere Puzzles, an denen die einfachen Regeln scheitern. |

Im Terminal sieht ein Lösungsweg so aus:

```
$ starbattle generate -n 8 -seed 42 | starbattle solve -runner sync -verbose
1. In Farbe C ist nur noch das Feld (5,3) frei → Stern
2. In Farbe D ist nur noch das Feld (0,2) frei → Stern
3. Spalte 1 hat nur noch freie Felder der Farbe B → alle anderen Felder der Farbe B X
    X: (3,0) (3,4) (4,0) (4,5) (6,0) (7,0)
4. Farbe H liegt nur noch in Zeile 7 → alle anderen Felder der Zeile 7 X
    X: (7,1) (7,6) (7,7)
…
7. Die markierten Felder berühren jeweils alle freien Felder von Zeile 6 → X
    X: (7,5)
8. In Zeile 7 ist nur noch das Feld (7,4) frei → Stern
…
```

## Herunterladen

Das Programm ist eine einzelne Datei ohne Abhängigkeiten. Bei jedem Push baut GitHub Actions es für alle Plattformen: unter [Actions](../../actions/workflows/ci.yml?query=branch%3Amain+is%3Asuccess) den letzten erfolgreichen Lauf auf `main` öffnen und unten bei „Artifacts“ die passende Datei herunterladen (nur mit GitHub-Konto; GitHub löscht Artefakte nach 90 Tagen).

<details>
<summary>Welche Datei für welches System?</summary>

| Datei | für |
|---|---|
| `starbattle-linux-amd64.tar.gz` | Linux auf PC/Server (x86-64) |
| `starbattle-linux-arm64.tar.gz` | Raspberry Pi 3/4/5 mit 64-Bit-OS (Standard bei aktuellem Raspberry Pi OS) |
| `starbattle-linux-armv7.tar.gz` | Raspberry Pi 2/3/4 mit 32-Bit-OS |
| `starbattle-macos-arm64.tar.gz` | Mac mit Apple-Chip |
| `starbattle-windows-amd64.zip` | Windows |

Für andere Systeme (z. B. Mac mit Intel-Chip) einfach [selbst bauen](docs/development.md#bauen-und-testen).

</details>

Dann:

1. Archiv entpacken (`tar xzf …` bzw. unter Windows per Doppelklick).
2. `./starbattle serve` starten. Auf dem Mac ist das Programm nicht signiert; einmal `xattr -d com.apple.quarantine starbattle` lässt es starten.
3. `http://localhost:6369/` im Browser öffnen.

Zum Hosten auf einem Raspberry Pi gibt es [deploy/deploy.sh](deploy/README.md), das passend baut und als Dienst installiert.

## Online spielen

Eine Instanz läuft unter **[meow.ninasworkshop.de](https://meow.ninasworkshop.de)** auf meinem Raspberry Pi 3B zu Hause. Sie ist **nur über IPv6** erreichbar und mit Basic Auth geschützt: Wer mich kennt, kann gerne nach einem Zugang fragen, alle anderen können das Ganze ebenso gerne selbst hosten.

## Weiterlesen

| | |
|---|---|
| [Spielen im Browser](docs/playing.md) | Lösungsweg, Bibliothek, Partys, Einstellungen |
| [Befehle](docs/commands.md) | Puzzle-Format, `generate`, `show`, `solve`, `html`, `serve` mit allen Optionen |
| [Entwicklung](docs/development.md) | Bauen und testen, Aufbau des Codes, die Werkzeuge `design` und `hard` |
| [Symbole](symbols/README.md) | eigene Symbole statt ★ hinzufügen |
| [Raspberry Pi](deploy/README.md) | Installation als Dienst |
| [PLAN.md](PLAN.md) | Regeln, Messungen und Entscheidungen |

## Lizenz

[MIT](LICENSE): Alles ist erlaubt, auch kommerziell und in veränderter Form, solange der Copyright-Hinweis und der Lizenztext erhalten bleiben.
