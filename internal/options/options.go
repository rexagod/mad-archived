package options

type Options struct {
	ScrapeInterval     uint
	TimeSeriesSelector string
	Endpoint           string
}

func New() *Options {

	return &Options{}
}
