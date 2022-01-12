package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"webhook-controller-demo/server"

	"github.com/golang/glog"
)

const (
	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `tls.crt`
	tlsKeyFile  = `tls.key`
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			glog.Error(fmt.Sprint(err, "\n", string(debug.Stack())))
		}
	}()
	glog.Info("admission webhook controller")

	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	mux := http.NewServeMux()
	mux.Handle("/mutatedns", server.AdmitFuncHandle(server.MutateDnsConfig))
	server := &http.Server{
		Addr:    ":443",
		Handler: mux,
	}
	// whsvr.Server.Handler = mux
	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}
