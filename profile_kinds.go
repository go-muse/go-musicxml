package musicxml

import "github.com/go-muse/go-musicxml/validation"

// SourceProfile names a built-in profile that assesses one original MusicXML
// document as bytes: the ProfileSource* constants, accepted by InspectXML and,
// for ProfileSourceLine, by DecodeAssessed. Distinct profile types make
// passing a model, graph or MXL profile to a source entry point a compile
// error.
type SourceProfile validation.ProfileID

// ID returns the profile's engine identity, as reported in Report.Profile.ID.
func (p SourceProfile) ID() validation.ProfileID { return validation.ProfileID(p) }

// ModelProfile names a built-in profile that assesses a converted Go document:
// the ProfileModel* constants, accepted by AssessModel.
type ModelProfile validation.ProfileID

// ID returns the profile's engine identity, as reported in Report.Profile.ID.
func (p ModelProfile) ID() validation.ProfileID { return validation.ProfileID(p) }

// GraphProfile names a built-in resource-graph profile: the
// ProfileResourceGraph* constants, accepted by InspectXMLWithResources and
// AssessModelWithResources.
type GraphProfile validation.ProfileID

// ID returns the profile's engine identity, as reported in the package report.
func (p GraphProfile) ID() validation.ProfileID { return validation.ProfileID(p) }

// MXLProfile names a built-in MXL package profile: the ProfileMXL* constants.
// InspectMXL and DecodeMXLAssessed accept the source ones and
// AssessMXLPackage the model ones; that distinction is checked when the call
// runs.
type MXLProfile validation.ProfileID

// ID returns the profile's engine identity, as reported in the package report.
func (p MXLProfile) ID() validation.ProfileID { return validation.ProfileID(p) }
