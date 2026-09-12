package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/nav"
)

func Test170ProjectDiscoveryMyBatisExactProjectsMapperXML(t *testing.T) {
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
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
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

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	}()

	navigator := nav.Navigator{RepoRoot: root, AstGrepPath: exe}
	call := nav.DirectMethodCall{
		FromSymbol:   "OrderService.run",
		TargetSymbol: "OrderMapper.updateStatus",
		Receiver:     "mapper",
		ReceiverType: "OrderMapper",
		Method:       "updateStatus",
		Path:         "src/main/java/demo/OrderService.java",
		Line:         4,
		Resolved:     true,
	}
	paths, unresolved, err := projectResolveCall163(context.Background(), root, navigator, call, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(unresolved) != 0 {
		t.Fatalf("exact mapper statement must stay resolved: %+v", unresolved)
	}
	if len(paths) != 1 {
		t.Fatalf("exact mapper statement must produce one chain path: %+v", paths)
	}
	if len(paths[0].Nodes) != 1 || paths[0].Nodes[0].Role != "MAPPER" || paths[0].Nodes[0].Symbol != "OrderMapper.updateStatus" {
		t.Fatalf("mapper node projection=%+v", paths[0].Nodes)
	}
	if len(paths[0].Resources) != 1 {
		t.Fatalf("mapper resource projection=%+v", paths[0].Resources)
	}
	resource := paths[0].Resources[0]
	if resource.Role != "MAPPER_XML" || resource.Path != "src/main/resources/OrderMapper.xml" || resource.Symbol != "OrderMapper.updateStatus" {
		t.Fatalf("mapper XML projection=%+v", resource)
	}
}
