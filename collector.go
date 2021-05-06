// (C) 2020 by Marco Paganini <paganini@paganini.net>
//
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
// License for the specific language governing permissions and limitations
// under the License.

package main

import (
	"fmt"
	"github.com/kofalt/go-memoize"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net/url"
	"strings"
	"time"

	finance "github.com/piquette/finance-go"
	quote "github.com/piquette/finance-go/quote"
)

var (
	// These are metrics for the collector itself
	queryDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name: "quotes_exporter_query_duration_seconds",
			Help: "Duration of queries to the upstream API",
		},
	)
	queryCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "quotes_exporter_queries_total",
			Help: "Count of completed queries",
		},
	)
	errorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "quotes_exporter_failed_queries_total",
			Help: "Count of failed queries",
		},
	)

	// Cache external API consuming calls for 10 minutes.
	cache *memoize.Memoizer = memoize.NewMemoizer(10*time.Minute, 20*time.Minute)

	// flags
	flagPort int
	flagVolume bool
)

// collector holds data for a prometheus collector.
type collector struct {
	symbols []string
}

// newCollector returns a new collector object with parsed data from the URL object.
func newCollector(myurl *url.URL) (collector, error) {
	var symbols []string

	// Typical query is formatted as: ?symbols=AAA,BBB...&symbols=CCC,DDD...
	// We fetch all symbols into a single slice.
	qvalues, ok := myurl.Query()["symbols"]
	if !ok {
		return collector{}, fmt.Errorf("missing symbols in query")
	}
	for _, qvalue := range qvalues {
		symbols = append(symbols, strings.Split(qvalue, ",")...)
	}
	return collector{symbols}, nil
}

// Describe outputs description for prometheus timeseries.
func (c collector) Describe(ch chan<- *prometheus.Desc) {
	// Must send one description, or the registry panics.
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

// Collect retrieves quote data and ouputs prometheus compatible timeseries on
// the output channel.
func (c collector) Collect(ch chan<- prometheus.Metric) {
	queryCount.Inc()

	for _, symbol := range c.symbols {
		// Try not to hit the end point too hard.
		cachedFetcher := func() (interface{}, error) {
			return quote.Get(symbol)
		}

		start := time.Now()
		qret, err, cached := cache.Memoize(symbol, cachedFetcher)
		queryDuration.Observe(float64(time.Since(start).Seconds()))

		if err != nil {
			errorCount.Inc()
			log.Printf("Error looking up %s: %v\n", symbol, err)
			return
		}
		// Convert to native type as Memoize returns an interface.
		qq, ok := qret.(*finance.Quote)
		if !ok {
			errorCount.Inc()
			log.Printf("Invalid quote data for %s: %v\n", symbol, qret)
			return
		}
		if qq == nil {
			errorCount.Inc()
			log.Printf("Empty data from symbol lookup for %s. Assuming not found\n", symbol)
			return
		}

		// ls contains the list of labels and lvs the corresponding values.
		ls := []string{"symbol", "name"}
		lvs := []string{qq.Symbol, qq.ShortName}

		c := ""
		if cached {
			c = " (cached)"
		}
		log.Printf("Retrieved %s (%s), price: %f, volume: %d%s\n",
			qq.Symbol, qq.ShortName, qq.RegularMarketPrice, qq.RegularMarketVolume, c)

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("quotes_exporter_price", "Asset Price.", ls, nil),
			prometheus.GaugeValue,
			qq.RegularMarketPrice,
			lvs...,
		)

		if flagVolume {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("quotes_exporter_volume", "Asset Volume.", ls, nil),
				prometheus.GaugeValue,
				float64(qq.RegularMarketVolume),
				lvs...,
			)
		}
	}
}
