package validation

import (
 "context"
 "errors"
 "testing"
)

func noteOriginBindings() *timelineBindings {
 b := stateBindings()
 for _, role := range []timelineRole{timelineRoleNoteAttack, timelineRoleNoteRelease, timelineRoleBendReleaseOffset, timelineRoleBendFirstBeat, timelineRoleBendLastBeat, timelineRoleBendBeats, timelineRoleBendAccelerate, timelineRoleGraceStealPrevious, timelineRoleGraceStealFollowing, timelineRoleGraceMakeTime} {
  b.Roles[role][0].Attribute = true
  b.Roles[role][0].Name = QName{Local: string(rune(1000 + role))}
 }
 return b
}
func noteOriginFact(t *testing.T) timelineEventFact {
 return timelineEventFact{Position: stateKnown(statePosition(t, 0, 2, 1)), Duration: stateKnown(mustRational(t, 2, 1)), Part: stateKnown(NodeRef{"d", 1}), Staff: stateKnown(stateInt(t, 2)), Voice: stateAbsent[string]()}
}
func noteOriginCheckPosition(t *testing.T, got Evidence[musicalPosition], want string) {
 t.Helper()
 if got.State != EvidenceKnown || got.Value.Offset.String() != want { t.Errorf("position = %+v; want %s", got, want) }
}
func TestNoteOriginsExactIndependentAdjustmentsBothOrders(t *testing.T) {
 for _, order := range []OrderKind{OrderSource, OrderModelContent, OrderModelFields} { t.Run(string(order), func(t *testing.T) {
  s := rationalSession(t, Budgets{}); s.documentID = "d"; defer s.cleanup()
  service := &timelineService{}; b := noteOriginBindings(); n := runtimeNote(t,b,20,6,false); n.end.Order = stateKnown(order)
  stateAttribute(b,n,timelineRoleNoteAttack,stateAtomicDecimal(t,-1,1).Value)
  stateAttribute(b,n,timelineRoleNoteRelease,stateAtomicDecimal(t,2,1).Value)
  fact := noteOriginFact(t)
  if err := service.readNoteOrigins(context.Background(),s,b,n,fact,stateKnown(stateDecimal(t,3,1))); err != nil { t.Fatal(err) }
  got := service.NoteOrigins[20]
  noteOriginCheckPosition(t,got.ScoreOnset,"2"); noteOriginCheckPosition(t,got.ScoreEnd,"4")
  noteOriginCheckPosition(t,got.Attack.Position,"5/3"); noteOriginCheckPosition(t,got.Release.Position,"14/3")
  if got.Attack.Raw.Value.String() != "-1" || got.Release.Delta.Value.String() != "2/3" || got.Voice.State != EvidenceAbsent { t.Fatalf("raw/provenance = %+v", got) }
  noteOriginCheckPosition(t,fact.Position,"2")
 }) }
}
func TestNoteOriginsAbsentZeroInvalidAndUnitsRemainDistinct(t *testing.T) {
 for _, tc := range []struct{name string; raw *Atomic; units Evidence[Decimal]; want EvidenceState}{
  {"absent",nil,stateUnknown[Decimal]("unproved-units"),EvidenceAbsent},
  {"known-zero",func()*Atomic { a:=stateAtomicDecimal(t,0,1).Value; return &a }(),stateKnown(stateDecimal(t,3,1)),EvidenceKnown},
  {"zero-unresolved-units",func()*Atomic { a:=stateAtomicDecimal(t,0,1).Value; return &a }(),stateUnknown[Decimal]("unproved-units"),EvidenceUnknown},
 } { t.Run(tc.name,func(t *testing.T) {
  s:=rationalSession(t,Budgets{}); defer s.cleanup(); b:=noteOriginBindings(); service:=&timelineService{}; n:=runtimeNote(t,b,20,6,false)
  if tc.raw != nil { stateAttribute(b,n,timelineRoleNoteAttack,*tc.raw) }
  if err:=service.readNoteOrigins(context.Background(),s,b,n,noteOriginFact(t),tc.units);err!=nil {t.Fatal(err)}
  got:=service.NoteOrigins[20]
  if got.Attack.Delta.State!=tc.want {t.Fatalf("delta = %+v",got.Attack)}
  if tc.raw==nil { noteOriginCheckPosition(t,got.Attack.Position,"2") }
 }) }
 s:=rationalSession(t,Budgets{});defer s.cleanup();b:=noteOriginBindings();service:=&timelineService{};n:=runtimeNote(t,b,20,6,false)
 stateAttribute(b,n,timelineRoleNoteRelease,stateAtomicDecimal(t,1,1).Value)
 cause:=&DecisionRef{Rule:RuleKey{"scalar",1},Subject:NodeRef{"d",20}}
 n.attributes[0].Scalar.Value=Evidence[Atomic]{State:EvidenceInvalid,Reason:"bad-release",Cause:cause}
 if err:=service.readNoteOrigins(context.Background(),s,b,n,noteOriginFact(t),stateKnown(stateDecimal(t,3,1)));err!=nil {t.Fatal(err)}
 got:=service.NoteOrigins[20]
 if got.Release.Position.State!=EvidenceInvalid || got.Release.Position.Cause!=cause {t.Fatalf("cause lost: %+v",got.Release)}
 noteOriginCheckPosition(t,got.Attack.Position,"2")
}
func TestNoteOriginsBendReleaseKeepsOriginalPercentBase(t *testing.T) {
 s:=rationalSession(t,Budgets{});s.documentID="d";defer s.cleanup();b:=noteOriginBindings();service:=&timelineService{}
 release:=stateNode(b,timelineRoleBendRelease,26,stateAbsent[Atomic]())
 bend:=stateNode(b,timelineRoleTechnicalBend,25,stateAbsent[Atomic](),release)
 technical:=stateNode(b,timelineRoleNotationsTechnical,24,stateAbsent[Atomic](),bend)
 notations:=stateNode(b,timelineRoleNoteNotations,23,stateAbsent[Atomic](),technical)
 n:=runtimeNote(t,b,20,6,false);n.children=append(n.children,notations)
 stateAttribute(b,release,timelineRoleBendReleaseOffset,stateAtomicDecimal(t,3,1).Value)
 stateAttribute(b,n,timelineRoleNoteAttack,stateAtomicDecimal(t,3,1).Value)
 if err:=service.readNoteOrigins(context.Background(),s,b,n,noteOriginFact(t),stateKnown(stateDecimal(t,3,1)));err!=nil {t.Fatal(err)}
 got:=service.NoteOrigins[20]
 if len(got.Bends)!=1 || got.BendsComplete.State!=EvidenceKnown {t.Fatalf("bends = %+v",got)}
 bendGot:=got.Bends[0]
 noteOriginCheckPosition(t,bendGot.Release.Position,"3")
 noteOriginCheckPosition(t,bendGot.FirstBeat.Position,"5/2")
 noteOriginCheckPosition(t,bendGot.LastBeat.Position,"7/2")
 if bendGot.FirstBeat.Raw.State!=EvidenceAbsent || bendGot.FirstBeat.Percent.Value.String()!="25" || bendGot.LastBeat.Percent.Value.String()!="75" {t.Fatalf("bend default provenance = %+v",bendGot)}
}
func TestNoteOriginsGraceRetainsRawTimingWithoutInventedDuration(t *testing.T) {
 s:=rationalSession(t,Budgets{});defer s.cleanup();b:=noteOriginBindings();service:=&timelineService{}
 grace:=stateNode(b,timelineRoleNoteGrace,21,stateAbsent[Atomic]());n:=stateNode(b,timelineRoleNote,20,stateAbsent[Atomic](),grace)
 stateAttribute(b,grace,timelineRoleGraceStealPrevious,stateAtomicDecimal(t,25,1).Value)
 stateAttribute(b,grace,timelineRoleGraceMakeTime,stateAtomicDecimal(t,1,1).Value)
 fact:=noteOriginFact(t);fact.Duration=stateAbsent[Rational]()
 if err:=service.readNoteOrigins(context.Background(),s,b,n,fact,stateKnown(stateDecimal(t,3,1)));err!=nil {t.Fatal(err)}
 got:=service.NoteOrigins[20]
 if got.ScoreEnd.State==EvidenceKnown || got.Duration.State!=EvidenceAbsent || got.GraceTiming.Value.StealPrevious.Value.String()!="25" || got.GraceTiming.Value.MakeTime.Delta.Value.String()!="1/3" {t.Fatalf("grace fabricated elapsed duration: %+v",got)}
 if got.GraceTiming.Value.PreviousNote.State!=EvidenceUnknown {t.Fatalf("guessed neighboring note = %+v",got)}
}
func TestNoteOriginsPublicationTransactionalAndExactRoles(t *testing.T) {
 s:=rationalSession(t,Budgets{});defer s.cleanup();b:=noteOriginBindings();service:=&timelineService{};n:=runtimeNote(t,b,20,6,false)
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 if err:=service.readNoteOrigins(ctx,s,b,n,noteOriginFact(t),stateKnown(stateDecimal(t,3,1)));!errors.Is(err,context.Canceled)||len(service.NoteOrigins)!=0 {t.Fatalf("cancel = %v; map = %+v",err,service.NoteOrigins)}
 n.header.Declaration=stateKnown(DeclarationID{Occurrence:"foreign-note"});n.header.Name=QName{Local:"note"}
 if err:=service.readNoteOrigins(context.Background(),s,b,n,noteOriginFact(t),stateKnown(stateDecimal(t,3,1)));err==nil||len(service.NoteOrigins)!=0 {t.Fatalf("name fallback accepted = %v",err)}
}
