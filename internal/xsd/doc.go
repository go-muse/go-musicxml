// Package xsd is the internal XML Schema kernel shared by the root musicxml
// package, the validation package and the code generator.
//
// Public API boundary: the validation package re-exports SchemaData, QName,
// TypeSchema, TypeRef, SimpleSchema, SimpleMember, ComplexSchema,
// AttributeSchema, ValueConstraint, AnyAttributeSchema, ParticleSchema,
// Occurrence, AnySchema, ElementSchema, SimpleForm, ComplexForm, AttributeUse,
// ParticleKind and Derivation as validation.Schema* type aliases. Their
// exported fields are therefore public API: renaming, removing or retyping
// one is a breaking change for validation users, and adding a field must keep
// keyed composite literals valid. TestPublicAPISnapshot in the module root
// records these field sets.
package xsd
