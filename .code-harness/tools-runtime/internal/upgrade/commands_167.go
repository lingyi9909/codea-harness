package upgrade

// Canonical Host command templates are framework resources; the installed
// .opencode command is covered separately by the Host ownership transaction.
func init() {
	managedDirs = append(managedDirs, "commands")
}
