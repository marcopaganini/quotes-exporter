// (C) 2019 by Marco Paganini <paganini@paganini.net>
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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kofalt/go-memoize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	queryTypes = map[string]int{
		"stock": assetTypeStock,
		"fund":  assetTypeMutualFund,
	}

	// Cache expensive WTD calls for 3 hours.
	cache *memoize.Memoizer = memoize.NewMemoizer(3*time.Hour, 6*time.Hour)

	// flags
	flagPort                  int
	flagWTDToken              string
	flagReadWTDTokenFromStdin bool
)

type collector struct {
	symbols []string
	qtype   int
}

func (c collector) Describe(ch chan<- *prometheus.Desc) {
	// Must send one description, or the registry panics.
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

func (c collector) Collect(ch chan<- prometheus.Metric) {
	queryCount.Inc()

	// Printable list of symbols.
	symparam := strings.Join(c.symbols, ",")

	start := time.Now()
	log.Printf("Looking for %s\n", symparam)
	queryDuration.Observe(float64(time.Since(start).Seconds()))

	cachedGetAssetsFromWTD := func() (interface{}, error) {
		return getAssetsFromWTD(c.symbols, c.qtype)
	}

	assets, err, cached := cache.Memoize(symparam, cachedGetAssetsFromWTD)
	if err != nil {
		errorCount.Inc()
		log.Printf("error looking up %s: %v\n", symparam, err)
		return
	}
	if cached {
		log.Printf("Using cached results for %s", symparam)
	}

	// ls contains the list of labels and lvs the corresponding values.
	ls := []string{"symbol", "name"}

	for _, asset := range assets.([]map[string]string) {
		lvs := []string{asset["symbol"], asset["name"]}

		price, err := strconv.ParseFloat(asset["price"], 64)
		if err != nil {
			errorCount.Inc()
			log.Printf("error converting asset price to numeric %s: %v\n", asset["price"], err)
			continue
		}
		log.Printf("Found %s (%s), price: %f\n", asset["symbol"], asset["name"], price)

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("quote_exporter_price", "Asset Price.", ls, nil),
			prometheus.GaugeValue,
			price,
			lvs...,
		)
	}
}

// getQuote returns information about a stock, mutual fund, or ETF. It uses the
// "type" in the list of symbols prefix to determine the type of securities to
// retrieve.
func getQuote(w http.ResponseWriter, r *http.Request) {
	// qtype is used by Collect to determine which type of securities to fetch.
	qtype, symbols, err := parseQuery(r.URL)
	if err != nil {
		log.Print(err)
		return
	}

	registry := prometheus.NewRegistry()
	collector := &collector{
		symbols: symbols,
		qtype:   qtype,
	}

	// These will be collected every time the /stock or /fund endpoint is reached.
	registry.MustRegister(
		collector,
		queryCount,
		queryDuration,
		errorCount)

	// Delegate http serving to Promethues client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

// parseQuery parses the URL, fetching the Query type by prefix. Returns the
// query type and the list of symbols
func parseQuery(myurl *url.URL) (int, []string, error) {
	// Typical query is formatted as: ?symbols=[type:]symbol,symbol...
	qvalues, ok := myurl.Query()["symbols"]
	if !ok {
		return 0, nil, fmt.Errorf("missing \"symbols\" in query")
	}
	qvalue := qvalues[0]

	for typePrefix, qtype := range queryTypes {
		t := fmt.Sprintf("%s:", typePrefix)
		// Ignore if values are shorter than the query type prefix.
		if len(qvalue) < len(t) {
			continue
		}
		if strings.HasPrefix(qvalue, t) {
			return qtype, strings.Split(qvalue[len(typePrefix)+1:], ","), nil
		}
	}
	// Not found
	return 0, nil, fmt.Errorf("unknown type in query: %s", qvalue)
}

// help returns a help message for those using the root URL.
func help(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Prometheus Quotes Exporter</h1>")
	fmt.Fprintf(w, "<p>To fetch quotes, your URL must be formatted as:</p>")
	fmt.Fprintf(w, "http://localhost:%d/price?symbols=type:AAAA,BBBB,CCCC", flagPort)
	fmt.Fprintf(w, "<p>")
	fmt.Fprintf(w, "The \"type\" designator above could be \"stock\" or \"fund\" to indicate<br>")
	fmt.Fprintf(w, "the symbols following refer to stocks or mutual funds, respectively.</p>")
	fmt.Fprintf(w, "<p><b>Examples:</b></p>")
	fmt.Fprintf(w, "<ul>")

	symbols := []string{
		"stock:AMZN,GOOG,SNAP",
		"fund:VTIAX",
	}

	for _, s := range symbols {
		fmt.Fprintf(w, "<li><a href=\"http://localhost:%d/price?symbols=%s\">", flagPort, s)
		fmt.Fprintf(w, "http://localhost:%d/price?symbols=%s</a></li>", flagPort, s)
	}
}

func main() {
	flag.IntVar(&flagPort, "port", 9340, "Port to listen for HTTP requests.")
	flag.StringVar(&flagWTDToken, "wtdtoken", "", "Token for worldtradingdata.com.")
	flag.BoolVar(&flagReadWTDTokenFromStdin, "read-wtd-token-from-stdin", false, "Read token from stdin (ignore wtdtoken)")
	flag.Parse()

	// Override flagWTDTOken if read from stdin specified (removing newlines).
	if flagReadWTDTokenFromStdin {
		in, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Unable to read token from stdin: %v", err)
		}
		flagWTDToken = strings.TrimRight(string(in), "\n")
	}

	reg := prometheus.NewRegistry()

	// Add standard process and Go metrics.
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)

	// Add handlers.
	http.HandleFunc("/", help)
	http.HandleFunc("/price", getQuote)
	http.Handle("/metrics", promhttp.Handler())

	log.Print("Listening on port ", flagPort)
	http.ListenAndServe(fmt.Sprintf(":%d", flagPort), nil)
}
