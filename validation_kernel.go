package musicxml

// This file bridges the legacy root validator, generated schema metadata and
// root XSD parity tests to the authoritative internal/xsd kernel. New assessment
// production code uses internal/xsd directly rather than adding aliases here.
// Retire the compatibility aliases when the schema emitter and root parity
// tests migrate to the kernel API; that migration is outside this source-preview
// batch. ValidationIssue remains a named public type for API compatibility.

import (
	"errors"
	"reflect"
	"sync"

	"github.com/go-muse/go-musicxml/internal/xsd"
	"github.com/go-muse/go-musicxml/validation"
)

// ValidationIssue describes one validation failure. Every operation that
// reports issues, Validate as well as DecodeAssessed and DecodeMXLAssessed,
// fills the fields the same way: Constraint is the failure reason code, such
// as "datatype", or "representation" for a model value that cannot be written
// as XML; Message is the offending value when the rule reports one, and may be
// empty; Rule is the ID of the violated rule.
//
// Use the validation.Report returned by the assessment functions for a
// complete, structured view.
type ValidationIssue struct {
	// Path locates the failing element or attribute.
	Path string
	// Constraint identifies the violated constraint; see the type comment.
	Constraint string
	// Message describes the failure; see the type comment.
	Message string
	// Rule is the ID of the violated rule.
	Rule string
}

// String returns the issue path and message, or the constraint when the
// message is empty.
func (i ValidationIssue) String() string { return xsd.Issue(i).String() }

type validationKernelIssue = xsd.Issue

var betterValidationFailure = xsd.BetterFailure
var collapseValidationWhitespace = xsd.CollapseWhitespace
var isValidationWhitespace = xsd.IsWhitespace
var matchValidationPattern = xsd.MatchPattern
var mergeValidationAnyAttribute = xsd.MergeAnyAttribute
var mergeValidationAttributes = xsd.MergeAttributes
var normalizeValidationWhitespace = xsd.NormalizeWhitespace
var parseValidationBoolean = xsd.ParseBoolean
var parseValidationDecimal = xsd.ParseDecimal
var parseValidationInteger = xsd.ParseInteger
var splitValidationList = xsd.SplitList
var uniqueValidationNames = xsd.UniqueNames
var validValidationIPv6 = xsd.ValidIPv6
var validValidationInteger = xsd.ValidInteger
var validValidationURIAuthority = xsd.ValidURIAuthority
var validXSDAnyURI = xsd.ValidXSDAnyURI
var validXSDDateMatch = xsd.ValidXSDDateMatch
var validXSDDateTime = xsd.ValidXSDDateTime
var validXSDDuration = xsd.ValidXSDDuration
var validXSDNCName = xsd.ValidXSDNCName
var validXSDNMTOKEN = xsd.ValidXSDNMTOKEN
var validXSDName = xsd.ValidXSDName
var validXSDQName = xsd.ValidXSDQName
var validXSDYear = xsd.ValidXSDYear
var validateBounds = xsd.ValidateBounds
var validateBoundsValue = xsd.ValidateBoundsValue
var validateBuiltin = xsd.ValidateBuiltin
var validateBuiltinValue = xsd.ValidateBuiltinValue
var validateDecimalBounds = xsd.ValidateDecimalBounds
var validateDigitFacets = xsd.ValidateDigitFacets
var validateIntegerBounds = xsd.ValidateIntegerBounds
var validateLengthFacets = xsd.ValidateLengthFacets
var validationAnyAttributeAllows = xsd.AnyAttributeAllows

type validationAnyAttributeSchema = xsd.AnyAttributeSchema
type validationAnySchema = xsd.AnySchema

var validationAtomicValue = xsd.AtomicValue

type validationAttribute = xsd.Attribute

const validationAttributeOptional = xsd.AttributeOptional

var validationAttributePath = xsd.AttributePath

const validationAttributeProhibited = xsd.AttributeProhibited

var validationAttributeQNamePath = xsd.AttributeQNamePath

const validationAttributeRequired = xsd.AttributeRequired

type validationAttributeSchema = xsd.AttributeSchema
type validationAttributeUse = xsd.AttributeUse

const validationBooleanDomain = xsd.BooleanDomain

var validationBuiltinBase = xsd.BuiltinBase
var validationBuiltinListItem = xsd.BuiltinListItem
var validationBuiltinSemantics = xsd.BuiltinSemantics
var validationBuiltinType = xsd.BuiltinType
var validationCanonicalBuiltin = xsd.CanonicalBuiltin
var validationChildPath = xsd.ChildPath
var validationChildPaths = xsd.ChildPaths

const validationComplexComplexContentExtension = xsd.ComplexComplexContentExtension
const validationComplexComplexContentRestriction = xsd.ComplexComplexContentRestriction
const validationComplexDirect = xsd.ComplexDirect

type validationComplexForm = xsd.ComplexForm
type validationComplexSchema = xsd.ComplexSchema

const validationComplexSimpleContentExtension = xsd.ComplexSimpleContentExtension
const validationComplexSimpleContentRestriction = xsd.ComplexSimpleContentRestriction

type validationDecimal = xsd.Decimal

var validationDecimalPattern = xsd.DecimalPattern
var validationDecimalValuesEqual = xsd.DecimalValuesEqual

type validationDerivation = xsd.Derivation

const validationDerivationExtension = xsd.DerivationExtension

type validationDerivationKey = xsd.DerivationKey

const validationDerivationList = xsd.DerivationList
const validationDerivationRestriction = xsd.DerivationRestriction
const validationDerivationSubstitution = xsd.DerivationSubstitution
const validationDerivationUnion = xsd.DerivationUnion

var validationDisplayName = xsd.DisplayName
var validationDisplayQName = xsd.DisplayQName

type validationEffectiveComplex = xsd.EffectiveComplex
type validationElementMatch = xsd.ElementMatch
type validationElementSchema = xsd.ElementSchema
type validationEnumerationKey = xsd.EnumerationKey
type validationEnumerationValue = xsd.EnumerationValue

var validationExpectedNames = xsd.ExpectedNames
var validationHexPattern = xsd.HexPattern

type validationIdentityBindings = xsd.IdentityBindings
type validationIdentityEntry = xsd.IdentityEntry
type validationIdentityReference = xsd.IdentityReference

var validationIndexedChildPath = xsd.IndexedChildPath
var validationInstanceQName = xsd.InstanceQName

type validationInteger = xsd.Integer

var validationIntegerLimits = xsd.IntegerLimits
var validationIntegerValuesEqual = xsd.IntegerValuesEqual
var validationLanguagePattern = xsd.LanguagePattern

const validationLexicalDomain = xsd.LexicalDomain
const validationListDomain = xsd.ListDomain

type validationMatchFailure = xsd.MatchFailure
type validationMatchResult = xsd.MatchResult

var validationName = xsd.Name
var validationNamespaceDeclaration = xsd.NamespaceDeclaration
var validationNilAttribute = xsd.NilAttribute

type validationNode = xsd.Node

const validationNumberDomain = xsd.NumberDomain

type validationOccurrence = xsd.Occurrence

var validationOrderedPositions = xsd.OrderedPositions

const validationParticleAll = xsd.ParticleAll
const validationParticleAny = xsd.ParticleAny
const validationParticleChoice = xsd.ParticleChoice
const validationParticleElement = xsd.ParticleElement

type validationParticleEndpoints = xsd.ParticleEndpoints
type validationParticleKey = xsd.ParticleKey
type validationParticleKind = xsd.ParticleKind
type validationParticleMatcher = xsd.ParticleMatcher
type validationParticlePositionSet = xsd.ParticlePositionSet
type validationParticlePredecessors = xsd.ParticlePredecessors
type validationParticleSchema = xsd.ParticleSchema

const validationParticleSequence = xsd.ParticleSequence

type validationParticleWork = xsd.ParticleWork

var validationPath = xsd.Path

type validationQName = xsd.QName

var validationReplaceWhitespace = xsd.ReplaceWhitespace

type validationSchemaSet = xsd.SchemaData

// publicSchemas caches the generated schemas converted to the validation
// package's types, so that engines built one after another share one
// conversion. CompileSchema copies the cached value and never mutates it.
var publicSchemas sync.Map // map pointer of Types -> validation.SchemaData

// compileValidationSchema compiles a schema held in the kernel's types through
// the validation package, whose metadata types mirror them. The conversion
// keeps sharing and interior pointers, so the digest is that of the input.
// Only the generated schemas, recognized by their maps, are cached.
func compileValidationSchema(assembly validation.Assembly, data validationSchemaSet) (*validation.CompiledSchema, error) {
	generated := false
	for _, schema := range []*validationSchemaSet{&scoreValidationSchema, &opusValidationSchema, &containerValidationSchema, &soundsValidationSchema} {
		if sameSchemaMaps(data, *schema) {
			generated = true
		}
	}
	key := reflect.ValueOf(data.Types).Pointer()
	if generated {
		if public, cached := publicSchemas.Load(key); cached {
			return validation.CompileSchema(assembly, public.(validation.SchemaData))
		}
	}
	var public validation.SchemaData
	if err := xsd.ConvertGraph(&public, data); err != nil {
		return nil, err
	}
	if generated {
		publicSchemas.Store(key, public)
	}
	return validation.CompileSchema(assembly, public)
}

// sameSchemaMaps reports whether a and b hold the same maps, not merely equal
// contents.
func sameSchemaMaps(a, b validationSchemaSet) bool {
	return reflect.ValueOf(a.Types).Pointer() == reflect.ValueOf(b.Types).Pointer() &&
		reflect.ValueOf(a.Elements).Pointer() == reflect.ValueOf(b.Elements).Pointer() && a.Types != nil
}

var validationSequence = xsd.Sequence

type validationSimpleAssessment = xsd.SimpleAssessment
type validationSimpleFailure = xsd.SimpleFailure
type validationSimpleForm = xsd.SimpleForm

const validationSimpleList = xsd.SimpleList

type validationSimpleMember = xsd.SimpleMember

const validationSimpleRestriction = xsd.SimpleRestriction

type validationSimpleSchema = xsd.SimpleSchema
type validationSimpleSemantics = xsd.SimpleSemantics

const validationSimpleUnion = xsd.SimpleUnion

type validationSimpleValue = xsd.SimpleValue

var validationStandardXSIName = xsd.StandardXSIName
var validationString = xsd.String

const validationStringDomain = xsd.StringDomain

var validationTechnicalFailure = xsd.TechnicalFailure

type validationTypeIdentity = xsd.TypeIdentity
type validationTypeRef = xsd.TypeRef
type validationTypeSchema = xsd.TypeSchema

var validationURIAuthority = xsd.URIAuthority
var validationURIHex = xsd.URIHex

type validationValueConstraint = xsd.ValueConstraint
type validationValueDomain = xsd.ValueDomain

const validationWhitespaceCollapse = xsd.WhitespaceCollapse
const validationWhitespaceMember = xsd.WhitespaceMember

type validationWhitespacePolicy = xsd.WhitespacePolicy

const validationWhitespacePreserve = xsd.WhitespacePreserve
const validationWhitespaceReplace = xsd.WhitespaceReplace
const validationXMLNameCharacters = xsd.XMLNameCharacters
const validationXMLNameStartCharacters = xsd.XMLNameStartCharacters
const validationXSDBaseCharCharacters = xsd.XSDBaseCharCharacters
const validationXSDCombiningCharCharacters = xsd.XSDCombiningCharCharacters
const validationXSDDigitCharacters = xsd.XSDDigitCharacters
const validationXSDExtenderCharacters = xsd.XSDExtenderCharacters
const validationXSDIdeographicCharacters = xsd.XSDIdeographicCharacters

var validationXSDNMTOKENPattern = xsd.XSDNMTOKENPattern
var validationXSDNamePattern = xsd.XSDNamePattern

const validationXSDNamespace = xsd.XSDNamespace
const validationXSINamespace = xsd.XSINamespace

var xsdDatePattern = xsd.XsdDatePattern
var xsdDateTimePattern = xsd.XsdDateTimePattern

const xsdDayFragment = xsd.XsdDayFragment

var xsdDaysInMonth = xsd.XsdDaysInMonth
var xsdDurationPattern = xsd.XsdDurationPattern
var xsdGDayPattern = xsd.XsdGDayPattern
var xsdGMonthDayPattern = xsd.XsdGMonthDayPattern
var xsdGMonthPattern = xsd.XsdGMonthPattern
var xsdGYearMonthPattern = xsd.XsdGYearMonthPattern
var xsdGYearPattern = xsd.XsdGYearPattern

const xsdMonthFragment = xsd.XsdMonthFragment
const xsdTimeFragment = xsd.XsdTimeFragment

var xsdTimePattern = xsd.XsdTimePattern

const xsdTimezoneFragment = xsd.XsdTimezoneFragment
const xsdYearFragment = xsd.XsdYearFragment

var xsdYearModulo = xsd.XsdYearModulo

type validationKernelContext = xsd.Context
type validationRawAttribute = xsd.Attr
type validationContext struct{ xsd.Context }

func newValidationParticleMatcher(c *validationContext, children []*validationNode) *validationParticleMatcher {
	return xsd.NewParticleMatcher(&c.Context, children)
}
func (c *validationContext) result() error {
	var technical error
	if c.AssessmentErr != nil {
		technical = translateValidationTechnical(c.AssessmentErr)
	}
	if len(c.Issues) == 0 {
		return technical
	}
	issues := make([]ValidationIssue, len(c.Issues))
	for i, issue := range c.Issues {
		issues[i] = ValidationIssue(issue)
	}
	invalid := &ValidationError{Issues: issues}
	if technical == nil {
		return invalid
	}
	return errors.Join(invalid, technical)
}
func translateValidationTechnical(err error) error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var all []error
		for _, cause := range joined.Unwrap() {
			all = append(all, translateValidationTechnical(cause))
		}
		return errors.Join(all...)
	}
	if technical, ok := err.(*xsd.TechnicalError); ok {
		return &validation.IncompleteError{Reason: "incomplete XSD assessment: " + technical.Message}
	}
	return err
}
