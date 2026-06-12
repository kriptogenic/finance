package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"finance/internal/app"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		//nolint:forbidigo // logger not configured yet
		fmt.Println("Warning: no .env file found or unable to load it")
	}

	fx.New(app.CreateApp()).Run()
}
