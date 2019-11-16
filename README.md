# quotes-exporter

This is a simple stock and funds quotes exporter for
[prometheus](http://prometheus.io). This exporter allows a prometheus instance
to monitor prices for stocks, ETFs, and mutual funds, possibly alerting the
user on any desirable condition (note: prometheus configuration not covered here.)

## Data Provider Setup

Unlike similar projects, which attempt to scrape data from various websites,
with different degrees of success, quotes-exporter uses data from [World
Trading Data](worldtradingdata.com), a free (up to 250 queries per day)
provider of financial quote data. Create a free account and annotate your API
token. Guard your token carefully as anyone with it can exhaust your free daily
quota.

The program is smart enough to "memoize" calls to worldtradingdata (by default
at most one call for each set of symbols every 3 hours). This should prevent
accidental quota exhaustion, as prometheus tends to scrape exporters on
aggressive time intervals.

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

Please note that we remove the temporary GOPATH using by repeating the
directory name instead of referencing $GOPATH. This should prevent accidental
removal of the real GOPATH.

## Running the exporter

To run the exporter, just type:

```
echo "your_token" | quotes-exporter --read-wtd-token-from-stdin
```

Make sure to replace "your token" above with the real token from [World Trading
Data](worldtradingdata.com).

The exporter listens on port 9977 by default. You can use the `--port` command-line
flag to change the port number, if necessary.

## Testing

Use your browser to access [localhost:9977](http://localhost:9977). The exporter should display a simple
help page. If that's OK, you can attempt to fetch a stock using something like:

[http://localhost:9977/stocks?symbols=GOOGL](http://localhost:9977/stocks?symbols=GOOGL)

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
