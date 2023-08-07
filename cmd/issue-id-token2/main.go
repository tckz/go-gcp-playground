package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// https://cloud.google.com/run/docs/authenticating/service-to-service
// 指定audienceでIDトークン発行
// 指定SAへのimpersonationを行う
// 発行する側の権限はGOOGLE_APPLICATION_CREDENTIALSなどで指定。この権限には、指定SAへのimpersonate権限＝serviceAccountTokenCreatorが必要
// 発行する側はSAでもユーザーでもよい

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
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Fatalf("google.DefaultTokenSource: %v", err)
	}

	c, err := credentials.NewIamCredentialsClient(ctx, option.WithTokenSource(ts))
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
