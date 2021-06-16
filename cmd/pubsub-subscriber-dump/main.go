package main

import (
	"context"
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
	vegeta "github.com/tsenart/vegeta/lib"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optWorkers      = flag.Uint64("workers", vegeta.DefaultWorkers, "Number of workers")
	optLogLevel     = flag.String("log-level", "info", "info|warn|error")
	optSubscription = flag.String("subscription", "", "subscription name")
)

func init() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	// Until log initialization complete, use default json logger instead of it.
	zl, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	logger = zl.Sugar().With(zap.String("app", myName))

	encConfig := zap.NewProductionEncoderConfig()
	encConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var al zap.AtomicLevel
	err = al.UnmarshalText([]byte(*optLogLevel))
	if err != nil {
		logger.With(zap.Error(err)).Fatalf("al.UnmarshalText: %s", *optLogLevel)
	}

	zc := zap.Config{
		DisableCaller:     true,
		DisableStacktrace: true,
		Level:             al,
		Development:       false,
		Encoding:          "json",
		EncoderConfig:     encConfig,
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	zl, err = zc.Build()
	if err != nil {
		logger.Fatalf("*** Failed to Build: %v", err)
	}

	logger = zl.Sugar().With(zap.String("app", myName))

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
		eg.Go(func() error {
			subs := cl.Subscription(*optSubscription)
			return subs.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				fmt.Fprintf(os.Stdout, "ID=%s, Data=%s\n", msg.ID, string(msg.Data))
				msg.Ack()
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
