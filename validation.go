package musicxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

// ValidationError reports XSD validation issues found in a document.
// For DecodeAssessed, Issues contains only stored normative failure details,
// bounded by the assessment diagnostic count and byte budgets. It may be empty
// if every detail was omitted. The returned operation error explains truncation;
// consult DecodeResult.Source for the independent result and rule-evaluation
// counts, which do not count individual issue details.
type ValidationError struct {
	Issues []ValidationIssue
}

// Error summarizes the validation failure.
func (e *ValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ErrInvalidDocument.Error()
	}
	if len(e.Issues) == 1 {
		return fmt.Sprintf("%s: %s", ErrInvalidDocument, e.Issues[0])
	}

	return fmt.Sprintf(
		"%s: %d issues; first: %s",
		ErrInvalidDocument,
		len(e.Issues),
		e.Issues[0],
	)
}

// Unwrap makes ValidationError match ErrInvalidDocument.
func (e *ValidationError) Unwrap() error {
	return ErrInvalidDocument
}

// Validate checks the current generated Go document against the bounded
// MusicXML 4.0 XSD model profile. It walks current fields directly; it does not
// encode the document, invoke XML marshal methods, or assess historical XML.
// No local musical prose clauses are selected. Decode and Encode remain explicit
// transport operations and do not invoke validation automatically.
//
// Genuine XSD and model-representation failures preserve ErrInvalidDocument and
// ValidationError. A reached unsupported capability or resource interruption is
// incomplete, not proof of invalidity. The legacy maximum depth is 4096 and its
// cycle/depth sentinels remain recognizable. Use AssessModel for separate result
// axes, explicit profiles and configurable finite budgets.
func Validate(document Document) error {
	report, status, err := assessModel(context.Background(), document,
		AssessmentOptions{Profile: ProfileModelXSD, Budgets: validation.Budgets{MaxDepth: maximumDocumentDepth}},
		modelWalkOptions{LegacyPaths: true})
	if !report.Requested {
		return err
	}
	var invalid error
	if report.Conformance == validation.ConformanceViolated || status.HasProblems {
		issues := make([]ValidationIssue, 0)
		for _, finding := range report.Findings {
			if finding.Outcome != validation.OutcomeFail {
				continue
			}
			switch finding.Category {
			case validation.CategoryNormative:
				issues = append(issues, ValidationIssue{Path: finding.Location.Path, Constraint: string(finding.Reason), Message: finding.Actual})
			case validation.CategoryModel:
				issues = append(issues, ValidationIssue{Path: finding.Location.Path, Constraint: "representation", Message: finding.Actual})
			}
		}
		invalid = &ValidationError{Issues: issues}
	}
	if status.HasCycle {
		invalid = errors.Join(invalid, fmt.Errorf("%w: %w", ErrInvalidDocument, ErrDocumentCycle))
	}
	if status.DepthExceeded {
		invalid = errors.Join(invalid, fmt.Errorf("%w: %w", ErrInvalidDocument, ErrDocumentTooDeep))
	}
	if !report.Satisfied() && (!report.InputComplete || !report.AssessmentComplete || report.Terminal != nil) {
		err = errors.Join(err, &validation.IncompleteError{Reason: "model-xsd-profile-not-satisfied"})
	}
	return errors.Join(invalid, err)
}

// Validate checks a partwise score against the MusicXML 4.0 XSD.
func (value *ScorePartwise) Validate() error {
	return Validate(value)
}

// Validate checks a timewise score against the MusicXML 4.0 XSD.
func (value *ScoreTimewise) Validate() error {
	return Validate(value)
}

// Validate checks an opus against the MusicXML 4.0 opus XSD.
func (value *OpusDocument) Validate() error {
	return Validate(value)
}

func validationSchemaForDocument(
	document Document,
) (*validationSchemaSet, error) {
	switch value := document.(type) {
	case nil:
		return nil, ErrNilDocument
	case *ScorePartwise:
		if value == nil {
			return nil, ErrNilDocument
		}
		return &scoreValidationSchema, nil
	case *ScoreTimewise:
		if value == nil {
			return nil, ErrNilDocument
		}
		return &scoreValidationSchema, nil
	case *OpusDocument:
		if value == nil {
			return nil, ErrNilDocument
		}
		return &opusValidationSchema, nil
	default:
		return nil, fmt.Errorf(
			"%w: %T",
			ErrUnsupportedDocument,
			document,
		)
	}
}

func validationDocumentPath(document Document) string {
	switch document.(type) {
	case *ScorePartwise:
		return "/score-partwise"
	case *ScoreTimewise:
		return "/score-timewise"
	case *OpusDocument:
		return "/opus"
	default:
		return "/"
	}
}

func parseValidationDocument(source []byte) (*validationNode, error) {
	tokens, err := newSourceXMLTokenReader(bytes.NewReader(source), sourceXMLConfig{Context: context.Background()})
	if err != nil {
		return nil, err
	}
	defer tokens.Close()
	decoder := xml.NewTokenDecoder(tokens)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyDocument
		}
		if err != nil {
			return nil, fmt.Errorf("read document root: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			root, err := readValidationNode(decoder, tokens, value, 1, map[string]string{"xml": "http://www.w3.org/XML/1998/namespace"})
			if err != nil {
				return nil, err
			}
			return root, readValidationTail(decoder)
		case xml.CharData:
			if len(bytes.Trim(value, xmlWhitespace)) != 0 {
				return nil, errors.New("unexpected character data before root")
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
		default:
			return nil, fmt.Errorf("unexpected %T before root", token)
		}
	}
}

func readValidationNode(
	decoder *xml.Decoder,
	tokens *sourceXMLTokenReader,
	start xml.StartElement,
	depth int,
	namespaces map[string]string,
) (*validationNode, error) {
	if depth > maximumDocumentDepth {
		return nil, fmt.Errorf(
			"%w: maximum is %d elements",
			ErrDocumentTooDeep,
			maximumDocumentDepth,
		)
	}

	attributes := tokens.Metadata().Attributes
	if len(attributes) != len(start.Attr) {
		return nil, fmt.Errorf(
			"musicxml: namespace declaration provenance out of sync at %s: got %d flags for %d attributes",
			validationDisplayName(start.Name), len(attributes), len(start.Attr),
		)
	}

	result := &validationNode{Name: start.Name, Namespaces: namespaces}
	// Share unchanged scopes; clone before applying actual declaration attributes.
	// Name expansion has already happened exactly once in the XML decoder.
	for _, attribute := range attributes {
		if attribute.NamespaceDeclaration {
			result.Namespaces = make(map[string]string, len(namespaces)+len(start.Attr))
			for prefix, uri := range namespaces {
				result.Namespaces[prefix] = uri
			}
			break
		}
	}
	for index, attribute := range start.Attr {
		if attributes[index].NamespaceDeclaration {
			prefix := ""
			if attribute.Name.Space == "xmlns" {
				prefix = attribute.Name.Local
			}
			result.Namespaces[prefix] = attribute.Value
		}
	}
	for index, attribute := range start.Attr {
		result.Attrs = append(result.Attrs, validationAttribute{
			Attr: validationRawAttribute{Name: attribute.Name, Value: attribute.Value}, NamespaceDeclaration: attributes[index].NamespaceDeclaration,
		})
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf(
				"read %s: %w",
				validationDisplayName(start.Name),
				err,
			)
		}

		switch value := token.(type) {
		case xml.StartElement:
			child, err := readValidationNode(decoder, tokens, value, depth+1, result.Namespaces)
			if err != nil {
				return nil, err
			}
			result.Children = append(result.Children, child)
		case xml.EndElement:
			if value.Name != start.Name {
				return nil, fmt.Errorf(
					"unexpected closing element %s",
					validationDisplayName(value.Name),
				)
			}
			return result, nil
		case xml.CharData:
			result.Text.Write([]byte(value))
		case xml.Comment, xml.ProcInst, xml.Directive:
		}
	}
}

func readValidationTail(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read document tail: %w", err)
		}

		switch value := token.(type) {
		case xml.CharData:
			if len(bytes.Trim(value, xmlWhitespace)) == 0 {
				continue
			}
		case xml.Comment, xml.ProcInst:
			continue
		}

		return fmt.Errorf("unexpected %T after root element", token)
	}
}
