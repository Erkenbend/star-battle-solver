// Package collection is the format of the project's puzzle collections: the
// hard puzzles in testdata/hard and the motifs in designs/. The web library
// serves them; the developer tools `starbattle hard` and `starbattle design`
// add to them.
//
// A collection is a directory with
//
//   - puzzle files in the puzzle text format, named nNN_STEM.txt (see
//     FileName), with header lines on top that the puzzle parser skips as
//     comments (see RatingKey, NameKey);
//   - an index (Index) that lists the files in library order.
//
// The index is append-only: the library numbers the puzzles of each size in
// index order ("10-3" is the third 10×10), and shared results use those
// names. Links and the browser's storage use ID instead, which depends on the
// regions only. Files are created, never overwritten (Create).
package collection

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"starbattle/internal/puzzle"
)

// Index is the file of a collection that lists its puzzles in order: one
// file name per line, '#' starts a comment, empty lines are skipped. New
// puzzles are only ever appended.
const Index = "bibliothek.idx"

// FileName is the name of a puzzle file of size n: "n08_herz.txt" for stem
// "herz". Hard puzzles use the seed as stem ("s453"), motifs their name.
func FileName(n int, stem string) string { return fmt.Sprintf("n%02d_%s.txt", n, stem) }

// stem returns the stem of a puzzle file name (see FileName), or the base
// name without extension if it does not follow the scheme.
func stem(file string) string {
	base := strings.TrimSuffix(path.Base(file), path.Ext(file))
	if len(base) > 4 && base[0] == 'n' && base[3] == '_' && isDigit(base[1]) && isDigit(base[2]) {
		return base[4:]
	}
	return base
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// ID identifies a puzzle by its regions alone: the regions renumbered in
// order of first appearance (row by row), hashed. Renaming the letters or the
// file keeps the ID.
func ID(p *puzzle.Puzzle) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:", p.N)
	renum := map[int]int{}
	for _, row := range p.Regions {
		for _, reg := range row {
			k, ok := renum[reg]
			if !ok {
				k = len(renum)
				renum[reg] = k
			}
			fmt.Fprintf(h, "%d,", k)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

// ReadIndex reads an index file of fsys (Index, or another file in the same
// format such as testdata/hard/sample.idx).
func ReadIndex(fsys fs.FS, name string) ([]string, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, sc.Err()
}

// Entry is one puzzle of a collection.
type Entry struct {
	File   string // file name, as in the index
	Text   []byte // the file's contents, with its header lines
	Puzzle *puzzle.Puzzle
	ID     string // see ID
}

// Load reads the collection in dir of fsys, in index order. It fails if the
// index lists a file twice, a file is missing or unreadable, or two files
// hold the same puzzle (same ID).
func Load(fsys fs.FS, dir string) ([]Entry, error) {
	names, err := ReadIndex(fsys, path.Join(dir, Index))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	byFile := map[string]bool{}
	byID := map[string]string{}
	for _, name := range names {
		if byFile[name] {
			return nil, fmt.Errorf("%s: %s doppelt", Index, name)
		}
		byFile[name] = true
		text, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, err
		}
		p, err := puzzle.Parse(bytes.NewReader(text))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		id := ID(p)
		if prev, ok := byID[id]; ok {
			return nil, fmt.Errorf("%s: dasselbe Puzzle wie %s", name, prev)
		}
		byID[id] = name
		entries = append(entries, Entry{File: name, Text: text, Puzzle: p, ID: id})
	}
	return entries, nil
}

// Find names the file of the collection in dir of fsys that holds p (same
// ID); "" if none or if there is no index yet. Files that cannot be read are
// skipped: Load reports them.
func Find(fsys fs.FS, dir string, p *puzzle.Puzzle) (string, error) {
	names, err := ReadIndex(fsys, path.Join(dir, Index))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	id := ID(p)
	for _, name := range names {
		text, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			continue
		}
		if q, err := puzzle.Parse(bytes.NewReader(text)); err == nil && ID(q) == id {
			return name, nil
		}
	}
	return "", nil
}

// WithStem names a puzzle file in the directory dir with the given stem, of
// any size (see FileName); "" if there is none.
func WithStem(dir, stem string) (string, error) {
	same, err := filepath.Glob(filepath.Join(dir, "n[0-9][0-9]_"+stem+".txt"))
	if err != nil || len(same) == 0 {
		return "", err
	}
	return filepath.Base(same[0]), nil
}

// Exists reports whether the directory dir has a file of this name.
func Exists(dir, file string) bool {
	_, err := os.Stat(filepath.Join(dir, file))
	return !errors.Is(err, fs.ErrNotExist)
}

// Create writes a new puzzle file to the directory dir. An existing file is
// never overwritten: the error then matches os.ErrExist.
func Create(dir, file, text string) error {
	f, err := os.OpenFile(filepath.Join(dir, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, text)
	return errors.Join(err, f.Close())
}

// AppendIndex appends file to the index in the directory dir. A new index
// starts with header, comment lines that say what the collection is.
func AppendIndex(dir, file, header string) error {
	f, err := os.OpenFile(filepath.Join(dir, Index), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	var b strings.Builder
	if st, err := f.Stat(); err == nil && st.Size() == 0 {
		b.WriteString(header)
	}
	b.WriteString(file + "\n")
	_, err = io.WriteString(f, b.String())
	return errors.Join(err, f.Close())
}
