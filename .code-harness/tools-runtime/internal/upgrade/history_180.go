package upgrade

// 1.8 isolates pre-1.8 ordinary Review instructions under history/. These are
// Framework documentation, not Project State, so they participate in the same
// manifest-owned delta/rollback transaction as commands and templates.
func init() {
	managedDirs = append(managedDirs, "history")
}
