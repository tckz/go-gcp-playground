package main

import (
	"context"
	"flag"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/tckz/go-gcp-playground/internal/log"
	vh "github.com/tckz/vegetahelper"
	vegeta "github.com/tsenart/vegeta/v12/lib"
	"go.uber.org/zap"
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
	optDuration  = flag.Duration("duration", 10*time.Second, "Duration of the test [0 = forever]")
	optOutput    = flag.String("output", "", "/path/to/results.bin or 'stdout'")
	optWorkers   = flag.Uint64("workers", vegeta.DefaultWorkers, "Number of workers")
	optLogLevel  = flag.String("log-level", "info", "info|warn|error")
	optNameSpace = flag.String("ns", "", "namespace")
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

	if *optOutput == "" {
		logger.Fatalf("*** --output must be specified.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pjID := os.Getenv("PROJECT_ID")

	cl, err := datastore.NewClient(context.Background(), pjID)
	if err != nil {
		logger.Fatalf("*** datastore.NewClient: %v", err)
	}
	defer cl.Close()

	atk := vh.NewAttacker(func(ctx context.Context) (result *vh.HitResult, retErr error) {
		kind := "mykind"
		name := uuid.New().String()
		key := datastore.NameKey(kind, name, nil)
		key.Namespace = *optNameSpace
		rec := MyKind{Name: "my name is " + name}
		_, err := cl.Put(ctx, key, &rec)
		if err != nil {
			return nil, err
		}

		return result, nil
	}, vh.WithWorkers(*optWorkers))
	res := atk.Attack(ctx, *optRate.Rate, *optDuration, "datastore")

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

	cancel()
}
