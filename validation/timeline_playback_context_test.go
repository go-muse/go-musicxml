package validation

import (
 "context"
 "errors"
 "testing"
)

func playbackFixture(t *testing.T) (*Session, *timelinePlaybackContext, *timelineBindings, timelineMeasureBlock, timelineEventFact) {
 t.Helper()
 s:=rationalSession(t,Budgets{}); s.documentID="d"; t.Cleanup(s.cleanup)
 b:=stateBindings(); block:=runtimeBlock(b,0)
 fact:=timelineEventFact{Part:block.part,Position:timelineAt(block,stateKnown(mustRational(t,3,2))),Staff:stateAbsent[Integer](),Voice:stateAbsent[string](),Principal:stateAbsent[NodeRef]()}
 return s,&timelinePlaybackContext{},b,block,fact
}
func TestPlaybackBarlinePreservesObservedPositionAndLocation(t *testing.T) {
 for _,location:=range []string{"","left","middle","right"} { t.Run(location,func(t *testing.T){
  s,p,b,block,fact:=playbackFixture(t)
  bar:=stateNode(b,timelineRoleBarline,50,stateAbsent[Atomic]())
  if location!="" { stateAttribute(b,bar,timelineRoleBarlineLocation,Atomic{Kind:AtomicString,Text:location}) }
  if err:=p.readBarline(context.Background(),s,b,bar,block,fact);err!=nil {t.Fatal(err)}
  got:=p.Barlines[50]
  want:=location; if want=="" {want="right"}
  if got.Location.State!=EvidenceKnown || got.Location.Value!=want || got.LocationDefaulted!=(location=="") {t.Fatalf("location %+v",got)}
  if got.Origin.Position.Value.Offset.String()!="3/2" || got.Origin.Measure!=fact.Position.Value.Measure {t.Fatalf("invented barline position %+v",got)}
  if got.RawLocation.State==EvidenceAbsent && location!="" {t.Fatal("lost explicit location")}
 }) }
}
func TestPlaybackNavigationDivisionsAreMarkerScopedAndRaw(t *testing.T) {
 s,p,b,block,fact:=playbackFixture(t)
 for i,marker:=range []bool{false,true} {
  bar:=stateNode(b,timelineRoleBarline,NodeID(50+i),stateAbsent[Atomic]())
  stateAttribute(b,bar,timelineRoleBarlineDivisions,Atomic{Kind:AtomicDecimal,Decimal:stateDecimal(t,-3,2)})
  stateAttribute(b,bar,timelineRoleBarlineSegno,Atomic{Kind:AtomicString,Text:"S"})
  if marker {bar.children=[]*factNode{stateNode(b,timelineRoleBarlineSegnoElement,60,stateAbsent[Atomic]())}}
  if err:=p.readBarline(context.Background(),s,b,bar,block,fact);err!=nil {t.Fatal(err)}
  got:=p.Barlines[bar.header.ID].Navigation
  if got.Divisions.Value.String()!="-1.5" || got.DivisionsApplicable.State!=EvidenceKnown || got.DivisionsApplicable.Value!=marker {t.Fatalf("navigation scope %+v",got)}
 }
 sound:=stateNode(b,timelineRoleSound,70,stateAbsent[Atomic]())
 stateAttribute(b,sound,timelineRoleSoundDivisions,Atomic{Kind:AtomicDecimal,Decimal:stateDecimal(t,0,1)})
 stateAttribute(b,sound,timelineRoleSoundCoda,Atomic{Kind:AtomicString,Text:"C"})
 if err:=p.readSound(context.Background(),s,b,sound,block,fact);err!=nil {t.Fatal(err)}
 got:=p.Sounds[70]
 if got.Navigation.Divisions.Value.String()!="0" || !got.Navigation.DivisionsApplicable.Value || got.Origin.Position.Value.Offset.String()!="3/2" {t.Fatalf("sound navigation %+v",got)}
}
func TestPlaybackNotePlayIsCurrentChordToneOnly(t *testing.T) {
 for _,order:=range []OrderKind{OrderSource,OrderModelContent} {t.Run(string(order),func(t *testing.T){
  s,p,b,block,fact:=playbackFixture(t)
  n:=stateNode(b,timelineRoleNote,20,stateAbsent[Atomic](),stateNode(b,timelineRoleNoteChord,21,stateAbsent[Atomic]()),stateNode(b,timelineRoleNotePlay,22,stateAbsent[Atomic](),stateNode(b,timelineRolePlayMute,23,stateAtomicText("on"))))
  n.end.Order=stateKnown(order)
  fact.Principal=stateKnown(NodeRef{DocumentID:"d",Node:10})
  stateAttribute(b,n,timelineRoleNotePizzicato,Atomic{Kind:AtomicString,Text:"no"})
  if err:=p.readNote(context.Background(),s,b,n,block,fact);err!=nil {t.Fatal(err)}
  got:=p.Notes[20]
  if len(got.Play)!=1 || got.Play[0].Scope!=playbackCurrentNote || got.Origin.Source.Node!=20 || got.Principal.Value.Node!=10 || !got.Chord.Value {t.Fatalf("note scope %+v",got)}
  if got.Origin.Position.Value.Offset.String()!="3/2" || got.Pizzicato.State!=EvidenceKnown || got.Pizzicato.Value {t.Fatalf("note origin %+v",got)}
  other:=stateNode(b,timelineRoleNote,30,stateAbsent[Atomic]())
  if err:=p.readNote(context.Background(),s,b,other,block,fact);err!=nil {t.Fatal(err)}
  if len(p.Notes[30].Play)!=0 || p.Notes[30].Pizzicato.State!=EvidenceAbsent || len(p.Pizzicato.Publications)!=0 {t.Fatal("note playback leaked to another note or continuing state")}
 })}
}
func TestPlaybackSoundStateUsesScoreOrderAndSeparateFamilies(t *testing.T) {
 s,p,b,block,fact:=playbackFixture(t); h:=&timelineState{}; stateClose(t,s,h,2)
 for i,at:=range []int64{2,0,1} {
  n:=stateNode(b,timelineRoleSound,NodeID(50+i),stateAbsent[Atomic]())
  stateAttribute(b,n,timelineRoleSoundTempo,Atomic{Kind:AtomicDecimal,Decimal:stateDecimal(t,60+at,1)})
  fact.Position=timelineAt(block,stateKnown(mustRational(t,at,1)))
  if at==0 {stateAttribute(b,n,timelineRoleSoundPizzicato,Atomic{Kind:AtomicString,Text:"yes"})}
  if err:=p.readSound(context.Background(),s,b,n,block,fact);err!=nil {t.Fatal(err)}
 }
 at:=statePosition(t,0,3,1)
 tempo,err:=p.effectiveTempo(context.Background(),s,h,at)
 if err!=nil || tempo.Value.State!=EvidenceKnown || tempo.Value.Value.String()!="62" || tempo.Provenance[0].Node.Node!=50 {t.Fatalf("tempo %+v,%v",tempo,err)}
 pizz,err:=p.effectivePizzicato(context.Background(),s,h,at)
 if err!=nil || pizz.Value.State!=EvidenceKnown || !pizz.Value.Value {t.Fatalf("pizzicato %+v,%v",pizz,err)}
 dynamics,err:=p.effectiveDynamics(context.Background(),s,h,at)
 if err!=nil || dynamics.Value.State!=EvidenceKnown || dynamics.Value.Value.String()!="100" || dynamics.Provenance[0].Kind!=stateInitialDefault {t.Fatalf("documented initial forte %+v,%v",dynamics,err)}
}
func TestPlaybackNoTempoDefaultConditionalStateAndZero(t *testing.T) {
 s,p,b,block,fact:=playbackFixture(t); h:=&timelineState{}; stateClose(t,s,h,0)
 before,err:=p.effectiveTempo(context.Background(),s,h,statePosition(t,0,0,1)); if err!=nil || before.Value.State!=EvidenceAbsent {t.Fatalf("invented tempo %+v,%v",before,err)}
 n:=stateNode(b,timelineRoleSound,50,stateAbsent[Atomic]())
 stateAttribute(b,n,timelineRoleSoundTempo,Atomic{Kind:AtomicDecimal,Decimal:stateDecimal(t,0,1)})
 stateAttribute(b,n,timelineRoleSoundDynamics,Atomic{Kind:AtomicDecimal,Decimal:stateDecimal(t,0,1)})
 stateAttribute(b,n,timelineRoleSoundTimeOnly,Atomic{Kind:AtomicString,Text:"2,3"})
 if err:=p.readSound(context.Background(),s,b,n,block,fact);err!=nil {t.Fatal(err)}
 if p.Sounds[50].Tempo.Value.String()!="0" || p.Sounds[50].TimeOnly.Value!="2,3" {t.Fatal("lost raw conditional zero")}
 after,err:=p.effectiveTempo(context.Background(),s,h,statePosition(t,0,2,1)); if err!=nil || after.Value.State!=EvidenceUnknown || after.Value.Reason!="playback-repeat-pass-unavailable" {t.Fatalf("unconditional repeat state %+v,%v",after,err)}
}
func TestPlaybackInvalidCauseOwnedAndIndependentFromRaw(t *testing.T) {
 s,p,b,block,fact:=playbackFixture(t); h:=&timelineState{}; stateClose(t,s,h,0)
 n:=stateNode(b,timelineRoleSound,50,stateAbsent[Atomic]())
 stateAttribute(b,n,timelineRoleSoundTempo,Atomic{Kind:AtomicDecimal,Decimal:stateDecimal(t,88,1)})
 cause:=&DecisionRef{Rule:RuleKey{ID:"scalar",Revision:1},Subject:NodeRef{DocumentID:"d",Node:50}}
 n.attributes[0].Scalar.Value=Evidence[Atomic]{State:EvidenceInvalid,Reason:"tempo-invalid",Cause:cause}
 if err:=p.readSound(context.Background(),s,b,n,block,fact);err!=nil {t.Fatal(err)}
 cause.Rule.ID="mutated"
 got:=p.Sounds[50]; if got.Tempo.Cause==nil || got.Tempo.Cause.Rule.ID!="scalar" {t.Fatalf("borrowed input cause %+v",got)}
 got.Tempo.Cause.Rule.ID="raw-mutated"
 state,err:=p.effectiveTempo(context.Background(),s,h,statePosition(t,0,2,1)); if err!=nil || state.Value.State!=EvidenceInvalid || state.Value.Cause.Rule.ID!="scalar" {t.Fatalf("history shared raw cause %+v,%v",state,err)}
 dynamics,err:=p.effectiveDynamics(context.Background(),s,h,statePosition(t,0,2,1)); if err!=nil || dynamics.Value.State!=EvidenceKnown {t.Fatalf("unrelated family masked %+v,%v",dynamics,err)}
}
func TestPlaybackQuotaCancellationAndNoPublication(t *testing.T) {
 for _,kind:=range []string{"context","work","retention","cancel"} {t.Run(kind,func(t *testing.T){
  s,p,b,block,fact:=playbackFixture(t)
  n:=stateNode(b,timelineRoleBarline,50,stateAbsent[Atomic]())
  ctx:=context.Background()
  switch kind {case "context": s.account.limits.MaxContextFacts=1; if err:=s.account.Charge(BudgetContextFacts,1);err!=nil{t.Fatal(err)}
  case "work": s.account.limits.MaxRuleSteps=1
  case "retention": s.account.limits.MaxBufferedBytes=1
  case "cancel": var cancel context.CancelFunc;ctx,cancel=context.WithCancel(ctx);cancel()}
  err:=p.readBarline(ctx,s,b,n,block,fact)
  if err==nil || len(p.Barlines)!=0 {t.Fatalf("quota/cancellation publication %v,%d",err,len(p.Barlines))}
  if kind=="cancel" && !errors.Is(err,context.Canceled){t.Fatal(err)}
 })}
}
