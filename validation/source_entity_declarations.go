package validation

import (
	"context"
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
// available: known locations identify real declaration sites, either in the
// document's original bytes or, with CoordinateSpaceExternalDTD and the entity
// named by Location.Entry, in an external DTD entity.
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
// An optional known-true reason is explanatory and does not alter completeness.
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

// sourceEntityDeclarations admits one immutable source declaration snapshot.
// Call immediately after root OpenNode selects its assembly, before any
// Attribute, CompleteAttributes, child OpenNode or CloseNode. The active
// assembly must select an available declaration-aware XSD operation. Omitting this event
// leaves declaration membership unavailable only when its datatype is reached.
// Every rejection is terminal; Finish still releases all owned live gauges.
func (s *Session) sourceEntityDeclarations(ctx context.Context, facts SourceEntityDeclarations) (err error) {
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
	if len(facts.Complete.Reason) > maxIdentityBytes || facts.Complete.Cause != nil {
		return s.fail(contract("source entity declaration completeness", "bounded cause-free evidence required"))
	}
	if err = s.step(ctx, 1+int64(len(facts.DocumentID))+int64(len(facts.Complete.Reason))); err != nil {
		return s.fail(err)
	}
	if err = validateEvidence(facts.Complete, "source entity declaration completeness"); err != nil {
		return s.fail(err)
	}
	if facts.Complete.State == EvidenceAbsent || facts.Complete.State == EvidenceInvalid || facts.Complete.State == EvidenceKnown && !facts.Complete.Value && !validID(string(facts.Complete.Reason)) {
		return s.fail(contract("source entity declaration completeness", "explicit enumeration evidence and reason for incompleteness required"))
	}
	mapBytes, sizeErr := sourceEntityMapBytes(len(facts.Declarations), int64(len(facts.Complete.Reason)))
	if sizeErr != nil {
		return s.fail(sizeErr)
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
	if err = s.retain(mapBytes); err != nil {
		return err
	}
	owned := &sourceEntityDeclarations{domain: xsd.EntityDeclarations{Bindings: make(map[string]xsd.EntityDeclaration, len(facts.Declarations)), Complete: facts.Complete.State == EvidenceKnown && facts.Complete.Value, Reason: strings.Clone(string(facts.Complete.Reason))}, origins: make(map[uint64]Evidence[Location], len(facts.Declarations))}
	for _, row := range facts.Declarations {
		if err = s.account.Check(BudgetScalarBytes, int64(len(row.Name))); err != nil {
			return err
		}
		originBytes, metadataErr := sourceEntityOriginBytes(row.Origin)
		if metadataErr != nil {
			return metadataErr
		}
		if err = s.step(ctx, 1+originBytes); err != nil {
			return err
		}
		if err = s.validateSourceEntityName(ctx, row.Name); err != nil {
			return err
		}
		// String-key hashing and copying are separately admitted before map lookup
		// or allocation. The name scan above polls cancellation every 256 bytes.
		if err = s.step(ctx, int64(len(row.Name))); err != nil {
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
			position := row.Origin.Value.Position
			external := position != nil && position.CoordinateSpace == CoordinateSpaceExternalDTD && row.Origin.Value.Entry != ""
			if position == nil || position.CoordinateSpace != CoordinateSpaceOriginalBytes && !external {
				return contract("source entity declaration origin", "known site requires original-byte or external-DTD coordinates")
			}
		}
		nameBytes := int64(len(row.Name))
		hostMax := int64(^uint(0) >> 1)
		if nameBytes > hostMax-originBytes {
			return contract("source entity declarations", "snapshot entry exceeds host allocation size")
		}
		if err = s.retain(nameBytes + originBytes); err != nil {
			return err
		}
		if err = s.step(ctx, nameBytes); err != nil {
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

// sourceEntityMapBytes proves both conservative map allowance arithmetic and
// host allocation width before make, even when callers raise default budgets.
func sourceEntityMapBytes(count int, extra int64) (int64, error) {
	hostMax := int64(^uint(0) >> 1)
	if count < 0 || extra < 0 || extra > hostMax-512 || int64(count) > (hostMax-512-extra)/512 {
		return 0, contract("source entity declarations", "snapshot size exceeds host allocation size")
	}
	return 512 + extra + int64(count)*512, nil
}

// Bound every metadata string before calling shared logical-size helpers, whose
// native-int sums are safe only after these fixed ingress contracts hold.
func sourceEntityOriginBytes(origin Evidence[Location]) (int64, error) {
	v := origin.Value
	if origin.Cause != nil || len(origin.Reason) > maxIdentityBytes || len(v.DocumentID) > maxIdentityBytes || len(v.Path) > 16384 || len(v.Entry) > 4096 || v.Position != nil && len(v.Position.CoordinateSpace) > maxIdentityBytes {
		return 0, contract("source entity declaration origin", "bounded cause-free location evidence required")
	}
	return evidenceLogicalBytes(origin) + locationLogicalBytes(v), nil
}

// Source names use XML 1.0 Fifth Edition, independently of the narrower pinned
// XSD 1.0 scalar-name repertoire. Decode one rune at a time; invalid UTF-8 cannot
// masquerade as the legal replacement character. Each bounded chunk is charged
// and cancellation-checked before its scan.
func (s *Session) validateSourceEntityName(ctx context.Context, name string) error {
	return s.validateSourceNCName(ctx, name, "source entity name")
}

func (s *Session) validateSourceNCName(ctx context.Context, name, field string) error {
	if name == "" {
		return contract(field, "nonempty source XML NCName required")
	}
	first := true
	for offset := 0; offset < len(name); {
		end := offset + min(256, len(name)-offset)
		if err := s.step(ctx, int64(min(259, len(name)-offset))); err != nil {
			return err
		}
		for offset < end {
			r, width := utf8.DecodeRuneInString(name[offset:])
			if r == utf8.RuneError && width == 1 {
				return contract(field, "valid UTF-8 required")
			}
			start := r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= 0xc0 && r <= 0xd6 || r >= 0xd8 && r <= 0xf6 || r >= 0xf8 && r <= 0x2ff || r >= 0x370 && r <= 0x37d || r >= 0x37f && r <= 0x1fff || r >= 0x200c && r <= 0x200d || r >= 0x2070 && r <= 0x218f || r >= 0x2c00 && r <= 0x2fef || r >= 0x3001 && r <= 0xd7ff || r >= 0xf900 && r <= 0xfdcf || r >= 0xfdf0 && r <= 0xfffd || r >= 0x10000 && r <= 0xeffff
			if !start && (first || !(r == '-' || r == '.' || r >= '0' && r <= '9' || r == 0xb7 || r >= 0x300 && r <= 0x36f || r >= 0x203f && r <= 0x2040)) {
				return contract(field, "valid source XML NCName required")
			}
			offset += width
			first = false
		}
	}
	return s.step(ctx, 0)
}
