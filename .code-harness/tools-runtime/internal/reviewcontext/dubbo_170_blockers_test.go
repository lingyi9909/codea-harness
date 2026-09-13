package reviewcontext

import (
	"context"
	"strings"
	"testing"
)

func Test170DubboCompetingProviderWithUnresolvedConfigBlocksExact(t *testing.T) {
	current, providerA, consumer := newDubboFixture170(t,
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
class RiskServiceImplA implements RiskService { public void check(String orderId) {} }
`, nil, nil)
	providerB := t.TempDir()
	writeDubboFiles170(t, providerB, map[string]string{
		"src/main/java/demo/RiskServiceImplB.java": `package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="${unknown.group}", version="1.0")
class RiskServiceImplB implements RiskService { public void check(String orderId) {} }
`,
	})

	relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{
		{Workspace: "provider-a", Root: providerA, verified: true},
		{Workspace: "provider-b", Root: providerB, verified: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution == "EXACT" {
		t.Fatalf("unresolved competing provider must block EXACT: relations=%+v issues=%+v", relations, issues)
	}
	if !strings.Contains(relations[0].Reason, "DUBBO_CONFIG_UNRESOLVED") || !hasIssue170(issues, "DUBBO_CONFIG_UNRESOLVED") {
		t.Fatalf("missing fail-closed unresolved-config evidence: relations=%+v issues=%+v", relations, issues)
	}
}

func Test170DubboDifferentFQCNParameterTypesNeverMatchBySimpleName(t *testing.T) {
	current, provider, _ := newDubboFixture170(t,
		`package demo;
import a.dto.Order;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="risk", version="1.0")
    RiskService risk;
    void submit(Order order) { risk.check(order); }
}
`,
		`package demo;
import b.dto.Order;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0")
class RiskServiceImpl implements RiskService { public void check(Order order) {} }
`, nil, nil)
	consumer := dubboConsumerRef170()
	consumer.ParameterTypes = []string{"a.dto.Order"}

	relations, _, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].Resolution == "EXACT" {
		t.Fatalf("different FQCN parameter types must not match: relations=%+v", relations)
	}
}

func Test170DubboReferenceReceiverShadowingIsNotRemoteField(t *testing.T) {
	t.Run("method parameter shadows field", func(t *testing.T) {
		current, provider, _ := newDubboFixture170(t,
			`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="risk", version="1.0")
    RiskService risk;
    void submit(String orderId, RiskService risk) { risk.check(orderId); }
}
`,
			`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0")
class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`, nil, nil)
		consumer := dubboConsumerRef170()
		consumer.ParameterTypes = []string{"java.lang.String", "demo.RiskService"}

		relations, _, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
		if err != nil {
			t.Fatal(err)
		}
		if len(relations) != 1 || relations[0].Resolution == "EXACT" {
			t.Fatalf("shadowed parameter receiver must not resolve as Dubbo field: relations=%+v", relations)
		}
	})

	t.Run("inner local shadows field", func(t *testing.T) {
		current, provider, consumer := newDubboFixture170(t,
			`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="risk", version="1.0")
    RiskService risk;
    void submit(String orderId) {
        RiskService risk = localRisk();
        risk.check(orderId);
    }
}
`,
			`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0")
class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`, nil, nil)

		relations, _, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
		if err != nil {
			t.Fatal(err)
		}
		if len(relations) != 1 || relations[0].Resolution == "EXACT" {
			t.Fatalf("shadowed local receiver must not resolve as Dubbo field: relations=%+v", relations)
		}
	})

	t.Run("explicit this field remains eligible", func(t *testing.T) {
		current, provider, _ := newDubboFixture170(t,
			`package demo;
import org.apache.dubbo.config.annotation.DubboReference;
class OrderService {
    @DubboReference(group="risk", version="1.0")
    RiskService risk;
    void submit(String orderId, RiskService risk) { this.risk.check(orderId); }
}
`,
			`package demo;
import org.apache.dubbo.config.annotation.DubboService;
@DubboService(group="risk", version="1.0")
class RiskServiceImpl implements RiskService { public void check(String orderId) {} }
`, nil, nil)
		consumer := dubboConsumerRef170()
		consumer.ParameterTypes = []string{"java.lang.String", "demo.RiskService"}

		relations, issues, err := ResolveDubbo170(context.Background(), current, consumer, []ProviderRoot170{{Workspace: "risk-provider", Root: provider, verified: true}})
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) != 0 || len(relations) != 1 || relations[0].Resolution != "EXACT" {
			t.Fatalf("explicit this.field Dubbo invocation should remain EXACT: relations=%+v issues=%+v", relations, issues)
		}
	})
}
