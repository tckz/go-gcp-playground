package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
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
		logger.Fatalf("*** zap.Build: %v", err)
	}

	logger = zl.Sugar().With(zap.String("app", myName))

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
