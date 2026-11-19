package gymserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"code.pecheny.me/pecheny/dopesuite/dopecore/buildinfo"
	"code.pecheny.me/pecheny/dopesuite/dopecore/session"
	"code.pecheny.me/pecheny/dopesuite/dopecore/webassets"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 120 * time.Second
	defaultPort       = "9689"
)

// gym's deployed environment names its own production switch.
func init() { session.ProdEnvVar = "GYM_ENV" }

// Main is the server entry point, invoked by cmd/gym-server.
func Main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println(buildinfo.Version())
		default:
			log.Fatalf("unknown command %q (version)", os.Args[1])
		}
		return
	}

	srv, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	srv.startBot(ctx)
	// The first read happens here rather than on the first page view, so a
	// broken sheet URL shows in the log at boot.
	if _, err := srv.sheet.Sessions(ctx); err != nil {
		log.Printf("sheet: first read: %v", err)
	}

	port := strings.TrimPrefix(os.Getenv("PORT"), ":")
	if port == "" {
		port = defaultPort
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		log.Fatalf("bind %s: %v", port, err)
	}
	log.Printf("gym %s serving on %s (assets from %s)", buildinfo.Version(), listener.Addr(), srv.assets.Mode)
	httpSrv := &http.Server{
		Handler:           webassets.Gzip(routes(srv)),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
	log.Fatal(httpSrv.Serve(listener))
}
