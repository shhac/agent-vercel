// Command mockvercel runs the fixture Vercel API server for manual testing:
//
//	go run ./cmd/mockvercel -addr 127.0.0.1:8765
//	agent-vercel --base-url http://127.0.0.1:8765 auth test
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "address to listen on")
	flag.Parse()

	srv := &http.Server{Addr: *addr, Handler: mockvercel.New()}
	log.Printf("mockvercel listening on http://%s", *addr)
	log.Fatal(srv.ListenAndServe())
}
