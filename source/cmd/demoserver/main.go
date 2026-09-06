// demoserver runs a small shared practice area for goThoom's Free Demo login.
package main

import (
	"context"
	"flag"
	"gothoom/internal/demoserver"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:5010", "TCP and UDP listen address; use :5010 for LAN access")
	slots := flag.Int("slots", 8, "number of demo characters (1-16)")
	flag.Parse()
	server, err := demoserver.Listen(*addr, *slots)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("goThoom demo server on %s (TCP + UDP), %d slots; select Free Demo in the client", server.Addr(), *slots)
	if err := server.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
