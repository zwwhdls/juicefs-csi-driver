package apps

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook"
	"k8s.io/klog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	Port     int    // webhook server port
	CertFile string // path to the x509 certificate for https
	KeyFile  string // path to the x509 private key matching `CertFile`
)

func init() {
	// get command line parameters
	flag.IntVar(&Port, "port", 8443, "Webhook server port.")
	flag.StringVar(&CertFile, "tlsCertFile", "/server.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&KeyFile, "tlsKeyFile", "/serverkey.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&webhook.SidecarImage, "sidecarImage", "", "")
	flag.StringVar(&webhook.SidecarCpuLimit, "sidecarCpuLimit", "", "")
	flag.StringVar(&webhook.SidecarMemLimit, "sidecarMemLimit", "", "")
}

func SideCarRun() {
	pair, err := tls.LoadX509KeyPair(CertFile, KeyFile)
	if err != nil {
		klog.Errorf("Failed to load key pair: %v", err)
	}

	whServer := &webhook.Server{
		Server: &http.Server{
			Addr:      fmt.Sprintf(":%v", Port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", whServer.Serve)
	whServer.Server.Handler = mux

	// start webhook server in new rountine
	go func() {
		if err := whServer.Server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	// listening OS shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	whServer.Server.Shutdown(context.Background())
}
