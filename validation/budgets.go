package validation

import (
	"math"
	"sync"
)

// Budgets limits assessment resources. All fields are int64; zero selects the
// documented finite default, and negative values are invalid. Byte limits count
// bytes, and other limits count individual objects, events, or operations.
// A nonzero MaxRuleSteps is a fixed cap; zero selects a limit that grows with
// the document (see DefaultRuleStepBase).
type Budgets struct {
	MaxInputBytes    int64
	MaxExternalBytes int64
	MaxBufferedBytes int64
	MaxDepth         int64
	MaxNodes         int64
	MaxAttributes    int64
	MaxEvents        int64
	MaxScalarBytes   int64
	MaxIDs           int64
	MaxRuleSteps     int64
	MaxContextFacts  int64
	// MaxLinkedDocuments counts nonroot adoptions, including aliases and package
	// internals. A first linked raw MXL may need container, primary and edge
	// adoptions; it does not count just resolver calls.
	MaxLinkedDocuments int64
	MaxDiagnostics     int64
	MaxDiagnosticBytes int64
}

// The default rule-step limit, selected by a zero MaxRuleSteps, allows
// DefaultRuleStepBase steps plus an allowance that grows with the document:
// DefaultRuleStepsPerNode for every BudgetNodes charge on the account, but at
// most DefaultRuleStepsPerDocumentByte for every byte of XML document text the
// assessment has read on the account: the primary input, linked documents and
// MXL members after decompression, but not external DTDs such as the bundled
// ones, and not ZIP bookkeeping. Nodes from internal entity expansion therefore
// add at most the work of the document bytes, and an account that has read no
// document bytes, such as one assessing an in-memory model alone, gets the node
// allowance alone. The limit never shrinks: nodes charged before the first
// document byte, such as those of an in-memory model or of ZIP central-directory
// entries, keep their allowance, and later work on the account may spend it.
// Every assessment line charges its own nodes, so strict loading counts a
// document's nodes once per line. Raising MaxNodes,
// MaxInputBytes or MaxExternalBytes raises the worst-case default work
// accordingly.
const (
	DefaultRuleStepBase             = 20_000_000
	DefaultRuleStepsPerNode         = 10_000
	DefaultRuleStepsPerDocumentByte = 1_000
)

// DefaultBudgets returns independent finite defaults: 64 MiB each of input,
// external input and live buffered data; depth 1024; one million nodes and IDs;
// four million attributes; eight million events; 1 MiB per scalar; one million
// context facts; 128 linked documents; and 1000 stored diagnostics occupying at
// most 1 MiB. MaxRuleSteps is zero, which selects the limit that grows with the
// document (see Budgets). These are safety limits, not promises of throughput or
// complete assessment for every input within the byte limits.
func DefaultBudgets() Budgets {
	return Budgets{
		MaxInputBytes: 64 << 20, MaxExternalBytes: 64 << 20, MaxBufferedBytes: 64 << 20,
		MaxDepth: 1024, MaxNodes: 1_000_000, MaxAttributes: 4_000_000,
		MaxEvents: 8_000_000, MaxScalarBytes: 1 << 20, MaxIDs: 1_000_000,
		MaxContextFacts: 1_000_000, MaxLinkedDocuments: 128,
		MaxDiagnostics: 1000, MaxDiagnosticBytes: 1 << 20,
	}
}

// BudgetKind identifies one resource and its accounting mode.
type BudgetKind string

const (
	// BudgetInputBytes counts cumulative primary-input bytes.
	BudgetInputBytes BudgetKind = "input-bytes"
	// BudgetExternalBytes counts cumulative explicitly resolved input bytes.
	BudgetExternalBytes BudgetKind = "external-bytes"
	// BudgetBufferedBytes is the live retained-data byte gauge.
	BudgetBufferedBytes BudgetKind = "buffered-bytes"
	// BudgetDepth is the live open-scope gauge.
	BudgetDepth BudgetKind = "depth"
	// BudgetNodes counts cumulative input nodes.
	BudgetNodes BudgetKind = "nodes"
	// BudgetAttributes counts cumulative input attributes.
	BudgetAttributes BudgetKind = "attributes"
	// BudgetEvents counts cumulative raw-token or fact-event work. The explicitly
	// selected internal-DTD source profile also charges replacement UTF-8 bytes,
	// tokenized-normalization bytes and entity/frame invocations before processing.
	// Internal expansion is not external or original-input byte traffic.
	BudgetEvents BudgetKind = "events"
	// BudgetScalarBytes is a per-item scalar byte cap, not a cumulative quota.
	BudgetScalarBytes BudgetKind = "scalar-bytes"
	// BudgetIDs counts cumulative retained identity entries.
	BudgetIDs BudgetKind = "ids"
	// BudgetRuleSteps counts cumulative evaluation work units.
	BudgetRuleSteps BudgetKind = "rule-steps"
	// BudgetContextFacts counts cumulative context facts.
	BudgetContextFacts BudgetKind = "context-facts"
	// BudgetLinkedDocuments counts cumulative nonroot adoptions, including
	// aliases and package-internal container/primary adoptions.
	BudgetLinkedDocuments BudgetKind = "linked-documents"
	// BudgetDiagnostics counts cumulative stored diagnostics.
	BudgetDiagnostics BudgetKind = "diagnostics"
	// BudgetDiagnosticBytes counts cumulative stored diagnostic bytes.
	BudgetDiagnosticBytes BudgetKind = "diagnostic-bytes"
)

const (
	budgetKindCount      = 14
	budgetNodesIndex     = 4
	budgetRuleStepsIndex = 9
)

type budgetMode uint8

const (
	budgetCumulative budgetMode = iota
	budgetGauge
	budgetItem
)

func budgetInfo(kind BudgetKind) (int, budgetMode, bool) {
	switch kind {
	case BudgetInputBytes:
		return 0, budgetCumulative, true
	case BudgetExternalBytes:
		return 1, budgetCumulative, true
	case BudgetBufferedBytes:
		return 2, budgetGauge, true
	case BudgetDepth:
		return 3, budgetGauge, true
	case BudgetNodes:
		return budgetNodesIndex, budgetCumulative, true
	case BudgetAttributes:
		return 5, budgetCumulative, true
	case BudgetEvents:
		return 6, budgetCumulative, true
	case BudgetScalarBytes:
		return 7, budgetItem, true
	case BudgetIDs:
		return 8, budgetCumulative, true
	case BudgetRuleSteps:
		return budgetRuleStepsIndex, budgetCumulative, true
	case BudgetContextFacts:
		return 10, budgetCumulative, true
	case BudgetLinkedDocuments:
		return 11, budgetCumulative, true
	case BudgetDiagnostics:
		return 12, budgetCumulative, true
	case BudgetDiagnosticBytes:
		return 13, budgetCumulative, true
	default:
		return 0, 0, false
	}
}

func budgetValues(b *Budgets) [budgetKindCount]*int64 {
	return [budgetKindCount]*int64{&b.MaxInputBytes, &b.MaxExternalBytes, &b.MaxBufferedBytes, &b.MaxDepth, &b.MaxNodes, &b.MaxAttributes, &b.MaxEvents, &b.MaxScalarBytes, &b.MaxIDs, &b.MaxRuleSteps, &b.MaxContextFacts, &b.MaxLinkedDocuments, &b.MaxDiagnostics, &b.MaxDiagnosticBytes}
}

// Account is a concurrency-safe resource account shared by an assessment and
// its children. Only NewBudgetAccount initializes an account; its zero value is
// invalid. Its limits cannot be reset or widened; only the default rule-step
// limit grows, with the nodes and document bytes the account charges, so a
// refused rule-step charge can succeed after further input. Do not copy an
// Account after first use; share its pointer instead. Callers must release
// only live resources they acquired, and must never refund cumulative work.
type Account struct {
	mu          sync.Mutex
	initialized bool
	limits      Budgets
	maxima      [budgetKindCount]int64
	usage       [budgetKindCount]int64
	// stepsPerNode is nonzero when the rule-step limit grows with charged nodes.
	stepsPerNode int64
	// documentBytes counts the XML document bytes read on the account.
	documentBytes int64
	// stepPeak is the highest growing rule-step limit reached: the byte cap
	// starts with the first document byte and must not lower the limit below
	// what nodes charged before it already allowed.
	stepPeak int64
}

// NewBudgetAccount validates limits and substitutes finite defaults for zeros.
// A zero MaxRuleSteps stays zero in Limits and selects the growing default.
// Large cumulative limits remain int64; each concrete allocation is validated
// separately before int conversion.
func NewBudgetAccount(limits Budgets) (*Account, error) {
	defaults := DefaultBudgets()
	fields, fallback := budgetValues(&limits), budgetValues(&defaults)
	names := [budgetKindCount]string{"MaxInputBytes", "MaxExternalBytes", "MaxBufferedBytes", "MaxDepth", "MaxNodes", "MaxAttributes", "MaxEvents", "MaxScalarBytes", "MaxIDs", "MaxRuleSteps", "MaxContextFacts", "MaxLinkedDocuments", "MaxDiagnostics", "MaxDiagnosticBytes"}
	var maxima [budgetKindCount]int64
	for i, field := range fields {
		if *field < 0 {
			return nil, &ConfigurationError{Field: names[i], Reason: "must not be negative"}
		}
		if *field == 0 {
			*field = *fallback[i]
		}
		maxima[i] = *field
	}
	account := &Account{initialized: true, limits: limits, maxima: maxima}
	if limits.MaxRuleSteps == 0 {
		account.maxima[budgetRuleStepsIndex] = DefaultRuleStepBase
		account.stepsPerNode = DefaultRuleStepsPerNode
	}
	return account, nil
}

// limit returns the current maximum of a resource. The growing rule-step
// default saturates at math.MaxInt64.
func (a *Account) limit(index int) int64 {
	maximum := a.maxima[index]
	if index != budgetRuleStepsIndex || a.stepsPerNode == 0 {
		return maximum
	}
	allowance := saturatingProduct(a.usage[budgetNodesIndex], a.stepsPerNode)
	if a.documentBytes != 0 {
		allowance = min(allowance, saturatingProduct(a.documentBytes, DefaultRuleStepsPerDocumentByte))
	}
	a.stepPeak = max(a.stepPeak, saturatingSum(maximum, allowance))
	return a.stepPeak
}

// saturatingProduct multiplies nonnegative values, saturating at math.MaxInt64.
func saturatingProduct(a, b int64) int64 {
	if b != 0 && a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
}

// saturatingSum adds nonnegative values, saturating at math.MaxInt64.
func saturatingSum(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// Limits returns a value copy of the normalized limits, or zero for an invalid
// account. A zero MaxRuleSteps, which selects the growing default, stays
// zero; Limit reports the current rule-step maximum.
func (a *Account) Limits() Budgets {
	if a == nil || !a.initialized {
		return Budgets{}
	}
	return a.limits
}

func (a *Account) validate(kind BudgetKind, amount int64) (int, budgetMode, error) {
	if a == nil || !a.initialized {
		return 0, 0, &ConfigurationError{Field: "Account", Reason: "must be created by NewBudgetAccount"}
	}
	index, mode, ok := budgetInfo(kind)
	if !ok {
		return 0, 0, &ConfigurationError{Field: "BudgetKind", Reason: "unknown resource kind"}
	}
	if amount < 0 {
		return 0, 0, &ConfigurationError{Field: "amount", Reason: "must not be negative"}
	}
	return index, mode, nil
}

// Charge consumes cumulative quota or acquires a live gauge amount. Exact-limit
// charges succeed. Failure, including overflow, never changes usage.
func (a *Account) Charge(kind BudgetKind, amount int64) error {
	index, mode, err := a.validate(kind, amount)
	if err != nil {
		return err
	}
	if mode == budgetItem {
		return &ConfigurationError{Field: "Charge", Reason: "per-item resources require Check"}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if limit := a.limit(index); amount > limit-a.usage[index] {
		return &ResourceError{Kind: kind, Limit: limit, Used: a.usage[index], Requested: amount}
	}
	a.usage[index] += amount
	return nil
}

// Release relinquishes a live depth or buffer gauge. It rejects cumulative and
// per-item resources, negative amounts, and amounts above current live usage.
func (a *Account) Release(kind BudgetKind, amount int64) error {
	index, mode, err := a.validate(kind, amount)
	if err != nil {
		return err
	}
	if mode != budgetGauge {
		return &ConfigurationError{Field: "Release", Reason: "only live gauges can be released"}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if amount > a.usage[index] {
		return &ConfigurationError{Field: "Release", Reason: "amount exceeds live usage"}
	}
	a.usage[index] -= amount
	return nil
}

// Check tests a per-item scalar cap without consuming any shared quota. It is
// invalid for cumulative quotas and gauges; use Charge for those resources.
func (a *Account) Check(kind BudgetKind, amount int64) error {
	index, mode, err := a.validate(kind, amount)
	if err != nil {
		return err
	}
	if mode != budgetItem {
		return &ConfigurationError{Field: "Check", Reason: "only per-item resources can be checked"}
	}
	if amount > a.maxima[index] {
		return &ResourceError{Kind: kind, Limit: a.maxima[index], Requested: amount}
	}
	return nil
}

// Usage returns a concurrency-safe snapshot of one resource's usage. A per-item
// resource always has zero usage, since Check never consumes quota.
func (a *Account) Usage(kind BudgetKind) (int64, error) {
	index, _, err := a.validate(kind, 0)
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.usage[index], nil
}

// chargeDocumentBytes records XML document bytes the parser has read, which cap
// the default rule-step allowance. It never refuses: the byte limits apply
// where the bytes are charged.
func (a *Account) chargeDocumentBytes(amount int64) error {
	if _, _, err := a.validate(BudgetInputBytes, amount); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.limit(budgetRuleStepsIndex) // keep what the uncapped nodes allowed
	a.documentBytes = saturatingSum(a.documentBytes, amount)
	return nil
}

// Limit returns one resource's current maximum, safe for concurrent use.
// It equals the normalized limit except for the default rule-step limit, which
// includes the allowance for the nodes and document bytes charged so far and
// grows with them.
func (a *Account) Limit(kind BudgetKind) (int64, error) {
	index, _, err := a.validate(kind, 0)
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.limit(index), nil
}

// allocationSize converts a nonnegative byte allocation to the host int size
// only after checking its range. It does not allocate or charge an account;
// callers must first apply the appropriate scalar and retained-buffer limits.
func allocationSize(bytes int64) (int, error) {
	if bytes < 0 {
		return 0, &ConfigurationError{Field: "allocation", Reason: "must not be negative"}
	}
	max := int64(math.MaxInt)
	if bytes > max {
		return 0, &ResourceError{Kind: BudgetBufferedBytes, Limit: max, Requested: bytes}
	}
	return int(bytes), nil
}

// chargeDiagnostic atomically retains a diagnostic and its encoded bytes. A
// failed pair cannot consume the count quota while refusing the byte quota.
func (a *Account) chargeDiagnostic(bytes int64) error {
	countIndex, _, err := a.validate(BudgetDiagnostics, 1)
	if err != nil {
		return err
	}
	byteIndex, _, err := a.validate(BudgetDiagnosticBytes, bytes)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, item := range []struct {
		kind   BudgetKind
		index  int
		amount int64
	}{{BudgetDiagnostics, countIndex, 1}, {BudgetDiagnosticBytes, byteIndex, bytes}} {
		if item.amount > a.maxima[item.index]-a.usage[item.index] {
			return &ResourceError{Kind: item.kind, Limit: a.maxima[item.index], Used: a.usage[item.index], Requested: item.amount}
		}
	}
	a.usage[countIndex]++
	a.usage[byteIndex] += bytes
	return nil
}
