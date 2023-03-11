package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func WaitForTCP(addr string, timeout time.Duration) error {
	timeoutReached := time.Now().Add(timeout)
	start := time.Now()
	log.Debug("wait for tcp connection",
		log.String("addr", addr),
		log.String("timeout", timeout.String()))
	for time.Now().Before(timeoutReached) {
		conn, err := net.Dial("tcp", addr)
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

func WaitForHttpResponse(url string, timeout time.Duration) error {
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
