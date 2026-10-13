package validation

const packageDiagnosticReserveBytes int64 = 6 * 1024

// PackageDiagnosticSummary preserves the first omitted blocking package decision
// and a saturating count of all omitted blocking package diagnostics. It is not
// a Finding and does not consume or expand MaxDiagnostics. Package is populated
// when the subject has an exact owning boundary. Full causal detail, locations,
// messages and rule metadata may be omitted; authoritative report counts remain
// independent. All copied identities and reason codes are bounded to 1024 bytes.
type PackageDiagnosticSummary struct {
	Omitted  uint64
	Rule     RuleKey
	Subject  NodeRef
	Package  PackageID
	Category DiagnosticCategory
	Outcome  Outcome
	Reason   Code
}

// The fixed payload is prepaid at Begin. Capturing an omission after diagnostics
// or buffered bytes are exhausted performs no allocation and keeps no caller
// string backing storage. API copies are materialized only by the bundle getter.
type packageDiagnosticSummaryData struct {
	omitted                           uint64
	rule, document, packageID, reason [maxIdentityBytes]byte
	lengths                           [4]uint16
	revision                          uint32
	node                              NodeID
	category                          DiagnosticCategory
	outcome                           Outcome
}

func (v *packageDiagnosticSummaryData) capture(rule RuleDefinition, decision Decision, pkg PackageID) {
	if decision.Outcome == OutcomePass || decision.Outcome == OutcomeNotApplicable || rule.Category == CategoryAdvisory || rule.Class == ClassRecommendation || rule.Role == RoleAdvisory {
		return
	}
	if v.omitted == 0 {
		v.lengths[0] = uint16(copy(v.rule[:], string(decision.Rule.ID)))
		v.lengths[1] = uint16(copy(v.document[:], decision.Subject.DocumentID))
		v.lengths[2] = uint16(copy(v.packageID[:], string(pkg)))
		v.lengths[3] = uint16(copy(v.reason[:], string(decision.Reason)))
		v.revision, v.node, v.category, v.outcome = decision.Rule.Revision, decision.Subject.Node, rule.Category, decision.Outcome
	}
	if v.omitted != ^uint64(0) {
		v.omitted++
	}
}

// OmittedPackageDiagnostics returns an owned bounded summary when the shared
// diagnostic quota omitted blocking package findings. It preserves original
// subject identity and deterministic first-observed selection, independently of
// child findings and terminal infrastructure summaries.
func (b AssessmentBundle) OmittedPackageDiagnostics() (PackageDiagnosticSummary, bool) {
	if b.data == nil || b.data.omittedDiagnostics == nil || b.data.omittedDiagnostics.omitted == 0 {
		return PackageDiagnosticSummary{}, false
	}
	v := b.data.omittedDiagnostics
	return PackageDiagnosticSummary{
		Omitted:  v.omitted,
		Rule:     RuleKey{ID: RuleID(string(v.rule[:v.lengths[0]])), Revision: v.revision},
		Subject:  NodeRef{DocumentID: string(v.document[:v.lengths[1]]), Node: v.node},
		Package:  PackageID(string(v.packageID[:v.lengths[2]])),
		Category: v.category, Outcome: v.outcome, Reason: Code(string(v.reason[:v.lengths[3]])),
	}, true
}
