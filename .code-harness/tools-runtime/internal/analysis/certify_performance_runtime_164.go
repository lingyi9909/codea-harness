package analysis

import "codea-harness-tools/internal/changeset"

type certificationPerformanceRuntime164 interface {
	InventoryWithPerformance(root, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, entrypointExecutionMetrics164, error)
}

func (defaultCertificationRuntime153) InventoryWithPerformance(root, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, entrypointExecutionMetrics164, error) {
	return buildEntrypointInventoryWithMetrics164(root, runID, snapshot, intent)
}

func (r *certifyPerformanceRecorder164) applyEntrypointMetrics(metrics entrypointExecutionMetrics164) {
	if r == nil {
		return
	}
	r.doc.Counts.ProductionJavaCurrent = metrics.ProductionJavaCurrent
	r.doc.Counts.ProductionJavaBase = metrics.ProductionJavaBase
	r.doc.Counts.ControllerFilesCurrent = metrics.ControllerFilesCurrent
	r.doc.Counts.ControllerFilesBase = metrics.ControllerFilesBase
	r.doc.Processes.CurrentTypeAst = metrics.CurrentTypeAstProcessCount
	r.doc.Processes.CurrentMethodAst = metrics.CurrentMethodAstProcessCount
	r.doc.Processes.BaseTypeAst = metrics.BaseTypeAstProcessCount
	r.doc.Processes.BaseMethodAst = metrics.BaseMethodAstProcessCount
	r.doc.Processes.BaseCatFile = metrics.BaseGitBatchProcessCount
	r.doc.TimingMS.CurrentTypeAst = metrics.CurrentTypeAstDuration.Milliseconds()
	r.doc.TimingMS.CurrentMethodAst = metrics.CurrentMethodAstDuration.Milliseconds()
	r.doc.TimingMS.BaseSourceLoad = metrics.BaseSourceLoadDuration.Milliseconds()
	r.doc.TimingMS.BaseTypeAst = metrics.BaseTypeAstDuration.Milliseconds()
	r.doc.TimingMS.BaseMethodAst = metrics.BaseMethodAstDuration.Milliseconds()
}
