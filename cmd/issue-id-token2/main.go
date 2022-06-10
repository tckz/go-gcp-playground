package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

// https://cloud.google.com/run/docs/authenticating/service-to-service
// 指定audienceでIDトークン発行
// 発行する権限はGOOGLE_APPLICATION_CREDENTIALSなどで指定

var (
	optSA       = flag.String("sa", "", "sub of id token to be issued")
	optAudience = flag.String("audience", "", "aud of id token to be issued")
)

func main() {
	godotenv.Load()

	flag.Parse()

	if *optSA == "" {
		log.Fatalf("*** --sa must be specified")
	}

	if *optAudience == "" {
		log.Fatalf("*** --audience must be specified")
	}

	ctx := context.Background()

	// https://cloud.google.com/iam/docs/reference/credentials/rest/v1/projects.serviceAccounts/generateIdToken
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Fatalf("google.FindDefaultCredentials: %v", err)
	}
	fmt.Println(string(creds.JSON))

	c, err := credentials.NewIamCredentialsClient(ctx, option.WithCredentials(creds))
	if err != nil {
		log.Fatalf("credentials.NewIamCredentialsClient: %v", err)
	}
	defer c.Close()

	t, err := c.GenerateIdToken(ctx, &credentialspb.GenerateIdTokenRequest{
		Name:         "projects/-/serviceAccounts/" + *optSA,
		Audience:     *optAudience,
		IncludeEmail: true,
	})
	if err != nil {
		log.Fatalf("GenerateIdToken: %v", err)
	}

	fmt.Println(t.Token)
}
