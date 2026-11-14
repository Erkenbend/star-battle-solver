package collection

import (
	"strings"
)

// Header lines of puzzle files. They start with '#', so the puzzle parser
// skips them as comments; each key is at most once in a file.
const (
	// RatingKey starts the line of a hard puzzle that gives its difficulty
	// for players, 1 to 3 stars, maybe followed by the reason:
	// "# rating: 2 (4 lookahead steps, longest 5)". `starbattle hardsearch` writes
	// it (see hardsearch.Rating); motifs have none.
	RatingKey = "# rating:"
	// NameKey starts the line of a motif that gives its name in the library:
	// "# name: Trauriger Smiley". The designer writes it as the first line.
	NameKey = "# name:"
)

// HeaderLine is the header line for key with value, newline included.
func HeaderLine(key, value string) string { return key + " " + value + "\n" }

// header returns the value of key's line in text, if it has one.
func header(text []byte, key string) (string, bool) {
	for line := range strings.Lines(string(text)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key); ok {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// SetHeader returns text with key's line set to value: replaced where it is,
// or else added after the comment lines at the top.
func SetHeader(text, key, value string) string {
	line := HeaderLine(key, value)
	lines := strings.SplitAfter(text, "\n")
	at := 0
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, key) {
			lines[i] = line
			return strings.Join(lines, "")
		}
		if at == i && strings.HasPrefix(t, "#") {
			at = i + 1
		}
	}
	return strings.Join(lines[:at], "") + line + strings.Join(lines[at:], "")
}

// Rating is the rating in a puzzle file's text (see RatingKey), 0 if it has
// none or it is not 1 to 3.
func Rating(text []byte) int {
	v, _ := header(text, RatingKey)
	if len(v) > 0 && '1' <= v[0] && v[0] <= '3' && (len(v) == 1 || v[1] == ' ') {
		return int(v[0] - '0')
	}
	return 0
}

// Name is the name of the motif in file (its base name) with contents text:
// the NameKey line, or else NameFromFile.
func Name(file string, text []byte) string {
	if v, _ := header(text, NameKey); v != "" {
		return v
	}
	return NameFromFile(file)
}

// NameFromFile makes a name from a puzzle file name: the stem (see FileName)
// with '_' and '-' as spaces, each word capitalized ("n08_sad_smiley.txt":
// "Sad Smiley").
func NameFromFile(file string) string {
	words := strings.FieldsFunc(stem(file), func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
