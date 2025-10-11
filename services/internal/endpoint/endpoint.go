package endpoint

import "fmt"

func Normalize(addr string) string {
	if addr == "" {
		return ":0"
	}

	if addr[0] == ':' {
		return addr
	}

	return fmt.Sprintf(":%s", addr)
}
