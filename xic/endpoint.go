package xic

import (
	"fmt"
	"math"
	"strings"
	"strconv"

	"mangos/xstr"
)


type EndpointInfo struct {
	proto string
	host string
	port int
	timeout int
	closeTimeout int
	connectTimeout int
}

// this endpoint has no '@' prefix
func parseEndpoint(endpoint string) (*EndpointInfo, error) {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "@") {
		endpoint = endpoint[1:]
	}
	ei := &EndpointInfo{}
	tk := xstr.NewTokenizerSpace(endpoint)
	netSp := xstr.NewSplitter(tk.Next(), "+")

	ei.proto = netSp.Next()
	ei.host = netSp.Next()
	port, err := strconv.Atoi(netSp.Next())
	if err != nil || port <= 0 || port > math.MaxUint16 {
		return nil, fmt.Errorf("Invalid port in endpoint \"%s\"", endpoint)

	}
	ei.port = port

	if netSp.HasMore() || netSp.Count() != 3 {
		return nil, fmt.Errorf("Invalid format. endpoint=%s", endpoint)
	}

	if ei.proto == "" || strings.EqualFold(ei.proto, "tcp") {
		ei.proto = "tcp"
	} else {
		return nil, fmt.Errorf("Unsupported transport protocol \"%s\"", ei.proto)
	}

	for tk.HasMore() {
		key, value := xstr.SplitKeyValue(tk.Next(), "=")
		if value == "" {
			continue
		}

		if key == "timeout" {
			sp := xstr.NewSplitter(value, ",")
			ei.timeout, _ = strconv.Atoi(sp.Next())

			ei.closeTimeout, _ = strconv.Atoi(sp.Next())
			if ei.closeTimeout < 0 {
				ei.closeTimeout = 0
			}

			ei.connectTimeout, _ = strconv.Atoi(sp.Next())
			if ei.connectTimeout < 0 {
				ei.connectTimeout = 0
			}
		}
	}
	// TODO
	return ei, nil
}

func (ei *EndpointInfo) Proto() string {
	return ei.proto
}

func (ei *EndpointInfo) Address() string {
	var address string
	if strings.IndexByte(ei.host, ':') >= 0 {
		address = fmt.Sprintf("[%s]:%d", ei.host, ei.port)
	} else {
		address = fmt.Sprintf("%s:%d", ei.host, ei.port)
	}
	return address
}

func (ei *EndpointInfo) String() string {
	ep := fmt.Sprintf("@%s+%s+%d", ei.proto, ei.host, ei.port)
	if ei.timeout > 0 || ei.closeTimeout > 0 || ei.connectTimeout > 0 {
		ep += fmt.Sprintf(" timeout=%d,%d,%d", ei.timeout, ei.closeTimeout, ei.connectTimeout)
	}
	return ep
}

