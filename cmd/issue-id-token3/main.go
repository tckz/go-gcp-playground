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
// 発行する側はユーザーに限られる。おそらくrefresh_tokenがADC.jsonに含まれる必要がある
// SAだとアクセストークンは得られるが、IDトークンは得られない
// audienceを指定できない

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

	idToken := t.Extra("id_token")
	fmt.Println(idToken)
}
