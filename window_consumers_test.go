package musicxml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// windowConsumerRun is what one windowed run settled of the decisions its
// windows made provisionally: how many Finish adopted, how many it dropped
// and for which assumption, with the report and the run's rule steps and
// context facts.
type windowConsumerRun struct {
	adopted, dropped int
	failed           string
	report           string
	steps, facts     int64
	// account is the run's account, to read after the run returned.
	account *validation.Account
}

// windowConsumerRuns runs one semantics line on data and returns what the
// run settled. A buffered run settles nothing.
func windowConsumerRuns(t *testing.T, line string, data []byte, document Document, windowed bool) windowConsumerRun {
	t.Helper()
	return windowConsumerRunsWithin(t, line, data, document, windowed, validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34})
}

func windowConsumerRunsWithin(t *testing.T, line string, data []byte, document Document, windowed bool, big validation.Budgets) windowConsumerRun {
	t.Helper()
	var run windowConsumerRun
	previousStrategy := validationSetWindowedStrategy(windowed)
	defer validationSetWindowedStrategy(previousStrategy)
	previous := validationSetWindowConsumersObserver(func(failed string, decisions int) {
		if failed == "" {
			run.adopted += decisions
		} else {
			run.dropped += decisions
			run.failed = failed
		}
	})
	defer validationSetWindowConsumersObserver(previous)
	previousFinish := validationSetFinishObserver(func(stage string, account *validation.Account) {
		run.account = account
		if stage == "finished" {
			run.steps, _ = account.Usage(validation.BudgetRuleSteps)
			run.facts, _ = account.Usage(validation.BudgetContextFacts)
		}
	})
	defer validationSetFinishObserver(previousFinish)
	var result any
	if line == "source" {
		result, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
	} else {
		result, _ = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	run.report = string(encoded)
	return run
}

// Under the windowed strategy the chord duration bound, the key a numeral
// requires and the barline location are decided as each measure's window
// closes, and Finish adopts the decisions when their assumptions hold. On the
// corpus it adopts them for at least nine documents in ten; a document whose
// timeline Finish reads again or rebuilds drops them, counted by the
// assumption that failed, and its phases decide the rules.
func TestWindowConsumersOnCorpus(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	documents, adoptedDocuments, adopted, dropped := 0, 0, 0, 0
	failed := map[string]int{}
	var steps, bufferedSteps int64
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		run := windowConsumerRuns(t, "source", data, nil, true)
		if run.adopted+run.dropped == 0 {
			continue
		}
		documents++
		adopted += run.adopted
		dropped += run.dropped
		if run.failed != "" {
			failed[run.failed]++
			continue
		}
		adoptedDocuments++
		buffered := windowConsumerRuns(t, "source", data, nil, false)
		if buffered.facts != run.facts {
			t.Errorf("%s: %d context facts buffered, %d windowed", files[i], buffered.facts, run.facts)
		}
		steps += run.steps
		bufferedSteps += buffered.steps
	}
	t.Logf("%d documents with provisional decisions, %d adopted (%d decisions), %d decisions dropped by %v; adopted runs take %d rule steps, buffered %d", documents, adoptedDocuments, adopted, dropped, failed, steps, bufferedSteps)
	if documents == 0 || 10*adoptedDocuments < 9*documents {
		t.Fatalf("%d of %d documents adopted their provisional decisions", adoptedDocuments, documents)
	}
	// Adopting costs far less than deciding again: the windowed runs take
	// under a hundredth more steps than the buffered ones.
	if 100*steps > 101*bufferedSteps {
		t.Fatalf("adopted runs take %d rule steps, buffered %d", steps, bufferedSteps)
	}
}

const (
	consumerAttr     = `<attributes><divisions>2</divisions></attributes>`
	consumerKeyAttr  = `<attributes><divisions>2</divisions><key><fifths>0</fifths></key></attributes>`
	consumerNote     = `<note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration></note>`
	consumerChord    = `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>2</duration></note>`
	consumerLong     = `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>4</duration></note>`
	consumerNumeral  = `<harmony><numeral><numeral-root text="I">1</numeral-root></numeral><kind>major</kind></harmony>`
	consumerLocalKey = `<harmony><numeral><numeral-root text="I">1</numeral-root><numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>major</numeral-mode></numeral-key></numeral><kind>major</kind></harmony>`
	consumerKey      = `<attributes><key><fifths>1</fifths></key></attributes>`
	// consumerMisplaced fails each of the three rules.
	consumerMisplaced = consumerNumeral + consumerNote + consumerLong + `<barline location="left"/>`
)

// windowConsumerCases are documents chosen for the risks of deciding the
// timeline consumers as windows close, each with what Finish does with the
// provisional decisions ("" adopts them, "none" means windows made none),
// how many the windows of the source line made, and a finding the source
// report holds.
var windowConsumerCases = []struct {
	name, settled string
	decisions     int
	finding, data string
}{
	{"chords within and beyond their principal", "", 4, "chord-duration-exceeds-principal", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerChord, consumerNote+consumerLong))},
	{"principal after a backup", "", 3, "chord-duration-exceeds-principal", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+`<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration></note><backup><duration>4</duration></backup><note><pitch><step>G</step><octave>3</octave></pitch><duration>1</duration></note>`+consumerChord))},
	{"barlines inside and at the ends of measures", "", 11, "chord-duration-exceeds-principal", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerLong+`<barline location="middle"/>`+consumerNote+`<barline location="right"/>`, `<barline location="left"/>`+consumerNote+consumerChord+`<barline location="left"/>`+consumerNote+`<barline location="right"/>`))},
	{"numeral without a key", "", 3, "numeral-key-required", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNumeral+consumerNote, consumerNote))},
	{"numeral under a key", "", 4, "", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerKeyAttr+consumerNumeral+consumerNote, consumerNumeral+consumerNote))},
	{"numeral with its own key", "", 1, "", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerLocalKey+consumerNote))},
	{"key later in the measure and in the next", "", 5, "numeral-key-required", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNumeral+consumerNote+consumerKey+consumerNote, consumerKey+consumerNumeral+consumerNote))},
	{"key on one staff of two", "", 3, "numeral-key-required", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", `<attributes><divisions>2</divisions><key number="1"><fifths>0</fifths></key><staves>2</staves></attributes>`+strings.Replace(consumerNumeral, `</harmony>`, `<staff>2</staff></harmony>`, 1)+strings.Replace(consumerNumeral, `</harmony>`, `<staff>1</staff></harmony>`, 1)+consumerNote))},
	{"invalid key in a later measure", "", 2, "numeral-key-required", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNumeral+consumerNote, `<attributes><key><fifths>x</fifths></key></attributes>`+consumerNote))},
	{"key after a backup in a later measure", "", 4, "numeral-key-required", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNumeral+consumerNote, consumerNote+`<backup><duration>2</duration></backup>`+consumerKey+consumerNote))},
	// A later backup before its measure's start leaves the order of the
	// key it precedes and the earlier numeral unresolved: the answer the
	// numeral's window read changes, and Finish rebuilds the timeline.
	{"key displaced before an earlier numeral", "rebuilt", 2, "timeline-state-order-unresolved", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNumeral+consumerNote, consumerNote+`<backup><duration>8</duration></backup>`+consumerKey+consumerNote))},
	{"divisions change behind a backup", "rebuilt", 2, "divisions-rewind-placement-unresolved", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerChord, `<attributes><divisions>4</divisions></attributes><backup><duration>8</duration></backup>`+consumerNote+consumerLong))},
	{"excursion in the first measure", "none", 0, "divisions-rewind-placement-unresolved", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerChord+`<backup><duration>6</duration></backup>`+consumerNote+`<attributes><divisions>4</divisions></attributes>`+consumerNote+consumerLong, consumerNote))},
	{"part naming no score-part", "speculation", 2, "chord-duration-exceeds-principal", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerLong)+timelineWindowPart("P9", consumerAttr+consumerNote))},
	// More findings than a tight diagnostic quota stores, in the phases of
	// all three rules: the first and the last two of the profile.
	{"failures in every measure", "", 12, "barline-location-mismatch", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerMisplaced, consumerMisplaced, consumerMisplaced))},
	// A measure whose binding proves a failure waits for Finish while its
	// neighbours' decisions are adopted: the findings keep their node order.
	{"invalid attribute in a middle measure", "", 4, "chord-duration-exceeds-principal", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerLong, `<note dynamics="abc"><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration></note>`+consumerLong, consumerNote+consumerLong))},
	// The decoded model has no stray element; only the source line reads it.
	{"content error in a later measure", "speculation", 2, "", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote+consumerLong, consumerNote+`<stray/>`))},
	{"timewise", "none", 0, "", windowedTimewiseScore},
}

// Each risk of deciding the timeline consumers as windows close has a
// document: Finish adopts or drops the provisional decisions as named, and
// every report, with and without the complete projection, is byte-identical
// to buffered evaluation's, as is the cumulative context-fact quota. Input
// cut off inside a measure settles nothing and records none of them.
func TestWindowConsumersRiskCases(t *testing.T) {
	check := func(t *testing.T, name, settled string, decisions int, finding string, data []byte) {
		document, decodeErr := Decode(bytes.NewReader(data))
		for _, line := range []string{"source", "model"} {
			if line == "model" && (decodeErr != nil || strings.Contains(string(data), "<stray/>")) {
				continue
			}
			run := windowConsumerRuns(t, line, data, document, true)
			got := run.failed
			if run.adopted+run.dropped == 0 {
				got = "none"
			}
			if got != settled || (settled == "") != (run.adopted != 0) || line == "source" && run.adopted+run.dropped != decisions {
				t.Errorf("%s: settled %q with %d adopted and %d dropped, want %q with %d", line, got, run.adopted, run.dropped, settled, decisions)
			}
			if !strings.Contains(run.report, finding) {
				t.Errorf("%s: no finding %q", line, finding)
			}
			buffered := windowConsumerRuns(t, line, data, document, false)
			if buffered.report != run.report {
				t.Errorf("%s: windowed report differs from buffered at %s", line, windowedDifference(buffered.report, run.report))
			}
			if buffered.facts != run.facts {
				t.Errorf("%s: %d context facts buffered, %d windowed", line, buffered.facts, run.facts)
			}
		}
		buffered := windowedReports(t, name, data)
		previous := validationSetWindowedStrategy(true)
		windowed := windowedReports(t, name, data)
		validationSetWindowedStrategy(previous)
		for label, want := range buffered {
			if windowed[label] != want {
				t.Errorf("%s: windowed report differs from buffered at %s", label, windowedDifference(want, windowed[label]))
			}
		}
	}
	for _, tc := range windowConsumerCases {
		t.Run(tc.name, func(t *testing.T) { check(t, tc.name, tc.settled, tc.decisions, tc.finding, []byte(tc.data)) })
	}
	t.Run("cut inside a measure", func(t *testing.T) {
		whole := []byte(windowConsumerCases[0].data)
		check(t, "cut", "unsettled", 2, "", whole[:bytes.LastIndex(whole, []byte(`<note>`))+20])
	})
}

// Finish asks again every key presence a numeral's window asked, when it
// reaches the numeral rule's phase. No document found changes one without
// rebuilding its timeline, so the test changes the first answer directly:
// the numeral's decisions are dropped for it, after the earlier phases
// adopted theirs, and the phase then decides the rule to the report
// buffered evaluation gives.
func TestWindowConsumersRecheck(t *testing.T) {
	data := []byte(windowConsumerCases[3].data)
	whole := windowConsumerRuns(t, "source", data, nil, true)
	if whole.failed != "" || whole.adopted == 0 {
		t.Fatalf("untampered: %d adopted, %d dropped by %q", whole.adopted, whole.dropped, whole.failed)
	}
	buffered := windowConsumerRuns(t, "source", data, nil, false)
	previous := validationSetWindowConsumersTamper("query")
	run := windowConsumerRuns(t, "source", data, nil, true)
	validationSetWindowConsumersTamper(previous)
	if run.failed != "query" || run.dropped == 0 || run.adopted+run.dropped != whole.adopted {
		t.Fatalf("%d adopted, %d dropped by %q, want some of %d dropped by a query", run.adopted, run.dropped, run.failed, whole.adopted)
	}
	if run.report != buffered.report {
		t.Fatalf("windowed report differs from buffered at %s", windowedDifference(buffered.report, run.report))
	}
}

// A context-fact quota ends Finish in a phase between those of the three
// rules. Whatever the quota, the report is byte-identical to buffered
// evaluation's: a phase Finish does not reach records nothing windows
// decided in its place.
func TestWindowConsumersUnderContextFactQuotas(t *testing.T) {
	for _, name := range []string{"failures in every measure"} {
		var data []byte
		for _, tc := range windowConsumerCases {
			if tc.name == name {
				data = []byte(tc.data)
			}
		}
		document, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range []string{"source", "model"} {
			whole := windowConsumerRuns(t, line, data, document, true)
			if whole.adopted == 0 || whole.facts == 0 {
				t.Fatalf("%s %s: %d decisions adopted, %d context facts", name, line, whole.adopted, whole.facts)
			}
			partial := 0
			// Every quota with MUSICXML_WINDOWED_FULL=1, every fourth by
			// default.
			for limit := int64(1); limit <= whole.facts+1; limit += int64(windowedStride()) {
				budgets := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34, MaxContextFacts: limit}
				buffered := windowConsumerRunsWithin(t, line, data, document, false, budgets)
				windowed := windowConsumerRunsWithin(t, line, data, document, true, budgets)
				if buffered.report != windowed.report {
					t.Fatalf("%s %s, %d context facts allowed: windowed report differs from buffered at %s", name, line, limit, windowedDifference(buffered.report, windowed.report))
				}
				if windowed.adopted != 0 && windowed.dropped != 0 {
					partial++
				}
			}
			// Some quota ends Finish after one phase adopted its decisions
			// and before another did.
			if partial == 0 {
				t.Errorf("%s %s: no quota ended Finish between two consuming phases", name, line)
			}
		}
	}
}

// Windows decide the three rules only when the timeline rule is the only
// prerequisite of each: a chord duration left waiting for another
// prerequisite would be read as undecided by the barline that evaluates it.
// With one more prerequisite on the chord duration rule, windows decide
// none of them, and the report is buffered evaluation's.
func TestWindowConsumersNeedOnlyTheirTimeline(t *testing.T) {
	data := []byte(windowConsumerCases[2].data)
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	extra := validation.Dependency{Kind: validation.DependencyRule, Rule: validation.RuleKey{ID: "MX40-LOCAL-part-id-target/use-1", Revision: 1}, Relation: validation.RelationDocument}
	for _, line := range []string{"source", "model"} {
		target, engine := validation.TargetSource, &sourceSemanticEngine
		if line == "model" {
			target, engine = validation.TargetModel, &modelSemanticEngine
		}
		d, err := buildSemanticAssessmentDefinitions(target)
		if err != nil {
			t.Fatal(err)
		}
		changed := false
		for i := range d.rules {
			if d.rules[i].Key.ID == "MX40-TIMELINE-chord-duration-bound" {
				d.rules[i].Requires = append(append([]validation.Dependency(nil), d.rules[i].Requires...), extra)
				changed = true
			}
		}
		if !changed {
			t.Fatal("no chord duration rule")
		}
		catalog, err := newCatalog(d.rules, d.profiles, d.plans)
		if err != nil {
			t.Fatal(err)
		}
		custom, err := validation.NewEngine(catalog)
		if err != nil {
			t.Fatal(err)
		}
		if run := windowConsumerRuns(t, line, data, document, true); run.adopted == 0 {
			t.Fatalf("%s: the shipped catalog adopts %d decisions", line, run.adopted)
		}
		previous := *engine
		*engine = func() (*validation.Engine, error) { return custom, nil }
		windowed, buffered := func() (windowConsumerRun, windowConsumerRun) {
			defer func() { *engine = previous }()
			return windowConsumerRuns(t, line, data, document, true), windowConsumerRuns(t, line, data, document, false)
		}()
		if windowed.adopted+windowed.dropped != 0 {
			t.Errorf("%s: windows decided %d consumers", line, windowed.adopted+windowed.dropped)
		}
		if windowed.report != buffered.report {
			t.Errorf("%s: windowed report differs from buffered at %s", line, windowedDifference(buffered.report, windowed.report))
		}
	}
}

// What windows decide provisionally is live when Finish starts. On a
// document that fails the three rules in every measure, the heap it adds
// over a run that decides them in Finish is covered by what it charges.
func TestWindowConsumersChargeCoversTheHeap(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	part := func(first, measure string) string {
		measures := []string{first + measure}
		for i := 1; i < 300; i++ {
			measures = append(measures, measure)
		}
		return timelineWindowPart("P1", measures...)
	}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	for name, doc := range map[string]string{
		"failures": timelineWindowScore(timelineWindowBare, part(consumerAttr, consumerNumeral+consumerNumeral+consumerNote+strings.Repeat(consumerLong, 6)+`<barline location="left"/>`)),
	} {
		data := []byte(doc)
		measure := func(off bool) (heap, used int64, decisions int) {
			defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
			defer validationSetWindowConsumersOff(validationSetWindowConsumersOff(off))
			defer validationSetWindowConsumersObserver(validationSetWindowConsumersObserver(func(_ string, n int) { decisions += n }))
			run := func() {
				_, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
			}
			run()
			decisions = 0
			before := liveHeap()
			previous := validationSetFinishObserver(func(stage string, account *validation.Account) {
				if stage == "evaluating" {
					heap = liveHeap() - before
					used, _ = account.Usage(validation.BudgetBufferedBytes)
				}
			})
			run()
			validationSetFinishObserver(previous)
			return heap, used, decisions
		}
		offHeap, offUsed, _ := measure(true)
		onHeap, onUsed, decisions := measure(false)
		t.Logf("%s: %d provisional decisions hold %d live bytes and charge %d", name, decisions, onHeap-offHeap, onUsed-offUsed)
		if decisions < 3000 {
			t.Errorf("%s: %d provisional decisions", name, decisions)
		}
		if onHeap-offHeap > onUsed-offUsed+finishHeapNoise {
			t.Errorf("%s: provisional decisions hold %d live bytes and charge %d", name, onHeap-offHeap, onUsed-offUsed)
		}
	}
}

// keyedNumerals is a part whose every measure changes the key and asks for
// it at a numeral: each window's question builds the key history's index in
// the speculative timeline, and the next measure's key discards it.
func keyedNumerals(measures int, last string) string {
	all := make([]string, measures)
	for i := range all {
		all[i] = fmt.Sprintf(`<attributes><key><fifths>%d</fifths></key></attributes>`, i%7) + consumerNumeral + consumerNote
	}
	all[0] = consumerAttr + all[0]
	all[measures-1] += last
	return timelineWindowPart("P1", all...)
}

// Whatever Finish does with the provisional decisions, the session ends
// with nothing charged to the buffer, and its report is buffered
// evaluation's. An index a numeral's question builds in the speculative
// timeline is discarded by the next measure's read; were it not counted for
// the speculation, dropping the speculation would release bytes it never
// retained and leave the account unbalanced. That the count is exact is
// TestConsumerQuestionIndexBelongsToTheSpeculation's to prove.
func TestWindowConsumersLeaveNothingCharged(t *testing.T) {
	whole := timelineWindowScore(timelineWindowBare, keyedNumerals(80, ""))
	for _, tc := range []struct {
		name, settled, tamper, data string
	}{
		{"adopted", "", "", whole},
		{"speculation dropped", "speculation", "", timelineWindowScore(timelineWindowBare, keyedNumerals(80, "")+timelineWindowPart("P9", consumerAttr+consumerNote))},
		{"timeline rebuilt", "rebuilt", "", timelineWindowScore(timelineWindowBare, keyedNumerals(80, `<attributes><divisions>4</divisions></attributes><backup><duration>8</duration></backup>`+consumerNote))},
		{"key presence changed", "query", "query", whole},
		{"input cut", "unsettled", "", whole[:strings.LastIndex(whole, `<note>`)+20]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.data)
			charged := func(windowed bool) (windowConsumerRun, int64) {
				run := windowConsumerRuns(t, "source", data, nil, windowed)
				if run.account == nil {
					t.Fatal("no account observed")
				}
				used, _ := run.account.Usage(validation.BudgetBufferedBytes)
				return run, used
			}
			buffered, used := charged(false)
			if used != 0 {
				t.Fatalf("buffered: %d bytes charged after the run", used)
			}
			previous := validationSetWindowConsumersTamper(tc.tamper)
			windowed, used := charged(true)
			validationSetWindowConsumersTamper(previous)
			if windowed.failed != tc.settled || windowed.adopted+windowed.dropped == 0 {
				t.Errorf("settled %q with %d adopted and %d dropped, want %q", windowed.failed, windowed.adopted, windowed.dropped, tc.settled)
			}
			if used != 0 {
				t.Errorf("windowed: %d bytes charged after the run", used)
			}
			if windowed.report != buffered.report {
				t.Errorf("windowed report differs from buffered at %s", windowedDifference(buffered.report, windowed.report))
			}
			if tc.settled != "speculation" {
				return
			}
			// Through every loader, on accounts the session shares.
			reports := windowedReports(t, tc.name, data)
			previousStrategy := validationSetWindowedStrategy(true)
			windowedAll := windowedReports(t, tc.name, data)
			validationSetWindowedStrategy(previousStrategy)
			for label, want := range reports {
				if windowedAll[label] != want {
					t.Errorf("%s: windowed report differs from buffered at %s", label, windowedDifference(want, windowedAll[label]))
				}
			}
		})
	}
}

// A speculation dropped with less retained than nothing would release what
// it never counted and corrupt the buffer's account: the session fails
// loudly instead.
func TestDroppedSpeculationNeverReleasesMoreThanItRetained(t *testing.T) {
	data := []byte(timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", consumerAttr+consumerNote)+timelineWindowPart("P9", consumerAttr+consumerNote)))
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
	if _, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big}); err != nil {
		t.Fatal(err)
	}
	defer validationSetTimelineSpeculationTamper(validationSetTimelineSpeculationTamper("released"))
	if _, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big}); err == nil || !strings.Contains(err.Error(), "released more than it retained") {
		t.Fatalf("error %v", err)
	}
}

// With the test switch off, windows decide no timeline consumer and the
// phases of Finish decide them all, to the same report. Nor do windows
// decide them when their node rules wait for the root: the consumers'
// selector mismatches would then be counted twice.
func TestWindowConsumersOff(t *testing.T) {
	data := []byte(windowConsumerCases[2].data)
	on := windowConsumerRuns(t, "source", data, nil, true)
	if on.adopted == 0 {
		t.Fatal("no decision adopted")
	}
	for _, off := range []func(bool) bool{validationSetWindowConsumersOff, validationSetWindowRulesOff} {
		previous := off(true)
		run := windowConsumerRuns(t, "source", data, nil, true)
		off(previous)
		if run.adopted+run.dropped != 0 {
			t.Fatalf("%d decisions made by windows", run.adopted+run.dropped)
		}
		if on.report != run.report {
			t.Fatalf("reports differ at %s", windowedDifference(run.report, on.report))
		}
	}
}
