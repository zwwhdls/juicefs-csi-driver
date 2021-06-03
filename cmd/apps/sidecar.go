package apps

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook"
	"k8s.io/klog"
	"net/http"
)

var (
	Port     int    // webhook server port
	CertFile string // path to the x509 certificate for https
	KeyFile  string // path to the x509 private key matching `CertFile`
)

func init() {
	// get command line parameters
	flag.IntVar(&Port, "port", 8443, "Webhook server port.")
	flag.StringVar(&CertFile, "tlsCertFile", "", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&KeyFile, "tlsKeyFile", "", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&webhook.SidecarImage, "sidecar-image", "", "JuiceFS daemon sidecar image.")
	flag.StringVar(&webhook.SidecarCpuLimit, "sidecar-cpu-limit", "", "JuiceFS daemon sidecar cpu limit.")
	flag.StringVar(&webhook.SidecarMemLimit, "sidecar-mem-limit", "", "JuiceFS daemon sidecar mem limit.")
	flag.BoolVar(&juicefs.EnableSidecar, "enable", false, "Enable JuiceFS sidecar or not.")
}

func JfsdSidecarWebhookRun(ctx context.Context) {
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
		if err := whServer.Server.ListenAndServeTLS(CertFile, KeyFile); err != nil {
			klog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		klog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
		whServer.Server.Shutdown(context.Background())
	}
}
