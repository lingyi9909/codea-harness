package reviewauthority

import (
    "testing"
)

func Test180SelectNeedsNextActualUser(t *testing.T) {
    cases := []struct{name string; data string; messageID string; want bool}{
        {"next user selects", export180("m-menu","assistant","run-1 options=hash-1\nC1 create\nC2 cancel","m-user","user","选择 C1"), "m-user", true},
        {"assistant selects", export180("m-menu","assistant","run-1 options=hash-1\nC1 create\nC2 cancel","m-a","assistant","选择 C1"), "m-a", false},
        {"user before menu", export180("m-user","user","选择 C1","m-menu","assistant","run-1 options=hash-1\nC1 create\nC2 cancel"), "m-user", false},
        {"continue is not choice", export180("m-menu","assistant","run-1 options=hash-1\nC1 create\nC2 cancel","m-user","user","继续"), "m-user", false},
        {"old menu user then new menu", `{"info":{"id":"s","directory":"C:/repo"},"messages":[{"info":{"id":"old","role":"assistant"},"parts":[{"type":"text","text":"run-1 options=hash-1 C1 create C2 cancel"}]},{"info":{"id":"m-user","role":"user"},"parts":[{"type":"text","text":"全部"}]},{"info":{"id":"new","role":"assistant"},"parts":[{"type":"text","text":"run-1 options=hash-2 C1 create C2 cancel"}]}]}`, "m-user", false},
    }
    for _, tc := range cases { t.Run(tc.name, func(t *testing.T){
        err := VerifySelectionTurnBytes180([]byte(tc.data), SelectionTurnRequest180{SessionID:"s",MessageID:tc.messageID,RunID:"run-1",OptionsHash:"hash-1",SelectionIDs:[]string{"C1"},AllIDs:[]string{"C1","C2"},MenuMarker:"C1 create\nC2 cancel"})
        if tc.want && err != nil { t.Fatalf("expected accept: %v", err) }
        if !tc.want && err == nil { t.Fatal("expected rejection") }
    })}
}

func Test180SelectTurnRejectsOtherProject(t *testing.T) {
    data := export180("m-menu","assistant","run-1 options=hash-1\nC1 create\nC2 cancel","m-user","user","选择 C1")
    err := verifySelectionTurnBytesForRoot180([]byte(data), `D:\other`, SelectionTurnRequest180{SessionID:"s",MessageID:"m-user",RunID:"run-1",OptionsHash:"hash-1",SelectionIDs:[]string{"C1"},AllIDs:[]string{"C1","C2"},MenuMarker:"C1 create\nC2 cancel"})
    if err == nil { t.Fatal("cross-project user turn accepted") }
}

func export180(id1,role1,text1,id2,role2,text2 string) string {
    return `{"info":{"id":"s","directory":"C:/repo"},"messages":[{"info":{"id":"`+id1+`","role":"`+role1+`"},"parts":[{"type":"text","text":"`+text1+`"}]},{"info":{"id":"`+id2+`","role":"`+role2+`"},"parts":[{"type":"text","text":"`+text2+`"}]}]}`
}
