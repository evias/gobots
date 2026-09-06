package robot

import (
	"strconv"
	"strings"
)

// XXX
func hostWithPort(host string, port int) string {
	if !strings.Contains(host, ":"+strconv.Itoa(port)) {
		host += ":" + strconv.Itoa(port)
	}

	return host
}
