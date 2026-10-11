package xsd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type scalarMeter struct {
	steps, calls, retained int64
	limit                  int64
	cause                  error
	reserveFailure         error
	cancel                 context.CancelFunc
	cancelAt               int64
}

func (m *scalarMeter) Step(ctx context.Context, n int64) error {
	m.calls++
	if m.cancel != nil && m.calls == m.cancelAt {
		m.cancel()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.limit > 0 && n > m.limit-m.steps {
		return m.cause
	}
	m.steps += n
	return nil
}
func (m *scalarMeter) Reserve(n int64) error {
	if m.reserveFailure != nil {
		return m.reserveFailure
	}
	m.retained += n
	return nil
}
func (m *scalarMeter) Release(n int64) error { m.retained -= n; return nil }
func (*scalarMeter) Scalar(int64) error      { return nil }
func (*scalarMeter) Identity(int64) error    { return nil }
func scalarTestContext(m *scalarMeter, ctx context.Context) *Context {
	return &Context{run: &Run{meter: m, current: ctx}}
}
func scalarAbort(t *testing.T, cause error, operation func()) {
	t.Helper()
	defer func() {
		value := recover()
		stop, ok := value.(runAbort)
		if !ok || stop.err != cause {
			t.Errorf("abort = %#v; want original cause %v", value, cause)
		}
	}()
	operation()
}
func TestScalarMeterNumberScan(t *testing.T) {
	cause := errors.New("number work exhausted")
	m := &scalarMeter{limit: 64, cause: cause}
	c := scalarTestContext(m, context.Background())
	value := strings.Repeat("1", 4096)
	scalarAbort(t, cause, func() { c.AssessBuiltinValue("decimal", value, value) })
}
func TestScalarMeterComparisonAbort(t *testing.T) {
	cause := errors.New("comparison exhausted")
	m := &scalarMeter{limit: 64, cause: cause}
	c := scalarTestContext(m, context.Background())
	value := strings.Repeat("1", 4096)
	scalarAbort(t, cause, func() { c.AggregateValuesEqual(BuiltinSemantics("decimal"), value, value) })
}
func TestScalarMeterListReserve(t *testing.T) {
	cause := errors.New("list allocation exhausted")
	m := &scalarMeter{reserveFailure: cause}
	c := scalarTestContext(m, context.Background())
	scalarAbort(t, cause, func() { c.AssessBuiltinValue("NMTOKENS", "one two three", "one two three") })
}
func TestScalarMeterCancellationDuringSemantics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &scalarMeter{cancel: cancel, cancelAt: 4}
	c := scalarTestContext(m, ctx)
	schema := &SimpleSchema{Form: SimpleUnion}
	for range 64 {
		schema.Members = append(schema.Members, &SimpleMember{Name: QName{Space: XSDNamespace, Local: "string"}})
	}
	scalarAbort(t, context.Canceled, func() { c.ResolveSimpleSemantics(schema) })
	if m.calls != 4 {
		t.Errorf("checkpoints = %d; want cancellation at fourth checkpoint", m.calls)
	}
}
func TestScalarLegacyNumberAllocations(t *testing.T) {
	value := strings.Repeat("0", 2048) + "123.4500"
	if got := testing.AllocsPerRun(10, func() {
		left, ok := ParseDecimal(value)
		if !ok || left.Compare(left) != 0 {
			panic("decimal mismatch")
		}
	}); got != 0 {
		t.Fatalf("decimal allocations=%v; want 0", got)
	}
	integer := strings.Repeat("0", 2048) + "123"
	if got := testing.AllocsPerRun(10, func() {
		left, ok := ParseInteger(integer)
		if !ok || left.Compare(left) != 0 {
			panic("integer mismatch")
		}
	}); got != 0 {
		t.Fatalf("integer allocations=%v; want 0", got)
	}
}

func TestScalarMeterDirectAlgorithms(t *testing.T) {
	text := strings.Repeat("7", 4096)
	list := SimpleValue{Domain: ListDomain, Items: make([]SimpleValue, 512)}
	for index := range list.Items {
		list.Items[index] = SimpleValue{Domain: StringDomain, Text: "same"}
	}
	tests := []struct {
		name string
		call func(*Context)
	}{
		{"integer scan", func(c *Context) { parseInteger(text, c) }},
		{"decimal scan", func(c *Context) { parseDecimal(text, c) }},
		{"integer comparison", func(c *Context) { Integer{Digits: text}.compare(Integer{Digits: text}, c) }},
		{"decimal comparison", func(c *Context) { Decimal{Fraction: text}.compare(Decimal{Fraction: text}, c) }},
		{"year modulo", func(c *Context) { xsdYearModulo(text, 400, c) }},
		{"list comparison", func(c *Context) { list.equal(list, c) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name + " exhausted")
			m := &scalarMeter{limit: 128, cause: cause}
			scalarAbort(t, cause, func() { test.call(scalarTestContext(m, context.Background())) })
			if m.calls == 0 {
				t.Fatal("no work checkpoint")
			}
		})
	}
}

func TestScalarMeterURIConstructionLoop(t *testing.T) {
	text := strings.Repeat("a", 4096)
	cause := errors.New("URI construction exhausted")
	// UTF-8 and fragment scans are prepaid; the following byte loop must check
	// its own actual progress, rather than merely charging the whole operation.
	m := &scalarMeter{limit: int64(2*len(text) + 128), cause: cause}
	scalarAbort(t, cause, func() { validXSDAnyURI(text, scalarTestContext(m, context.Background())) })
	if m.steps != m.limit {
		t.Fatalf("work=%d; want byte-loop exhaustion at %d", m.steps, m.limit)
	}
}

func TestScalarMeterPostRegexpCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// One short input prepayment precedes the VM; the second Step is the
	// immediate post-VM cancellation check.
	m := &scalarMeter{cancel: cancel, cancelAt: 2}
	scalarAbort(t, context.Canceled, func() { scalarPatternMatch(XSDNamePattern, "validName", scalarTestContext(m, ctx)) })
	if m.calls != 2 {
		t.Fatalf("regexp checkpoints=%d; want 2", m.calls)
	}
}

func TestScalarMeterSemanticsAbortDoesNotInventEpoch(t *testing.T) {
	cause := errors.New("metadata interrupted")
	c := scalarTestContext(&scalarMeter{limit: 1, cause: cause}, context.Background())
	c.AssessmentVersion = 7
	scalarAbort(t, cause, func() { c.SimpleTypeSemantics(&TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}) })
	if c.AssessmentVersion != 7 {
		t.Fatalf("abort changed technical epoch to %d", c.AssessmentVersion)
	}
}

func TestScalarMeterCanonicalAllocation(t *testing.T) {
	for _, name := range []string{"integer", "decimal"} {
		t.Run(name, func(t *testing.T) {
			cause := errors.New("canonical allocation refused")
			c := scalarTestContext(&scalarMeter{reserveFailure: cause}, context.Background())
			scalarAbort(t, cause, func() { canonicalBuiltin(name, "-123", c) })
		})
	}
}

func TestScalarMeterControlledLexicalParity(t *testing.T) {
	tests := []struct {
		name       string
		values     []string
		legacy     func(string) bool
		controlled func(string, *Context) bool
	}{
		{"URI", []string{"", "a b", "https://example.test/a%20b#x", "//[::ffff:192.168.001.001]/", "//[::1%25zone]/", "a%z0", "a#b#c", "\xff"}, ValidXSDAnyURI, validXSDAnyURI},
		{"Name", []string{"", "alpha", "éclair", "a:b", ":x", "1x", "\xff"}, ValidXSDName, validXSDName},
		{"NCName", []string{"", "alpha", "éclair", "a:b", ":x", "1x", "\xff"}, ValidXSDNCName, validXSDNCName},
		{"QName", []string{"", "alpha", "éclair", "a:b", "a:b:c", ":x", "1x", "\xff"}, ValidXSDQName, validXSDQName},
		{"NMTOKEN", []string{"", "alpha", "éclair", "a:b", ":x", "1x", "\xff"}, ValidXSDNMTOKEN, validXSDNMTOKEN},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range test.values {
				c := scalarTestContext(&scalarMeter{}, context.Background())
				if got, want := test.controlled(value, c), test.legacy(value); got != want {
					t.Errorf("%q: controlled=%v legacy=%v", value, got, want)
				}
			}
		})
	}
	for _, test := range []struct{ name, value string }{
		{"date", "2024-02-29"}, {"date", "2023-02-29"}, {"dateTime", "2024-02-29T24:00:00Z"},
		{"gYear", strings.Repeat("9", 1024)}, {"gYear", "0000"}, {"duration", "P3Y2M"}, {"duration", "P"}, {"time", "24:00:00.000"},
	} {
		c := scalarTestContext(&scalarMeter{}, context.Background())
		if got, want := validXSDDateTime(test.name, test.value, c), ValidXSDDateTime(test.name, test.value); got != want {
			t.Errorf("%s %q: controlled=%v legacy=%v", test.name, test.value, got, want)
		}
	}
}

type scalarLimitMeter struct {
	scalarMeter
	scalarLimit int64
	scalarCause error
}

func (m *scalarLimitMeter) Scalar(n int64) error {
	if n > m.scalarLimit {
		return m.scalarCause
	}
	return nil
}
func TestScalarMeterLiteralSizeBeforeScan(t *testing.T) {
	cause := errors.New("literal scalar bound")
	m := &scalarLimitMeter{scalarLimit: 8, scalarCause: cause}
	c := &Context{run: &Run{meter: m, current: context.Background()}}
	scalarAbort(t, cause, func() { parseInteger(strings.Repeat("1", 9), c) })
	if m.steps != 0 {
		t.Fatalf("oversized literal performed %d work before size rejection", m.steps)
	}
}
func TestScalarMeterCanonicalOutputSize(t *testing.T) {
	cause := errors.New("canonical scalar bound")
	m := &scalarLimitMeter{scalarLimit: 1, scalarCause: cause}
	c := &Context{run: &Run{meter: m, current: context.Background()}}
	scalarAbort(t, cause, func() { canonicalBuiltin("decimal", "1", c) })
	if m.retained != 0 {
		t.Fatalf("oversized canonical output allocated %d retained bytes", m.retained)
	}
}

func TestScalarMeterMalformedWhitespaceExpansion(t *testing.T) {
	cause := errors.New("normalized scalar bound")
	m := &scalarLimitMeter{scalarLimit: 2, scalarCause: cause}
	c := &Context{run: &Run{meter: m, current: context.Background()}}
	scalarAbort(t, cause, func() { collapseWhitespace("\xff", c) })
	if m.retained != 0 {
		t.Fatalf("oversized normalized output allocated %d bytes before size rejection", m.retained)
	}
}
