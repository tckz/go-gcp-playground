package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/tckz/go-gcp-playground/internal/log"
	vh "github.com/tckz/vegetahelper"
	vegeta "github.com/tsenart/vegeta/v12/lib"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optRate = &vh.RateFlag{
		Rate: &vegeta.Rate{
			Freq: 30,
			Per:  1 * time.Second,
		}}
	optDuration = flag.Duration("duration", 10*time.Second, "Duration of the test [0 = forever]")
	optOutput   = flag.String("output", "", "/path/to/results.bin or 'stdout'")
	optWorkers  = flag.Uint64("workers", vegeta.DefaultWorkers, "Number of workers")
	optLogLevel = flag.String("log-level", "info", "info|warn|error")
	optTopic    = flag.String("topic", "", "topic name")
)

func init() {
	godotenv.Load()

	flag.Var(optRate, "rate", "Number of requests per time unit")
	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel(*optLogLevel))).Sugar().With(zap.String("app", myName))
}

type nopWriteCloser struct {
	io.Writer
}

func (c nopWriteCloser) Close() error {
	return nil
}

func openResultFile(out string) (io.WriteCloser, error) {
	switch out {
	case "stdout":
		return &nopWriteCloser{os.Stdout}, nil
	default:
		return os.Create(out)
	}
}

type MyKind struct {
	Name string
}

func main() {
	logger.Infof("ver=%s, args=%s", version, os.Args)
	defer logger.Infof("done")

	if *optOutput == "" {
		logger.Fatalf("*** --output must be specified.")
	}

	if *optTopic == "" {
		logger.Fatalf("*** --topic must be specified.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pjID := os.Getenv("PROJECT_ID")

	cl, err := pubsub.NewClient(ctx, pjID)
	if err != nil {
		logger.Fatalf("*** pubsub.NewClient: %v", err)
	}
	defer cl.Close()

	topic := cl.Topic(*optTopic)
	topic.PublishSettings.NumGoroutines = 30

	chRes := make(chan *pubsub.PublishResult, 30)
	eg, ctx := errgroup.WithContext(ctx)
	var gotID int64
	for i := 0; i < 30; i++ {
		eg.Go(func() error {
			for {
				select {
				case res, ok := <-chRes:
					if !ok {
						return nil
					}

					if _, err := res.Get(ctx); err != nil {
						logger.Errorf("*** Get: %v", err)
						return err
					}
					atomic.AddInt64(&gotID, 1)
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	atk := vh.NewAttacker(func(ctx context.Context) (result *vh.HitResult, retErr error) {
		msg := &pubsub.Message{
			Data: []byte(fmt.Sprintf("hello: %s", uuid.New().String())),
		}
		res := topic.Publish(ctx, msg)
		chRes <- res

		return result, nil
	}, vh.WithWorkers(*optWorkers))
	res := atk.Attack(ctx, *optRate.Rate, *optDuration, "publish-random")

	out, err := openResultFile(*optOutput)
	if err != nil {
		logger.Fatal(err)
	}
	defer out.Close()
	enc := vegeta.NewEncoder(out)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)

loop:
	for {
		select {
		case s := <-sig:
			logger.Infof("Received signal: %s", s)
			cancel()
			// keep loop until 'res' is closed.
		case r, ok := <-res:
			if !ok {
				break loop
			}
			if err := enc.Encode(r); err != nil {
				logger.Errorf("*** Encode: %v", err)
				break loop
			}
		}
	}

	close(chRes)
	logger.Infof("waiting goroutines for res.Get exit")
	if err := eg.Wait(); err != nil {
		logger.Errorf("Wait: %v", err)
	}
	logger.Infof("gotID=%d", gotID)

	cancel()
}
