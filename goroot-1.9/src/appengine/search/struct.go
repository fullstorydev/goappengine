// Copyright 2015 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package search

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ErrFacetMismatch is returned when a facet is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct. StructType is the type of the struct
// pointed to by the destination argument passed to Iterator.Next.
type ErrFacetMismatch struct {
	StructType reflect.Type
	FacetName  string
	Reason     string
}

func (e *ErrFacetMismatch) Error() string {
	return fmt.Sprintf("search: cannot load facet %q into a %q: %s",
		e.FacetName, e.StructType, e.Reason)
}

// structCodec defines how to convert a given struct to/from a search document.
type structCodec struct {
	// byIndex returns the struct tag for the i'th struct field.
	byIndex []structTag

	// fieldByName returns the index of the struct field for the given field name.
	fieldByName map[string]int

	// facetByName returns the index of the struct field for the given facet name,
	facetByName map[string]int
}

// structTag holds a structured version of each struct field's parsed tag.
type structTag struct {
	name  string
	facet bool
}

var (
	codecsMu sync.RWMutex
	codecs   = map[reflect.Type]*structCodec{}
)

func loadCodec(t reflect.Type) (*structCodec, error) {
	codecsMu.RLock()
	codec, ok := codecs[t]
	codecsMu.RUnlock()
	if ok {
		return codec, nil
	}

	codecsMu.Lock()
	defer codecsMu.Unlock()
	if codec, ok := codecs[t]; ok {
		return codec, nil
	}

	codec = &structCodec{
		fieldByName: make(map[string]int),
		facetByName: make(map[string]int),
	}

	for i, I := 0, t.NumField(); i < I; i++ {
		f := t.Field(i)
		name, opts := f.Tag.Get("search"), ""
		if i := strings.Index(name, ","); i != -1 {
			name, opts = name[:i], name[i+1:]
		}
		// TODO: Support name=="-" as per datastore.
		if name == "" {
			name = f.Name
		} else if !validFieldName(name) {
			return nil, fmt.Errorf("search: struct tag has invalid field name: %q", name)
		}
		facet := opts == "facet"
		codec.byIndex = append(codec.byIndex, structTag{name: name, facet: facet})
		if facet {
			codec.facetByName[name] = i
		} else {
			codec.fieldByName[name] = i
		}
	}

	codecs[t] = codec
	return codec, nil
}

// structFMLS adapts a struct to be a FieldMetadataLoadSaver.
type structFMLS struct {
	v     reflect.Value
	codec *structCodec
}

func (s structFMLS) Load(fields []Field, meta *DocumentMetadata) error {
	for _, field := range fields {
		i, ok := s.codec.fieldByName[field.Name]
		if !ok {
			// Ideally we would return an error, as per datastore, but for
			// backwards-compatibility we silently ignore these fields.
			continue
		}
		f := s.v.Field(i)
		if !f.CanSet() {
			// As above.
			continue
		}
		v := reflect.ValueOf(field.Value)
		if ft, vt := f.Type(), v.Type(); ft != vt {
			return fmt.Errorf("search: type mismatch: %v for %v data", ft, vt)
		}
		f.Set(v)
	}
	if meta == nil {
		return nil
	}
	var err error
	for _, facet := range meta.Facets {
		i, ok := s.codec.facetByName[facet.Name]
		if !ok {
			// Note the error, but keep going.
			if err == nil {
				err = &ErrFacetMismatch{
					StructType: s.v.Type(),
					FacetName:  facet.Name,
					Reason:     "no matching field found",
				}
			}
			continue
		}
		f := s.v.Field(i)
		if !f.CanSet() {
			// Note the error, but keep going.
			if err == nil {
				err = &ErrFacetMismatch{
					StructType: s.v.Type(),
					FacetName:  facet.Name,
					Reason:     "unable to set unexported field of struct",
				}
			}
			continue
		}
		v := reflect.ValueOf(facet.Value)
		if ft, vt := f.Type(), v.Type(); ft != vt {
			if err == nil {
				err = &ErrFacetMismatch{
					StructType: s.v.Type(),
					FacetName:  facet.Name,
					Reason:     fmt.Sprintf("type mismatch: %v for %d data", ft, vt),
				}
				continue
			}
		}
		f.Set(v)
	}
	return err
}

func (s structFMLS) Save() ([]Field, *DocumentMetadata, error) {
	fields := make([]Field, 0, len(s.codec.fieldByName))
	var facets []Facet
	for i, tag := range s.codec.byIndex {
		f := s.v.Field(i)
		if !f.CanSet() {
			continue
		}
		if tag.facet {
			facets = append(facets, Facet{Name: tag.name, Value: f.Interface()})
		} else {
			fields = append(fields, Field{Name: tag.name, Value: f.Interface()})
		}
	}
	return fields, &DocumentMetadata{Facets: facets}, nil
}

// newStructFMLS returns a FieldMetadataLoadSaver for the struct pointer p.
func newStructFMLS(p interface{}) (FieldMetadataLoadSaver, error) {
	v := reflect.ValueOf(p)
	if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return nil, ErrInvalidDocumentType
	}
	codec, err := loadCodec(v.Elem().Type())
	if err != nil {
		return nil, err
	}
	return structFMLS{v.Elem(), codec}, nil
}

func loadStructWithMeta(dst interface{}, f []Field, meta *DocumentMetadata) error {
	x, err := newStructFMLS(dst)
	if err != nil {
		return err
	}
	return x.Load(f, meta)
}

func saveStructWithMeta(src interface{}) ([]Field, *DocumentMetadata, error) {
	x, err := newStructFMLS(src)
	if err != nil {
		return nil, nil, err
	}
	return x.Save()
}

// LoadStruct loads the fields from f to dst. dst must be a struct pointer.
func LoadStruct(dst interface{}, f []Field) error {
	return loadStructWithMeta(dst, f, nil)
}

// SaveStruct returns the fields from src as a slice of Field.
// src must be a struct pointer.
func SaveStruct(src interface{}) ([]Field, error) {
	f, _, err := saveStructWithMeta(src)
	return f, err
}
