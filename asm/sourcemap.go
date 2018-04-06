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

// Encoding flags
const (
	continued        byte = 1 << 7
	negative         byte = 1 << 6
	fileIndexChanged byte = 1 << 5
)

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
	if lineCount > 0 {
		var line SourceLine
		for i := 0; i < lineCount; i++ {
			var nn int
			line, nn, err = decodeSourceLine(rr, line)
			n += int64(nn)
			if err != nil {
				return n, err
			}
			s.Lines = append(s.Lines, line)
		}
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

// WriteTo writes the contents of an assembly source map to an output
// stream.
func (s *SourceMap) WriteTo(w io.Writer) (n int64, err error) {
	fileCount := uint16(len(s.Files))
	lineCount := uint32(len(s.Lines))
	exportCount := uint32(len(s.Exports))

	ww := bufio.NewWriter(w)

	var hdr [16]byte
	copy(hdr[:], []byte(sourceMapSignature))
	hdr[4] = versionMajor
	hdr[5] = versionMinor
	binary.LittleEndian.PutUint16(hdr[6:8], fileCount)
	binary.LittleEndian.PutUint32(hdr[8:12], lineCount)
	binary.LittleEndian.PutUint32(hdr[12:16], exportCount)
	nn, err := ww.Write(hdr[:])
	n += int64(nn)
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

	if len(s.Lines) > 0 {
		var prev SourceLine
		for _, line := range s.Lines {
			nn, err = encodeSourceLine(ww, prev, line)
			n += int64(nn)
			if err != nil {
				return n, err
			}
			prev = line
		}
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

		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], e.Addr)
		nn, err = ww.Write(b[:])
		n += int64(nn)
		if err != nil {
			return n, err
		}
	}

	ww.Flush()

	return n, nil
}

func decodeSourceLine(r *bufio.Reader, prev SourceLine) (line SourceLine, n int, err error) {
	var nn int
	da, nn, err := decode67(r)
	n += nn
	if err != nil {
		return line, n, err
	}

	var dl int
	var f bool
	dl, f, nn, err = decode57(r)
	n += nn
	if err != nil {
		return line, n, err
	}

	var df int
	if f {
		df, nn, err = decode67(r)
		n += nn
		if err != nil {
			return line, n, err
		}
	}

	line.Address = prev.Address + da
	line.FileIndex = prev.FileIndex + df
	line.Line = prev.Line + dl
	return line, n, nil
}

func decode7(r *bufio.Reader) (value int, n int, err error) {
	var shift uint = 0
	for {
		var b byte
		b, err = r.ReadByte()
		if err != nil {
			return 0, n, err
		}
		n++

		value |= (int(b&0x7f) << shift)
		shift += 7

		if (b & continued) == 0 {
			break
		}
	}
	return value, n, nil
}

func decode57(r *bufio.Reader) (value int, f bool, n int, err error) {
	var b byte
	b, err = r.ReadByte()
	if err != nil {
		return 0, f, n, err
	}
	n++

	value = int(b & 0x1f)

	f = (b & fileIndexChanged) != 0

	neg := (b & negative) != 0

	if (b & continued) != 0 {
		var vl, nn int
		vl, nn, err = decode7(r)
		n += nn
		if err != nil {
			return 0, f, n, err
		}

		value |= vl << 5
	}

	if neg {
		value = -value
	}

	return value, f, n, nil
}

func decode67(r *bufio.Reader) (value int, n int, err error) {
	var b byte
	b, err = r.ReadByte()
	if err != nil {
		return 0, n, err
	}
	n++

	value = int(b & 0x3f)

	neg := (b & negative) != 0

	if (b & continued) != 0 {
		var vl, nn int
		vl, nn, err = decode7(r)
		n += nn
		if err != nil {
			return 0, n, err
		}

		value |= vl << 6
	}

	if neg {
		value = -value
	}

	return value, n, nil
}

func encodeSourceLine(w *bufio.Writer, l0, l1 SourceLine) (n int, err error) {
	da := l1.Address - l0.Address
	df := l1.FileIndex - l0.FileIndex
	dl := l1.Line - l0.Line

	nn, err := encode67(w, da)
	n += nn
	if err != nil {
		return n, err
	}

	nn, err = encode57(w, dl, df != 0)
	n += nn
	if err != nil {
		return n, err
	}

	if df != 0 {
		nn, err = encode67(w, df)
		n += nn
	}
	return n, err
}

func encode7(w *bufio.Writer, v int) (n int, err error) {
	for v != 0 {
		var b byte
		if v >= 0x7f {
			b |= continued
		}
		b |= (byte(v) & 0x7f)

		err = w.WriteByte(b)
		if err != nil {
			return n, err
		}
		n++

		v >>= 7
	}
	return n, nil
}

func encode57(w *bufio.Writer, v int, f bool) (n int, err error) {
	var b byte
	if f {
		b |= fileIndexChanged
	}
	if b < 0 {
		b |= negative
		v = -v
	}
	if v >= 0x20 {
		b |= continued
	}

	b |= (byte(v) & 0x1f)
	err = w.WriteByte(b)
	if err != nil {
		return n, err
	}
	n++
	v >>= 5

	nn, err := encode7(w, v)
	n += nn
	return n, err
}

func encode67(w *bufio.Writer, v int) (n int, err error) {
	var b byte
	if v < 0 {
		b |= negative
		v = -v
	}
	if v >= 0x40 {
		b |= continued
	}

	b |= (byte(v) & 0x3f)
	err = w.WriteByte(b)
	if err != nil {
		return n, err
	}
	n++
	v >>= 6

	nn, err := encode7(w, v)
	n += nn
	return n, err
}
