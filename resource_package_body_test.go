package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func packageBodyRun(t *testing.T, ctx context.Context, options MXLOptions) *resourceGraphRun {
	t.Helper()
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	return &resourceGraphRun{ctx: ctx, options: resourceRunOptions{account: account}, mxlOptions: options, packages: map[validation.PackageID]*graphPackage{"inside": {id: "inside"}}}
}
func TestPackageBodyExactAndOneUnderCaps(t *testing.T) {
	for _, tc := range []struct {
		name           string
		options        MXLOptions
		representation validation.ResourceRepresentation
	}{
		{"XML document", MXLOptions{MaxDocumentBytes: 5}, validation.ResourceSourceXML},
		{"XML resource", MXLOptions{MaxResourceBytes: 5}, validation.ResourceSourceXML},
		{"MXL archive", MXLOptions{MaxArchiveBytes: 5}, validation.ResourceRawMXL},
		{"MXL resource", MXLOptions{MaxResourceBytes: 5}, validation.ResourceRawMXL},
		{"shared expanded", MXLOptions{MaxResourcesBytes: 5}, validation.ResourceRawMXL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, size := range []int{5, 6} {
				g := packageBodyRun(t, context.Background(), tc.options)
				body := &resourceTestBody{Reader: bytes.NewReader(bytes.Repeat([]byte{'x'}, size))}
				out, err := g.readExternalPackageBody(graphExpansion{pkg: "inside"}, validation.ResourcePin{Representation: tc.representation}, body)
				if size == 5 && (err != nil || out == nil) {
					t.Fatal(err)
				}
				if size == 6 && (!errors.Is(err, ErrMXLTooLarge) || out != nil) {
					t.Fatal(err)
				}
				if body.closed != 1 {
					t.Fatal(body.closed)
				}
				if out != nil {
					out.close()
				}
				live, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
				if live != 0 {
					t.Fatal(live)
				}
				if g.mxlExpanded != 5 {
					t.Fatal(g.mxlExpanded)
				}
			}
		})
	}
}
func TestPackageBodyAggregateAndPlainXMLException(t *testing.T) {
	g := packageBodyRun(t, context.Background(), MXLOptions{MaxResourcesBytes: 5, MaxDocumentBytes: 3})
	for i := 0; i < 2; i++ {
		body := &resourceTestBody{Reader: bytes.NewReader([]byte("abc"))}
		out, err := g.readExternalPackageBody(graphExpansion{pkg: "inside"}, validation.ResourcePin{Representation: validation.ResourceSourceXML}, body)
		if i == 0 {
			if err != nil {
				t.Fatal(err)
			}
			out.close()
		} else if !errors.Is(err, ErrMXLTooLarge) {
			t.Fatal(err)
		}
	}
	body := &resourceTestBody{Reader: bytes.NewReader([]byte("uncapped plain XML"))}
	out, err := g.readExternalPackageBody(graphExpansion{pkg: "plain"}, validation.ResourcePin{Representation: validation.ResourceSourceXML}, body)
	if err != nil {
		t.Fatal(err)
	}
	out.close()
	if g.mxlExpanded != 5 {
		t.Fatal(g.mxlExpanded)
	}
}
func TestPackageBodyCancellationJoinedCausesAndClose(t *testing.T) {
	boom := errors.New("transport")
	closed := errors.New("close")
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		g := packageBodyRun(t, ctx, MXLOptions{})
		body := &resourceTestBody{Reader: &resourceReadError{data: []byte("abc"), err: errors.Join(io.EOF, boom)}, closeErr: closed}
		out, err := g.readExternalPackageBody(graphExpansion{pkg: "inside"}, validation.ResourcePin{Representation: validation.ResourceRawMXL}, body)
		cancel()
		if out != nil || !errors.Is(err, closed) || body.closed != 1 {
			t.Fatal(out, err, body.closed)
		}
		if cancelled && !errors.Is(err, context.Canceled) || !cancelled && !errors.Is(err, boom) {
			t.Fatal(err)
		}
		live, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
		if live != 0 {
			t.Fatal(live)
		}
	}
}
