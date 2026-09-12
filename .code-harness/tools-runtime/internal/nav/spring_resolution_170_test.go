package nav

import (
	"context"
	"strings"
	"testing"
)

func springInjection170(path, owner, declaredType, name, kind string) Injection170 {
	return Injection170{
		Owner: ReviewRef170{
			Workspace: "current",
			Path:      path,
			Side:      "CURRENT",
			Kind:      "TYPE",
			OwnerFQCN: owner,
			Name:      owner,
		},
		Name:         name,
		DeclaredType: declaredType,
		Kind:         kind,
	}
}

func Test170SpringQualifierUniqueExact(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; @Service("a") class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/B.java": `package demo; import org.springframework.stereotype.Service; @Service("b") class B implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; import org.springframework.beans.factory.annotation.Qualifier; class Checkout { @Qualifier("b") PayService service; }`,
	})
	in := springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD")
	in.Qualifier = "b"
	rel, err := n.ResolveInjection170(context.Background(), in)
	if err != nil { t.Fatal(err) }
	if rel.Resolution != "EXACT" || len(rel.Targets) != 1 || rel.Targets[0].OwnerFQCN != "demo.B" {
		t.Fatalf("relation=%+v", rel)
	}
	if err := ValidateRelation170(rel); err != nil { t.Fatalf("invalid exact relation: %v", err) }
}

func Test170SpringUniquePrimaryExact(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; @Service class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/B.java": `package demo; import org.springframework.stereotype.Service; import org.springframework.context.annotation.Primary; @Service @Primary class B implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; class Checkout { PayService service; }`,
	})
	in := springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD")
	rel, err := n.ResolveInjection170(context.Background(), in)
	if err != nil { t.Fatal(err) }
	if rel.Resolution != "EXACT" || len(rel.Targets) != 1 || rel.Targets[0].OwnerFQCN != "demo.B" {
		t.Fatalf("relation=%+v", rel)
	}
}

func Test170SpringTwoPrimaryAmbiguous(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; import org.springframework.context.annotation.Primary; @Service @Primary class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/B.java": `package demo; import org.springframework.stereotype.Service; import org.springframework.context.annotation.Primary; @Service @Primary class B implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; class Checkout { PayService service; }`,
	})
	rel, err := n.ResolveInjection170(context.Background(), springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD"))
	if err != nil { t.Fatal(err) }
	if rel.Resolution != "AMBIGUOUS" || !strings.Contains(rel.Reason, "SPRING_BEAN_AMBIGUOUS") || len(rel.Targets) != 2 {
		t.Fatalf("relation=%+v", rel)
	}
}

func Test170SpringConditionalNeverExact(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; import org.springframework.context.annotation.Profile; @Service @Profile("prod") class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; class Checkout { PayService service; }`,
	})
	rel, err := n.ResolveInjection170(context.Background(), springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD"))
	if err != nil { t.Fatal(err) }
	if rel.Resolution == "EXACT" || !strings.Contains(rel.Reason, "SPRING_CONDITION_UNRESOLVED") {
		t.Fatalf("relation=%+v", rel)
	}
}

func Test170SpringLombokGeneratedConstructorUnsupported(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; @Service class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; import lombok.RequiredArgsConstructor; @RequiredArgsConstructor class Checkout { private final PayService service; }`,
	})
	in := springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "CONSTRUCTOR")
	rel, err := n.ResolveInjection170(context.Background(), in)
	if err != nil { t.Fatal(err) }
	if rel.Resolution != "UNRESOLVED" || !strings.Contains(rel.Reason, "GENERATED_CONSTRUCTOR_UNSUPPORTED") {
		t.Fatalf("relation=%+v", rel)
	}
}

func Test170SpringNonSpringSameNamedAnnotationIgnored(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Service.java": `package demo; public @interface Service { String value() default ""; }`,
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/Fake.java": `package demo; import demo.Service; @Service class Fake implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; class Checkout { PayService service; }`,
	})
	rel, err := n.ResolveInjection170(context.Background(), springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD"))
	if err != nil { t.Fatal(err) }
	if rel.Resolution == "EXACT" { t.Fatalf("non-Spring annotation produced exact binding: %+v", rel) }
	for _, target := range rel.Targets {
		if target.OwnerFQCN == "demo.Fake" { t.Fatalf("fake Spring registration accepted: %+v", rel) }
	}
}

func Test170SpringResourceNameSelectsBean(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; @Service("invoiceRepo") class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/B.java": `package demo; import org.springframework.stereotype.Service; @Service("other") class B implements PayService { public void pay() {} }`,
		"src/main/java/demo/Checkout.java": `package demo; import jakarta.annotation.Resource; class Checkout { @Resource(name="invoiceRepo") PayService service; }`,
	})
	in := springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD")
	in.ResourceName = "invoiceRepo"
	rel, err := n.ResolveInjection170(context.Background(), in)
	if err != nil { t.Fatal(err) }
	if rel.Resolution != "EXACT" || len(rel.Targets) != 1 || rel.Targets[0].OwnerFQCN != "demo.A" {
		t.Fatalf("relation=%+v", rel)
	}
}

func Test170SpringExplicitConstructorAndSetter(t *testing.T) {
	files := map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; import org.springframework.stereotype.Service; @Service class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/CtorCheckout.java": `package demo; class CtorCheckout { private final PayService service; CtorCheckout(PayService service) { this.service = service; } }`,
		"src/main/java/demo/SetterCheckout.java": `package demo; import org.springframework.beans.factory.annotation.Autowired; class SetterCheckout { private PayService service; @Autowired void setService(PayService service) { this.service = service; } }`,
	}
	n := newReviewFixture170(t, files)
	for _, tc := range []Injection170{
		springInjection170("src/main/java/demo/CtorCheckout.java", "demo.CtorCheckout", "demo.PayService", "service", "CONSTRUCTOR"),
		springInjection170("src/main/java/demo/SetterCheckout.java", "demo.SetterCheckout", "demo.PayService", "service", "SETTER"),
	} {
		rel, err := n.ResolveInjection170(context.Background(), tc)
		if err != nil { t.Fatal(err) }
		if rel.Resolution != "EXACT" || len(rel.Targets) != 1 || rel.Targets[0].OwnerFQCN != "demo.A" {
			t.Fatalf("kind=%s relation=%+v", tc.Kind, rel)
		}
	}
}

func Test170SpringBeanMethodRegistration(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/Config.java": `package demo; import org.springframework.context.annotation.Configuration; import org.springframework.context.annotation.Bean; @Configuration class Config { @Bean("special") PayService special() { return new A(); } }`,
		"src/main/java/demo/Checkout.java": `package demo; class Checkout { PayService service; }`,
	})
	in := springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD")
	in.Qualifier = "special"
	rel, err := n.ResolveInjection170(context.Background(), in)
	if err != nil { t.Fatal(err) }
	if rel.Resolution != "EXACT" || len(rel.Targets) != 1 {
		t.Fatalf("relation=%+v", rel)
	}
	got := rel.Targets[0]
	if got.Kind != "METHOD" || got.OwnerFQCN != "demo.Config" || got.Name != "special" {
		t.Fatalf("bean target=%+v", got)
	}
}
