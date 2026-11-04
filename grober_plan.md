# Star Battle Solver

=Meowdoku
Fancy Solver mit Go und Channels

Idee: Mehrere Observers, die in parallel laufen und bestimmte Aspekte beobachten

* Line/Column: wenn nur eine Farbe drin ist -> alle andere Kacheln dieser Farbe X
* Multiple Line/Column: wenn exakt N Farben in N -> alle andere Kacheln dieser Farben X
* Form: wenn eine Farbe eine bestimme Form hat (2x1, 3x1, L, Z) -> bestimmte Kacheln drum herum X
* Color: wenn nur in einer Zeile/Spalte anwesend -> alle anderen Kacheln dieser Zeile/Spalte X
* Color Finish: wenn nur eine Kachel frei -> M
* Multiple Color: wenn exakt N in N Zeilen/Spalten anwesend -> alle anderen Kacheln dieser Zeilen/Spalten X
* vlt später komplexere Strategien wenn nötig

Jeder Observer bekommt den kompletten Board-State read-only: es kann nur Informationen hinzugefügt werden. Oder es
bekommt den Layout Initial und muss den Board-State selber konstruieren in dem es auf Updates über einen Channel hört (
ignoriert, was ihn nicht betrifft). Observer exit wenn die Zeile/Spalte/Farbe voll ist. Zuerst den Input Channel leer
hören, dann versuchen M/X zu platzieren bis es nicht mehr geht, dann wieder hören.

Läuft bis zur kompletten Lösung, publiziert M/X auf einen anderen Channel, der das an alle Observers verteilt.

Einzelne observers sollten in Isolation gut testbar sein.

Main Runner: verteilt die M/X Markierungen der einzelnen Observers wieder an alle und beendet wenn final gelöst. Bei M
verteilt er auch X in der passenden Farbe+Zeile+Spalte+Nachbarschaft

Wenn das nicht reicht (später): Hypothesen aufstellen und mit Backtracking Optionen erkunden.
