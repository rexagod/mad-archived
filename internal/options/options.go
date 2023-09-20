// Package options holds the options used to initialize the scraper.
package options

// Options holds the options for the scraper.
type Options struct {
	ScrapeInterval     uint
	TimeSeriesSelector string
	Endpoint           string
}

// New instantiates an empty Options object.
func New() *Options {

	return &Options{}
}
