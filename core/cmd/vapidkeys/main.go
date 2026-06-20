package main

import (
	"fmt"
	"os"

	wp "github.com/SherClockHolmes/webpush-go"
)

// Generates a VAPID keypair for Web Push and prints it in .env form. The public
// key is served to browsers; keep the private key secret (out of git).
//
//	go run ./cmd/vapidkeys
func main() {
	priv, pub, err := wp.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate vapid keys:", err)
		os.Exit(1)
	}

	fmt.Printf("VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", pub, priv)
}
