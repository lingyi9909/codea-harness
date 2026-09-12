package reviewcontext

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

func Test170MyBatisBaseCurrentEvidenceShowsTenantConditionDeletion(t *testing.T) {
	baseRoot, baseSource := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int updateStatus(@Param("tenantId") Long tenantId, @Param("id") Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">
    UPDATE orders SET status = 'DONE'
    WHERE id = #{id} AND tenant_id = #{tenantId}
  </update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")
	baseSource.Ref.Side = "BASE"

	currentRoot, currentSource := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int updateStatus(@Param("tenantId") Long tenantId, @Param("id") Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">
    UPDATE orders SET status = 'DONE'
    WHERE id = #{id}
  </update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	baseRelations, baseIssues, err := ResolveMapper170(context.Background(), baseRoot, baseSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseIssues) != 0 {
		t.Fatalf("base issues=%+v", baseIssues)
	}
	currentRelations, currentIssues, err := ResolveMapper170(context.Background(), currentRoot, currentSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(currentIssues) != 0 {
		t.Fatalf("current issues=%+v", currentIssues)
	}

	baseStatement := relationByKind170(t, baseRelations, "MYBATIS_STATEMENT")
	currentStatement := relationByKind170(t, currentRelations, "MYBATIS_STATEMENT")
	if baseStatement.Resolution != "EXACT" || currentStatement.Resolution != "EXACT" {
		t.Fatalf("expected exact statements: base=%+v current=%+v", baseStatement, currentStatement)
	}
	baseText, baseEvidence := myBatisStatementEvidenceText170(t, baseRoot, baseStatement)
	currentText, currentEvidence := myBatisStatementEvidenceText170(t, currentRoot, currentStatement)
	if baseEvidence.Ref.Side != "BASE" || currentEvidence.Ref.Side != "CURRENT" {
		t.Fatalf("statement evidence sides not preserved: base=%+v current=%+v", baseEvidence.Ref, currentEvidence.Ref)
	}
	if !strings.Contains(baseText, "tenant_id = #{tenantId}") {
		t.Fatalf("BASE statement evidence lost tenant condition: %q", baseText)
	}
	if strings.Contains(currentText, "tenant_id = #{tenantId}") {
		t.Fatalf("CURRENT statement evidence still contains deleted tenant condition: %q", currentText)
	}
}

func myBatisStatementEvidenceText170(t *testing.T, root string, relation nav.Relation170) (string, nav.SourceRange170) {
	t.Helper()
	for _, evidence := range relation.Evidence {
		if evidence.Ref.Kind != "STATEMENT" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(evidence.Ref.Path)))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if evidence.StartLine < 1 || evidence.EndLine < evidence.StartLine || evidence.EndLine > len(lines) {
			t.Fatalf("invalid statement evidence range: %+v", evidence)
		}
		return strings.Join(lines[evidence.StartLine-1:evidence.EndLine], "\n"), evidence
	}
	t.Fatalf("statement XML evidence not found: %+v", relation)
	return "", nav.SourceRange170{}
}
