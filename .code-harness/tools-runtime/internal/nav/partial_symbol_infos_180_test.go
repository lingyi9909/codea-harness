package nav

import (
  "context"
  "errors"
  "testing"
)

func Test180PartialSymbolInfoRetainsLocalSymbolsWhenExternalReceiverMissing(t *testing.T) {
 r:=&batchRunner167{out: append(
   batchJSON167("src/main/java/acme/OrderController.java", "public class OrderController { public void create() {} }",1,1),
   batchJSON167("src/main/java/acme/OrderController.java", "public void create() {}",1,1)...,
 )}
 n:=Navigator{AstGrepPath:"ast-grep",Runner:r}
 got,err:=n.GetSymbolInfosPartial180(context.Background(),[]string{"NonexistentExternalType","OrderController.create"},"src/main/java")
 if err!=nil{t.Fatal(err)}
 if _,ok:=got["OrderController.create"];!ok{t.Fatalf("local symbol lost when external receiver absent: %+v",got)}
 if _,ok:=got["NonexistentExternalType"];ok{t.Fatalf("invented external symbol: %+v",got)}
 _,err=n.GetSymbolInfos(context.Background(),[]string{"NonexistentExternalType","OrderController.create"},"src/main/java")
 if !errors.Is(err,ErrSymbolNotFound){t.Fatalf("strict legacy resolver weakened: %v",err)}
}
