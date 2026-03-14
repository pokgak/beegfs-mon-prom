package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pokgak/beegfs-mon-prom/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenAddr := flag.String("listen", ":9100", "address to listen on")
	beegfsPath := flag.String("beegfs-path", "/opt/beegfs/sbin/beegfs", "path to beegfs CLI binary")
	mgmtdAddr := flag.String("mgmtd-addr", "", "management daemon gRPC address (e.g. 10.0.0.1:8010)")
	authFile := flag.String("auth-file", "", "path to BeeGFS auth file")
	tlsDisable := flag.Bool("tls-disable", false, "disable TLS for gRPC communication")
	tlsCertFile := flag.String("tls-cert-file", "", "path to TLS certificate file")
	interval := flag.Duration("interval", 30*time.Second, "collection interval for cached metrics")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := collector.Config{
		BeegfsPath:  *beegfsPath,
		MgmtdAddr:   *mgmtdAddr,
		AuthFile:     *authFile,
		TLSDisable:   *tlsDisable,
		TLSCertFile:  *tlsCertFile,
		Interval:     *interval,
	}

	c := collector.New(cfg)
	prometheus.MustRegister(c)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><a href="/metrics">Metrics</a></body></html>`))
	})

	slog.Info("starting beegfs-mon-prom", "listen", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
