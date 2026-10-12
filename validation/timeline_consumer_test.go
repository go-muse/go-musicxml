package validation

import (
 "context"
 "errors"
 "testing"
)

func consumerFixture(t *testing.T) (*Session, *timelineConsumerPlan) {
 t.Helper()
 s := rationalSession(t, Budgets{})
 s.documentID = "d"
 s.nodes = map[NodeID]*factNode{}
 p := &timelineConsumerPlan{Provider: RuleKey{"timeline", 1}, Bindings: &timelineBindings{}}
 for role, name := range map[timelineRole]string{
  timelineRoleNote:"note", timelineRoleNoteChord:"chord", timelineRoleNoteGrace:"grace", timelineRoleNoteDuration:"duration",
  timelineRoleHarmony:"harmony", timelineRoleHarmonyNumeral:"numeral", timelineRoleNumeralKey:"numeral-key",
  timelineRoleNumeralKeyFifths:"numeral-fifths", timelineRoleNumeralKeyMode:"numeral-mode",
 } { p.Bindings.Roles[role] = []SchemaUse{{Declaration:localDecl(name)}} }
 s.timeline = map[RuleKey]*timelineService{p.Provider:{Facts:map[NodeID]timelineEventFact{}, Final:true}}
 return s,p
}
func consumerNode(s *Session, id NodeID, parent *factNode, name string) *factNode {
 pid := NodeID(0)
 if parent != nil { pid = parent.header.ID }
 n := localNode(id,pid,name)
 s.nodes[id] = n
 if parent != nil {parent.children=append(parent.children,n)}
 return n
}
func consumerEvaluate(t *testing.T,s *Session,n *factNode,p *timelineConsumerPlan,kind planKind) Decision {
 t.Helper()
 d,pending,e:=s.evaluateTimelineConsumer(context.Background(),n,p,kind,true)
 if e!=nil||pending {t.Fatalf("decision=%+v pending=%v error=%v",d,pending,e)}
 return d
}
func consumerLocalKey(t *testing.T,s *Session,n *factNode) *factNode {
 t.Helper()
 key:=consumerNode(s,30,n,"numeral-key")
 fifths:=consumerNode(s,31,key,"numeral-fifths")
 fifths.end.Scalar.Value=stateKnown(localInteger(t,0))
 mode:=consumerNode(s,32,key,"numeral-mode")
 mode.end.Scalar.Value=stateKnown(localString("major"))
 return key
}
func TestTimelineConsumerChordBoundUsesPrincipal(t *testing.T) {
 for _, tc:=range []struct{name string;duration int64;want Outcome}{
  {"shorter",1,OutcomePass},{"increasing-tone-still-below-principal",3,OutcomePass},{"equal",4,OutcomePass},{"longer",5,OutcomeFail},
 } {t.Run(tc.name,func(t *testing.T){
  s,p:=consumerFixture(t);defer s.cleanup()
  principal:=consumerNode(s,1,nil,"note")
  n:=consumerNode(s,2,nil,"note");consumerNode(s,3,n,"chord");consumerNode(s,4,n,"duration")
  s.timeline[p.Provider].Facts[2]=timelineEventFact{Duration:stateKnown(mustRational(t,tc.duration,4)),PrincipalDuration:stateKnown(mustRational(t,4,4)),Principal:stateKnown(NodeRef{"d",1}),Position:stateUnknown[musicalPosition]("unrelated-position-gap")}
  d:=consumerEvaluate(t,s,n,p,planChordDuration)
  if d.Outcome!=tc.want {t.Fatalf("got %+v want %s",d,tc.want)}
  if tc.want==OutcomeFail&&(len(d.Related)!=1||d.Related[0]!=principal.header.Location) {t.Fatalf("missing principal location %+v",d)}
 })}
}
func TestTimelineConsumerChordApplicabilityAndCausality(t *testing.T) {
 cause:=&DecisionRef{Rule:RuleKey{"xsd.scalar",1},Subject:NodeRef{"d",4}}
 for _,tc:=range []struct{name string;chord,grace bool;duration,principalDuration Evidence[Rational];principal Evidence[NodeRef];want Outcome;reason Code;causal bool}{
  {name:"nonchord",want:OutcomeNotApplicable},
  {name:"grace-chord",chord:true,grace:true,want:OutcomeNotApplicable},
  {name:"principal-grace-duration-absent",chord:true,duration:stateKnown(mustRational(t,1,1)),principalDuration:stateAbsent[Rational](),principal:stateKnown(NodeRef{"d",1}),want:OutcomeUnknown},
  {name:"missing-duration",chord:true,duration:stateAbsent[Rational](),principalDuration:stateKnown(mustRational(t,1,1)),principal:stateKnown(NodeRef{"d",1}),want:OutcomeUnknown},
  {name:"invalid-duration",chord:true,duration:Evidence[Rational]{State:EvidenceInvalid,Reason:"duration-invalid",Cause:cause},principal:stateKnown(NodeRef{"d",1}),want:OutcomeBlocked,reason:"duration-invalid",causal:true},
  {name:"interrupted-principal",chord:true,duration:stateKnown(mustRational(t,1,1)),principal:stateUnknown[NodeRef]("chord-principal-interrupted"),want:OutcomeUnknown,reason:"chord-principal-interrupted"},
 } {t.Run(tc.name,func(t *testing.T){
  s,p:=consumerFixture(t);defer s.cleanup()
  consumerNode(s,1,nil,"note");n:=consumerNode(s,2,nil,"note")
  if tc.chord {consumerNode(s,3,n,"chord")};if tc.grace {consumerNode(s,5,n,"grace")} else {consumerNode(s,4,n,"duration")}
  s.timeline[p.Provider].Facts[2]=timelineEventFact{Duration:tc.duration,PrincipalDuration:tc.principalDuration,Principal:tc.principal}
  d:=consumerEvaluate(t,s,n,p,planChordDuration)
  if d.Outcome!=tc.want||(tc.reason!=""&&d.Reason!=tc.reason)||(tc.causal&&(d.Cause==nil||*d.Cause!=*cause)){t.Fatalf("got %+v",d)}
 })}
}
func TestTimelineConsumerLocalNumeralKeyAlternative(t *testing.T) {
 cause:=&DecisionRef{Rule:RuleKey{"xsd.scalar",1},Subject:NodeRef{"d",32}}
 for _,tc:=range []struct{name string;change func(*Session,*factNode);want Outcome;conclusive bool}{
  {"valid-without-service",nil,OutcomePass,true},
  {"malformed-mode",func(s *Session,_ *factNode){s.nodes[32].end.Scalar.Value=Evidence[Atomic]{State:EvidenceInvalid,Reason:"invalid-mode",Cause:cause}},OutcomeBlocked,true},
  {"missing-mode",func(_ *Session,k *factNode){k.children=k.children[:1]},OutcomeUnknown,true},
  {"incomplete-key",func(_ *Session,k *factNode){k.end.ChildrenComplete=stateUnknown[bool]("incomplete-key")},OutcomeUnknown,true},
  {"incomplete-numeral",func(s *Session,_ *factNode){s.nodes[2].end.ChildrenComplete=stateUnknown[bool]("incomplete-numeral")},OutcomeUnknown,true},
  {"wrong-exact-use",func(_ *Session,k *factNode){k.header.Declaration=stateKnown(localDecl("foreign-key"))},"",false},
 } {t.Run(tc.name,func(t *testing.T){
  s,p:=consumerFixture(t);defer s.cleanup();s.timeline=nil
  harmony:=consumerNode(s,1,nil,"harmony");n:=consumerNode(s,2,harmony,"numeral");k:=consumerLocalKey(t,s,n)
  if tc.change!=nil {tc.change(s,k)}
  d,yes,e:=s.localNumeralKeyProof(context.Background(),n,p)
  if e!=nil||yes!=tc.conclusive||(yes&&d.Outcome!=tc.want){t.Fatalf("proof=%+v yes=%v error=%v",d,yes,e)}
  if tc.name=="malformed-mode"&&(d.Cause==nil||*d.Cause!=*cause){t.Fatalf("lost scalar cause %+v",d)}
 })}
}
func TestTimelineConsumerNumeralEffectiveKeyEvidence(t *testing.T) {
 cause:=&DecisionRef{Rule:RuleKey{"xsd.scalar",1},Subject:NodeRef{"d",40}}
 for _,tc:=range []struct{name string;prepare func(*testing.T,*Session,*timelineService);want Outcome;causal bool}{
  {"closed-absence",nil,OutcomeFail,false},
  {"traditional-zero",func(t *testing.T,s *Session,v *timelineService){statePublish(t,s,&v.State.Keys,statePosition(t,0,0,1),stateScope{AllStaves:true},stateKnown(keyState{Traditional:true,Fifths:stateKnown(stateInt(t,0))}),40)},OutcomePass,false},
  {"empty-nontraditional",func(t *testing.T,s *Session,v *timelineService){statePublish(t,s,&v.State.Keys,statePosition(t,0,0,1),stateScope{AllStaves:true},stateKnown(keyState{}),40)},OutcomePass,false},
  {"same-position-known-conflict-still-encoded",func(t *testing.T,s *Session,v *timelineService){for i:=int64(0);i<2;i++ {statePublish(t,s,&v.State.Keys,statePosition(t,0,0,1),stateScope{AllStaves:true},stateKnown(keyState{Traditional:true,Fifths:stateKnown(stateInt(t,i))}),NodeID(40+i))}},OutcomePass,false},
  {"other-staff-key",func(t *testing.T,s *Session,v *timelineService){statePublish(t,s,&v.State.Keys,statePosition(t,0,0,1),stateScope{Staff:stateInt(t,2)},stateKnown(keyState{}),40)},OutcomeFail,false},
  {"other-part-key",func(t *testing.T,s *Session,v *timelineService){at:=statePosition(t,0,0,1);at.Part.Node=9;statePublish(t,s,&v.State.Keys,at,stateScope{AllStaves:true},stateKnown(keyState{}),40)},OutcomeFail,false},
  {"unclosed-state",func(_ *testing.T,_ *Session,v *timelineService){v.State.Closures=nil},OutcomeUnknown,false},
  {"invalid-key",func(t *testing.T,s *Session,v *timelineService){statePublish(t,s,&v.State.Keys,statePosition(t,0,0,1),stateScope{AllStaves:true},Evidence[keyState]{State:EvidenceInvalid,Reason:"bad-key",Cause:cause},40)},OutcomeBlocked,true},
  {"position-gap",func(_ *testing.T,_ *Session,v *timelineService){f:=v.Facts[1];f.Position=Evidence[musicalPosition]{State:EvidenceInvalid,Reason:"bad-position",Cause:cause};v.Facts[1]=f},OutcomeBlocked,true},
  {"staff-gap",func(_ *testing.T,_ *Session,v *timelineService){f:=v.Facts[1];f.Staff=stateUnknown[Integer]("staff-unresolved");v.Facts[1]=f},OutcomeUnknown,false},
 } {t.Run(tc.name,func(t *testing.T){
  s,p:=consumerFixture(t);defer s.cleanup();v:=s.timeline[p.Provider]
  h:=consumerNode(s,1,nil,"harmony");n:=consumerNode(s,2,h,"numeral")
  v.Facts[1]=timelineEventFact{Position:stateKnown(statePosition(t,0,1,1)),Staff:stateKnown(stateInt(t,1))}
  stateClose(t,s,&v.State,0)
  if tc.prepare!=nil{tc.prepare(t,s,v)}
  d:=consumerEvaluate(t,s,n,p,planNumeralKey)
  if d.Outcome!=tc.want||(tc.causal&&(d.Cause==nil||*d.Cause!=*cause)){t.Fatalf("got %+v want %s",d,tc.want)}
 })}
}
func TestTimelineConsumerPendingAndCancellation(t *testing.T) {
 s,p:=consumerFixture(t);defer s.cleanup();h:=consumerNode(s,1,nil,"harmony");n:=consumerNode(s,2,h,"numeral")
 s.timeline[p.Provider].Final=false
 d,pending,e:=s.evaluateTimelineConsumer(context.Background(),n,p,planNumeralKey,false)
 if e!=nil||!pending {t.Fatalf("premature result %+v %v %v",d,pending,e)}
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 _,_,e=s.localNumeralKeyProof(ctx,n,p)
 if !errors.Is(e,context.Canceled){t.Fatalf("ignored cancellation %v",e)}
}
