package main

import "os"

func init() {
	_ = os.Setenv("GIT_CONFIG_COUNT", "1")
	_ = os.Setenv("GIT_CONFIG_KEY_0", "core.autocrlf")
	_ = os.Setenv("GIT_CONFIG_VALUE_0", "false")
}
