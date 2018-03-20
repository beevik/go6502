package asm

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"sort"
)

// A SourceMap describes the mapping between source code line numbers and
// assembly code addresses.
type SourceMap struct {
	Files   []string
	Lines   []SourceLine
	Exports []Export
}

// A SourceLine represents a mapping between a machine code address and
// the source code file and line number used to generate it.
type SourceLine struct {
	Address   int // Machine code address
	FileIndex int // Source code file index
	Line      int // Source code line number
}

// Search searches the source map for a mapping with the requested address.
func (s *SourceMap) Search(addr int) (filename string, line int) {
	i := sort.Search(len(s.Lines), func(i int) bool {
		return s.Lines[i].Address >= addr
	})
	if i < len(s.Lines) && s.Lines[i].Address == addr {
		return s.Files[s.Lines[i].FileIndex], s.Lines[i].Line
	}
	return "", -1
}

// ReadFrom reads the contents of an exported source map file.
func (s *SourceMap) ReadFrom(r io.Reader) (n int64, err error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal(b, s)
	if err != nil {
		return 0, err
	}
	return int64(len(b)), nil
}

// WriteTo writes the contents of the source map to an output stream.
func (s *SourceMap) WriteTo(w io.Writer) (n int64, err error) {
	b, err := json.Marshal(*s)
	if err != nil {
		return 0, err
	}

	nn, err := w.Write(b)
	return int64(nn), err
}
