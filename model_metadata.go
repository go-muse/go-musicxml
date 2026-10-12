package musicxml

// modelDescriptor is a closed generated Go representation, not a second schema.
// Exact declaration uses are selected exclusively by the shared XSD grammar.
// Field paths come from the final renderer plan, never from runtime XML tags.
type modelDescriptor struct {
	Name                   string
	GoFields, MappedFields int
	Root, SchemaType       modelName
	Scalar                 *modelField
	Attributes, Elements   []modelField
}

type modelName struct{ Space, Local string }
type modelScalarKind uint8

const (
	modelString modelScalarKind = iota + 1
	modelBoolean
	modelSigned
	modelUnsigned
	modelDecimal
)

type modelField struct {
	Name               string
	Index              []int
	XML                modelName
	Optional, Repeated bool
	Kind               modelScalarKind
	Child              string
	Variants           []modelField
}

// Maps are initialized once by generated declarations and read-only thereafter.
// Anonymous structs have stable renderer owner keys rather than reflect names.
var modelDescriptors = map[string]*modelDescriptor{}
