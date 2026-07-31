package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.String("p", "8000", "port to listen on")
	dir := flag.String("d", ".", "directory to serve files from")
	flag.Parse()

	http.Handle("/", handler{
		fileserverHandler: http.FileServer(http.Dir(*dir)),
	})

	log.Printf("Serving from %s over HTTP on port: %s\n", *dir, *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

type handler struct {
	fileserverHandler http.Handler
}

var _ http.Handler = handler{}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Got request with headers:")
	for name, values := range r.Header {
		for _, value := range values {
			fmt.Println(name, value)
		}
	}
	fmt.Println()

	h.fileserverHandler.ServeHTTP(w, r)
}
