package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/joho/godotenv"
	"github.com/tckz/go-gcp-playground/internal/log"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optWorkers      = flag.Uint64("workers", 8, "Number of workers")
	optLogLevel     = flag.String("log-level", "info", "info|warn|error")
	optSubscription = flag.String("subscription", "", "subscription name")
	optOutPrefix    = flag.String("out-prefix", "out/out-", "path/to/prefix")
)

func init() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel(*optLogLevel))).Sugar().With(zap.String("app", myName))
}

func main() {
	logger.Infof("ver=%s, args=%s", version, os.Args)
	defer logger.Infof("done")

	if *optSubscription == "" {
		logger.Fatalf("*** --subscription must be specified.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pjID := os.Getenv("PROJECT_ID")

	cl, err := pubsub.NewClient(ctx, pjID)
	if err != nil {
		logger.Fatalf("*** pubsub.NewClient: %v", err)
	}
	defer cl.Close()

	eg, ctx := errgroup.WithContext(ctx)
	for i := uint64(0); i < *optWorkers; i++ {
		index := i
		eg.Go(func() error {
			fn := filepath.Join(*optOutPrefix + fmt.Sprintf("%03d", index))
			os.MkdirAll(filepath.Dir(fn), os.ModePerm)
			fp, err := os.Create(fn)
			if err != nil {
				return err
			}
			defer fp.Close()
			logger.Infof("out=%s", fn)

			subs := cl.Subscription(*optSubscription)
			enc := json.NewEncoder(fp)
			return subs.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				msg.Ack()
				m := map[string]interface{}{
					"id":   msg.ID,
					"data": string(msg.Data),
					"attr": msg.Attributes,
				}
				enc.Encode(m)
			})
		})
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)

	s := <-sig
	logger.Infof("Received signal: %v", s)
	cancel()

	logger.Infof("Waiting goroutines exit")
	if err := eg.Wait(); err != nil {
		logger.Errorf("Wait: %v", err)
	}

}
