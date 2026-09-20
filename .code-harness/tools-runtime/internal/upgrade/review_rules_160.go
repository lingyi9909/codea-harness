package upgrade

// review-rules/** is Framework State introduced by Codea Harness 1.6.
// Register it with the existing managed-path machinery so install/upgrade
// replaces the catalog while Project State remains byte-preserved.
func init() {
	managedDirs = append(managedDirs, "review-rules")
}
