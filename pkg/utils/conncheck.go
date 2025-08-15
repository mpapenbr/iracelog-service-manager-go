package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func WaitForTCP(addr string, timeout time.Duration) error {
	timeoutReached := time.Now().Add(timeout)
	start := time.Now()
	log.Debug("wait for tcp connection",
		log.String("addr", addr),
		log.String("timeout", timeout.String()))
	var d net.Dialer
	for time.Now().Before(timeoutReached) {
		conn, err := d.DialContext(context.Background(), "tcp", addr)
		if err == nil {
			conn.Close()

			log.Debug("tcp connection successful",
				log.String("addr", addr),
				log.String("duration", time.Since(start).String()))
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("%s could not be reached after %v", addr, timeout)
}

func WaitForHTTPResponse(url string, timeout time.Duration) error {
	timeoutReached := time.Now().Add(timeout)
	start := time.Now()
	log.Debug("wait for http request",
		log.String("url", url),
		log.String("timeout", timeout.String()))
	cli := &http.Client{}
	for time.Now().Before(timeoutReached) {
		req, _ := http.NewRequestWithContext(
			context.Background(), http.MethodGet, url, http.NoBody)
		_, err := cli.Do(req)
		if err == nil {
			log.Debug("http request successful",
				log.String("url", url),
				log.String("duration", time.Since(start).String()))
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("%s could not be reached after %v", url, timeout)
}

func ExtractFromWebsocketURL(url string) (addr, proto string) {
	param := resolveRegex(
		"^(?P<proto>ws|wss)://(?P<addr>(?P<host>.*?)(:(?P<port>\\d+))?)/.*", url)
	if len(param) == 0 {
		return "", ""
	}
	if port, ok := param["port"]; ok && port != "" {
		// if port is found, the addr contains our wanted value
		return param["addr"], param["proto"]
	} else if proto := param["proto"]; proto == "wss" {
		return fmt.Sprintf("%s:443", param["addr"]), proto
	} else {
		return fmt.Sprintf("%s:80", param["addr"]), proto
	}
}

func ExtractFromDBURL(url string) string {
	param := resolveRegex(
		"^postgresql://(.*@)(?P<addr>(?P<host>.*?)(:(?P<port>\\d+))?)/.*", url)
	if len(param) == 0 {
		return ""
	}
	if port, ok := param["port"]; ok && port != "" {
		return param["addr"] // if port is found, the addr contains our wanted value
	} else {
		return fmt.Sprintf("%s:5432", param["addr"])
	}
}

func resolveRegex(regEx, url string) (paramsMap map[string]string) {
	compRegEx := regexp.MustCompile(regEx)
	match := compRegEx.FindStringSubmatch(url)

	paramsMap = make(map[string]string)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	return paramsMap
}
