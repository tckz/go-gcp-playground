package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/joho/godotenv"
	"github.com/tckz/go-gcp-playground/internal/log"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optLogLevel  = flag.String("log-level", "info", "info|warn|error")
	optNameSpace = flag.String("ns", "", "namespace")
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

	q := datastore.NewQuery(*optKind).Namespace(*optNameSpace)
	it := cl.Run(ctx, q)
	for {
		var rec MyKind
		key, err := it.Next(&rec)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logger.Errorf("Next: %v", err)
			return
		}
		fmt.Printf("Key=%v, MyKind=%+v\n", key, rec)
	}

	logger.Info("done")
}
