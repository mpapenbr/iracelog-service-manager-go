package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/certs/traefik"
)

type certs struct {
	ctx       context.Context
	tlsconfig *tls.Config
	log       *log.Logger
	cert      *tls.Certificate
	mu        sync.RWMutex
}

func NewTlsConfigProvider(ctx context.Context) *tls.Config {
	c := &certs{
		ctx: ctx,
		log: log.GetFromContext(ctx).Named("grpc.certs"),
	}
	c.loadCert()
	//nolint:nestif //false positive
	if c.cert != nil {
		c.tlsconfig = &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				c.mu.RLock()
				defer c.mu.RUnlock()
				return c.cert, nil
			},
			MinVersion: tls.VersionTLS13,
		}
		if config.TLSCAFile != "" {
			c.log.Info("Loading ca cert",
				log.String("file", config.TLSCAFile))

			caCert, err := os.ReadFile(config.TLSCAFile)
			if err != nil {
				c.log.Error("could not read TLS root CA", log.ErrorField(err))
			}
			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
				c.log.Error("could not append cert to pool")
			}
			c.tlsconfig.ClientCAs = caCertPool
			c.tlsconfig.ClientAuth = tls.VerifyClientCertIfGiven
		}
		go c.watchAndReloadCerts()
	}
	return c.tlsconfig
}

//nolint:funlen,gocognit,cyclop,gocyclo // by design
func (c *certs) watchAndReloadCerts() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.log.Error("could not create fsnotify watcher", log.ErrorField(err))
		return
	}
	defer watcher.Close()
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				c.log.Info("context done, stopping cert reload")
				return
			case event, ok := <-watcher.Events:
				if !ok {
					c.log.Info("watcher events channel closed, stopping cert reload")
					return
				}
				c.log.Debug("change detected",
					log.String("file", event.Name), log.Any("event", event))
				if event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Chmod == fsnotify.Chmod {

					c.log.Info("cert file changed, reloading cert",
						log.String("file", event.Name))
					c.loadCert()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					c.log.Info("watcher events channel closed, stopping cert reload")
					return
				}
				c.log.Error("watcher error", log.ErrorField(err))
			}
		}
	}()
	if config.TLSCertFile != "" {
		if err := watcher.Add(config.TLSCertFile); err != nil {
			c.log.Error("could not watch cert file", log.ErrorField(err))
		}
	}
	if config.TLSKeyFile != "" {
		if err := watcher.Add(config.TLSKeyFile); err != nil {
			c.log.Error("could not watch key file", log.ErrorField(err))
		}
	}
	if config.TraefikCerts != "" {
		if err := watcher.Add(config.TraefikCerts); err != nil {
			c.log.Error("could not watch traefik certs file", log.ErrorField(err))
		}
	}
	<-done
}

func (c *certs) loadCert() {
	if config.TraefikCerts != "" && config.TraefikCertDomain != "" {
		c.log.Info("Looking up traefik certs",
			log.String("file", config.TraefikCerts),
			log.String("domain", config.TraefikCertDomain))
		cert, err := traefik.GetCertFromTraefik(
			config.TraefikCerts,
			config.TraefikCertDomain)
		if err != nil {
			c.log.Error("could not load traefik certs", log.ErrorField(err))
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.cert = &cert
		return
	}
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		c.log.Info("Loading cert",
			log.String("key", config.TLSKeyFile),
			log.String("cert", config.TLSCertFile))
		cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			c.log.Error("could not load TLS key pair", log.ErrorField(err))
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.cert = &cert
		return
	}
}
