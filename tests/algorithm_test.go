// Package tests holds the e2e test suite for the ED-PELT algorithm.
package tests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/prometheus/prometheus/promql/parser"
	"k8s.io/klog/v2"
	"pgregory.net/changepoint"
)

const metricsPath = "/metrics"
const port = ":8090"
const timeout = time.Minute

func serve(ctx context.Context, errChan chan<- error, isConsumed chan<- []int) {
	klog.Infoln("Starting server...")
	mux := http.NewServeMux()
	mux.HandleFunc(metricsPath, func(w http.ResponseWriter, r *http.Request) {
		// In a production scenario, the specified metric will be scraped at every scrape interval
		// from the endpoint and eventually sent to the sample channel, along with some metadata.
		scrape := []string{
			"# HELP mock_metric_scrape_N Mock metric from the Nth scrape",
			"# TYPE mock_metric_scrape_N gauge",
		}
		samples := []uint8{1, 1, 1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 1, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 0, 1, 0, 1, 1, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1}
		for i, sample := range samples {
			// nolint:gosec
			metric := fmt.Sprintf("mock_metric_scrape_" + strconv.Itoa(i) + " " + strconv.Itoa(int(sample)))
			scrape = append(scrape, metric)
		}
		_, err := fmt.Fprintln(w, strings.Join(scrape, "\n"))
		if err != nil {
			klog.Errorln("Could not write to response writer")
			errChan <- err

			return
		}
	})

	server := &http.Server{Addr: port, Handler: mux, ReadHeaderTimeout: timeout}
	go func() {
		for {
			if ctx.Err() != nil || len(errChan) > 0 || len(isConsumed) > 0 {
				err := server.Shutdown(ctx)
				if err != nil {
					klog.Errorf("Could not gracefully shutdown the server: %w\n", err)
				}
				klog.Info("Stopping server...")

				return
			}
		}
	}()

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		klog.Errorln("Could not start server")
		errChan <- err
		klog.Infoln("Stopping server...")

		return
	}
}

func fetch(ctx context.Context, errChan chan<- error, isConsumed chan<- []int) {
	klog.Infoln("Starting fetcher...")
	var body []byte
	get := func() error {
		endpoint := "http://localhost" + port + metricsPath
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			klog.Errorf("Could not create GET request: %w\n", err)
		}
		req = req.WithContext(ctx)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			klog.Errorf("Could not GET %s: %w\n", endpoint, err)
		}
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			klog.Errorf("Could not read response body: %w\n", err)
		}
		defer func() {
			err = resp.Body.Close()
			if err != nil {
				klog.Errorf("Could not close response body: %w\n", err)
			}
		}()
		if resp != nil && resp.StatusCode == http.StatusOK {

			return nil
		}
		err = fmt.Errorf("did not respond with 200 OK: %w", err)

		return err
	}
	_ = backoff.Retry(get, backoff.NewExponentialBackOff())

	reader := bufio.NewReader(bytes.NewBuffer(body))
	var samples []float64
	for {
		if ctx.Err() != nil || len(errChan) > 0 {
			klog.Infoln("Stopping fetcher...")

			return
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				anomalies := changepoint.NonParametric(samples, 1)
				isConsumed <- anomalies
				klog.Infoln("Stopping fetcher...")

				return
			}
			klog.Errorf("Could not read response body: %w\n", err)
		}
		if regexp.MustCompile("^[a-z]").MatchString(line) {
			_, v, err := parser.ParseSeriesDesc(line)
			if err != nil {
				klog.Errorf("Could not parse series description: %w\n", err)
			}
			if len(v) > 1 {
				klog.Errorf("Expected 1 value, got %d\n", len(v))
			}
			samples = append(samples, v[0].Value)
		}
	}
}

func TestAlgorithm(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errChan := make(chan error, 1)
	isConsumed := make(chan []int, 1)

	var wg sync.WaitGroup
	wgfn := func(fn func(ctx context.Context, errChan chan<- error, isConsumed chan<- []int)) {
		defer wg.Done()
		fn(ctx, errChan, isConsumed)
	}
	wg.Add(2)
	go wgfn(serve)
	go wgfn(fetch)

	exit0 := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		exit0 <- struct{}{}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		t.Fatal(err)
	case <-stop:
		cancel()
	case <-ctx.Done():
		cancel()
	case <-exit0:
		got := <-isConsumed
		want := []int{61, 94}
		if !reflect.DeepEqual(got, want) {
			t.Logf("Expected %v, got %v", want, got)
			t.Fail()
		}
	}
}
