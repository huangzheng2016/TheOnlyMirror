package main

import (
	"TheOnlyMirror/config"
	"log"
	"net/http"
)

func main() {
	if config.Load() != nil {
		log.Fatal("load config error")
		return
	}
	http.HandleFunc("/", handler)
	if config.GetTls() == true {
		log.Println("Starting proxy server with tls on :" + config.GetPort())
		crt, key := config.GetCert()
		if err := http.ListenAndServeTLS(":"+config.GetPort(), crt, key, nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	} else {
		log.Println("Starting proxy server on :" + config.GetPort())
		if err := http.ListenAndServe(":"+config.GetPort(), nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}

}
