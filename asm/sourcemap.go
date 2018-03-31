package asm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
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

// ReadFrom reads the contents of an assembly source map.
func (s *SourceMap) ReadFrom(r io.Reader) (n int64, err error) {
	rr := bufio.NewReader(r)

	b := make([]byte, 16)
	nn, err := io.ReadFull(rr, b)
	n += int64(nn)
	if err != nil {
		return n, err
	}

	if len(b) < 16 || bytes.Compare(b[0:4], []byte(sourceMapSignature)) != 0 {
		return n, errors.New("invalid source map format")
	}
	if b[4] != versionMajor || b[5] != versionMinor {
		return n, errors.New("invalid source map version")
	}

	fileCount := int(binary.LittleEndian.Uint16(b[6:8]))
	lineCount := int(binary.LittleEndian.Uint32(b[8:12]))
	exportCount := int(binary.LittleEndian.Uint32(b[12:16]))

	s.Files = make([]string, fileCount)
	for i := 0; i < fileCount; i++ {
		file, err := rr.ReadString(0)
		n += int64(len(file))
		if err != nil {
			return n, err
		}
		s.Files[i] = file[:len(file)-1]
	}

	s.Lines = make([]SourceLine, lineCount)
	for i := 0; i < lineCount; i++ {
		nn, err = io.ReadFull(rr, b[:8])
		n += int64(nn)
		if err != nil {
			return n, err
		}
		s.Lines[i].Address = int(binary.LittleEndian.Uint16(b[0:2]))
		s.Lines[i].FileIndex = int(binary.LittleEndian.Uint16(b[2:4]))
		s.Lines[i].Line = int(binary.LittleEndian.Uint32(b[4:8]))
	}

	s.Exports = make([]Export, exportCount)
	for i := 0; i < exportCount; i++ {
		label, err := rr.ReadString(0)
		n += int64(len(label))
		if err != nil {
			return n, err
		}
		s.Exports[i].Label = label[:len(label)-1]

		nn, err = io.ReadFull(rr, b[:2])
		n += int64(nn)
		if err != nil {
			return n, err
		}
		s.Exports[i].Addr = binary.LittleEndian.Uint16(b[0:2])
	}

	return n, nil
}

// WriteSourceMap writes the contents of an assembly source map to an output
// stream.
func (s *SourceMap) WriteTo(w io.Writer) (n int64, err error) {
	fileCount := uint16(len(s.Files))
	lineCount := uint32(len(s.Lines))
	exportCount := uint32(len(s.Exports))

	ww := bufio.NewWriter(w)

	b := make([]byte, 16)
	copy(b, []byte(sourceMapSignature))
	b[4] = versionMajor
	b[5] = versionMinor
	binary.LittleEndian.PutUint16(b[6:8], fileCount)
	binary.LittleEndian.PutUint32(b[8:12], lineCount)
	binary.LittleEndian.PutUint32(b[12:16], exportCount)
	nn, err := ww.Write(b)
	n += int64(nn)
	ww.Flush()
	if err != nil {
		return n, err
	}

	for _, f := range s.Files {
		nn, err = ww.WriteString(f)
		n += int64(nn)
		if err != nil {
			return n, err
		}
		err = ww.WriteByte(0)
		if err != nil {
			return 0, err
		}
		n++
	}

	for _, l := range s.Lines {
		binary.LittleEndian.PutUint16(b[0:2], uint16(l.Address))
		binary.LittleEndian.PutUint16(b[2:4], uint16(l.FileIndex))
		binary.LittleEndian.PutUint32(b[4:8], uint32(l.Line))
		nn, err = ww.Write(b[0:8])
		n += int64(nn)
		if err != nil {
			return n, err
		}
		ww.Flush()
	}

	for _, e := range s.Exports {
		nn, err = ww.WriteString(e.Label)
		n += int64(nn)
		if err != nil {
			return n, err
		}
		ww.WriteByte(0)
		if err != nil {
			return n, err
		}
		n++

		binary.LittleEndian.PutUint16(b[0:2], e.Addr)
		nn, err = ww.Write(b[0:2])
		n += int64(nn)
		if err != nil {
			return n, err
		}
		ww.Flush()
	}

	return n, nil
}
