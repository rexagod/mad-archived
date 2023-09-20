# Metrics Anomaly Detector

[![Continuous Integration](https://github.com/rexagod/mad/workflows/ci/badge.svg)](https://github.com/rexagod/mad/actions) [![Code Quality](https://goreportcard.com/badge/github.com/rexagod/mad)](https://goreportcard.com/report/github.com/rexagod/mad) [![API Reference](https://pkg.go.dev/badge/github.com/rexagod/mad.svg)](https://pkg.go.dev/github.com/rexagod/mad)

## Introduction

Metrics Anomaly Detector is a tool for detecting anomalies in time series data. One or more data points that significantly differ from the other observations can be qualified as an anomaly, or an outlier.

## Algorithm

### Change-Point Detection

Change-point detection is a statistical method used to identify points in a data set where the properties of the data significantly shift. This analysis is crucial in various fields to understand and monitor unexpected variations within a series of data over a period of time. This could involve changes in the mean, variance, distribution, or other statistical properties.

In terms of a sequence of component health checks, change-point detection helps identify when the system's health status undergoes significant changes. A change-point could indicate a shift from a period where the system is *mostly* "healthy" to a time when it is *frequently* "unhealthy". The sensitivity of change-point triggers can be adjusted to suit the needs of a particular scenario and reduce false alarms.

### Techniques

Below are the change-point techniques that were considered for this project.

- **Page-Hinkley (PH):** A quick, simple algorithm for detecting abrupt changes in data distribution, using a cumulative sum of data observations' differences from the average. Requires domain knowledge for threshold definition, may not be effective for gradual changes.

- **Adaptive Windowing (ADWIN):** Dynamically adjusts the window size of recent data according to the data distribution's volatility, accommodating both abrupt and gradual changes. The tradeoff is a higher computational expense and potential overreaction to noise due to sensitivity.

- **Cumulative Sum (CUSUM):** A robust method calculating a sum of differences between observed values and a reference value, effective for detecting both small and large shifts. Similar to PH, it requires a pre-determined threshold and may be less effective for gradual changes.

- **Generalized Likelihood Ratio (GLR):** Uses likelihood ratios to test changes at every data point, assuming the data follows different distributions before and after a change. Though powerful and versatile in change-detection, it can be complex to implement and requires more computational resources, making it a heavy-duty option for large, real-time datasets.

- **Pruned Exact Linear Time (PELT)**: Designed for data distributions with known characteristics, this algorithm employs a cost function to manage change points, helping to prevent over-fitting. With its 'exact linear time' efficiency, PELT is apt for larger datasets and scenarios where data distribution is assumed.

- **Exponential Density-Pruning Exact Linear Time (ED-PELT)**: ED-PELT requires minimal assumptions on data distributions. It models inter-arrival times using an exponential distribution. Like PELT, it works swiftly even on large datasets and particularly shines in complex scenarios without clear data distribution information.

### Evaluation

For time-series-based scenarios that encompass various metrics, ED-PELT shines among change-point detection methods, thanks to its distribution-free nature. It works without needing data distribution assumptions required by PELT, CUSUM, or GLR, accommodating various datasets effectively. With 'exact linear time' computational complexity, like PELT, it handles large datasets efficiently.

Its edge over ADWIN and PHT, known for their adaptive features, is its ability to model inter-arrival times using an exponential distribution, prime for tackling complex situations, making ED-PELT a highly versatile and efficient tool.

## Usage

```
┌[rexagod@nebuchadnezzar] [/dev/ttys001] [main ⚡] 
└[~/repositories/oss/mad]> make && mad -h
I0920 00:11:06.816062   44601 version.go:9] metrics-anomaly-detector, version v0.0.1 (branch: main, revision: 75471ce)
  build user:       rexagod@nebuchadnezzar
  build date:       2023-09-19T18:40:49Z
  go version:       go1.20
  platform:         darwin/arm64
  tags:             unknown
Usage of mad:
  -endpoint string
        
        The endpoint to scrape, must be a valid URL.
        Only the non-empty blob lines that are not comments (starting with a "#") will
                be processed.
  -scrape-interval uint
        
        The time interval in seconds at which the endpoint is scraped.
        The value must be an integer greater than 0. (default 1)
  -time-series-selector string
        
        The selector used to select the time series to scrape at the scrape interval,
                from within the set of various time series fetched from the endpoint.
        The value must be a valid PromQL metric selector. For eg., 'foo{bar="baz"}'.
        If multiple time series match the selector, the first one will be selected.
```

## Bibliography

- Haynes, K., Fearnhead, P., & Eckley, I.A. (2017). A computationally efficient nonparametric approach for changepoint detection. Statistics and Computing, 27(5), 1293-1305. https://doi.org/10.1007/s11222-016-9687-5 <!--vale off-->
- Killick, R., Fearnhead, P., & Eckley, I.A. (2012). Optimal detection of changepoints with a linear computational cost. Journal of the American Statistical Association, 107(500), 1590-1598. https://arxiv.org/pdf/1101.1438.pdf <!--vale off-->
