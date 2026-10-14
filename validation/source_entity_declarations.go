package validation

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// SourceEntityDeclaration is one effective first-bound general entity in the
// source document. Unparsed distinguishes NDATA declarations from parsed
// bindings, which are exact negative membership witnesses. Parameter entities,
// ignored duplicates and declarations past an unread-parameter-entity cutoff
// must not be supplied. Sequence is a positive document-local declaration
// identity, not a schema declaration or element NodeID. Origin is independently
// available: known locations identify real original-byte declaration sites.
type SourceEntityDeclaration struct {
	Name     string
	Unparsed bool
	Sequence uint64
	Origin   Evidence[Location]
}

// SourceEntityDeclarations supplies a document-qualified declaration domain.
// Complete describes enumeration independently of document EOF, source origins,
// replacement-content well-formedness and effective attribute completeness.
// Known false and unavailable completeness require an explicit reason. Only
// known true permits an absent name to disprove unparsed-entity membership.
// The Session copies facts; the caller may mutate or release them after return.
type SourceEntityDeclarations struct {
	DocumentID   string
	Declarations []SourceEntityDeclaration
	Complete     Evidence[bool]
}

type sourceEntityDeclarations struct {
	domain  xsd.EntityDeclarations
	origins map[uint64]Evidence[Location]
}

// SourceEntityDeclarations admits one immutable source declaration snapshot.
// Call immediately after root Open selects its assembly, before any Attribute,
// CompleteAttributes, child Open or Close. The active assembly must select an
// available NewXSDPlanWithSourceDeclarations operation. Omitting this event
// leaves declaration membership unavailable only when its datatype is reached.
// Every rejection is terminal; Finish still releases all owned live gauges.
func (s *Session) SourceEntityDeclarations(ctx context.Context, facts SourceEntityDeclarations) (err error) {
	if err = s.event(ctx, "SourceEntityDeclarations"); err != nil {
		return err
	}
	if s.profile.Target != TargetSource || s.schema == nil || s.schema.active == nil || s.catalog.sourceDeclarations[s.profile.ID][s.schema.active.assembly] != xsdSourceDeclarationsV1 {
		return s.fail(contract("source entity declarations", "available selected source-declaration XSD mode required"))
	}
	if s.root == nil || s.root.closed || len(s.stack) != 1 || s.stack[0] != s.root || len(s.sequence) != 1 || len(s.root.attributes) != 0 || s.root.attributeSeal != nil || s.schema.bound || s.sourceEntities != nil {
		return s.fail(&SessionError{Operation: "SourceEntityDeclarations", Reason: "one snapshot immediately after root Open required"})
	}
	if facts.DocumentID != s.documentID {
		return s.fail(contract("source entity declarations", "document mismatch"))
	}
	if err = s.step(ctx, 1+int64(len(facts.DocumentID))+evidenceLogicalBytes(facts.Complete)); err != nil {
		return s.fail(err)
	}
	if err = validateEvidence(facts.Complete, "source entity declaration completeness"); err != nil {
		return s.fail(err)
	}
	if facts.Complete.State == EvidenceAbsent || facts.Complete.State == EvidenceInvalid || facts.Complete.State == EvidenceKnown && !facts.Complete.Value && !validID(string(facts.Complete.Reason)) {
		return s.fail(contract("source entity declaration completeness", "explicit enumeration evidence and reason for incompleteness required"))
	}
	if int64(len(facts.Declarations)) > math.MaxInt64/512 {
		return s.fail(contract("source entity declarations", "snapshot size overflow"))
	}
	before := s.retained
	defer func() {
		if err != nil {
			if delta := s.retained - before; delta > 0 {
				_ = s.account.Release(BudgetBufferedBytes, delta)
				s.retained = before
			}
			s.fail(err)
		}
	}()
	// Admit map headers and conservative per-entry capacities before allocation.
	if err = s.step(ctx, int64(len(facts.Declarations))); err != nil {
		return err
	}
	if err = s.retain(512 + int64(len(facts.Declarations))*512 + evidenceLogicalBytes(facts.Complete)); err != nil {
		return err
	}
	owned := &sourceEntityDeclarations{domain: xsd.EntityDeclarations{Bindings: make(map[string]xsd.EntityDeclaration, len(facts.Declarations)), Complete: facts.Complete.State == EvidenceKnown && facts.Complete.Value, Reason: strings.Clone(string(facts.Complete.Reason))}, origins: make(map[uint64]Evidence[Location], len(facts.Declarations))}
	for _, row := range facts.Declarations {
		if err = s.account.Check(BudgetScalarBytes, int64(len(row.Name))); err != nil {
			return err
		}
		if err = s.step(ctx, 1+int64(len(row.Name))*3+evidenceLogicalBytes(row.Origin)+locationLogicalBytes(row.Origin.Value)); err != nil {
			return err
		}
		if !sourceEntityNCName(row.Name) {
			return contract("source entity name", "valid source XML NCName required")
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if row.Sequence == 0 {
			return contract("source entity sequence", "positive declaration identity required")
		}
		if _, exists := owned.domain.Bindings[row.Name]; exists {
			return contract("source entity name", "duplicate effective binding")
		}
		if _, exists := owned.origins[row.Sequence]; exists {
			return contract("source entity sequence", "duplicate declaration identity")
		}
		if err = validateEvidence(row.Origin, "source entity declaration origin"); err != nil {
			return err
		}
		if row.Origin.State == EvidenceAbsent || row.Origin.State == EvidenceInvalid {
			return contract("source entity declaration origin", "known or independently unavailable site required")
		}
		if row.Origin.State == EvidenceKnown {
			if err = validateLocation(row.Origin.Value, TargetSource, s.documentID); err != nil {
				return err
			}
			if row.Origin.Value.Position == nil || row.Origin.Value.Position.CoordinateSpace != "original-bytes" {
				return contract("source entity declaration origin", "known site requires original-byte coordinates")
			}
		}
		if err = s.retain(int64(len(row.Name)) + evidenceLogicalBytes(row.Origin) + locationLogicalBytes(row.Origin.Value)); err != nil {
			return err
		}
		name := strings.Clone(row.Name)
		origin := cloneEvidence(row.Origin)
		if origin.State == EvidenceKnown {
			origin.Value = cloneLocation(row.Origin.Value)
		}
		owned.domain.Bindings[name] = xsd.EntityDeclaration{Unparsed: row.Unparsed, Sequence: row.Sequence}
		owned.origins[row.Sequence] = origin
	}
	if err = s.step(ctx, 0); err != nil {
		return err
	}
	s.sourceEntities = owned
	return nil
}

// Source names use XML 1.0 Fifth Edition, independently of the narrower pinned
// XSD 1.0 scalar-name repertoire. The caller admits the whole byte scan first.
func sourceEntityNCName(name string) bool {
	if name == "" || !utf8.ValidString(name) {
		return false
	}
	first := true
	for _, r := range name {
		start := r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= 0xc0 && r <= 0xd6 || r >= 0xd8 && r <= 0xf6 || r >= 0xf8 && r <= 0x2ff || r >= 0x370 && r <= 0x37d || r >= 0x37f && r <= 0x1fff || r >= 0x200c && r <= 0x200d || r >= 0x2070 && r <= 0x218f || r >= 0x2c00 && r <= 0x2fef || r >= 0x3001 && r <= 0xd7ff || r >= 0xf900 && r <= 0xfdcf || r >= 0xfdf0 && r <= 0xfffd || r >= 0x10000 && r <= 0xeffff
		if !start && (first || !(r == '-' || r == '.' || r >= '0' && r <= '9' || r == 0xb7 || r >= 0x300 && r <= 0x36f || r >= 0x203f && r <= 0x2040)) {
			return false
		}
		first = false
	}
	return true
}
