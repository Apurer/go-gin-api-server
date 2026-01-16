package main

import (
	"context"
	"log"

	apiapp "github.com/Apurer/go-gin-api-server/internal/app/api"
)

func main() {
	if err := apiapp.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
