package musicxml

// PartDefinition contains the identity and display name of a score part.
//
// IDs must be unique within a document. Validate reports empty, duplicate, or
// otherwise invalid values after the document has been assembled.
type PartDefinition struct {
	ID   string
	Name string
}

// NewScorePartwise creates a MusicXML 4.0 partwise score.
//
// Each definition is added both to PartList and as an empty part body. Measures
// can then be appended to the corresponding Part entry.
func NewScorePartwise(parts ...PartDefinition) *ScorePartwise {
	result := &ScorePartwise{
		Version: Ptr(MusicXMLVersion),
		PartList: PartList{
			Content: make([]PartListContent, 0, len(parts)),
		},
		Part: make([]ScorePartwisePart, 0, len(parts)),
	}

	for _, part := range parts {
		result.AddPart(part)
	}

	return result
}

// AddPart declares a part and appends its empty partwise body.
func (value *ScorePartwise) AddPart(part PartDefinition) {
	value.PartList.AddScorePart(newScorePart(part))
	value.Part = append(value.Part, ScorePartwisePart{ID: part.ID})
}

// NewScoreTimewise creates a MusicXML 4.0 timewise score.
//
// Each definition is added to PartList. Use NewScoreTimewiseMeasure to create
// measure-local part bodies in the same order.
func NewScoreTimewise(parts ...PartDefinition) *ScoreTimewise {
	result := &ScoreTimewise{
		Version: Ptr(MusicXMLVersion),
		PartList: PartList{
			Content: make([]PartListContent, 0, len(parts)),
		},
	}

	for _, part := range parts {
		result.AddPart(part)
	}

	return result
}

// AddPart declares a part in a timewise score.
//
// Existing measures are not modified.
func (value *ScoreTimewise) AddPart(part PartDefinition) {
	value.PartList.AddScorePart(newScorePart(part))
}

// NewOpusDocument creates an empty MusicXML 4.0 opus.
func NewOpusDocument() *OpusDocument {
	return &OpusDocument{Version: Ptr(MusicXMLVersion)}
}

// NewScorePartwiseMeasure creates an empty numbered partwise measure.
func NewScorePartwiseMeasure(
	number string,
) ScorePartwisePartMeasure {
	return ScorePartwisePartMeasure{Number: number}
}

// AddMeasure appends a measure to a partwise part.
func (value *ScorePartwisePart) AddMeasure(
	measure ScorePartwisePartMeasure,
) {
	value.Measure = append(value.Measure, measure)
}

// NewScoreTimewiseMeasure creates a numbered timewise measure with one empty
// measure-local body for every part ID.
func NewScoreTimewiseMeasure(
	number string,
	partIDs ...string,
) ScoreTimewiseMeasure {
	result := ScoreTimewiseMeasure{
		Number: number,
		Part:   make([]ScoreTimewiseMeasurePart, 0, len(partIDs)),
	}

	for _, partID := range partIDs {
		result.AddPart(partID)
	}

	return result
}

// AddPart appends an empty part body to a timewise measure.
func (value *ScoreTimewiseMeasure) AddPart(id string) {
	value.Part = append(value.Part, ScoreTimewiseMeasurePart{ID: id})
}

// AddMeasure appends a measure to a timewise score.
func (value *ScoreTimewise) AddMeasure(
	measure ScoreTimewiseMeasure,
) {
	value.Measure = append(value.Measure, measure)
}

// NewPitchedNote creates a regular pitched note with a duration.
func NewPitchedNote(
	step Step,
	octave Octave,
	duration PositiveDivisions,
) *Note {
	return &Note{
		Pitch: &Pitch{
			Step:   step,
			Octave: octave,
		},
		Duration: Ptr(duration),
	}
}

// NewRestNote creates a regular rest with a duration.
func NewRestNote(duration PositiveDivisions) *Note {
	return &Note{
		Rest:     &Rest{},
		Duration: Ptr(duration),
	}
}

// Ptr returns a pointer to an independent copy of value.
//
// It is useful for optional fields in generated MusicXML types.
func Ptr[T any](value T) *T {
	return &value
}

func newScorePart(part PartDefinition) *ScorePart {
	return &ScorePart{
		ID: part.ID,
		PartName: PartName{
			Value: part.Name,
		},
	}
}
