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
