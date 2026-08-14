package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsdgen"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("xsdgen", flag.ContinueOnError)
	flags.SetOutput(stderr)

	schemaDirectory := flags.String(
		"schema-dir",
		"schema/musicxml-4.0",
		"directory containing the XSD set",
	)
	schemaPath := flags.String(
		"schema",
		"musicxml.xsd",
		"root XSD path within schema-dir",
	)
	catalogPath := flags.String(
		"catalog",
		"catalog.xml",
		"XML catalog path within schema-dir",
	)
	packageName := flags.String(
		"package",
		"musicxml",
		"generated Go package name",
	)
	kind := flags.String(
		"kind",
		"simple",
		"generated declaration kind: simple, complex, element, document, or validation",
	)
	validationName := flags.String(
		"validation-name",
		"",
		"Go variable name for a generated validation schema",
	)
	elementNamespace := flags.String(
		"element-namespace",
		"",
		"namespace of selected global elements",
	)
	var elementNames stringListFlag
	flags.Var(
		&elementNames,
		"element",
		"global element local name; may be repeated",
	)
	elementGoName := flags.String(
		"element-go-name",
		"",
		"Go type name for a document root element",
	)
	var typeNames nameMappingFlag
	flags.Var(
		&typeNames,
		"type-name",
		"XSD local name and Go type name as xsd=Go; may be repeated",
	)
	var externalTypes stringListFlag
	flags.Var(
		&externalTypes,
		"external-type",
		"XSD type supplied by the destination package; may be repeated",
	)
	var orderedTypes stringListFlag
	flags.Var(
		&orderedTypes,
		"ordered-type",
		"XSD complex type represented as ordered content; may be repeated",
	)
	outputPath := flags.String(
		"output",
		"zz_generated_simple.go",
		"generated Go file",
	)

	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf(
			"xsdgen: unexpected arguments: %v",
			flags.Args(),
		)
	}
	if *outputPath == "" {
		return errors.New("xsdgen: empty output path")
	}

	set, err := xsdgen.Load(
		os.DirFS(*schemaDirectory),
		*schemaPath,
		*catalogPath,
	)
	if err != nil {
		return fmt.Errorf("xsdgen: load schema set: %w", err)
	}

	index, err := xsdgen.NewIndex(set)
	if err != nil {
		return fmt.Errorf("xsdgen: index schema set: %w", err)
	}

	var source []byte
	switch *kind {
	case "simple":
		source, err = xsdgen.GenerateSimpleTypes(index, *packageName)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: generate simple types: %w",
				err,
			)
		}

	case "complex":
		ordered := make([]xsdgen.QName, len(orderedTypes))
		for index, name := range orderedTypes {
			ordered[index] = xsdgen.QName{
				Namespace: *elementNamespace,
				Local:     name,
			}
		}

		source, err = xsdgen.GenerateComplexTypesWithOptions(
			index,
			*packageName,
			xsdgen.ComplexGenerationOptions{
				OrderedContentTypes: ordered,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: generate complex types: %w",
				err,
			)
		}

	case "element":
		elements := make([]xsdgen.QName, len(elementNames))
		for index, name := range elementNames {
			elements[index] = xsdgen.QName{
				Namespace: *elementNamespace,
				Local:     name,
			}
		}

		source, err = xsdgen.GenerateElements(
			index,
			*packageName,
			elements...,
		)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: generate elements: %w",
				err,
			)
		}

	case "document":
		if len(elementNames) != 1 {
			return fmt.Errorf(
				"xsdgen: document generation requires exactly one element",
			)
		}
		if *elementGoName == "" {
			return errors.New(
				"xsdgen: document generation requires element-go-name",
			)
		}

		overrides := make(
			[]xsdgen.TypeNameOverride,
			len(typeNames),
		)
		for index, name := range typeNames {
			overrides[index] = xsdgen.TypeNameOverride{
				Name: xsdgen.QName{
					Namespace: *elementNamespace,
					Local:     name.XSDName,
				},
				GoName: name.GoName,
			}
		}

		external := make([]xsdgen.QName, len(externalTypes))
		for index, name := range externalTypes {
			external[index] = xsdgen.QName{
				Namespace: *elementNamespace,
				Local:     name,
			}
		}

		source, err = xsdgen.GenerateDocument(
			index,
			*packageName,
			xsdgen.DocumentGenerationOptions{
				Element: xsdgen.QName{
					Namespace: *elementNamespace,
					Local:     elementNames[0],
				},
				GoName:        *elementGoName,
				TypeNames:     overrides,
				ExternalTypes: external,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: generate document: %w",
				err,
			)
		}

	case "validation":
		if *validationName == "" {
			return errors.New(
				"xsdgen: validation generation requires validation-name",
			)
		}

		source, err = xsdgen.GenerateValidationSchema(
			index,
			*packageName,
			*validationName,
		)
		if err != nil {
			return fmt.Errorf(
				"xsdgen: generate validation schema: %w",
				err,
			)
		}

	default:
		return fmt.Errorf(
			"xsdgen: unsupported declaration kind %q",
			*kind,
		)
	}

	if err := writeFileAtomically(*outputPath, source, 0o644); err != nil {
		return fmt.Errorf(
			"xsdgen: write %q: %w",
			*outputPath,
			err,
		)
	}

	return nil
}

func writeFileAtomically(
	path string,
	content []byte,
	mode fs.FileMode,
) error {
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(
		directory,
		"."+filepath.Base(path)+".*",
	)
	if err != nil {
		return err
	}

	temporaryPath := file.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}

	removeTemporary = false
	return nil
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *stringListFlag) Set(value string) error {
	if value == "" {
		return errors.New("xsdgen: empty element name")
	}

	*f = append(*f, value)
	return nil
}

type nameMapping struct {
	XSDName string
	GoName  string
}

type nameMappingFlag []nameMapping

func (f *nameMappingFlag) String() string {
	return fmt.Sprint([]nameMapping(*f))
}

func (f *nameMappingFlag) Set(value string) error {
	xsdName, goName, found := strings.Cut(value, "=")
	if !found || xsdName == "" || goName == "" {
		return fmt.Errorf(
			"xsdgen: invalid type name mapping %q; want xsd=Go",
			value,
		)
	}

	*f = append(*f, nameMapping{
		XSDName: xsdName,
		GoName:  goName,
	})

	return nil
}
