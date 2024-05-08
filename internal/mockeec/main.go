package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	port := flag.String("p", "8000", "port to listen on")
	dir := flag.String("d", ".", "directory to serve files from")
	flag.Parse()

	http.Handle("/", http.FileServer(http.Dir(*dir)))

	log.Printf("Serving from %s over HTTP on port: %s\n", *dir, *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
