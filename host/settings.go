// Copyright 2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package host

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/beevik/prefixtree/v2"
)

type settings struct {
	HexMode         bool   `doc:"hexadecimal input mode"`
	CompactMode     bool   `doc:"compact disassembly output"`
	MemDumpBytes    int    `doc:"default number of memory bytes to dump"`
	DisasmLines     int    `doc:"default number of lines to disassemble"`
	SourceLines     int    `doc:"default number of source lines to display"`
	MaxStepLines    int    `doc:"max lines to disassemble when stepping"`
	NextDisasmAddr  uint16 `doc:"address of next disassembly"`
	NextSourceAddr  uint16 `doc:"address of next source line display"`
	NextMemDumpAddr uint16 `doc:"address of next memory dump"`
}

func newSettings() *settings {
	return &settings{
		HexMode:         false,
		CompactMode:     false,
		MemDumpBytes:    64,
		DisasmLines:     10,
		SourceLines:     10,
		MaxStepLines:    20,
		NextDisasmAddr:  0,
		NextMemDumpAddr: 0,
	}
}

type settingsField struct {
	name  string
	index int
	kind  reflect.Kind
	typ   reflect.Type
	doc   string
}

var (
	settingsTree   = prefixtree.New[*settingsField]()
	settingsFields []settingsField
)

func init() {
	settingsType := reflect.TypeOf(settings{})
	settingsFields = make([]settingsField, settingsType.NumField())
	for i := 0; i < len(settingsFields); i++ {
		f := settingsType.Field(i)
		doc, _ := f.Tag.Lookup("doc")
		settingsFields[i] = settingsField{
			name:  f.Name,
			index: i,
			kind:  f.Type.Kind(),
			typ:   f.Type,
			doc:   doc,
		}
		settingsTree.Add(strings.ToLower(f.Name), &settingsFields[i])
	}
}

func (s *settings) Display(w io.Writer) {
	value := reflect.ValueOf(s).Elem()
	for i, f := range settingsFields {
		v := value.Field(i)
		var s string
		switch f.kind {
		case reflect.String:
			s = fmt.Sprintf("    %-16s \"%s\"", f.name, v.String())
		case reflect.Uint8:
			s = fmt.Sprintf("    %-16s $%02X", f.name, uint8(v.Uint()))
		case reflect.Uint16:
			s = fmt.Sprintf("    %-16s $%04X", f.name, uint16(v.Uint()))
		default:
			s = fmt.Sprintf("    %-16s %v", f.name, v)
		}
		fmt.Fprintf(w, "%-28s (%s)\n", s, f.doc)
	}
}

func (s *settings) Kind(key string) reflect.Kind {
	f, err := settingsTree.FindValue(strings.ToLower(key))
	if err != nil {
		return reflect.Invalid
	}
	return f.kind
}

func (s *settings) Set(key string, value any) error {
	f, err := settingsTree.FindValue(strings.ToLower(key))
	if err != nil {
		return err
	}

	vIn := reflect.ValueOf(value)
	if (f.kind == reflect.String && vIn.Type().Kind() != reflect.String) ||
		(f.kind != reflect.String && vIn.Type().Kind() == reflect.String) ||
		!vIn.Type().ConvertibleTo(f.typ) {
		return errors.New("invalid type")
	}
	vInConverted := vIn.Convert(f.typ)

	vOut := reflect.ValueOf(s).Elem().Field(f.index).Addr().Elem()
	vOut.Set(vInConverted)

	return nil
}
