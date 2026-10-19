// Package xsd is the internal XML Schema kernel shared by the root musicxml
// package, the validation package and the code generator.
//
// Public API boundary: the validation package declares its own Schema* types
// with the shape of SchemaData and its components (the same field names and
// kinds), and CompileGraph and ConvertGraph move graphs between the two
// families. A field added here must be added there in the same position;
// CompileGraph rejects a family of another shape.
package xsd
