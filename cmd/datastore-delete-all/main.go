package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
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

type MyKind struct {
	Name string
	Time time.Time
}

func main() {
	logger.Infof("ver=%s, args=%s", version, os.Args)
	{
		now := time.Now()
		defer func() {
			logger.Infof("done, dur=%s", time.Since(now))
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pjID := os.Getenv("PROJECT_ID")

	cl, err := datastore.NewClient(context.Background(), pjID)
	if err != nil {
		logger.Fatalf("*** datastore.NewClient: %v", err)
	}
	defer cl.Close()

	const MaxDeleteItem = 500
	chKey := make(chan *datastore.Key, MaxDeleteItem)
	chReport := make(chan int)

	egReport, ctxReport := errgroup.WithContext(ctx)
	egReport.Go(func() (retErr error) {
		defer func() {
			if retErr != nil {
				cancel()
			}
		}()
		sum := 0
		for {
			select {
			case <-ctxReport.Done():
				return ctxReport.Err()
			case e, ok := <-chReport:
				if !ok {
					logger.Infof("%d entities deleted", sum)
					return nil
				}
				sum = sum + e
			}
		}
	})

	egDel, ctxDel := errgroup.WithContext(ctx)
	for i := 0; i < 8; i++ {
		egDel.Go(func() (retErr error) {
			defer func() {
				if retErr != nil {
					cancel()
				}
			}()
			keys := make([]*datastore.Key, 0, MaxDeleteItem)
			flush := func() error {
				if len(keys) == 0 {
					return nil
				}
				l := len(keys)
				now := time.Now()
				logger.Infof("Try to delete %d keys", l)
				if err := cl.DeleteMulti(ctxDel, keys); err != nil {
					return err
				}
				logger.Infof("Try to delete %d keys, dur=%s", l, time.Since(now))
				keys = keys[:0]
				chReport <- l
				return nil
			}

			for {
				select {
				case <-ctxDel.Done():
					return ctxDel.Err()
				case e, ok := <-chKey:
					if !ok {
						if err := flush(); err != nil {
							return err
						}
						return nil
					}
					keys = append(keys, e)
					if len(keys) == MaxDeleteItem {
						if err := flush(); err != nil {
							return err
						}
					}
				}
			}
		})
	}

	q := datastore.NewQuery(*optKind).Namespace(*optNameSpace)
	it := cl.Run(ctx, q)
	for {
		var rec MyKind
		key, err := it.Next(&rec)
		if err == iterator.Done {
			break
		}
		if err != nil {
			logger.Errorf("Next: %v", err)
			return
		}
		fmt.Printf("Key=%v, MyKind=%+v\n", key, rec)
		chKey <- key
	}
	close(chKey)

	logger.Infof("Waiting for deleting done")
	if err := egDel.Wait(); err != nil {
		logger.Errorf("Wait.Del: %v", err)
		return
	}
	close(chReport)

	if err := egReport.Wait(); err != nil {
		logger.Errorf("Wait.Report: %v", err)
		return
	}
}