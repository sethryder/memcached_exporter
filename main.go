package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Snapbug/gomemcache/memcache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "memcached"
)

// Exporter collects metrics from a memcached server.
type Exporter struct {
	mc *memcache.Client

	up                    *prometheus.Desc
	uptime                *prometheus.Desc
	version               *prometheus.Desc
	bytesRead             *prometheus.Desc
	bytesWritten          *prometheus.Desc
	currentConnections    *prometheus.Desc
	maxConnections        *prometheus.Desc
	connectionsTotal      *prometheus.Desc
	currentBytes          *prometheus.Desc
	limitBytes            *prometheus.Desc
	commands              *prometheus.Desc
	items                 *prometheus.Desc
	itemsTotal            *prometheus.Desc
	evictions             *prometheus.Desc
	reclaimed             *prometheus.Desc
	malloced              *prometheus.Desc
	itemsNumber           *prometheus.Desc
	itemsAge              *prometheus.Desc
	itemsCrawlerReclaimed *prometheus.Desc
	itemsEvicted          *prometheus.Desc
	itemsEvictedNonzero   *prometheus.Desc
	itemsEvictedTime      *prometheus.Desc
	itemsEvictedUnfetched *prometheus.Desc
	itemsExpiredUnfetched *prometheus.Desc
	itemsOutofmemory      *prometheus.Desc
	itemsReclaimed        *prometheus.Desc
	itemsTailrepairs      *prometheus.Desc
	slabsChunkSize        *prometheus.Desc
	slabsChunksPerPage    *prometheus.Desc
	slabsCurrentPages     *prometheus.Desc
	slabsCurrentChunks    *prometheus.Desc
	slabsChunksUsed       *prometheus.Desc
	slabsChunksFree       *prometheus.Desc
	slabsChunksFreeEnd    *prometheus.Desc
	slabsMemRequested     *prometheus.Desc
	slabsCommands         *prometheus.Desc
}

// NewExporter returns an initialized exporter.
func NewExporter(server string, timeout time.Duration) *Exporter {
	c := memcache.New(server)
	c.Timeout = timeout

	return &Exporter{
		mc: c,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the memcached server be reached.",
			nil,
			prometheus.Labels{"server": server},
		),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uptime_seconds"),
			"Number of seconds since the server started.",
			nil,
			prometheus.Labels{"server": server},
		),
		version: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "version"),
			"The version of this memcached server.",
			[]string{"version"},
			prometheus.Labels{"server": server},
		),
		bytesRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "read_bytes_total"),
			"Total number of bytes read by this server from network.",
			nil,
			prometheus.Labels{"server": server},
		),
		bytesWritten: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "written_bytes_total"),
			"Total number of bytes sent by this server to network.",
			nil,
			prometheus.Labels{"server": server},
		),
		currentConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "current_connections"),
			"Current number of open connections.",
			nil,
			prometheus.Labels{"server": server},
		),
		maxConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "max_connections"),
			"Maximum number of clients allowed.",
			nil,
			prometheus.Labels{"server": server},
		),
		connectionsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "connections_total"),
			"Total number of connections opened since the server started running.",
			nil,
			prometheus.Labels{"server": server},
		),
		currentBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "current_bytes"),
			"Current number of bytes used to store items.",
			nil,
			prometheus.Labels{"server": server},
		),
		limitBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "limit_bytes"),
			"Number of bytes this server is allowed to use for storage.",
			nil,
			prometheus.Labels{"server": server},
		),
		commands: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "commands_total"),
			"Total number of all requests broken down by command (get, set, etc.) and status.",
			[]string{"command", "status"},
			prometheus.Labels{"server": server},

		),
		items: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "current_items"),
			"Current number of items stored by this instance.",
			nil,
			prometheus.Labels{"server": server},
		),
		itemsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "items_total"),
			"Total number of items stored during the life of this instance.",
			nil,
			prometheus.Labels{"server": server},
		),
		evictions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "items_evicted_total"),
			"Total number of valid items removed from cache to free memory for new items.",
			nil,
			prometheus.Labels{"server": server},
		),
		reclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "items_reclaimed_total"),
			"Total number of times an entry was stored using memory from an expired entry.",
			nil,
			prometheus.Labels{"server": server},
		),
		malloced: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "malloced_bytes"),
			"Number of bytes of memory allocated to slab pages.",
			nil,
			prometheus.Labels{"server": server},
		),
		itemsNumber: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "current_items"),
			"Number of items currently stored in this slab class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsAge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_age_seconds"),
			"Number of seconds the oldest item has been in the slab class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsCrawlerReclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_crawler_reclaimed_total"),
			"Total number of items freed by the LRU Crawler.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsEvicted: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_evicted_total"),
			"Total number of times an item had to be evicted from the LRU before it expired.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsEvictedNonzero: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_evicted_nonzero_total"),
			"Total number of times an item which had an explicit expire time set had to be evicted from the LRU before it expired.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsEvictedTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_evicted_time_seconds"),
			"Seconds since the last access for the most recent item evicted from this class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsEvictedUnfetched: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_evicted_unfetched_total"),
			"Total nmber of items evicted and never fetched.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsExpiredUnfetched: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_expired_unfetched_total"),
			"Total number of valid items evicted from the LRU which were never touched after being set.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsOutofmemory: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_outofmemory_total"),
			"Total number of items for this slab class that have triggered an out of memory error.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsReclaimed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_reclaimed_total"),
			"Total number of items reclaimed.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		itemsTailrepairs: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "items_tailrepairs_total"),
			"Total number of times the entries for a particular ID need repairing.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsChunkSize: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "chunk_size_bytes"),
			"Number of bytes allocated to each chunk within this slab class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsChunksPerPage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "chunks_per_page"),
			"Number of chunks within a single page for this slab class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsCurrentPages: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "current_pages"),
			"Number of pages allocated to this slab class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsCurrentChunks: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "current_chunks"),
			"Number of chunks allocated to this slab class.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsChunksUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "chunks_used"),
			"Number of chunks allocated to an item.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsChunksFree: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "chunks_free"),
			"Number of chunks not yet allocated items.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsChunksFreeEnd: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "chunks_free_end"),
			"Number of free chunks at the end of the last allocated page.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsMemRequested: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "mem_requested_bytes"),
			"Number of bytes of memory actual items take up within a slab.",
			[]string{"slab"},
			prometheus.Labels{"server": server},
		),
		slabsCommands: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "slab", "commands_total"),
			"Total number of all requests broken down by command (get, set, etc.) and status per slab.",
			[]string{"slab", "command", "status"},
			prometheus.Labels{"server": server},
		),
	}
}

// Describe describes all the metrics exported by the memcached exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.uptime
	ch <- e.version
	ch <- e.bytesRead
	ch <- e.bytesWritten
	ch <- e.currentConnections
	ch <- e.maxConnections
	ch <- e.connectionsTotal
	ch <- e.currentBytes
	ch <- e.limitBytes
	ch <- e.commands
	ch <- e.items
	ch <- e.itemsTotal
	ch <- e.evictions
	ch <- e.reclaimed
	ch <- e.malloced
	ch <- e.itemsNumber
	ch <- e.itemsAge
	ch <- e.itemsCrawlerReclaimed
	ch <- e.itemsEvicted
	ch <- e.itemsEvictedNonzero
	ch <- e.itemsEvictedTime
	ch <- e.itemsEvictedUnfetched
	ch <- e.itemsExpiredUnfetched
	ch <- e.itemsOutofmemory
	ch <- e.itemsReclaimed
	ch <- e.itemsTailrepairs
	ch <- e.itemsExpiredUnfetched
	ch <- e.slabsChunkSize
	ch <- e.slabsChunksPerPage
	ch <- e.slabsCurrentPages
	ch <- e.slabsCurrentChunks
	ch <- e.slabsChunksUsed
	ch <- e.slabsChunksFree
	ch <- e.slabsChunksFreeEnd
	ch <- e.slabsMemRequested
	ch <- e.slabsCommands
}

// Collect fetches the statistics from the configured memcached server, and
// delivers them as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	stats, err := e.mc.Stats()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		log.Errorf("Failed to collect stats from memcached: %s", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)

	// TODO(ts): Clean up and consolidate metric mappings.
	itemsMetrics := map[string]*prometheus.Desc{
		"crawler_reclaimed": e.itemsCrawlerReclaimed,
		"evicted":           e.itemsEvicted,
		"evicted_nonzero":   e.itemsEvictedNonzero,
		"evicted_time":      e.itemsEvictedTime,
		"evicted_unfetched": e.itemsEvictedUnfetched,
		"expired_unfetched": e.itemsExpiredUnfetched,
		"outofmemory":       e.itemsOutofmemory,
		"reclaimed":         e.itemsReclaimed,
		"tailrepairs":       e.itemsTailrepairs,
	}

	for _, t := range stats {
		s := t.Stats
		ch <- prometheus.MustNewConstMetric(e.uptime, prometheus.CounterValue, parse(s, "uptime"))
		ch <- prometheus.MustNewConstMetric(e.version, prometheus.GaugeValue, 1, s["version"])

		for _, op := range []string{"get", "delete", "incr", "decr", "cas", "touch"} {
			ch <- prometheus.MustNewConstMetric(e.commands, prometheus.CounterValue, parse(s, op+"_hits"), op, "hit")
			ch <- prometheus.MustNewConstMetric(e.commands, prometheus.CounterValue, parse(s, op+"_misses"), op, "miss")
		}
		ch <- prometheus.MustNewConstMetric(e.commands, prometheus.CounterValue, parse(s, "cas_badval"), "cas", "badval")
		ch <- prometheus.MustNewConstMetric(e.commands, prometheus.CounterValue, parse(s, "cmd_flush"), "flush", "hit")

		// memcached includes cas operations again in cmd_set.
		set := math.NaN()
		if setCmd, err := strconv.ParseFloat(s["cmd_set"], 64); err == nil {
			if cas, casErr := sum(s, "cas_misses", "cas_hits", "cas_badval"); casErr == nil {
				set = setCmd - cas
			} else {
				log.Errorf("Failed to parse cas: %s", casErr)
			}
		} else {
			log.Errorf("Failed to parse set %q: %s", s["cmd_set"], err)
		}
		ch <- prometheus.MustNewConstMetric(e.commands, prometheus.CounterValue, set, "set", "hit")

		ch <- prometheus.MustNewConstMetric(e.currentBytes, prometheus.GaugeValue, parse(s, "bytes"))
		ch <- prometheus.MustNewConstMetric(e.limitBytes, prometheus.GaugeValue, parse(s, "limit_maxbytes"))
		ch <- prometheus.MustNewConstMetric(e.items, prometheus.GaugeValue, parse(s, "curr_items"))
		ch <- prometheus.MustNewConstMetric(e.itemsTotal, prometheus.CounterValue, parse(s, "total_items"))

		ch <- prometheus.MustNewConstMetric(e.bytesRead, prometheus.CounterValue, parse(s, "bytes_read"))
		ch <- prometheus.MustNewConstMetric(e.bytesWritten, prometheus.CounterValue, parse(s, "bytes_written"))

		ch <- prometheus.MustNewConstMetric(e.currentConnections, prometheus.GaugeValue, parse(s, "curr_connections"))
		ch <- prometheus.MustNewConstMetric(e.connectionsTotal, prometheus.CounterValue, parse(s, "total_connections"))

		ch <- prometheus.MustNewConstMetric(e.evictions, prometheus.CounterValue, parse(s, "evictions"))
		ch <- prometheus.MustNewConstMetric(e.reclaimed, prometheus.CounterValue, parse(s, "reclaimed"))

		ch <- prometheus.MustNewConstMetric(e.malloced, prometheus.GaugeValue, parse(s, "total_malloced"))

		for slab, u := range t.Items {
			slab := strconv.Itoa(slab)
			ch <- prometheus.MustNewConstMetric(e.itemsNumber, prometheus.GaugeValue, parse(u, "number"), slab)
			ch <- prometheus.MustNewConstMetric(e.itemsAge, prometheus.GaugeValue, parse(u, "age"), slab)
			for m, d := range itemsMetrics {
				if _, ok := u[m]; !ok {
					continue
				}
				ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, parse(u, m), slab)
			}
		}

		for slab, v := range t.Slabs {
			slab := strconv.Itoa(slab)

			for _, op := range []string{"get", "delete", "incr", "decr", "cas", "touch"} {
				ch <- prometheus.MustNewConstMetric(e.slabsCommands, prometheus.CounterValue, parse(v, op+"_hits"), slab, op, "hit")
			}
			ch <- prometheus.MustNewConstMetric(e.slabsCommands, prometheus.CounterValue, parse(v, "cas_badval"), slab, "cas", "badval")

			slabSet := math.NaN()
			if slabSetCmd, err := strconv.ParseFloat(v["cmd_set"], 64); err == nil {
				if slabCas, slabCasErr := sum(v, "cas_hits", "cas_badval"); slabCasErr == nil {
					slabSet = slabSetCmd - slabCas
				} else {
					log.Errorf("Failed to parse cas: %s", slabCasErr)
				}
			} else {
				log.Errorf("Failed to parse set %q: %s", v["cmd_set"], err)
			}
			ch <- prometheus.MustNewConstMetric(e.slabsCommands, prometheus.CounterValue, slabSet, slab, "set", "hit")

			ch <- prometheus.MustNewConstMetric(e.slabsChunkSize, prometheus.GaugeValue, parse(v, "chunk_size"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsChunksPerPage, prometheus.GaugeValue, parse(v, "chunks_per_page"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsCurrentPages, prometheus.GaugeValue, parse(v, "total_pages"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsCurrentChunks, prometheus.GaugeValue, parse(v, "total_chunks"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsChunksUsed, prometheus.GaugeValue, parse(v, "used_chunks"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsChunksFree, prometheus.GaugeValue, parse(v, "free_chunks"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsChunksFreeEnd, prometheus.GaugeValue, parse(v, "free_chunks_end"), slab)
			ch <- prometheus.MustNewConstMetric(e.slabsMemRequested, prometheus.GaugeValue, parse(v, "mem_requested"), slab)
		}
	}

	statsSettings, err := e.mc.StatsSettings()
	if err != nil {
		log.Errorf("Could not query stats settings: %s", err)
	}
	for _, settings := range statsSettings {
		ch <- prometheus.MustNewConstMetric(e.maxConnections, prometheus.GaugeValue, parse(settings, "maxconns"))
	}
}

func parse(stats map[string]string, key string) float64 {
	v, err := strconv.ParseFloat(stats[key], 64)
	if err != nil {
		log.Errorf("Failed to parse %s %q: %s", key, stats[key], err)
		v = math.NaN()
	}
	return v
}

func sum(stats map[string]string, keys ...string) (float64, error) {
	s := 0.
	for _, key := range keys {
		v, err := strconv.ParseFloat(stats[key], 64)
		if err != nil {
			return math.NaN(), err
		}
		s += v
	}
	return s, nil
}

func main() {
	var (
		servers       = kingpin.Flag("memcached.servers", "Memcached server(s) address.").Default("localhost:11211").String()
		timeout       = kingpin.Flag("memcached.timeout", "memcached connect timeout.").Default("1s").Duration()
		pidFile       = kingpin.Flag("memcached.pid-file", "Optional path to a file containing the memcached PID for additional metrics.").Default("").String()
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9150").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		server_arr 		[]string
	)
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("memcached_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting memcached_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	server_arr = strings.Split(*servers, ",")
	if len(server_arr) == 0 {
		server_arr = []string{"localhost:11211"}
	}
	for _, s := range server_arr {
		prometheus.MustRegister(NewExporter(s, *timeout))
	}

	if *pidFile != "" {
		procExporter := prometheus.NewProcessCollectorPIDFn(
			func() (int, error) {
				content, err := ioutil.ReadFile(*pidFile)
				if err != nil {
					return 0, fmt.Errorf("Can't read pid file %q: %s", *pidFile, err)
				}
				value, err := strconv.Atoi(strings.TrimSpace(string(content)))
				if err != nil {
					return 0, fmt.Errorf("Can't parse pid file %q: %s", *pidFile, err)
				}
				return value, nil
			}, namespace)
		prometheus.MustRegister(procExporter)
	}

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Memcached Exporter</title></head>
             <body>
             <h1>Memcached Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Infoln("Starting HTTP server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
