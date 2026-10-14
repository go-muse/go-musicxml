package musicxml

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// assessmentRunConfig binds an intrinsic occurrence to its root-owned account.
// It is deliberately private: child adapters cannot create/reset an account or
// substitute a catalog for a built-in profile. Engine identity here checks only
// configuration; it is never a resource, byte, or assessment-cache identity.
type assessmentRunConfig struct {
	Account    *validation.Account
	DocumentID string
	Entry      string
	Profile    validation.ProfileID
	Engine     *validation.Engine
	// Dictionaries is one root-owned, same-account capability set. Nil keeps
	// the historical session path and performs no dictionary work.
	Dictionaries *validation.DictionarySet
	// XMLDepth is an independent per-document representation cap. Zero leaves
	// inspection governed only by the shared account; MXL supplies its default.
	XMLDepth int
}

func (c assessmentRunConfig) begin(ctx context.Context) (*validation.Session, error) {
	opts := validation.RunOptions{DocumentID: c.DocumentID, Profile: c.Profile, Account: c.Account}
	if c.Dictionaries != nil {
		return c.Engine.BeginWithDictionaries(ctx, opts, c.Dictionaries)
	}
	return c.Engine.Begin(ctx, opts)
}

func assessmentContext(ctx context.Context, reason validation.Code) error {
	if ctx == nil {
		return &validation.ConfigurationError{Field: "context", Reason: "must not be nil"}
	}
	if err := ctx.Err(); err != nil {
		return &validation.IncompleteError{Reason: string(reason), Cause: err}
	}
	return nil
}

func (c assessmentRunConfig) validate(target validation.Target) error {
	if c.XMLDepth < 0 || c.XMLDepth > maximumXMLDepth {
		return &validation.ConfigurationError{Field: "XMLDepth", Reason: "intrinsic XML cap must be between 0 and 4096"}
	}
	if c.Account == nil {
		return &validation.ConfigurationError{Field: "Account", Reason: "intrinsic runs require the root account"}
	}
	if _, err := c.Account.Usage(validation.BudgetEvents); err != nil {
		return err
	}
	if len(c.Entry) > 4096 || !utf8.ValidString(c.Entry) || strings.ContainsRune(c.Entry, 0) {
		return &validation.ConfigurationError{Field: "entry", Reason: "invalid or excessive entry location"}
	}
	var engine *validation.Engine
	var err error
	switch target {
	case validation.TargetSource:
		engine, err = sourceAssessmentEngineForProfile(c.Profile)
	case validation.TargetModel:
		engine, err = modelAssessmentEngineForProfile(c.Profile)
	default:
		return &validation.ConfigurationError{Field: "target", Reason: "intrinsic adapter target required"}
	}
	if err != nil {
		return err
	}
	if c.Engine == nil || c.Engine != engine {
		return &validation.ConfigurationError{Field: "engine", Reason: "engine does not match the selected built-in profile"}
	}
	// Begin validates the occurrence ID and selected profile before any input.
	return nil
}

func sourceAssessmentEngineForProfile(profile validation.ProfileID) (*validation.Engine, error) {
	switch profile {
	case ProfileSourceInternalDTDContainer, ProfileSourceInternalDTDSoundsDictionaries:
		return sourceAuxiliaryCompositionEngine()
	case ProfileSourceInternalDTDLocal, ProfileSourceInternalDTDLinks, ProfileSourceInternalDTDDictionaries:
		return sourceCompositionEngine()
	case ProfileSourceDictionaries, ProfileSourceSoundsDictionaries:
		return newDictionaryAssessmentEngine()
	case ProfileSourceContainer, ProfileSourceSounds:
		return newAuxiliarySourceAssessmentEngine()
	case ProfileSourceXSDPreview:
		return newSourceAssessmentEngine()
	case ProfileSourceInternalDTD:
		return sourceInternalDTDEngine()
	case ProfileSourceLocal:
		return newModelAssessmentEngine()
	case ProfileSourceTimeline:
		return newTimelineAssessmentEngine()
	case ProfileSourceLinks:
		return newLinkAssessmentEngine()
	case ProfileSourceContext:
		return newContextAssessmentEngine()
	default:
		return nil, &validation.ConfigurationError{Field: "profile", Reason: "unknown or empty explicit source profile"}
	}
}

func modelAssessmentEngineForProfile(profile validation.ProfileID) (*validation.Engine, error) {
	switch profile {
	case ProfileModelDictionaries:
		return newDictionaryAssessmentEngine()
	case ProfileModelXSD, ProfileModelLocal:
		return newModelAssessmentEngine()
	case ProfileModelTimeline:
		return newTimelineAssessmentEngine()
	case ProfileModelLinks:
		return newLinkAssessmentEngine()
	case ProfileModelContext:
		return newContextAssessmentEngine()
	default:
		return nil, &validation.ConfigurationError{Field: "profile", Reason: "unknown, empty or incompatible explicit model profile"}
	}
}
