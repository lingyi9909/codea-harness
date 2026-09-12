package reviewcontext

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/workspace"
)

func Test170DubboExactContractAcrossVerifiedProvider(t *testing.T) {
	current, provider, consumer := newDubboFixture170(t,
		`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="risk", version="1.0")
    RiskService risk;
    void submit(String orderId) { risk.check(orderId); }
}
`,
		`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0")
class RiskServiceImpl implements RiskService {
    public void check(String orderId) {}
}
`, nil, nil)

	roots := ProviderRootsFromVerification170([]workspace.VerificationResult{{
		DependencyID:  "risk-provider",
		Status:        workspace.StatusVerified,
		ConfirmedRoot: provider,
	}})
	relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%+v", issues)
	}
	if len(relations) != 1 {
		t.Fatalf("relations=%+v", relations)
	}
	relation := relations[0]
	if relation.Kind != "DUBBO_CONTRACT" || relation.Resolution != "EXACT" || len(relation.Targets) != 1 {
		t.Fatalf("relation=%+v", relation)
	}
	target := relation.Targets[0]
	if target.Workspace != "risk-provider" || target.OwnerFQCN != "demo.RiskServiceImpl" || target.Name != "check" {
		t.Fatalf("target=%+v", target)
	}
	if len(target.ParameterTypes) != 1 || target.ParameterTypes[0] != "java.lang.String" {
		t.Fatalf("target parameter types=%v", target.ParameterTypes)
	}
	if !containsString170(relation.Assumptions, "dubbo.interface=demo.RiskService") || !containsString170(relation.Assumptions, "dubbo.group=risk") || !containsString170(relation.Assumptions, "dubbo.version=1.0") {
		t.Fatalf("assumptions=%v", relation.Assumptions)
	}
	if err := nav.ValidateRelation170(relation); err != nil {
		t.Fatalf("invalid relation: %v", err)
	}
}

func Test170DubboResolvedLocalPropertiesCanMatch(t *testing.T) {
	current, provider, consumer := newDubboFixture170(t,
		`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="${dubbo.group}", version="${dubbo.version}")
    RiskService risk;
    void submit(String orderId) { risk.check(orderId); }
}
`,
		`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0")
class RiskServiceImpl implements RiskService {
    public void check(String orderId) {}
}
`, map[string]string{
		"src/main/resources/application.properties": "dubbo.group=risk\ndubbo.version=1.0\n",
	}, nil)
	roots := []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}}
	relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 || len(relations) != 1 || relations[0].Resolution != "EXACT" {
		t.Fatalf("relations=%+v issues=%+v", relations, issues)
	}
}

func Test170DubboGroupAndVersionMismatchNeverExact(t *testing.T) {
	cases := []struct {
		name     string
		provider string
	}{
		{name: "group", provider: `package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="other", version="1.0") class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`},
		{name: "version", provider: `package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="2.0") class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			current, provider, consumer := newDubboFixture170(t,
				`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService { @DubboReference(group="risk", version="1.0") RiskService risk; void submit(String orderId) { risk.check(orderId); } }
`, tc.provider, nil, nil)
		relations, _, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
		if err != nil {
			t.Fatal(err)
		}
		if len(relations) != 1 || relations[0].Resolution == "EXACT" {
			t.Fatalf("relations=%+v", relations)
		}
	})
}

func Test170DubboUnresolvedPlaceholderFailsClosed(t *testing.T) {
	current, provider, consumer := newDubboFixture170(t,
		`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService { @DubboReference(group="${missing.group}", version="1.0") RiskService risk; void submit(String orderId) { risk.check(orderId); } }
`,
		`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0") class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`, nil, nil)
	relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution == "EXACT" || !strings.Contains(relations[0].Reason, "DUBBO_CONFIG_UNRESOLVED") {
		t.Fatalf("relations=%+v", relations)
	}
	if !hasIssue170(issues, "DUBBO_CONFIG_UNRESOLVED") {
		t.Fatalf("issues=%+v", issues)
	}
}

func Test170DubboMultipleProvidersStayAmbiguous(t *testing.T) {
	current, providerA, consumer := newDubboFixture170(t,
		`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService { @DubboReference(group="risk", version="1.0") RiskService risk; void submit(String orderId) { risk.check(orderId); } }
`,
		`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0") class RiskServiceImplA implements RiskService { public void check(String orderId) {} }
`, nil, nil)
	providerB := t.TempDir()
	writeDubboFiles170(t, providerB, map[string]string{
		"src/main/java/demo/RiskServiceImplB.java": `package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0") class RiskServiceImplB implements RiskService { public void check(String orderId) {} }
`,
	})
	relations, _, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{
		{Workspace: "provider-a", Root: providerA, verified: true},
		{Workspace: "provider-b", Root: providerB, verified: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution != "AMBIGUOUS" || len(relations[0].Targets) != 2 {
		t.Fatalf("relations=%+v", relations)
	}
}

func Test170DubboOrdinarySameNameAnnotationIsIgnored(t *testing.T) {
	current, provider, consumer := newDubboFixture170(t,
		`package demo;
import com.other.Reference;
class OrderService { @Reference(group="risk", version="1.0") RiskService risk; void submit(String orderId) { risk.check(orderId); } }
`,
		`package demo;
import com.other.Service;
@Service(group="risk", version="1.0") class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`, nil, nil)
	relations, _, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution == "EXACT" {
		t.Fatalf("relations=%+v", relations)
	}
}

func Test170DubboMissingProviderAndUndeclaredSiblingAreUnavailable(t *testing.T) {
	parent := t.TempDir()
	current := filepath.Join(parent, "consumer")
	sibling := filepath.Join(parent, "provider")
	writeDubboFiles170(t, current, map[string]string{
		"src/main/java/demo/OrderService.java": `package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService { @DubboReference(group="risk", version="1.0") RiskService risk; void submit(String orderId) { risk.check(orderId); } }
`,
	})
	writeDubboFiles170(t, sibling, map[string]string{
		"src/main/java/demo/RiskServiceImpl.java": `package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0") class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`,
	})
	consumer := dubboConsumerRef170()
	relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution == "EXACT" || !strings.Contains(relations[0].Reason, "PROVIDER_SOURCE_UNAVAILABLE") {
		t.Fatalf("relations=%+v", relations)
	}
	if !hasIssue170(issues, "PROVIDER_SOURCE_UNAVAILABLE") {
		t.Fatalf("issues=%+v", issues)
	}
}

func Test170DubboUnverifiedWorkspaceIsExcluded(t *testing.T) {
	current, provider, consumer := newDubboFixture170(t,
		`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService { @DubboReference(group="risk", version="1.0") RiskService risk; void submit(String orderId) { risk.check(orderId); } }
`,
		`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0") class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`, nil, nil)
	roots := ProviderRootsFromVerification170([]workspace.VerificationResult{
		{DependencyID: "risk-provider", Status: workspace.StatusVersionMismatch, ConfirmedRoot: provider},
	})
	if len(roots) != 0 {
		t.Fatalf("unverified workspace leaked into provider roots: %+v", roots)
	}
	relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider}})
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution == "EXACT" || !hasIssue170(issues, "PROVIDER_SOURCE_UNAVAILABLE") {
		t.Fatalf("relations=%+v issues=%+v", relations, issues)
	}
}

func newDubboFixture170(t *testing.T, consumerJava, providerJava string, currentExtra, providerExtra map[string]string) (string, string, nav.ReviewRef170) {
	t.Helper()
	current := t.TempDir()
	provider := t.TempDir()
	currentFiles := map[string]string{"src/main/java/demo/OrderService.java": consumerJava}
	for path, data := range currentExtra {
		currentFiles[path] = data
	}
	providerFiles := map[string]string{"src/main/java/demo/RiskServiceImpl.java": providerJava}
	for path, data := range providerExtra {
		providerFiles[path] = data
	}
	writeDubboFiles170(t, current, currentFiles)
	writeDubboFiles170(t, provider, providerFiles)
	return current, provider, dubboConsumerRef170()
}

func dubboConsumerRef170() nav.ReviewRef170 {
	return nav.ReviewRef170{
		Workspace:      "current",
		Path:           "src/main/java/demo/OrderService.java",
		Side:           "CURRENT",
		Kind:           "METHOD",
		OwnerFQCN:      "demo.OrderService",
		Name:           "submit",
		ParameterTypes: []string{"java.lang.String"},
	}
}

func writeDubboFiles170(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, data := range files {
		dst := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func containsString170(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
