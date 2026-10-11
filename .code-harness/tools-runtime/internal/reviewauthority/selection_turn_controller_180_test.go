package reviewauthority
import ("strings";"testing")
func controllerSelectionRequest180(messageID string, ids ...string) SelectionTurnRequest180 {
 return SelectionTurnRequest180{SessionID:"s",MessageID:messageID,RunID:"run-1",OptionsHash:"hash-1",SelectionIDs:ids,AllIDs:[]string{"C1","C2","C3"},MenuMarker:"C1 OrderController（5 个受影响方法）\nC2 UserController（1 个受影响方法）\nC3 AdminController（2 个受影响方法）"}
}
const currentControllerMenu180 = "run-1 options=hash-1\nC1 OrderController（5 个受影响方法）\nC2 UserController（1 个受影响方法）\nC3 AdminController（2 个受影响方法）"
func TestT5ActualHostUserSelectionNormalizationController(t *testing.T) {
 for _,tc:=range []struct{text string;ids []string}{
 {"C1",[]string{"C1"}},{"选择C1",[]string{"C1"}},{"选择 C1",[]string{"C1"}},
 {"C1,C3",[]string{"C1","C3"}},{"选择 C1,C3",[]string{"C1","C3"}},
 {"选择C3",[]string{"C3"}},{"选择 C3",[]string{"C3"}},
 {"全部",[]string{"C1","C2","C3"}},
 } {t.Run(strings.ReplaceAll(tc.text," ","_"),func(t *testing.T){
 data:=export180("menu","assistant",currentControllerMenu180,"actual","user",tc.text)
 if err:=VerifySelectionTurnBytes180([]byte(data),controllerSelectionRequest180("actual",tc.ids...));err!=nil{t.Fatalf("real user %q rejected: %v",tc.text,err)}
 })}
}
func TestT5RejectUnknownAmbiguousOrModelInventedSelection(t *testing.T) {
 for _,raw:=range []string{"C4","选C1","我想选C1","C1或C2","C1,C1","选择","C1;C3","c1","C1, C9"} {
 t.Run(raw,func(t *testing.T){
 data:=export180("menu","assistant",currentControllerMenu180,"actual","user",raw)
 if err:=VerifySelectionTurnBytes180([]byte(data),controllerSelectionRequest180("actual","C1"));err==nil{t.Fatalf("accepted invalid real user choice %q",raw)}
 })}
 data:=export180("menu","assistant",currentControllerMenu180,"actual","user","C3")
 if err:=VerifySelectionTurnBytes180([]byte(data),controllerSelectionRequest180("actual","C1"));err==nil{t.Fatal("model's selected ID overrode real user C3")}
}
func TestT5AuxiliaryConversationMenuMustBeReDisplayedInSameRun(t *testing.T) {
 msgs:=[]exportMessage180{
 {id:"first",role:"assistant",text:currentControllerMenu180},
 {id:"question",role:"user",text:"这些 Controller 分别是什么？"},
 {id:"answer",role:"assistant",text:"订单及用户 Controller。"},
 {id:"choice",role:"user",text:"选择 C3"},
 }
 if err:=VerifySelectionTurnBytes180([]byte(exportMessages180(msgs)),controllerSelectionRequest180("choice","C3"));err==nil{t.Fatal("accepted old menu without redisplay")}
 msgs=append(msgs[:3],exportMessage180{id:"resume",role:"assistant",text:currentControllerMenu180},msgs[3])
 if err:=VerifySelectionTurnBytes180([]byte(exportMessages180(msgs)),controllerSelectionRequest180("choice","C3"));err!=nil{t.Fatalf("resumed menu rejected: %v",err)}
 msgs[3].text=strings.Replace(currentControllerMenu180,"hash-1","hash-old",1)
 if err:=VerifySelectionTurnBytes180([]byte(exportMessages180(msgs)),controllerSelectionRequest180("choice","C3"));err==nil{t.Fatal("accepted stale re-rendered menu")}
}
func TestT5RejectSyntheticChoiceEvenWithValidText(t *testing.T) {
 data:=export180("menu","assistant",currentControllerMenu180,"actual","user","C1")
 data=strings.Replace(data,"\"text\":\"C1\"","\"text\":\"C1\",\"synthetic\":true",1)
 if err:=VerifySelectionTurnBytes180([]byte(data),controllerSelectionRequest180("actual","C1"));err==nil{t.Fatal("synthetic user choice accepted")}
}
