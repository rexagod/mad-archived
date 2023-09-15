package algorithm

import (
	"context"
	"os"
	"time"

	madscraper "github.com/rexagod/mad/internal/scraper"
	"k8s.io/klog/v2"
	edpelt "pgregory.net/changepoint"
)

func Run(ctx context.Context, scraper *madscraper.Scraper, stop chan os.Signal) {
	// samples holds the particular metric metadata after fetching and searching through the scrape payload.
	var samples []madscraper.TimestampedFloat64

	// Keep reading samples from the scraper till they hit the base volume and process them for anomalies.
	for {

		// Check if we need to stop.
		if len(stop) > 0 || ctx.Err() != nil {

			return
		}

		// Read samples until we have enough data to run the algorithm.
		for len(samples) < madscraper.MinSampleCount {
			samples = append(samples, scraper.ReadSample())

			continue
		}

		// Segregate samples into values and timestamps.
		var sampleValues []float64
		var sampleTimestamps []time.Time
		for _, sample := range samples {
			sampleValues = append(sampleValues, sample.Value())
			sampleTimestamps = append(sampleTimestamps, sample.Timestamp())
		}

		// indices is the list of indices where the change points were detected.
		indices := edpelt.NonParametric(sampleValues, 1)
		for _, index := range indices {
			klog.Infof("Change point %s detected at %v\n", samples[index], sampleTimestamps[index])
		}

		// Trim the data set till the last change point, if any.
		fromIndex := (len(samples) - 1) / 2
		if len(indices) > 0 {
			fromIndex = indices[len(indices)-1]
		}
		samples = samples[fromIndex:]
	}
}
