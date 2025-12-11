package main

import (
	"context"
	"log"

	apiapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/app/api"
)

func main() {
	if err := apiapp.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
