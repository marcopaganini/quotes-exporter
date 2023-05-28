// (C) 2023 by Marco Paganini <paganini@paganini.net>
//
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
// License for the specific language governing permissions and limitations
// under the License.

package stonks

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	stonksURL = "https://stonks.scd31.com/%s?f=i3"
)

// Quote returns the current value of a symbol.
func Quote(symbol string) (float64, error) {
	symbol = strings.ToUpper(symbol)

	resp, err := http.Get(fmt.Sprintf(stonksURL, symbol))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Parse first line of output from remote. Sample (for AMD):
	// AMD: $127.03 +5.55%

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	// Remove DOS CRLF cruft from output.
	result := strings.Split(string(body), "\n")[0]
	result = strings.TrimRight(result, "\r\n")
	log.Printf("Results from scd31: %+v", result)

	if result == "" {
		return 0, fmt.Errorf("empty results from upstream: %v", result)
	}
	if !strings.HasPrefix(result, symbol+":") {
		return 0, fmt.Errorf("missing symbol name on output (invalid symbol?): %v", result)
	}

	// Parse price from input.
	tok := strings.Split(result, " ")
	if len(tok) < 2 {
		return 0, fmt.Errorf("error parsing quote results: %v", result)
	}
	strval := strings.Trim(tok[1], "$,")

	val, err := strconv.ParseFloat(strval, 64)
	if err != nil {
		return 0, err
	}
	if val == 0 {
		return 0, fmt.Errorf("query returned price=0: %v", result)
	}
	return val, nil
}
