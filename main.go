package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rexagod/mad/internal/algorithm"
	madoptions "github.com/rexagod/mad/internal/options"
	madscraper "github.com/rexagod/mad/internal/scraper"
	_ "github.com/rexagod/mad/internal/version"
	"k8s.io/klog/v2"
)

func main() {

	// Initialize flags.
	scrapeInterval := flag.Uint("scrape-interval", 1, `
The time interval in seconds at which the endpoint is scraped.
The value must be an integer greater than 0.`)
	timeSeriesSelector := flag.String("time-series-selector", "", `
The selector used to select the time series to scrape at the scrape interval,
	from within the set of various time series fetched from the endpoint.
The value must be a valid PromQL metric selector. For eg., 'foo{bar="baz"}'.
If multiple time series match the selector, the first one will be selected.`)
	endpoint := flag.String("endpoint", "", `
The endpoint to scrape, must be a valid URL.
Only the non-empty blob lines that are not comments (starting with a "#") will
	be processed.`)
	flag.Parse()

	// Validate flags.
	if *scrapeInterval < 1 {
		klog.Fatal("scrape-interval must be greater than 0, exiting")
	}
	if *timeSeriesSelector == "" {
		klog.Fatal("time-series-selector must be set, exiting")
	}
	if *endpoint == "" {
		klog.Fatal("endpoint must be set, exiting")
	}

	// Initialize options.
	options := madoptions.New()
	options.ScrapeInterval = *scrapeInterval
	options.TimeSeriesSelector = *timeSeriesSelector
	options.Endpoint = *endpoint

	// Listen for SIGINT and SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Initialize scraper.
	scraper, err := madscraper.New(options)
	if err != nil {
		klog.Fatalf("Could not initialize scraper %s: %w", scraper.String(), err)
	}

	// Initialize context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scraper.
	go func() {
		err = scraper.Scrape(ctx, stop)
		if err != nil {
			klog.Errorf("Could not scrape endpoint: %w", err)
		}
	}()

	// Start detection.
	go algorithm.Run(ctx, scraper, stop)

	// Wait for SIGINT or SIGTERM.
	<-stop
}
