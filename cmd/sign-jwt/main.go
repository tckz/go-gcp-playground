package main

// APIGWのサービス間認証向けJWT
// https://cloud.google.com/api-gateway/docs/authenticate-service-account?hl=ja

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

var (
	optSA       = flag.String("sa", "", "The sub field of the ID token that is to be issued")
	optAudience = flag.String("aud", "", "The aud field of the ID token that is to be issued")
	optDuration = flag.Duration("duration", 1*time.Hour, "TTL of the ID token")
	optPayload  = flag.String("payload", "", "plain payload of jwt")
)

func main() {
	godotenv.Load()

	flag.Parse()

	if *optSA == "" {
		log.Fatalf("*** --sa must be specified")
	}

	ctx := context.Background()

	// https://cloud.google.com/iam/docs/reference/credentials/rest/v1/projects.serviceAccounts/generateIdToken
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Fatalf("google.DefaultTokenSource: %v", err)
	}

	c, err := credentials.NewIamCredentialsClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		log.Fatalf("credentials.NewIamCredentialsClient: %v", err)
	}
	defer c.Close()

	payload := *optPayload
	if payload == "" {
		now := time.Now()
		m := map[string]interface{}{
			"iat":   now.Unix(),
			"exp":   now.Add(*optDuration).Unix(),
			"iss":   *optSA,
			"sub":   *optSA,
			"email": *optSA,
		}
		if *optAudience != "" {
			m["aud"] = *optAudience
		}

		b, err := json.Marshal(m)
		if err != nil {
			log.Fatalf("json.Marshal: %v", err)
		}
		payload = string(b)
	}

	resp, err := c.SignJwt(ctx, &credentialspb.SignJwtRequest{
		Payload: payload,
		Name:    "projects/-/serviceAccounts/" + *optSA,
	})
	if err != nil {
		log.Fatalf("SignJwt: %v", err)
	}

	fmt.Println(resp.GetSignedJwt())
}
