package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
)

// 発行する側の権限はGOOGLE_APPLICATION_CREDENTIALSなどで指定
// 発行する側はユーザーでもSAでも良い

func main() {
	godotenv.Load()

	flag.Parse()

	ctx := context.Background()

	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Fatalf("google.DefaultTokenSource: %v", err)
	}

	t, err := ts.Token()
	if err != nil {
		log.Fatalf("ts.Token: %v", err)
	}

	fmt.Println(t.AccessToken)
}
