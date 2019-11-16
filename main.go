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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	//"github.com/kofalt/go-memoize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Asset types
	assetTypeStock      = iota
	assetTypeMutualFund = iota

	wtdTemplate = "https://api.worldtradingdata.com/api/v1/%s?symbol=%s&api_token=%s"
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

	port     = flag.Int("port", 9977, "Port to listen for HTTP requests.")
	wtdToken = flag.String("wtdtoken", "", "Token for worldtradingdata.com.")

	queryTypes = map[string]int{
		"stocks": assetTypeStock,
		"funds":  assetTypeMutualFund,
	}
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

	assets, err := getAssetsFromWTD(c.symbols, c.qtype)
	if err != nil {
		errorCount.Inc()
		log.Printf("error looking up %s: %v\n", symparam, err)
		return
	}

	// ls contains the list of labels and lvs the corresponding values.
	ls := []string{"symbol", "name"}

	for _, asset := range assets {
		lvs := []string{asset["symbol"], asset["name"]}

		price, err := strconv.ParseFloat(asset["price"], 64)
		if err != nil {
			errorCount.Inc()
			log.Printf("error converting asset price to numeric %s: %v\n", asset["price"], err)
			continue
		}
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

// getAssetsFromWTD retrieves asset (stock, mutualfunds) data about symbols and
// returns a slice of maps containing a list of key/value attributes from wtd
// for each of the symbols. Asset type (atype) should represent the type of
// asset to retrieve (assetTypeStock, assetTypeMutualFund.)
func getAssetsFromWTD(symbols []string, atype int) ([]map[string]string, error) {
	var (
		webdata map[string]interface{}
		query   string
	)

	switch atype {
	case assetTypeStock:
		query = "stock"
	case assetTypeMutualFund:
		query = "mutualfund"
	default:
		return nil, fmt.Errorf("invalid query type")
	}

	wtdurl := fmt.Sprintf(wtdTemplate, query, strings.Join(symbols, ","), *wtdToken)
	resp, err := http.Get(wtdurl)
	if err != nil {
		return nil, err
	}

	jdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jdata, &webdata); err != nil {
		return nil, fmt.Errorf("unable to decode json: %v", jdata)
	}

	// If we don't have a "data" array in the json return, something went wrong. WTD
	// returns 200 even on API errors (and sets the error message on the json itself)
	if webdata["data"] == nil {
		return nil, fmt.Errorf("invalid json response: %v", string(jdata))
	}

	ret := []map[string]string{}

	// The answer from WTD is formatted as:
	// {
	//   "symbols_requested": 3,
	//   "symbols_returned": 3,
	//   "data": [
	//     {
	//       "symbol": "FOO",
	//       "name": "Foo Inc.",
	//       "price": "14.38",
	//       (...)
	//     }
	//     {
	//       "symbol": "BAR",
	//       "name": "Bar Inc.",
	//       "price": "16.66",
	//       (...)
	//     }
	//   ]
	// }

	data := webdata["data"].([]interface{})
	for _, d := range data {
		item := map[string]string{}
		kv := d.(map[string]interface{})
		for k, v := range kv {
			item[k] = v.(string)
		}
		ret = append(ret, item)
	}

	return ret, nil
}

// help returns a help message for those using the root URL.
func help(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Prometheus Quotes Exporter</h1>")
	fmt.Fprintf(w, "<p>Use the following examples to retrieve quotes:</p>")
	for q := range queryTypes {
		fmt.Fprintf(w, "<a href=\"http://localhost:%d/%s?symbols=AAAA,BBBB,CCCC\">", *port, q)
		fmt.Fprintf(w, "http://localhost:%d/%s?symbol=AAAA,BBBB,DDDD</a>\n", *port, q)
		fmt.Fprintf(w, "<br>")
	}
	fmt.Fprintf(w, "<p>Replace symbols above by the desired stock or mutual fund symbols.</p>")
}

func main() {
	flag.Parse()

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

	log.Print("Listening on port ", *port)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}
