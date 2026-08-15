package musicxml

import "fmt"

const maximumDocumentDepth = maximumXMLDepth

type opusDocumentVisit struct {
	document *OpusDocument
	depth    int
	leaving  bool
}

func checkDocumentNesting(document Document) error {
	root, ok := document.(*OpusDocument)
	if !ok || root == nil {
		return nil
	}

	active := make(map[*OpusDocument]struct{})
	stack := []opusDocumentVisit{{document: root, depth: 1}}
	for len(stack) != 0 {
		last := len(stack) - 1
		visit := stack[last]
		stack = stack[:last]
		if visit.document == nil {
			continue
		}
		if visit.leaving {
			delete(active, visit.document)
			continue
		}
		if visit.depth > maximumDocumentDepth {
			return fmt.Errorf(
				"%w: maximum is %d elements",
				ErrDocumentTooDeep,
				maximumDocumentDepth,
			)
		}
		if _, found := active[visit.document]; found {
			return ErrDocumentCycle
		}

		active[visit.document] = struct{}{}
		stack = append(stack, opusDocumentVisit{
			document: visit.document,
			leaving:  true,
		})
		for index := len(visit.document.Content) - 1; index >= 0; index-- {
			child := visit.document.Content[index].Opus
			if child == nil {
				continue
			}
			stack = append(stack, opusDocumentVisit{
				document: child,
				depth:    visit.depth + 1,
			})
		}
	}

	return nil
}
