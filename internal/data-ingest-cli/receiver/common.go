package receiver

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
)

type Receiver interface {
	Start() error
	Stop()
}

func WriteToFile(dest string, raw []byte) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, raw, "", "\t"); err != nil {
		log.Printf("Could not format JSON string: %v\n", err)
		if err := os.WriteFile(dest, raw, os.ModePerm); err != nil {
			log.Printf("Could not write received raw data to file: %v\n", err)
		}
	}
	if err := os.WriteFile(dest, prettyJSON.Bytes(), os.ModePerm); err != nil {
		log.Printf("Could not write received data to file: %v\n", err)
	}
	log.Printf("Stored received data in %s\n", dest)
}
