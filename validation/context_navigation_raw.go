package validation

const (
	contextNavRawSound     contextRole = "navigation-raw-sound"
	contextNavRawTempo     contextRole = "navigation-raw-tempo"
	contextNavRawDynamics  contextRole = "navigation-raw-dynamics"
	contextNavRawPizzicato contextRole = "navigation-raw-pizzicato"
	contextNavRawDaCapo    contextRole = "navigation-raw-dacapo"
	contextNavRawDalSegno  contextRole = "navigation-raw-dalsegno"
	contextNavRawToCoda    contextRole = "navigation-raw-tocoda"
	contextNavRawFine      contextRole = "navigation-raw-fine"
	contextNavRawForward   contextRole = "navigation-raw-forward"
	contextNavRawPass      contextRole = "navigation-raw-pass"
)

func navigationRawBindings() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: contextNavRawSound, Name: "sound", Type: "sound", Count: -1},
		{Role: contextNavRawTempo, Owner: "sound", Name: "tempo", Type: "non-negative-decimal", Attribute: true, Count: 1},
		{Role: contextNavRawDynamics, Owner: "sound", Name: "dynamics", Type: "non-negative-decimal", Attribute: true, Count: 1},
		{Role: contextNavRawPizzicato, Owner: "sound", Name: "pizzicato", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextNavRawDaCapo, Owner: "sound", Name: "dacapo", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextNavRawDalSegno, Owner: "sound", Name: "dalsegno", Type: "xs:token", Attribute: true, Count: 1},
		{Role: contextNavRawToCoda, Owner: "sound", Name: "tocoda", Type: "xs:token", Attribute: true, Count: 1},
		{Role: contextNavRawFine, Owner: "sound", Name: "fine", Type: "xs:token", Attribute: true, Count: 1},
		{Role: contextNavRawForward, Owner: "sound", Name: "forward-repeat", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextNavRawPass, Owner: "sound", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
	}
}

// Sound's local attribute facts are independent of timeline availability. Only
// positioning and future state execution need the score-time provider.
func navigationRawSound(r *contextReader, n *factNode, timeline *timelineService) (soundPlaybackContext, error) {
	fact, err := contextRepeatOrigin(r.ctx, r.s, timeline, n)
	if err != nil {
		return soundPlaybackContext{}, err
	}
	v := soundPlaybackContext{Origin: playbackOrigin{Source: NodeRef{r.s.documentID, n.header.ID}, Part: fact.Part, Position: fact.Position}, Tempo: stateDecimalValue(r.attribute(n, contextNavRawTempo)), Dynamics: stateDecimalValue(r.attribute(n, contextNavRawDynamics)), Pizzicato: stateYesNo(r.attribute(n, contextNavRawPizzicato)), TimeOnly: stateText(r.attribute(n, contextNavRawPass)), PlayComplete: stateUnknown[bool]("navigation-play-provider-unavailable")}
	v.Navigation = navigationContext{Segno: stateText(r.attribute(n, contextNavGraphicSoundSegno)), Coda: stateText(r.attribute(n, contextNavGraphicSoundCoda)), Dacapo: stateYesNo(r.attribute(n, contextNavRawDaCapo)), Dalsegno: stateText(r.attribute(n, contextNavRawDalSegno)), Tocoda: stateText(r.attribute(n, contextNavRawToCoda)), Fine: stateText(r.attribute(n, contextNavRawFine)), ForwardRepeat: stateYesNo(r.attribute(n, contextNavRawForward)), TimeOnly: v.TimeOnly}
	return v, r.err
}

// NavigationDirective preserves each independently encoded sound control even
// when simultaneous controls have no established execution precedence.
type NavigationDirective struct {
	Kind      NavigationKind
	Label     Evidence[string]
	Enabled   Evidence[bool]
	TimeOnly  Evidence[string]
	Defaulted bool
}

func navigationRawDirectives(r *contextReader, raw navigationContext) ([]NavigationDirective, error) {
	var out []NavigationDirective
	for _, v := range []NavigationDirective{
		{Kind: NavigationSegno, Label: raw.Segno}, {Kind: NavigationCoda, Label: raw.Coda},
		{Kind: NavigationDalSegno, Label: raw.Dalsegno}, {Kind: NavigationToCoda, Label: raw.Tocoda},
		{Kind: NavigationFine, Label: raw.Fine}, {Kind: NavigationDaCapo, Enabled: raw.Dacapo},
		{Kind: NavigationForward, Enabled: raw.ForwardRepeat},
	} {
		present := v.Label.State != "" && v.Label.State != EvidenceAbsent || v.Enabled.State != "" && v.Enabled.State != EvidenceAbsent
		if !present {
			continue
		}
		if v.Label.State == "" {
			v.Label = Evidence[string]{State: EvidenceAbsent}
		}
		if v.Enabled.State == "" {
			v.Enabled = Evidence[bool]{State: EvidenceAbsent}
		}
		v.TimeOnly, v.Defaulted = navigationDefaultGuard(v.Kind, raw.TimeOnly)
		if _, err := stateReserve(r.ctx, r.s, v); err != nil {
			return nil, err
		}
		out = append(out, stateClone(v))
	}
	return out, r.err
}
