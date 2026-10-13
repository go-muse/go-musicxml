package musicxml

import "testing"

func TestSourceDTDReviewContentFrameClosure(t *testing.T) {
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY mt "<x>T</x>">]><r>&mt;</r>`,
		`<!DOCTYPE r [<!ENTITY outer "&inner;"><!ENTITY inner "A &amp; B">]><r>&outer;</r>`,
	} {
		_, _, summary, err := sourceEntityScan(t, source)
		if err != nil || !summary.SyntaxComplete {
			t.Errorf("want successful content closure: %s: err=%v summary=%+v", source, err, summary)
		}
	}
}
