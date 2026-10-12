package validation

import "context"

// SpanFamily identifies a notation-specific association domain. Equal schema
// types do not imply equal families (for example hammer-on and pull-off).
type SpanFamily string

// Supported span families keep graphical, sounding and vertical relations apart.
const (
	SpanSlur           SpanFamily = "slur"
	SpanLyricSyllable  SpanFamily = "lyric-syllable"
	SpanTied           SpanFamily = "tied"
	SpanTie            SpanFamily = "tie"
	SpanBeam           SpanFamily = "beam"
	SpanTuplet         SpanFamily = "tuplet"
	SpanGlissando      SpanFamily = "glissando"
	SpanSlide          SpanFamily = "slide"
	SpanOctaveShift    SpanFamily = "octave-shift"
	SpanBracket        SpanFamily = "bracket"
	SpanDashes         SpanFamily = "dashes"
	SpanWavyLine       SpanFamily = "wavy-line"
	SpanHammerOn       SpanFamily = "hammer-on"
	SpanPullOff        SpanFamily = "pull-off"
	SpanLyricExtend    SpanFamily = "lyric-extend"
	SpanFigureExtend   SpanFamily = "figure-extend"
	SpanPartGroup      SpanFamily = "part-group"
	SpanBarre          SpanFamily = "barre"
	SpanArpeggiate     SpanFamily = "arpeggiate"
	SpanNonArpeggiate  SpanFamily = "non-arpeggiate"
	SpanPedal          SpanFamily = "pedal"
	SpanWedge          SpanFamily = "wedge"
	SpanGrouping       SpanFamily = "grouping"
	SpanOtherNotation  SpanFamily = "other-notation"
	SpanPrincipalVoice SpanFamily = "principal-voice"
	SpanMetronomeTied  SpanFamily = "metronome-tied"
	SpanTremolo        SpanFamily = "tremolo"
)

// SpanIdentityKind describes the interpretation of a number or other identity.
type SpanIdentityKind string

// Span identity kinds are intentionally distinct even if values have equal text.
const (
	SpanIdentityNumber     SpanIdentityKind = "number-level"
	SpanIdentityImpliedTie SpanIdentityKind = "implied-tie"
	SpanIdentityBeamLevel  SpanIdentityKind = "beam-level"
	SpanIdentityLyric      SpanIdentityKind = "lyric-line"
	SpanIdentityPartGroup  SpanIdentityKind = "part-group-token"
	SpanIdentityUnnumbered SpanIdentityKind = "unnumbered"
	SpanIdentityGrouping   SpanIdentityKind = "grouping-token"
)

// SpanPosition describes an established score coordinate, not a playback pass.
type SpanPosition = ContextPosition

// SpanContextResult is an owned endpoint projection. A known Association means
// Members identifies a supported object, not that all score spans are balanced.
// Members lists semantic start/stop boundaries (or the single self-contained
// endpoint, or all identified simultaneous arpeggiation marks); continuations refer to those same boundaries. Included is used for
// established part-group score-part membership. Unavailable evidence retains
// its cause. NumberExplicit preserves provenance independently of value identity.
type SpanContextResult struct {
	Family            SpanFamily
	Role              Evidence[string]
	Domain, Anchor    Evidence[NodeRef]
	Number            Evidence[Atomic]
	NumberExplicit    Evidence[bool]
	Identity          SpanIdentityKind
	Staff             Evidence[Integer]
	Voice             Evidence[string]
	Position          Evidence[SpanPosition]
	Association       Evidence[bool]
	Members, Included []NodeRef
}

func cloneSpanContext(ctx context.Context, s *Session, p *SpanContextResult) (*SpanContextResult, error) {
	if p == nil {
		return nil, nil
	}
	// The payload is a closed value graph: only Evidence's causal pointers occur.
	// The existing measured copy owns strings, slices and causes and admits all
	// immutable numeric magnitudes before copying.
	if err := contextRetainProjection(ctx, s, *p); err != nil {
		return nil, err
	}
	q := stateClone(*p)
	return &q, nil
}

type spanEndpoint struct {
	ref                 NodeRef
	relationSide        NodeID
	payload             SpanContextResult
	complete            Evidence[bool]
	pitchAvailable      Evidence[bool]
	pitch, guard, lyric string
	grace, cue          bool
	categoryKnown       bool
	order               uint64
	scoreParts          []NodeRef
}
type contextSpanService struct {
	Final     bool
	endpoints []*spanEndpoint
}
