package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	vh "github.com/tckz/vegetahelper"
	vegeta "github.com/tsenart/vegeta/lib"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
)

func init() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano())

	flag.Var(optRate, "rate", "Number of requests per time unit")
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
		logger.Fatalf("*** zap.Build: %v", err)
	}

	logger = zl.Sugar().With(zap.String("app", myName))

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

	parentKey := datastore.IDKey("mykind", 5644004762845184, nil)
	atk := vh.NewAttacker(func(ctx context.Context) (result *vh.HitResult, retErr error) {
		kind := "mychild"
		name := uuid.New().String()
		key := datastore.NameKey(kind, name, parentKey)
		rec := MyKind{Name: "my name is " + name}
		_, err := cl.Put(ctx, key, &rec)
		if err != nil {
			return nil, err
		}

		var rec2 MyKind
		if err := cl.Get(ctx, key, &rec2); err != nil {
			return nil, err
		}

		if rec2.Name != rec.Name {
			return nil, errors.New("not match")
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
