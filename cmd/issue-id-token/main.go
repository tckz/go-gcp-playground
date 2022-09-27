package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"google.golang.org/api/idtoken"
)

// https://cloud.google.com/run/docs/authenticating/service-to-service
// 指定audienceでIDトークン発行
// 発行する側の権限はGOOGLE_APPLICATION_CREDENTIALSなどで指定

// 発行する側はSAである必要がある。
// idtoken: credential must be service_account, found "authorized_user"

var (
	optAudience = flag.String("audience", "", "aud of id token to be issued")
)

func main() {
	godotenv.Load()

	flag.Parse()

	if *optAudience == "" {
		log.Fatalf("*** --audience must be specified")
	}

	ts, err := idtoken.NewTokenSource(context.Background(), *optAudience)
	if err != nil {
		log.Fatalf("idtoken.NewTokenSource: %v", err)
	}

	token, err := ts.Token()
	if err != nil {
		log.Fatalf("ts.Token: %v", err)
	}

	fmt.Println(token.AccessToken)
}
