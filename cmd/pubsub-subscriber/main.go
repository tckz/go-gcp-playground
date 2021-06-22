package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	"github.com/tckz/go-gcp-playground/internal/log"
	vegeta "github.com/tsenart/vegeta/lib"
	"go.uber.org/zap"
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
	optRedis        = flag.String("redis", "", "addr:port of redis")
	optCounterKey   = flag.String("counter-key", "subscriber-counter", "key of redis")
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

	var counter Counter
	var processMarker ProcessMarker
	if *optRedis == "" {
		counter = &LocalCounter{}
		processMarker = &LocalMarker{cache: cache.New(1*time.Minute, 1*time.Minute)}
	} else {
		cl := redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:        []string{*optRedis},
			DialTimeout:  time.Second * 2,
			ReadTimeout:  time.Second * 2,
			WriteTimeout: time.Second * 2,
			PoolSize:     200,
			PoolTimeout:  time.Second * 5,
		})
		defer cl.Close()
		counter = &RedisCounter{key: *optCounterKey, client: cl}
		processMarker = &RedisMarker{client: cl}
	}

	eg, ctx := errgroup.WithContext(ctx)
	for i := uint64(0); i < *optWorkers; i++ {
		eg.Go(func() error {
			subs := cl.Subscription(*optSubscription)
			return subs.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				if got, err := processMarker.Acquire(ctx, msg.ID); err != nil {
					logger.Errorf("ProcessCheck: %v", err)
					return
				} else if !got {
					logger.Infof("msgID=%s already marked to be processed by other", msg.ID)
					return
				}

				if n, err := counter.Up(ctx); err != nil {
					logger.Errorf("Up: %v", err)
					return
				} else if n%1000 == 0 {
					logger.Infof("received=%d", n)
				}
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

	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		v, _ := counter.Get(ctx)
		logger.Infof("Counter=%d", v)
	}
}
