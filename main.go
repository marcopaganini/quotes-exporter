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
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// priceHandler handles the "/price" endpoint. It creates a new collector with
// the URL and a new prometheus registry to use that collector.
func priceHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("URL: %s\n", r.RequestURI)

	collector, err := newCollector(r.URL)
	if err != nil {
		log.Print(err)
		return
	}

	registry := prometheus.NewRegistry()

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

// help returns a help message for those using the root URL.
func help(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Prometheus Quotes Exporter</h1>")
	fmt.Fprintf(w, "<p>To fetch quotes, your URL must be formatted as:</p>")
	fmt.Fprintf(w, "http://localhost:%d/price?symbols=AAAA,BBBB,CCCC", flagPort)
	fmt.Fprintf(w, "<p><b>Examples:</b></p>")
	fmt.Fprintf(w, "<ul>")

	symbols := []string{
		"AMZN,GOOG,SNAP",
		"VTIAX",
	}

	for _, s := range symbols {
		fmt.Fprintf(w, "<li><a href=\"http://localhost:%d/price?symbols=%s\">", flagPort, s)
		fmt.Fprintf(w, "http://localhost:%d/price?symbols=%s</a></li>", flagPort, s)
	}
}

func main() {
	flag.IntVar(&flagPort, "port", 9340, "Port to listen for HTTP requests.")
	flag.BoolVar(&flagVolume, "quote.volume", false, "Exports volume.")
	flag.Parse()

	reg := prometheus.NewRegistry()

	// Add standard process and Go metrics.
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)

	// Add handlers.
	http.HandleFunc("/", help)
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/price", func(w http.ResponseWriter, r *http.Request) {
		priceHandler(w, r)
	})

	log.Print("Listening on port ", flagPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", flagPort), nil))
}
