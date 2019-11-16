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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	// Asset types
	assetTypeStock      = iota
	assetTypeMutualFund = iota

	wtdTemplate = "https://api.worldtradingdata.com/api/v1/%s?symbol=%s&api_token=%s"
)

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
