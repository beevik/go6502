package main

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/beevik/prefixtree"
)

type settings struct {
	DisasmLinesToDisplay int
	MemDumpBytes         int
	StepLinesToDisplay   int
	NextDisasmAddr       uint16
	NextMemDumpAddr      uint16
	HexMode              bool
}

func newSettings() *settings {
	return &settings{
		DisasmLinesToDisplay: 10,
		MemDumpBytes:         64,
		StepLinesToDisplay:   20,
		NextDisasmAddr:       0,
		NextMemDumpAddr:      0,
		HexMode:              false,
	}
}

type settingsField struct {
	name  string
	index int
	kind  reflect.Kind
	typ   reflect.Type
}

var (
	settingsTree   = prefixtree.New()
	settingsFields []settingsField
)

func init() {
	settingsType := reflect.TypeOf(settings{})
	settingsFields = make([]settingsField, settingsType.NumField())
	for i := 0; i < len(settingsFields); i++ {
		f := settingsType.Field(i)
		settingsFields[i] = settingsField{
			name:  f.Name,
			index: i,
			kind:  f.Type.Kind(),
			typ:   f.Type,
		}
		settingsTree.Add(strings.ToLower(f.Name), &settingsFields[i])
	}
}

func (s *settings) Display(w io.Writer) {
	value := reflect.ValueOf(s).Elem()
	for i, f := range settingsFields {
		v := value.Field(i)
		switch f.kind {
		case reflect.String:
			fmt.Fprintf(w, "    %-20s \"%s\"\n", f.name, v.String())
		case reflect.Uint8:
			fmt.Fprintf(w, "    %-20s $%02X\n", f.name, uint8(v.Uint()))
		case reflect.Uint16:
			fmt.Fprintf(w, "    %-20s $%04X\n", f.name, uint16(v.Uint()))
		default:
			fmt.Fprintf(w, "    %-20s %v\n", f.name, v)
		}
	}
}

func (s *settings) Kind(key string) reflect.Kind {
	f, err := settingsTree.Find(strings.ToLower(key))
	if err != nil {
		return reflect.Invalid
	}
	return f.(*settingsField).kind
}

func (s *settings) Set(key string, value interface{}) error {
	ff, err := settingsTree.Find(strings.ToLower(key))
	if err != nil {
		return err
	}
	f := ff.(*settingsField)

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
