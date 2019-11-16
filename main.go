// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
		"stocks": assetTypeStock,
		"funds":  assetTypeMutualFund,
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
			prometheus.NewDesc("quote_exporter_stock_price", "Asset Price.", ls, nil),
			prometheus.GaugeValue,
			price,
			lvs...,
		)
	}
}

// getQuote returns information about a stock, mutual fund, or ETF. It uses the
// path in the URL to determine the type of security we need to retrieve.
func getQuote(w http.ResponseWriter, r *http.Request) {
	// Get query type from URL. The collector uses this field to decide if we
	// want to fetch the prices of stocks or funds.
	qtype, err := queryType(r.URL.Path)
	if err != nil {
		log.Print(err)
		return
	}

	symbols, ok := r.URL.Query()["symbols"]
	if !ok {
		log.Printf("Missing symbols in query.")
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

// queryType parses the URL Path and returns the correct query type or error if it can't
// determine the type.
func queryType(path string) (int, error) {
	// Parse URL looking for /query?
	for query, qtype := range queryTypes {
		q := fmt.Sprintf("/%s", query)
		// Ignore short paths.
		if len(path) < len(q) {
			continue
		}
		if strings.HasPrefix(path, q) {
			return qtype, nil
		}
	}
	// Not found
	return 0, fmt.Errorf("unknown path: %s", path)
}

// help returns a help message for those using the root URL.
func help(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Prometheus Quotes Exporter</h1>")
	fmt.Fprintf(w, "<p>Use the following examples to retrieve quotes:</p>")
	for q := range queryTypes {
		fmt.Fprintf(w, "<a href=\"http://localhost:%d/%s?symbols=AAAA,BBBB,CCCC\">", flagPort, q)
		fmt.Fprintf(w, "http://localhost:%d/%s?symbol=AAAA,BBBB,DDDD</a>\n", flagPort, q)
		fmt.Fprintf(w, "<br>")
	}
	fmt.Fprintf(w, "<p>Replace symbols above by the desired stock or mutual fund symbols.</p>")
}

func main() {
	flag.IntVar(&flagPort, "port", 9977, "Port to listen for HTTP requests.")
	flag.StringVar(&flagWTDToken, "wtdtoken", "", "Token for worldtradingdata.com.")
	flag.BoolVar(&flagReadWTDTokenFromStdin, "read-wtd-token-from-stdin", false, "Read token from stdin (ignore wtdtoken)")
	flag.Parse()

	// Override flagWTDTOken if read from stdin specified (removing newlines).
	if flagReadWTDTokenFromStdin {
		in, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalln("Unable to read token from stdin: %v", err)
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
	for q := range queryTypes {
		http.HandleFunc(fmt.Sprintf("/%s", q), getQuote)
	}
	http.HandleFunc("/", help)
	http.Handle("/metrics", promhttp.Handler())

	log.Print("Listening on port ", flagPort)
	http.ListenAndServe(fmt.Sprintf(":%d", flagPort), nil)
}
