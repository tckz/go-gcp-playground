package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
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
	optWorkers      = flag.Uint("workers", 8, "Number of workers")
	optLogLevel     = flag.String("log-level", "info", "info|warn|error")
	optSubscription = flag.String("subscription", "", "subscription name")

	optLogStep = flag.Int64("log-step", 1000, "")

	// これを指定した場合得られたオブジェクトをdumpしない。EncodeもしないのでCPUパワーをセーブできる
	optOutDiscard = flag.Bool("out-discard", false, "discard output")

	// out-prefixはworkerごとに別ファイル出力する際に使う
	optOutPrefix = flag.String("out-prefix", "", "path/to/prefix")
)

func init() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel(*optLogLevel))).Sugar().With(zap.String("app", myName))
}

func main() {
	defer logger.Sync()
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

	egOut, ctxOut := errgroup.WithContext(ctx)
	ch := make(chan interface{}, *optWorkers)
	outLoop := func(ctx context.Context, enc *json.Encoder) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case v, ok := <-ch:
				if !ok {
					return nil
				}
				if err := enc.Encode(v); err != nil {
					return err
				}
			}
		}
	}
	if *optOutPrefix == "" {
		egOut.Go(func() (retErr error) {
			defer func() {
				if retErr != nil {
					cancel()
				}
			}()
			return outLoop(ctxOut, json.NewEncoder(os.Stdout))
		})
	} else {
		for i := uint(0); i < *optWorkers; i++ {
			index := i
			egOut.Go(func() (retErr error) {
				defer func() {
					if retErr != nil {
						cancel()
					}
				}()
				fn := filepath.Join(*optOutPrefix + fmt.Sprintf("%03d", index))
				os.MkdirAll(filepath.Dir(fn), os.ModePerm)
				fp, err := os.Create(fn)
				if err != nil {
					return err
				}
				defer fp.Close()
				logger.Infof("out=%s", fn)
				return outLoop(ctxOut, json.NewEncoder(fp))
			})
		}
	}

	egSubs, ctxSubs := errgroup.WithContext(ctx)
	var count int64
	egSubs.Go(func() error {
		subs := cl.Subscription(*optSubscription)
		subs.ReceiveSettings.NumGoroutines = int(*optWorkers)
		return subs.Receive(ctxSubs, func(ctx context.Context, msg *pubsub.Message) {
			msg.Ack()
			n := atomic.AddInt64(&count, 1)
			if *optLogStep > 0 && n%*optLogStep == 0 {
				logger.Infof("count=%d", n)
			}

			if *optOutDiscard {
				return
			}

			m := map[string]interface{}{
				"id":   msg.ID,
				"data": string(msg.Data),
				"attr": msg.Attributes,
			}

			select {
			case <-ctx.Done():
				cancel()
				return
			case ch <- m:
			}
		})
	})

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT)

		s := <-sig
		logger.Infof("Received signal: %v", s)
		cancel()
	}()

	if err := egSubs.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorf("egSubs.Wait: %v", err)
	}
	logger.Infof("received total=%d", count)

	close(ch)
	if err := egOut.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorf("egOut.Wait: %v", err)
	}
}
