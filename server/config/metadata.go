package config

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
)

const (
	typeDuration = "duration"
	typePlugin   = "plugin"
	typePassword = "password"
	typeText     = "text"
)

// description is the Fieldmetadata container describing a single type
type description struct {
	Type   string          `json:"type"`
	Label  string          `json:"label"`
	Fields []FieldMetadata `json:"fields"`
}

// FieldMetadata is the meta data format for the type description
type FieldMetadata struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Length   int             `json:"length,omitempty"`
	SubType  string          `json:"subtype,omitempty"`
	Required bool            `json:"required,omitempty"`
	Hidden   bool            `json:"hidden,omitempty"`
	Label    string          `json:"label,omitempty"`
	Enum     []interface{}   `json:"enum,omitempty"`
	Default  interface{}     `json:"default,omitempty"`
	Children []FieldMetadata `json:"children,omitempty"`
}

// tagKey returns tag key's value or key name if value is empty
func tagKey(f *structs.Field, tag, key string) string {
	keyvals := strings.Split(f.Tag(tag), ",")

	for _, kv := range keyvals {
		if splits := strings.SplitN(kv, "=", 2); splits[0] == key {
			if len(splits) > 1 {
				return splits[1]
			}

			return key
		}
	}

	return ""
}

// hasTagKey returns true if tag key exists; the key's value is not checked
func hasTagKey(f *structs.Field, tag, key string) bool {
	val := tagKey(f, tag, key)
	return val != ""
}

// enum converts list of strings to enum values
func enum(list []string) (enum []interface{}) {
	for _, v := range list {
		enum = append(enum, strings.TrimSpace(v))
	}
	return enum
}

// label is the exported field label
func label(f *structs.Field) string {
	val := tagKey(f, "ui", "de")
	if val == "" {
		val = translate(f.Name())
	}
	if val == "" {
		val = f.Name()
	}

	return val
}

// kind is the exported data type
func kind(f *structs.Field) string {
	val := f.Value()

	switch {
	case reflect.TypeOf(val) == reflect.TypeOf(time.Duration(0)):
		return typeDuration

	case f.Kind() == reflect.Struct && reflect.TypeOf(val).String() == "provider.Config":
		return typePlugin

	case hasTagKey(f, "ui", "mask"):
		return typePassword

	case hasTagKey(f, "ui", "text"):
		return typeText

	default:
		return f.Kind().String()
	}
}

// value kind is the exported default value
func value(f *structs.Field) interface{} {
	switch val := f.Value().(type) {
	case time.Duration:
		return val / time.Second
	default:
		return f.Value()
	}
}

func prependType(typ string, conf []FieldMetadata) []FieldMetadata {
	typeDef := struct {
		Type string `validate:"required" ui:",hide"`
	}{
		Type: typ,
	}

	return append(Annotate(typeDef), conf...)
}

// Annotate adds meta data to given configuration structure
func Annotate(s interface{}) (ds []FieldMetadata) {
	for _, f := range structs.Fields(s) {
		if !f.IsExported() {
			continue
		}

		// embedded fields
		if f.Kind() == reflect.Struct && f.IsEmbedded() {
			dd := Annotate(f.Value())
			ds = append(ds, dd...)
			continue
		}

		// normal fields including structs
		d := FieldMetadata{
			Name:     f.Name(),
			Type:     kind(f),
			Required: hasTagKey(f, "validate", "required"),
			Hidden:   hasTagKey(f, "ui", "hide"),
		}

		if !d.Hidden {
			// label
			d.Label = label(f)

			// enums
			if oneof := tagKey(f, "validate", "oneof"); oneof != "" {
				d.Enum = enum(strings.Split(oneof, " "))
			}
		}

		// add default values if not masked
		if !f.IsZero() && d.Type != typePassword {
			d.Default = value(f)
		}

		switch f.Kind() {
		case reflect.Interface, reflect.Func:
			continue

		case reflect.Ptr:
			t := reflect.PtrTo(reflect.TypeOf(f.Value()))
			if t.String() != "provider.Config" {
				continue
			}
			d.Type = typePlugin

		case reflect.Slice:
			t := reflect.TypeOf(f.Value()).Elem()
			if t.String() != "provider.Config" {
				continue
			}
			d.SubType = typePlugin
			if hasTagKey(f, "validate", "lte") {
				var err error
				if d.Length, err = strconv.Atoi(tagKey(f, "validate", "lte")); err != nil {
					panic(err)
				}
			}

		case reflect.Struct:
			if d.Type != typePlugin {
				d.Default = nil // no default for structs
				d.Children = Annotate(f.Value())
			}
		}

		ds = append(ds, d)
	}

	return ds
}