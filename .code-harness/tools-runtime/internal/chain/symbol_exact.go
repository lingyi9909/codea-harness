package chain

import "strings"

func parentSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	index := strings.LastIndex(symbol, ".")
	if index <= 0 {
		return ""
	}
	return symbol[:index]
}
