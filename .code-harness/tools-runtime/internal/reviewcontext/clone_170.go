package reviewcontext

import "codea-harness-tools/internal/nav"

func cloneRelation170(in nav.Relation170) nav.Relation170 {
	out := in
	out.From = cloneRef170(in.From)
	out.Targets = make([]nav.ReviewRef170, len(in.Targets))
	for i := range in.Targets {
		out.Targets[i] = cloneRef170(in.Targets[i])
	}
	out.Evidence = make([]nav.SourceRange170, len(in.Evidence))
	for i := range in.Evidence {
		out.Evidence[i] = in.Evidence[i]
		out.Evidence[i].Ref = cloneRef170(in.Evidence[i].Ref)
	}
	out.Assumptions = append([]string(nil), in.Assumptions...)
	return out
}

func cloneRef170(in nav.ReviewRef170) nav.ReviewRef170 {
	out := in
	out.ParameterTypes = append([]string(nil), in.ParameterTypes...)
	return out
}
