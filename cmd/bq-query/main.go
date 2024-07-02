package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/dustin/go-humanize"
	"github.com/joho/godotenv"
	"github.com/samber/lo"
	"github.com/tckz/go-gcp-playground/internal/log"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
)

var (
	optQuery    = flag.String("query", "", "path/to/query.sql")
	optOut      = flag.String("out", "/dev/stdout", "path/to/output")
	optFormat   = flag.String("format", "json", "csv|tsv|json")
	optLocation = flag.String("location", "", "location of dataset")
	optLogStep  = flag.Int("log-step", 1000, "How many rows between each log output")
)

var logger *zap.SugaredLogger

func main() {
	godotenv.Load()

	flag.Parse()

	logger = log.Must(log.NewLogger(log.WithLogLevel("info"))).Sugar()

	if *optQuery == "" {
		logger.Fatalf("*** --query must be specified")
	}

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	if err := run(ctx); err != nil {
		logger.Fatalf("*** run: %v", err)
	}
}

func run(ctx context.Context) error {
	pjID := os.Getenv("PROJECT_ID")
	client, err := bigquery.NewClient(ctx, pjID)
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %v", err)
	}
	defer client.Close()

	b, err := os.ReadFile(*optQuery)
	if err != nil {
		return fmt.Errorf("os.ReadFile: %v", err)
	}

	w, err := os.Create(*optOut)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer w.Close()

	q := client.Query(string(b))

	// Location must match that of the dataset(s) referenced in the query.
	q.Location = *optLocation

	// Run the query and print results when the query job is completed.
	job, err := q.Run(ctx)
	if err != nil {
		return fmt.Errorf("q.Run: %v", err)
	}

	logger.Infof("job.ID=%s", job.ID())

	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("job.Wait: %v", err)
	}

	logger.With(zap.Any("lastStatus", status)).Infof("totalProcessedSize: %s", humanize.Bytes(uint64(status.Statistics.TotalBytesProcessed)))

	if err := status.Err(); err != nil {
		return fmt.Errorf("status.Err: %v", err)
	}

	it, err := job.Read(ctx)
	if err != nil {
		return fmt.Errorf("job.Read: %v", err)
	}

	enc, err := NewEncoder(*optFormat, it, w)
	if err != nil {
		return err
	}
	defer enc.Flush()

	lc := 0
	for {
		done, err := enc.Next()
		if err != nil {
			return err
		}
		if done {
			break
		}

		lc++
		if lc%*optLogStep == 0 {
			logger.Infof("recs=%d/%d", lc, it.TotalRows)
		}
	}
	logger.Infof("total=%d", lc)

	return nil
}

type BigqueryRowEncoder interface {
	Next() (bool, error)
	Flush() error
}

func NewEncoder(format string, it *bigquery.RowIterator, w io.Writer) (BigqueryRowEncoder, error) {
	switch format {
	case "csv":
		return NewCSVEncoder(it, csv.NewWriter(w)), nil
	case "tsv":
		cw := csv.NewWriter(w)
		cw.Comma = '\t'
		return NewCSVEncoder(it, cw), nil
	case "json":
		return NewJSONEncoder(it, w), nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

var _ bigquery.ValueLoader = (*row)(nil)

type row struct {
	schema *bigquery.Schema
	vals   []bigquery.Value
}

func (r *row) Load(v []bigquery.Value, s bigquery.Schema) error {
	if r.schema == nil {
		r.schema = &s
	}
	r.vals = v
	return nil
}

var _ BigqueryRowEncoder = (*CSVEncoder)(nil)

type CSVEncoder struct {
	w      *csv.Writer
	it     *bigquery.RowIterator
	schema *bigquery.Schema
}

func (e *CSVEncoder) Flush() error {
	e.w.Flush()
	return nil
}

func (e *CSVEncoder) Next() (bool, error) {
	var row row

	first := e.schema == nil

	err := e.it.Next(&row)
	if err == iterator.Done {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("it.Next: %v", err)
	}

	if first {
		if err := e.w.Write(lo.Map(*row.schema, func(v *bigquery.FieldSchema, _ int) string { return v.Name })); err != nil {
			return false, fmt.Errorf("csv.Write.header: %v", err)
		}
		e.schema = row.schema
	}

	fields := make([]string, 0, len(row.vals))
	for i, v := range row.vals {
		cv, err := func() (string, error) {
			if v == nil {
				return "", nil
			}

			if s := (*e.schema)[i]; s.Repeated || s.Type == bigquery.RecordFieldType {
				b, err := json.Marshal(v)
				if err != nil {
					return "", fmt.Errorf("json.Marshal: %v", err)
				}
				return string(b), nil
			}

			switch v := v.(type) {
			case time.Time:
				return v.Format(time.RFC3339Nano), nil
			case bool:
				// %tだと大文字になるので
				if v {
					return "true", nil
				} else {
					return "false", nil
				}
			}

			return fmt.Sprintf("%v", v), nil
		}()
		if err != nil {
			return false, err
		}
		fields = append(fields, cv)
	}

	if err := e.w.Write(fields); err != nil {
		return false, fmt.Errorf("csv.Write: %v", err)
	}

	return false, nil
}

func NewCSVEncoder(it *bigquery.RowIterator, w *csv.Writer) *CSVEncoder {
	return &CSVEncoder{
		w:  w,
		it: it,
	}
}

var _ BigqueryRowEncoder = (*JSONEncoder)(nil)

type JSONEncoder struct {
	enc *json.Encoder
	it  *bigquery.RowIterator
}

func (e *JSONEncoder) Flush() error {
	return nil
}

func (e *JSONEncoder) Next() (bool, error) {
	var row map[string]bigquery.Value
	err := e.it.Next(&row)
	if err == iterator.Done {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("it.Next: %v", err)
	}

	if err := e.enc.Encode(row); err != nil {
		return false, fmt.Errorf("enc.Encode: %v", err)
	}

	return false, nil
}

func NewJSONEncoder(it *bigquery.RowIterator, w io.Writer) *JSONEncoder {
	return &JSONEncoder{
		it:  it,
		enc: json.NewEncoder(w),
	}
}
