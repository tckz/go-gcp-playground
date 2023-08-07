package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/joho/godotenv"
	"github.com/tckz/go-gcp-playground/internal/log"
	"go.uber.org/zap"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optLogLevel  = flag.String("log-level", "info", "info|warn|error")
	optNameSpace = flag.String("ns", "", "namespace")
	optName      = flag.String("name", "", "named key")
	optKind      = flag.String("kind", "mykind", "namespace")
)

func init() {
	godotenv.Load()

	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel(*optLogLevel))).Sugar().With(zap.String("app", myName))
}

type MyKind struct {
	Name string
	Time time.Time
}

func main() {
	logger.Infof("ver=%s, args=%s", version, os.Args)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pjID := os.Getenv("PROJECT_ID")

	cl, err := datastore.NewClient(context.Background(), pjID)
	if err != nil {
		logger.Fatalf("*** datastore.NewClient: %v", err)
	}
	defer cl.Close()

	key := datastore.NameKey(*optKind, *optName, nil)
	key.Namespace = *optNameSpace

	var rec MyKind
	if err := cl.Get(ctx, key, &rec); err != nil {
		logger.Errorf("Get: %v", err)
		return
	}

	fmt.Fprintf(os.Stdout, "%+v\n", rec)
}
