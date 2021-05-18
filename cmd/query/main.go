package main

import (
	"context"
	"flag"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optOutput   = flag.String("output", "", "/path/to/results.bin or 'stdout'")
	optLogLevel = flag.String("log-level", "info", "info|warn|error")
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

	out, err := openResultFile(*optOutput)
	if err != nil {
		logger.Fatal(err)
	}
	defer out.Close()

	datastore.NewQuery("mychild").Start(datastore.Cursor{})
}
