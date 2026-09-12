package reviewcontext

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"codea-harness-tools/internal/nav"
)

func Test170MyBatisStatementAndIncludeRelations(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
@Mapper
public interface OrderMapper {
    int updateStatus(@Param("tenantId") Long tenantId, @Param("status") String status, @Param("id") Long id);
}
`,
		"src/main/resources/mapper/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <sql id="tenantFilter">tenant_id = #{tenantId}</sql>
  <update id="updateStatus">
    UPDATE orders SET status = #{status}
    WHERE id = #{id} AND <include refid="tenantFilter"/>
  </update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%+v", issues)
	}
	if len(relations) != 2 {
		t.Fatalf("relations=%+v", relations)
	}
	statement := relationByKind170(t, relations, "MYBATIS_STATEMENT")
	if statement.Resolution != "EXACT" || len(statement.Targets) != 1 {
		t.Fatalf("statement=%+v", statement)
	}
	if got := statement.Targets[0]; got.Kind != "STATEMENT" || got.OwnerFQCN != "demo.OrderMapper" || got.Name != "updateStatus" || got.Path != "src/main/resources/mapper/OrderMapper.xml" {
		t.Fatalf("statement target=%+v", got)
	}
	include := relationByKind170(t, relations, "SQL_INCLUDE")
	if include.Resolution != "EXACT" || len(include.Targets) != 1 {
		t.Fatalf("include=%+v", include)
	}
	if got := include.Targets[0]; got.Kind != "SQL_FRAGMENT" || got.OwnerFQCN != "demo.OrderMapper" || got.Name != "tenantFilter" {
		t.Fatalf("include target=%+v", got)
	}
	for _, relation := range relations {
		if err := nav.ValidateRelation170(relation); err != nil {
			t.Fatalf("invalid relation %+v: %v", relation, err)
		}
	}
}

func Test170MyBatisExplicitParamContractMismatchIsReported(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int updateStatus(@Param("tenantId") Long tenantId, @Param("status") String status);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">UPDATE orders SET status = #{status} WHERE tenant_id = #{tenantCode}</update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	statement := relationByKind170(t, relations, "MYBATIS_STATEMENT")
	if statement.Resolution != "EXACT" {
		t.Fatalf("statement association should remain exact: %+v", statement)
	}
	if !hasIssue170(issues, "MYBATIS_PARAM_CONTRACT_MISMATCH") {
		t.Fatalf("issues=%+v", issues)
	}
}

func Test170MyBatisDynamicSQLStaysContextNotAutomaticFinding(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int list(@Param("tenantId") Long tenantId, @Param("orderBy") String orderBy);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <select id="list" resultType="demo.Order">
    SELECT * FROM orders WHERE 1 = 1
    <if test="tenantId != null">AND tenant_id = #{tenantId}</if>
    ORDER BY ${orderBy}
  </select>
</mapper>
`,
	}, "demo.OrderMapper", "list")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	statement := relationByKind170(t, relations, "MYBATIS_STATEMENT")
	if statement.Resolution != "EXACT" {
		t.Fatalf("statement=%+v", statement)
	}
	if !containsAssumption170(statement.Assumptions, "mybatis.dynamicSql=true") || !containsAssumption170(statement.Assumptions, "mybatis.rawSubstitution=true") || !containsAssumption170(statement.Assumptions, "mybatis.resultType=demo.Order") {
		t.Fatalf("statement assumptions=%+v", statement.Assumptions)
	}
	for _, issue := range issues {
		if strings.Contains(issue.Code, "INJECTION") || strings.Contains(issue.Code, "VULNERABILITY") {
			t.Fatalf("dynamic SQL must not become an automatic finding: %+v", issues)
		}
	}
}

func Test170MyBatisCrossNamespaceInclude(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE <include refid="demo.CommonSql.byId"/></update>
</mapper>
`,
		"src/main/resources/CommonSql.xml": `<mapper namespace="demo.CommonSql">
  <sql id="byId">id = #{id}</sql>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%+v", issues)
	}
	include := relationByKind170(t, relations, "SQL_INCLUDE")
	if include.Resolution != "EXACT" || len(include.Targets) != 1 || include.Targets[0].OwnerFQCN != "demo.CommonSql" || include.Targets[0].Name != "byId" {
		t.Fatalf("include=%+v", include)
	}
}

func Test170MyBatisIncludeCycleFailsClosed(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <sql id="a">x = 1 AND <include refid="b"/></sql>
  <sql id="b">y = 2 AND <include refid="a"/></sql>
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE <include refid="a"/></update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue170(issues, "SQL_INCLUDE_CYCLE") {
		t.Fatalf("issues=%+v", issues)
	}
	if !hasRelationReason170(relations, "SQL_INCLUDE_CYCLE") {
		t.Fatalf("relations=%+v", relations)
	}
}

func Test170MyBatisMissingIncludeIsUnresolved(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE <include refid="missingFilter"/></update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue170(issues, "SQL_INCLUDE_UNRESOLVED") {
		t.Fatalf("issues=%+v", issues)
	}
	if !hasRelationReason170(relations, "SQL_INCLUDE_UNRESOLVED") {
		t.Fatalf("relations=%+v", relations)
	}
}

func Test170MyBatisDatabaseIDWithoutConfigurationNeverExact(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus" databaseId="mysql">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
  <update id="updateStatus" databaseId="oracle">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("databaseId uncertainty is a relation boundary, issues=%+v", issues)
	}
	statement := relationByKind170(t, relations, "MYBATIS_STATEMENT")
	if statement.Resolution != "CONDITIONAL" || !strings.Contains(statement.Reason, "MYBATIS_DATABASE_ID_UNRESOLVED") {
		t.Fatalf("statement=%+v", statement)
	}
}

func Test170MyBatisRejectsDOCTYPE(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<!DOCTYPE mapper SYSTEM "https://example.invalid/mybatis.dtd">
<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	_, _, err := ResolveMapper170(context.Background(), root, source)
	if err == nil || !strings.Contains(err.Error(), "MYBATIS_XML_DTD_UNSUPPORTED") {
		t.Fatalf("err=%v", err)
	}
}

func newMyBatisFixture170(t *testing.T, files map[string]string, owner, method string) (string, nav.SourceRange170) {
	t.Helper()
	root := t.TempDir()
	javaPath := ""
	javaText := ""
	for path, content := range files {
		dst := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(path, ".java") && strings.Contains(content, method+"(") {
			javaPath, javaText = path, content
		}
	}
	if javaPath == "" {
		t.Fatal("mapper java source not found")
	}
	lines := strings.Split(strings.ReplaceAll(javaText, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.Contains(line, method+"(") {
			ref := nav.ReviewRef170{Workspace: "current", Path: javaPath, Side: "CURRENT", Kind: "METHOD", OwnerFQCN: owner, Name: method, ParameterTypes: []string{}}
			return root, nav.SourceRange170{Ref: ref, StartLine: i + 1, EndLine: i + 1, StartColumn: 1, EndColumn: utf8.RuneCountInString(line) + 1}
		}
	}
	t.Fatal("mapper method line not found")
	return "", nav.SourceRange170{}
}

func relationByKind170(t *testing.T, relations []nav.Relation170, kind string) nav.Relation170 {
	t.Helper()
	for _, relation := range relations {
		if relation.Kind == kind {
			return relation
		}
	}
	t.Fatalf("relation kind %s not found: %+v", kind, relations)
	return nav.Relation170{}
}

func hasIssue170(issues []nav.Issue170, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func hasRelationReason170(relations []nav.Relation170, code string) bool {
	for _, relation := range relations {
		if relation.Resolution != "EXACT" && strings.Contains(relation.Reason, code) {
			return true
		}
	}
	return false
}

func containsAssumption170(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
