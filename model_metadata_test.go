package musicxml

import (
	"reflect"
	"strings"
	"testing"
)

func TestModelMetadataCoversEveryReachableField(t *testing.T) {
	seen := map[string]bool{}
	var visit func(reflect.Type, string)
	visit = func(typ reflect.Type, name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		d := modelDescriptors[name]
		if d == nil {
			t.Fatalf("missing %s", name)
		}
		if typ.Kind() != reflect.Struct {
			t.Fatalf("%s has %s", name, typ)
		}
		used := map[string]bool{}
		var check func(modelField)
		check = func(f modelField) {
			ft := typ
			for _, i := range f.Index {
				if ft.Kind() != reflect.Struct || i >= ft.NumField() {
					t.Fatalf("%s.%s path mismatch", name, f.Name)
				}
				ft = ft.Field(i).Type
			}
			if used[f.Name] {
				t.Errorf("duplicate %s.%s", name, f.Name)
			}
			used[f.Name] = true
			if f.Repeated {
				if ft.Kind() != reflect.Slice {
					t.Fatalf("%s.%s not slice", name, f.Name)
				}
				ft = ft.Elem()
			}
			if len(f.Variants) > 0 {
				if len(f.Variants) != ft.NumField() {
					t.Errorf("%s choice coverage", name)
				}
				for _, v := range f.Variants {
					sf := ft.Field(v.Index[0])
					if sf.Name != v.Name || sf.Type.Kind() != reflect.Pointer {
						t.Fatalf("%s choice %s mismatch", name, v.Name)
					}
					if v.Child != "" {
						visit(sf.Type.Elem(), v.Child)
					}
				}
			} else {
				if f.Optional {
					if ft.Kind() != reflect.Pointer {
						t.Fatalf("%s.%s not pointer", name, f.Name)
					}
					ft = ft.Elem()
				}
				if f.Child != "" {
					visit(ft, f.Child)
				} else if f.Kind == 0 {
					t.Errorf("%s.%s no kind", name, f.Name)
				}
			}
		}
		if d.Scalar != nil {
			check(*d.Scalar)
		}
		for _, f := range d.Attributes {
			check(f)
		}
		for _, f := range d.Elements {
			check(f)
		}
		var fields func(reflect.Type, string)
		fields = func(v reflect.Type, prefix string) {
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				if f.Name == "XMLName" {
					continue
				}
				if f.Anonymous {
					fields(f.Type, prefix+f.Name+".")
					continue
				}
				if !used[prefix+f.Name] {
					t.Errorf("unmapped %s.%s", name, prefix+f.Name)
				}
			}
		}
		fields(typ, "")
	}
	visit(reflect.TypeFor[ScorePartwise](), "ScorePartwise")
	visit(reflect.TypeFor[ScoreTimewise](), "ScoreTimewise")
	visit(reflect.TypeFor[OpusDocument](), "OpusDocument")
	visit(reflect.TypeFor[EmptyPrintStyleAlign](), "EmptyPrintStyleAlign")
	visit(reflect.TypeFor[FormattedSymbol](), "FormattedSymbol")
	visit(reflect.TypeFor[EmptyPrintStyle](), "EmptyPrintStyle")
	for name := range modelDescriptors {
		if !seen[name] {
			t.Errorf("unreachable descriptor %s", name)
		}
	}
	if modelDescriptors["OpusDocument"].Root.Local != "opus" {
		t.Error("opus root override lost")
	}
	for _, d := range modelDescriptors {
		for _, f := range d.Attributes {
			if strings.Contains(f.Name, "XMLName") {
				t.Error("transport field included")
			}
		}
	}
}
