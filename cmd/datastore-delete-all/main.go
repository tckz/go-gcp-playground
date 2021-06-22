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
	"github.com/tckz/go-gcp-playground/internal/log"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
)

var (
	myName  = filepath.Base(os.Args[0])
	logger  *zap.SugaredLogger
	version string
)

var (
	optLogLevel   = flag.String("log-level", "info", "info|warn|error")
	optNameSpace  = flag.String("ns", "", "namespace")
	optKind       = flag.String("kind", "mykind", "namespace")
	optDisplayKey = flag.Bool("display-key", false, "")
)

func init() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel(*optLogLevel))).Sugar().With(zap.String("app", myName))
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
				logger.Infof("Deleted %d keys, dur=%s", l, time.Since(now))
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

	// 型にマップするとdatastore側にある列がtype側にないとエラーになってしまってdelete目的の場合にいちいち合わせるのが面倒なのでkeyのみ扱う
	q := datastore.NewQuery(*optKind).Namespace(*optNameSpace).KeysOnly()
	it := cl.Run(ctx, q)
	for {
		key, err := it.Next(nil)
		if err == iterator.Done {
			break
		}
		if err != nil {
			logger.Errorf("Next: %v", err)
			return
		}
		if *optDisplayKey {
			fmt.Printf("Key=%v\n", key)
		}
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
