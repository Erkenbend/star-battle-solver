# Symbole

Welches Symbol das Brett statt ★ zeigt, wählt man in den [Einstellungen](../docs/playing.md#einstellungen) aus Vorschlägen, die in [`symbols.json`](symbols.json) stehen und ins Programm eingebaut werden. Die ID `random` ist für „Zufällig“ reserviert.

## Arten

| Art | Beispiel | Eigenschaften |
|---|---|---|
| `svg` | Stern, Herz, Pfote | Pfad in einem 100×100-Feld. Sieht überall gleich aus, nimmt die Sternfarbe an und wird bei Fehlern rot. |
| `glyph` | ✿ | Schriftzeichen, als Text dargestellt (nie als Emoji). Nimmt die Farben an, sieht aber je nach Schrift des Geräts etwas anders aus. |
| `emoji` | 🐱, 🐶 | Emoji aus der Schrift des Geräts. Behält die eigenen Farben und sieht auf jedem System anders aus; fehlt eine Emoji-Schrift, erscheint ein leeres Kästchen. |
| `sprite` | Diamant, Kätzchen, Hündchen, Hüpfstern, Frosch, Küken, Gespenst, Schlafmond | Bilddatei (`.svg`, `.png`, `.webp`, höchstens 256 KiB) in diesem Verzeichnis. Mit `frames` und `fps` ein animiertes Bild: alle Einzelbilder (je 100×100) nebeneinander. Die Animation spielt nur nach dem Setzen eines Sterns etwa drei Sekunden lang, danach bleibt das erste Bild stehen; wünscht das System reduzierte Bewegung, gar nicht. Mit `clips` hat ein Bild mehrere Animationen (siehe unten). |

Was die Farben nicht annimmt, leuchtet bei Fehlern rot. Bilder lädt die Seite vom Server; in einer mit `starbattle html` gespeicherten Datei erscheint stattdessen der Stern, und „Zufällig“ kommt dort nur auf Symbole ohne Bilddatei. GIF und animiertes PNG/WebP sind absichtlich nicht vorgesehen, weil sie sich nicht anhalten lassen.

## Mehrere Animationen

`clips` teilt die Einzelbilder in Abschnitte `{"name": …, "start": …, "frames": …}` (der Name ist nur für Lesende). Beim Setzen eines Sterns spielt einer davon, zufällig gewählt und nach Möglichkeit nicht derselbe wie beim letzten Mal. Danach bleibt Bild 0 stehen, die Ruhepose; sie gehört meist zu keinem Abschnitt. Ohne `clips` sind alle Bilder eine Animation.

## Animierte Bilder erzeugen

Die animierten Bilder entstehen mit den Skripten hier (`cat.py`, `dog.py`, `hop.py`, `frog.py`, `chick.py`, `ghost.py`, `moon.py`), die auch die passenden `frames` und `clips` für `symbols.json` ausgeben:

```sh
python3 symbols/cat.py > symbols/cat.svg
```
