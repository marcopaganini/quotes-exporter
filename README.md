# quotes-exporter

![](https://github.com/marcopaganini/quotes-exporter/workflows/Go/badge.svg)

## ATTENTION: THIS PROJECT IS NOW DEPRECATED

**Q**: Why???

**A**: I couldn't find a financial data provider offering a reasonable amount
of free quota. Some providers have ridiculous limitations (20 requests per
*day*), while others will only allow historical data. In some cases, it would
still be possible to have users register their own accounts and use their own
API keys, but I feel this solution would be confusing and less than
satisfactory. I may resuscitate this project if a reasonable provider surfaces.

---

This is a simple stock and funds quotes exporter for
[prometheus](http://prometheus.io). This exporter allows a prometheus instance
to monitor prices of stocks, ETFs, and mutual funds, possibly alerting the user
on any desirable condition (note: prometheus configuration not covered here.)

## Data Provider Setup

This project uses the [stonks page](https://stonks.scd31.com) to fetch stock
price information. This method **does not** support Mutual Funds, but avoids
the hassle of having to create an API key and quota issues of most financial
API providers.

The program is smart enough to "memoize" calls to the financial data provider
and by default caches quotes for 10m. This should reduce the load on the
finance servers, as prometheus tends to scrape exporters on short time
intervals.

## Building the exporter

To build the exporter, you need a relatively recent version of the [Go
compiler](http://golang.org). Download and install the Go compiler and type the
following commands to download, compile, and install the quotes-exporter binary
to `/usr/local/bin`:

```bash
OLDGOPATH="$GOPATH"
export GOPATH="/tmp/tempgo"
go get -u -t -v github.com/marcopaganini/quotes-exporter
sudo mv $GOPATH/bin/quotes-exporter /usr/local/bin
export GOPATH=$OLDGOPATH
rm -rf /tmp/tempgo
```

## Docker image

The repository includes a ready to use `Dockerfile`. To build a new image, type:

```bash
make image
```

Run `docker images` to see the list of images. The new image is named as
$USER/quotes-exporter and exports port 9340 to your host.

## Running the exporter

To run the exporter, just type:

```base
quotes-exporter
```

The exporter listens on port 9340 by default. You can use the `--port` command-line
flag to change the port number, if necessary.

## Testing

Use your browser to access [localhost:9340](http://localhost:9340). The exporter should display a simple
help page. If that's OK, you can attempt to fetch a stock using something like:

[http://localhost:9340/price?symbols=GOOGL](http://localhost:9340/price?symbols=GOOGL)

The result should be similar to:

```
# HELP quote_exporter_stock_price Asset Price.
# TYPE quote_exporter_stock_price gauge
quote_exporter_stock_price{name="Alphabet Inc.",symbol="GOOGL"} 1333.54
# HELP quotes_exporter_failed_queries_total Count of failed queries
# TYPE quotes_exporter_failed_queries_total counter
quotes_exporter_failed_queries_total 1
# HELP quotes_exporter_queries_total Count of completed queries
# TYPE quotes_exporter_queries_total counter
quotes_exporter_queries_total 5
# HELP quotes_exporter_query_duration_seconds Duration of queries to the upstream API
# TYPE quotes_exporter_query_duration_seconds summary
quotes_exporter_query_duration_seconds_sum 0.000144555
quotes_exporter_query_duration_seconds_count 4
```

## Acknowledgements

I started looking around for a prometheus compatible quotes exporter but
couldn't find anything that satisfied my needs. The closest I found was
[Tristan Colgate-McFarlane](https://github.com/tcolgate)'s [yquotes
exporter](https://github.com/tcolgate/yquotes_exporter), which has stopped
working as Yahoo appears to have deprecated the endpoints required to download
stock data. My thanks to Tristan for his code, which served as the initial
template for this project.
