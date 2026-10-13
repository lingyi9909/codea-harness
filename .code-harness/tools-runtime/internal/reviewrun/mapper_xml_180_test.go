package reviewrun

import (
	"os"
	"path/filepath"
	"testing"
)

func Test180MapperXMLIndexesNamespaceAndStatementIDAttributes(t *testing.T) {
	root := t.TempDir()
	rel := "src/main/resources/mapper/OrderMapper.xml"
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	xml := `<mapper namespace="com.example.OrderMapper">
  <insert id="insertOrder">INSERT INTO orders(id) VALUES (1)</insert>
  <update id="cancelOrder">UPDATE orders SET status='CANCELLED'</update>
</mapper>`
	if err := os.WriteFile(full, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	got := indexMapperXML180(root, []string{rel})
	if got["com.example.OrderMapper.insertOrder"] != rel {
		t.Fatalf("insert mapper index=%v", got)
	}
	if got["com.example.OrderMapper.cancelOrder"] != rel {
		t.Fatalf("update mapper index=%v", got)
	}
	if _, ok := got["OrderMapper.insertOrder"]; ok {
		t.Fatalf("short mapper identity must not be indexed: %v", got)
	}
}

 
func Test180MapperXMLDuplicateNamespaceIsModuleScoped(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"module-a/src/main/resources/mapper/OrderMapper.xml",
		"module-b/src/main/resources/mapper/OrderMapper.xml",
	} {
		dest := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(dest, []byte(`<mapper namespace="com.example.OrderMapper"><select id="find">select 1</select></mapper>`), 0o600); err != nil { t.Fatal(err) }
	}
	files := []string{"module-a/src/main/resources/mapper/OrderMapper.xml","module-b/src/main/resources/mapper/OrderMapper.xml"}
	index := indexMapperXMLCandidates180(root,files)
	paths := index["com.example.OrderMapper.find"]
	if len(paths) != 2 { t.Fatalf("duplicate mapper declaration was collapsed: %+v",index) }
	for _, module := range []string{"module-a","module-b"} {
		want := module+"/src/main/resources/mapper/OrderMapper.xml"
		got,ok := selectMapperXML180(module+"/src/main/java/com/example/OrderMapper.java",paths)
		if !ok || got != want { t.Fatalf("incorrect Mapper XML module: %q %t want %q",got,ok,want) }
	}
	if selected,ok := selectMapperXML180("module-c/src/main/java/com/example/OrderMapper.java",paths); ok {
		t.Fatalf("unrelated module falsely resolved duplicate Mapper XML %q",selected)
	}
}

func Test180MapperXMLSingleCrossModuleCandidatePermitted(t *testing.T) {
	x := "module-data/src/main/resources/mapper/OrderMapper.xml"
	got,ok := selectMapperXML180("module-impl/src/main/java/com/example/OrderMapper.java",[]string{x})
	if !ok || got != x { t.Fatalf("unique cross-module mapper declaration lost: %q %t",got,ok) }
}
