package nav

import (
	"os"
	"path/filepath"
	"testing"
)

func Test152NestedClassMethodMustNotBelongToOuterWorkspaceClass(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	writeJava(t, current, "src/main/java/com/company/order/XxxServiceImpl.java", `package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate {
    class Helper {
        void submit() {
            execute();
        }
    }
    @Override protected void doExecute() { mapper.updateStatus(); }
}`)
	_ = os.Remove(filepath.Join(current, "src/main/java/com/company/order/Multi.java"))

	resolver := WorkspaceInheritanceResolver{CurrentRoot: current, Dependency: verifiedWorkspace(dependency)}
	result := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	if result.Status != NavigationPartial || result.Limitation == nil || result.Limitation.Code != CodeInheritedMethodNotFound {
		t.Fatalf("nested Helper.submit must not be owned by XxxServiceImpl: %#v", result)
	}
	if result.Fact != nil {
		t.Fatalf("nested-class ownership ambiguity must never create confirmed workspace fact: %#v", result.Fact)
	}
}
