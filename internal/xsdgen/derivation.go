package xsdgen

import (
	"errors"
	"fmt"
	"strings"
)

var errInvalidDerivation = errors.New("xsdgen: invalid derivation constraint")

// Keep the bit positions aligned with the generated runtime validationDerivation.
type derivationSet uint8

const (
	derivationExtension derivationSet = 1 << iota
	derivationRestriction
	derivationList
	derivationUnion
	derivationSubstitution

	complexDerivations = derivationExtension | derivationRestriction
	simpleDerivations  = derivationRestriction | derivationList | derivationUnion
	elementBlocks      = complexDerivations | derivationSubstitution
	schemaFinals       = complexDerivations | derivationList | derivationUnion
)

// parseDerivations accepts only XML whitespace; Unicode spacing characters are
// not separators in an XSD list value. #all must be the sole token.
func parseDerivations(value string, allowed derivationSet) (derivationSet, error) {
	tokens := strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == '\t' || r == '\r' || r == '\n' })
	if len(tokens) == 1 && tokens[0] == "#all" {
		return allowed, nil
	}
	var result derivationSet
	for _, token := range tokens {
		var bit derivationSet
		switch token {
		case "extension":
			bit = derivationExtension
		case "restriction":
			bit = derivationRestriction
		case "list":
			bit = derivationList
		case "union":
			bit = derivationUnion
		case "substitution":
			bit = derivationSubstitution
		default:
			return 0, fmt.Errorf("%w: unknown token %q", errInvalidDerivation, token)
		}
		if bit&allowed == 0 {
			return 0, fmt.Errorf("%w: token %q is not permitted here", errInvalidDerivation, token)
		}
		result |= bit
	}
	return result, nil
}

// Explicit empty attributes clear defaults. Irrelevant tokens in schema
// defaults are filtered for each component, while explicit tokens must belong
// to that component's derivation set.
func effectiveDerivations(value *string, fallback string, allowed, defaultAllowed derivationSet) (derivationSet, error) {
	if value != nil {
		return parseDerivations(*value, allowed)
	}
	defaults, err := parseDerivations(fallback, defaultAllowed)
	return defaults & allowed, err
}

func schemaDerivationDefaults(file *SchemaFile) (block, final string) {
	if file != nil && file.Schema != nil {
		return file.Schema.BlockDefault, file.Schema.FinalDefault
	}
	return "", ""
}

func validateSchemaDerivations(schema *Schema) error {
	if _, err := parseDerivations(schema.BlockDefault, elementBlocks); err != nil {
		return fmt.Errorf("blockDefault: %w", err)
	}
	if _, err := parseDerivations(schema.FinalDefault, schemaFinals); err != nil {
		return fmt.Errorf("finalDefault: %w", err)
	}
	return nil
}

func elementDerivations(file *SchemaFile, source *Element) (block, final derivationSet, err error) {
	blockDefault, finalDefault := schemaDerivationDefaults(file)
	block, err = effectiveDerivations(source.Block, blockDefault, elementBlocks, elementBlocks)
	if err != nil {
		return 0, 0, fmt.Errorf("element block: %w", err)
	}
	final, err = effectiveDerivations(source.Final, finalDefault, complexDerivations, schemaFinals)
	if err != nil {
		return 0, 0, fmt.Errorf("element final: %w", err)
	}
	return block, final, nil
}

func declarationFinal(declaration *Declaration) (derivationSet, error) {
	if declaration == nil {
		return 0, fmt.Errorf("%w: missing base declaration", errInvalidDerivation)
	}
	if declaration.Builtin() {
		return 0, nil
	}
	_, fallback := schemaDerivationDefaults(declaration.File)
	switch {
	case declaration.SimpleType != nil:
		return effectiveDerivations(declaration.SimpleType.Final, fallback, simpleDerivations, schemaFinals)
	case declaration.ComplexType != nil:
		return effectiveDerivations(declaration.ComplexType.Final, fallback, complexDerivations, schemaFinals)
	default:
		return 0, fmt.Errorf("%w: base has no type", errInvalidDerivation)
	}
}

func checkDeclarationDerivation(declaration *Declaration, method derivationSet) error {
	final, err := declarationFinal(declaration)
	if err != nil {
		return err
	}
	if final&method != 0 {
		return fmt.Errorf("%w: final of %s prohibits derivation", errInvalidDerivation, formatExpandedName(declaration.Name))
	}
	return nil
}

func checkSimpleMemberDerivation(member *SimpleTypeMember, method derivationSet) error {
	if member == nil {
		return fmt.Errorf("%w: missing simple member", errInvalidDerivation)
	}
	if member.Declaration != nil {
		return checkDeclarationDerivation(member.Declaration, method)
	}
	if member.Inline == nil {
		return fmt.Errorf("%w: missing inline type", errInvalidDerivation)
	}
	if member.Inline.Final&method != 0 {
		return fmt.Errorf("%w: inline type final prohibits derivation", errInvalidDerivation)
	}
	return nil
}
