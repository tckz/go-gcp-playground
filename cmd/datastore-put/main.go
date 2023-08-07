package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
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
	optLogLevel          = flag.String("log-level", "info", "info|warn|error")
	optAncestorNameSpace = flag.String("ancestor-ns", "", "ancestor namespace")
	optAncestorKind      = flag.String("ancestor-kind", "", "ancestor kind")
	optAncestorName      = flag.String("ancestor-name", "", "ancestor named key")
	optNameSpace         = flag.String("ns", "", "namespace")
	optName              = flag.String("name", "", "named key")
	optKind              = flag.String("kind", "mykind", "namespace")
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

	var parentKey *datastore.Key
	if *optAncestorKind != "" {
		parentKey = datastore.NameKey(*optAncestorKind, *optAncestorName, nil)
		parentKey.Namespace = *optAncestorNameSpace
	}

	name := *optName
	if name == "" {
		name = uuid.New().String()
	}
	key := datastore.NameKey(*optKind, name, parentKey)
	key.Namespace = *optNameSpace

	rec := MyKind{
		Name: "my name is " + name,
		Time: time.Now().UTC(),
	}
	if _, err := cl.Put(ctx, key, &rec); err != nil {
		logger.Errorf("Put: %v", err)
	}
}
