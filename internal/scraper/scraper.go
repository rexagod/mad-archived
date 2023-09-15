package scraper

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kr/pretty"
	"github.com/prometheus/prometheus/promql/parser"
	madoptions "github.com/rexagod/mad/internal/options"
	"k8s.io/klog/v2"
)

const (
	// MinSampleCount is the minimum number of samples needed by the detection algorithm to start estimating anomalies.
	MinSampleCount = 1 << 5
)

var (
	// vectorTypeMismatchErr is returned when the vector type of the metric does not match the vector type of the selector.
	vectorTypeMismatchErr = errors.New("vector type mismatch")

	// searchTargetNotFoundErr is returned when the search target is not found in the scrape payload.
	searchTargetNotFoundErr = errors.New("search target not found")
)

// TimestampedFloat64 holds a timestamped float64 value.
type TimestampedFloat64 struct {
	timestamp time.Time
	value     float64
}

// String returns a pretty-printed string representation of the TimestampedFloat64.
func (tsf *TimestampedFloat64) String() string {

	return pretty.Sprintf("%# v", tsf)
}

// Timestamp returns the timestamp of the sample.
func (tsf *TimestampedFloat64) Timestamp() time.Time {

	return tsf.timestamp
}

// Value returns the value of the sample.
func (tsf *TimestampedFloat64) Value() float64 {

	return tsf.value
}

// sampleChan is a thread-safe channel for sample operations.
type sampleChan struct {
	ch chan TimestampedFloat64
	m  sync.Mutex
}

// Read consumes a sample from the sample channel.
func (s *sampleChan) Read() TimestampedFloat64 {

	return <-s.ch
}

// Write writes a sample to the sample channel.
func (s *sampleChan) Write(sample float64) {
	s.m.Lock()
	defer s.m.Unlock()
	s.ch <- TimestampedFloat64{timestamp: time.Now(), value: sample}
}

// Len returns the length of the sample channel.
func (s *sampleChan) Len() uint8 {
	s.m.Lock()
	defer s.m.Unlock()

	return uint8(len(s.ch))
}

// Scraper holds the configuration for a scraper.
type Scraper struct {
	scrapeInterval     time.Duration
	timeSeriesSelector *parser.VectorSelector
	endpoint           url.URL
	sampleChan         *sampleChan
}

// String returns a pretty-printed string representation of the Scraper.
func (s *Scraper) String() string {

	return pretty.Sprintf("%# v", s)
}

// ReadSample is a wrapper over sampleChan.Read.
func (s *Scraper) ReadSample() TimestampedFloat64 {

	return s.sampleChan.Read()
}

// WriteSample is a wrapper over sampleChan.Write.
func (s *Scraper) WriteSample(sample float64) {
	s.sampleChan.Write(sample)
}

// LenSamples is a wrapper over sampleChan.Len.
func (s *Scraper) LenSamples() uint8 {

	return s.sampleChan.Len()
}

// New returns a new Scraper.
func New(options *madoptions.Options) (*Scraper, error) {
	timeSeriesSelector, err := parser.ParseExpr(options.TimeSeriesSelector)
	if err != nil {

		return nil, fmt.Errorf("encountered error while parsing time series selector: %w", err)
	}
	if timeSeriesSelector.Type() != parser.ValueTypeVector {

		return nil, fmt.Errorf("time series selector must be of %s type, got %s", parser.ValueTypeVector, timeSeriesSelector.Type())
	}
	endpoint, err := url.Parse(options.Endpoint)
	if err != nil {

		return nil, fmt.Errorf("%s must be a valid URL: %w", options.Endpoint, err)
	}

	return &Scraper{
		scrapeInterval:     time.Duration(options.ScrapeInterval) * time.Second,
		timeSeriesSelector: timeSeriesSelector.(*parser.VectorSelector),
		endpoint:           *endpoint,

		// A buffered channel is used to avoid blocking the scraper (in case the detection process ends up taking more time than expected).
		sampleChan: &sampleChan{ch: make(chan TimestampedFloat64, MinSampleCount)},
	}, nil
}

// Scrape scrapes the endpoint and sends the metric value to the sample channel.
func (s *Scraper) Scrape(ctx context.Context, stop chan os.Signal) error {
	ticker := time.NewTicker(s.scrapeInterval)
	defer ticker.Stop()

	// NOTE: The scrape interval should account for the time it takes to scrape the endpoint, i.e., max(scrape interval, time to scrape endpoint).
	for range ticker.C {
		if len(stop) > 0 || ctx.Err() != nil {

			// nolint: nilerr
			return nil
		}
		req, err := http.NewRequest(http.MethodGet, s.endpoint.String(), nil)
		if err != nil {

			return fmt.Errorf("could not create request: %w", err)
		}
		req = req.WithContext(ctx)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp == nil {

			return fmt.Errorf("could not GET %s: %w", s.endpoint.String(), err)
		}
		if resp.StatusCode != http.StatusOK {

			return fmt.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {

			return fmt.Errorf("could not read response body: %w", err)
		}
		err = resp.Body.Close()
		if err != nil {

			return fmt.Errorf("could not close response body: %w", err)
		}
		reader := bufio.NewReader(bytes.NewBuffer(body))
		go read(reader, s.timeSeriesSelector, s.sampleChan)
	}

	return nil
}

// read reads the response body and sends the metric value to the sample channel.
func read(reader *bufio.Reader, selector *parser.VectorSelector, sampleChan *sampleChan) {
	got := new(float64)
	for /* Search for the matching metric value. */ {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {

			break
		}
		if err != nil {
			klog.Errorf("could not read response body: %w", err)

			return
		}
		err = search(line, selector, got)
		if errors.Is(err, vectorTypeMismatchErr) {
			continue
		} else if errors.Is(err, searchTargetNotFoundErr) {
			break
		} else if err != nil {
			klog.Error(err)

			return
		}
	}
	sampleChan.Write(*got)
}

// search searches for the matching metric value.
func search(line string, selector *parser.VectorSelector, got *float64) error {
	if regexp.MustCompile("^[a-z]").MatchString(line) {
		metric, err := parser.ParseExpr(strings.SplitN(line, " ", 2)[0])
		if metric.Type() != parser.ValueTypeVector {

			return vectorTypeMismatchErr // Skip.
		}
		if err != nil {

			return fmt.Errorf("could not parse metric: %w", err)
		}
		found, err := vectorIsEqual(selector, metric.(*parser.VectorSelector))
		if err != nil {

			return fmt.Errorf("could not compare vectors: %w", err)
		}
		if found {
			_, v, err := parser.ParseSeriesDesc(line)
			if err != nil {

				return fmt.Errorf("could not parse series description: %w", err)
			}
			if len(v) != 1 {

				return fmt.Errorf("expected 1 value, got %d: %s", len(v), v)
			}
			*got = v[0].Value

			return nil
		}
	}
	if got == nil {

		return searchTargetNotFoundErr // Break.
	}

	return nil
}

// vectorIsEqual compares two vectors and returns true if they are equal.
func vectorIsEqual(a, b *parser.VectorSelector) (bool, error) {
	aparsed, err := parser.ParseMetric(a.String())
	if err != nil {

		return false, fmt.Errorf("could not parse metric: %w", err)
	}
	bparsed, err := parser.ParseMetric(b.String())
	if err != nil {

		return false, fmt.Errorf("could not parse metric: %w", err)
	}
	aparsedmap := aparsed.Map()
	bparsedmap := bparsed.Map()
	if len(aparsedmap) != len(bparsedmap) {

		return false, nil
	}
	for k, v := range aparsedmap {
		if bparsedmap[k] != v {

			return false, nil
		}
	}

	return true, nil
}
