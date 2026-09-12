package chain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

func Test170ProjectDiscoveryMyBatisDatabaseIDFailsClosed(t *testing.T) {
	exe := os.Getenv("CODEA_AST_GREP")
	if exe == "" {
		t.Fatal("CODEA_AST_GREP must point to the approved ast-grep binary")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	files := map[string]string{
		"src/main/java/demo/OrderService.java": `package demo;
class OrderService {
    OrderMapper mapper;
    void run() { mapper.updateStatus(1L); }
}
`,
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Mapper;
@Mapper
interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus" databaseId="mysql">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
  <update id="updateStatus" databaseId="oracle">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
</mapper>
`,
	}
	for path, content := range files {
		dst := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	navigator := nav.Navigator{RepoRoot: root, AstGrepPath: exe}
	paths, unresolved, err := projectWalkMethod163(context.Background(), root, navigator, "OrderService.run", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("databaseId-dependent mapper statement must not produce a guessed chain resource: %+v", paths)
	}
	found := false
	for _, reason := range unresolved {
		if strings.Contains(reason, "MYBATIS_DATABASE_ID_UNRESOLVED") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("databaseId boundary must remain visible in chain discovery: %+v", unresolved)
	}
}
