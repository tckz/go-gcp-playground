package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/tckz/go-gcp-playground/internal/log"
	"go.uber.org/zap"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optLogLevel  = flag.String("log-level", "info", "info|warn|error")
	optNameSpace = flag.String("ns", "", "namespace")
	optName      = flag.String("name", "", "named key")
	optKind      = flag.String("kind", "mykind", "namespace")
)

func init() {
	godotenv.Load()

	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel(*optLogLevel))).Sugar().With(zap.String("app", myName))
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

	name := *optName
	if name == "" {
		name = uuid.New().String()
	}

	wg := &sync.WaitGroup{}

	// 通信環境や実行時コア数によると思うが自分の環境では2つで十分再現できる
	numGoroutines := 2
	counter := make([]int64, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		index := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			logger := logger.With(zap.String("index", fmt.Sprintf("%d", index)))

			// ざっと見た感じ別インスタンスを作ればコネクションも別っぽいので1プロセスで確認できそう
			cl, err := datastore.NewClient(context.Background(), pjID)
			if err != nil {
				logger.Fatalf("*** datastore.NewClient: %v", err)
			}
			defer cl.Close()

			key := datastore.NameKey(*optKind, name, nil)
			key.Namespace = *optNameSpace

			alreadyExist := false
			_, err = cl.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
				// RunInTransactionに渡したfuncは複数回実行されうる
				logger.Infof("begin transaction")
				atomic.AddInt64(&counter[index], 1)

				alreadyExist = false
				var rec MyKind
				err := tx.Get(key, &rec)
				logger.Infof("Get: err=%v", err)
				if err == nil {
					alreadyExist = true
					return nil
				} else if err != datastore.ErrNoSuchEntity {
					return err
				}

				_, err = tx.Put(key, &MyKind{
					Name: "myname is " + name,
					Time: time.Now().UTC(),
				})
				logger.Infof("Put: err=%v", err)
				return err
			})

			logger.Infof("alreadyExist=%t, err=%v", alreadyExist, err)
		}()
	}

	logger.Infof("waiting all goroutines are done")
	wg.Wait()

	// トランザクションに負けた側は2回以上実行される
	/*
	 transaction func for index=0 called 2 times
	 transaction func for index=1 called 1 times
	*/
	for i, e := range counter {
		logger.Infof("transaction func for index=%d called %d times", i, e)
	}
}
