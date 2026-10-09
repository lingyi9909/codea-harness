package main

import (
 "encoding/json"
 "strings"
 "testing"
)

func Test180NativeWindowsJSONUnicodeRoundTrip(t *testing.T) {
 value:=map[string]string{"message":"中文提示：评审未完成😀"}
 raw,err:=json.Marshal(value)
 if err!=nil{t.Fatal(err)}
 safe:=escapeNonASCIIJSON180(raw)
 for _,b:=range safe {if b>=0x80 {t.Fatalf("native JSON remains non-ASCII: %q",safe)}}
 var result map[string]string
 if err:=json.Unmarshal(safe,&result);err!=nil{t.Fatal(err)}
 if result["message"]!=value["message"] {t.Fatalf("Unicode damaged: %+v",result)}
}

func Test180ReviewPrepareIntentAliasAndConflict(t *testing.T) {
 id:="review-"+strings.Repeat("a",32)
 // There is no such run on disk: accepted CLI parsing must proceed to a
 // REVIEW_RUN_READ_FAILED error rather than 'flag not defined'.
 err:=runReviewPrepare180([]string{"--run-id",id,"--intent","CHANGES"})
 if err==nil || !strings.Contains(err.Error(),"REVIEW_RUN_READ_FAILED") {
  t.Fatalf("--intent alias rejected before run lookup: %v",err)
 }
 err=runReviewPrepare180([]string{"--run-id",id,"--mode","CHANGES","--intent","CURRENT_IMPLEMENTATION"})
 if err==nil || !strings.Contains(err.Error(),"REVIEW_PREPARE_INTENT_CONFLICT") {
  t.Fatalf("conflicting alias accepted: %v",err)
 }
}

func Test180NativeWindowsErrorUnicodeTransport(t *testing.T) {
  input := []byte("REVIEW_PREPARE_FAILED: 中文路径/订单")
  safe := escapeNonASCIIJSON180(input)
  for _, b := range safe { if b >= 0x80 { t.Fatalf("native stderr transport is not ASCII-safe: %q", safe) } }
  if !strings.Contains(string(safe), "\\u4e2d") || !strings.Contains(string(safe), "REVIEW_PREPARE_FAILED") {
    t.Fatalf("error code or Unicode content lost: %q", safe)
  }
}
