package projectpath

import "testing"

func TestClassifyMavenRepoRelativePaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want Kind
	}{
		{"root main java", "src/main/java/acme/Root.java", MainJava},
		{"module main java", "order-service/src/main/java/acme/OrderService.java", MainJava},
		{"module test java", "order-service/src/test/java/acme/OrderServiceTest.java", TestJava},
		{"module mapper xml", "order-dao/src/main/resources/mapper/OrderMapper.xml", MapperXML},
		{"module yaml", "order-service/src/main/resources/application.yml", YAMLConfig},
		{"other java", "tools/src/foo/java/acme/Other.java", Other},
		{"substring trap", "xsrc/main/java/acme/Nope.java", Other},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.path); got != tt.want {
				t.Fatalf("Classify(%q)=%v want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestClassifyPreservesRepoRelativeModulePrefix(t *testing.T) {
	p, ok := Normalize("module-a/src/main/java/com/xxx/UserService.java")
	if !ok {
		t.Fatal("expected valid repo-relative path")
	}
	if p != "module-a/src/main/java/com/xxx/UserService.java" {
		t.Fatalf("normalized path=%q", p)
	}
}
