/*
Copyright 2019 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package discovery provides a way to discover all tablets e.g. within a
// specific shard and monitor their current health.
//
// Use the HealthCheck object to query for tablets and their health.
//
// For an example how to use the HealthCheck object, see worker/topo_utils.go.
//
// Tablets have to be manually added to the HealthCheck using AddTablet().
// Alternatively, use a Watcher implementation which will constantly watch
// a source (e.g. the topology) and add and remove tablets as they are
// added or removed from the source.
// For a Watcher example have a look at NewShardReplicationWatcher().
//
// tabletStatsCache is one implementation, that caches the known tablets
// and the healthy ones per keyspace/shard/tabletType.
//
// Internally, the HealthCheck module is connected to each tablet and has a
// streaming RPC (StreamHealth) open to receive periodic health infos.
package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"html/template"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"vitess.io/vitess/go/flagutil"

	vtrpcpb "vitess.io/vitess/go/vt/proto/vtrpc"
	"vitess.io/vitess/go/vt/vterrors"

	"vitess.io/vitess/go/vt/topo"

	"github.com/golang/protobuf/proto"
	"vitess.io/vitess/go/stats"
	"vitess.io/vitess/go/sync2"
	"vitess.io/vitess/go/vt/grpcclient"
	"vitess.io/vitess/go/vt/log"
	"vitess.io/vitess/go/vt/topo/topoproto"
	"vitess.io/vitess/go/vt/topotools"
	"vitess.io/vitess/go/vt/vttablet/queryservice"
	"vitess.io/vitess/go/vt/vttablet/tabletconn"

	querypb "vitess.io/vitess/go/vt/proto/query"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

var (
	hcErrorCounters          = stats.NewCountersWithMultiLabels("HealthcheckErrors", "Healthcheck Errors", []string{"Keyspace", "ShardName", "TabletType"})
	hcMasterPromotedCounters = stats.NewCountersWithMultiLabels("HealthcheckMasterPromoted", "Master promoted in keyspace/shard name because of health check errors", []string{"Keyspace", "ShardName"})
	healthcheckOnce          sync.Once

	// TabletURLTemplateString is a flag to generate URLs for the tablets that vtgate discovers.
	TabletURLTemplateString = flag.String("tablet_url_template", "http://{{.GetTabletHostPort}}", "format string describing debug tablet url formatting. See the Go code for getTabletDebugURL() how to customize this.")
	tabletURLTemplate       *template.Template

	//TODO(deepthi): change these vars back to unexported when discoveryGateway is removed

	// CellsToWatch is the list of cells this healthcheck operates over
	CellsToWatch = flag.String("cells_to_watch", "", "comma-separated list of cells for watching tablets")
	// AllowedTabletTypes is the list of allowed tablet types. e.g. {MASTER, REPLICA}
	AllowedTabletTypes []topodatapb.TabletType
	// TabletFilters are the keyspace|shard or keyrange filters to apply to the full set of tablets
	TabletFilters flagutil.StringListValue
	// KeyspacesToWatch - if provided this specifies which keyspaces should be
	// visible to a vtgate. By default the vtgate will allow access to any
	// keyspace.
	KeyspacesToWatch flagutil.StringListValue
	// RefreshInterval is the interval at which healthcheck refreshes its list of tablets from topo
	RefreshInterval = flag.Duration("tablet_refresh_interval", 1*time.Minute, "tablet refresh interval")
	// RefreshKnownTablets tells us whether to process all tablets or only new tablets
	RefreshKnownTablets = flag.Bool("tablet_refresh_known_tablets", true, "tablet refresh reloads the tablet address/port map from topo in case it changes")
	// TopoReadConcurrency tells us how many topo reads are allowed in parallel
	TopoReadConcurrency = flag.Int("topo_read_concurrency", 32, "concurrent topo reads")
)

// See the documentation for NewHealthCheck below for an explanation of these parameters.
const (
	DefaultHealthCheckRetryDelay = 5 * time.Second
	DefaultHealthCheckTimeout    = 1 * time.Minute

	// DefaultTopoReadConcurrency can be used as default value for the TopoReadConcurrency parameter of a TopologyWatcher.
	DefaultTopoReadConcurrency int = 5
	// DefaultTopologyWatcherRefreshInterval can be used as the default value for
	// the refresh interval of a topology watcher.
	DefaultTopologyWatcherRefreshInterval = 1 * time.Minute
	// HealthCheckTemplate is the HTML code to display a TabletsCacheStatusList
	HealthCheckTemplate = `
<style>
  table {
    border-collapse: collapse;
  }
  td, th {
    border: 1px solid #999;
    padding: 0.2rem;
  }
</style>
<table>
  <tr>
    <th colspan="5">HealthCheck Tablet Cache</th>
  </tr>
  <tr>
    <th>Cell</th>
    <th>Keyspace</th>
    <th>Shard</th>
    <th>TabletType</th>
    <th>tabletHealth</th>
  </tr>
  {{range $i, $ts := .}}
  <tr>
    <td>{{github_com_vitessio_vitess_vtctld_srv_cell $ts.Cell}}</td>
    <td>{{github_com_vitessio_vitess_vtctld_srv_keyspace $ts.Cell $ts.Target.Keyspace}}</td>
    <td>{{$ts.Target.Shard}}</td>
    <td>{{$ts.Target.TabletType}}</td>
    <td>{{$ts.StatusAsHTML}}</td>
  </tr>
  {{end}}
</table>
`
)

// ParseTabletURLTemplateFromFlag loads or reloads the URL template.
func ParseTabletURLTemplateFromFlag() {
	tabletURLTemplate = template.New("")
	_, err := tabletURLTemplate.Parse(*TabletURLTemplateString)
	if err != nil {
		log.Exitf("error parsing template: %v", err)
	}
}

func init() {
	// Flags are not parsed at this point and the default value of the flag (just the hostname) will be used.
	ParseTabletURLTemplateFromFlag()
	flag.Var(&TabletFilters, "tablet_filters", "Specifies a comma-separated list of 'keyspace|shard_name or keyrange' values to filter the tablets to watch")
	topoproto.TabletTypeListVar(&AllowedTabletTypes, "allowed_tablet_types", "Specifies the tablet types this vtgate is allowed to route queries to")
	flag.Var(&KeyspacesToWatch, "keyspaces_to_watch", "Specifies which keyspaces this vtgate should have access to while routing queries or accessing the vschema")
}

// HealthCheck defines the interface of health checking module.
// The goal of this object is to maintain a StreamHealth RPC
// to a lot of tablets. Tablets are added / removed by calling the
// AddTablet / RemoveTablet methods (other discovery module objects
// can for instance watch the topology and call these).
type HealthCheck interface {
	// AddTablet adds the tablet.
	// Name is an alternate name, like an address.
	AddTablet(tablet *topodatapb.Tablet)

	// RemoveTablet removes the tablet.
	RemoveTablet(tablet *topodatapb.Tablet)

	// ReplaceTablet does an AddTablet and RemoveTablet in one call, effectively replacing the old tablet with the new.
	ReplaceTablet(old, new *topodatapb.Tablet)
	// RegisterStats registers the connection counts and checksum stats.
	// It can only be called on one Healthcheck object per process.
	RegisterStats()
	// CacheStatus returns a displayable version of the cache.
	CacheStatus() TabletsCacheStatusList
	// Close stops the healthcheck.
	Close() error
	// GetTabletAndConnection gets a tablet and connection to execute a query on
	GetTabletAndConnection(target *querypb.Target, localCell string, invalidTablets map[string]bool) (string, queryservice.QueryService, error)
	// WaitForAllServingTablets
	WaitForAllServingTablets(ctx context.Context, targets []*querypb.Target) error
}

// HealthCheckImpl performs health checking and notifies downstream components about any changes.
// It contains a map of TabletHealth objects, each of which stores the health information for
// a tablet. A checkConn goroutine is spawned for each TabletHealth, which is responsible for
// keeping that TabletHealth up-to-date. This is done through callbacks to updateHealth.
// If checkConn terminates for any reason, it updates TabletHealth.Up as false. If a TabletHealth
// gets removed from the map, its cancelFunc gets called, which ensures that the associated
// checkConn goroutine eventually terminates.
type HealthCheckImpl struct {
	// Immutable fields set at construction time.
	retryDelay         time.Duration
	healthCheckTimeout time.Duration
	ts                 *topo.Server
	cell               string
	// mu protects all the following fields.
	mu sync.Mutex

	// TODO(deepthi): verify all access to following fields is actually being protected by mu
	// if not needed, move them up

	// a map keyed by keyspace.shard.tabletType
	// contains a map of tabletHealth keyed by tablet alias for each tablet relevant to the keyspace.shard.tabletType
	// TODO should we include cell in key?
	entries map[string]map[string]*tabletHealth

	// connsWG keeps track of all launched Go routines that monitor tablet connections.
	connsWG sync.WaitGroup

	topoWatchers []*TopologyWatcher

	// cellAliases is a cache of cell aliases
	cellAliases map[string]string
}

// HealthCheckConn is a structure that lives within the scope of
// the checkConn goroutine to maintain its internal state. Therefore,
// it does not require synchronization. Changes that are relevant to
// healthcheck are transmitted through calls to HealthCheckImpl.updateHealth.
type healthCheckConn struct {
	ctx context.Context

	tabletHealth          *tabletHealth
	loggedServingState    bool
	lastResponseTimestamp time.Time // timestamp of the last healthcheck response
}

// NewHealthCheck creates a new HealthCheck object.
// Parameters:
// retryDelay.
//   The duration to wait before retrying to connect (e.g. after a failed connection
//   attempt).
// healthCheckTimeout.
//   The duration for which we consider a health check response to be 'fresh'. If we don't get
//   a health check response from a tablet for more than this duration, we consider the tablet
//   not healthy.
func NewHealthCheck(ctx context.Context, retryDelay, healthCheckTimeout time.Duration, topoServer *topo.Server, localCell string) HealthCheck {
	log.Infof("loading tablets for cells: %v", *CellsToWatch)

	var topoWatchers []*TopologyWatcher
	var filter TabletFilter
	cells := strings.Split(*CellsToWatch, ",")
	if len(cells) == 0 {
		cells = append(cells, localCell)
	}
	for _, c := range cells {
		if c == "" {
			continue
		}
		if len(TabletFilters) > 0 {
			if len(KeyspacesToWatch) > 0 {
				log.Exitf("Only one of -keyspaces_to_watch and -tablet_filters may be specified at a time")
			}

			fbs, err := NewFilterByShard(TabletFilters)
			if err != nil {
				log.Exitf("Cannot parse tablet_filters parameter: %v", err)
			}
			filter = fbs
		} else if len(KeyspacesToWatch) > 0 {
			filter = NewFilterByKeyspace(c, KeyspacesToWatch)
		}
		topoWatchers = append(topoWatchers, NewCellTabletsWatcher(ctx, topoServer, filter, c, *RefreshInterval, *RefreshKnownTablets, *TopoReadConcurrency))
	}

	hc := &HealthCheckImpl{
		ts:                 topoServer,
		cell:               localCell,
		retryDelay:         retryDelay,
		healthCheckTimeout: healthCheckTimeout,
		cellAliases:        make(map[string]string),
		topoWatchers:       topoWatchers,
	}

	healthcheckOnce.Do(func() {
		http.Handle("/debug/gateway", hc)
	})

	// start the topo watches here
	for _, tw := range hc.topoWatchers {
		go hc.watchTopo(tw)
	}

	return hc
}

func (hc *HealthCheckImpl) watchTopo(tw *TopologyWatcher) {
	tw.wg.Add(1)
	defer tw.wg.Done()
	ticker := time.NewTicker(tw.refreshInterval)
	defer ticker.Stop()
	for {
		hc.loadTablets(tw)
		select {
		case <-tw.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (hc *HealthCheckImpl) loadTablets(tw *TopologyWatcher) {
	var wg sync.WaitGroup
	newTablets := make(map[string]*tabletInfo)
	replacedTablets := make(map[string]*tabletInfo)

	tabletAliases, err := tw.getTablets(tw)
	topologyWatcherOperations.Add(topologyWatcherOpListTablets, 1)
	if err != nil {
		topologyWatcherErrors.Add(topologyWatcherOpListTablets, 1)
		select {
		case <-tw.ctx.Done():
			return
		default:
		}
		log.Errorf("cannot get tablets for cell: %v: %v", tw.cell, err)
		return
	}

	// Accumulate a list of all known alias strings to use later
	// when sorting
	tabletAliasStrs := make([]string, 0, len(tabletAliases))

	tw.mu.Lock()
	for _, tAlias := range tabletAliases {
		aliasStr := topoproto.TabletAliasString(tAlias)
		tabletAliasStrs = append(tabletAliasStrs, aliasStr)

		if !tw.refreshKnownTablets {
			if val, ok := tw.tablets[aliasStr]; ok {
				newTablets[aliasStr] = val
				continue
			}
		}

		wg.Add(1)
		go func(alias *topodatapb.TabletAlias) {
			defer wg.Done()
			tw.sem <- 1 // Wait for active queue to drain.
			tablet, err := tw.topoServer.GetTablet(tw.ctx, alias)
			topologyWatcherOperations.Add(topologyWatcherOpGetTablet, 1)
			<-tw.sem // Done; enable next request to run
			if err != nil {
				topologyWatcherErrors.Add(topologyWatcherOpGetTablet, 1)
				select {
				case <-tw.ctx.Done():
					return
				default:
				}
				log.Errorf("cannot get tablet for alias %v: %v", alias, err)
				return
			}
			if !(hc.isTabletInCell(tablet.Tablet) && (tw.tabletFilter == nil || tw.tabletFilter.IsIncluded(tablet.Tablet))) {
				return
			}
			tw.mu.Lock()
			aliasStr := topoproto.TabletAliasString(alias)
			newTablets[aliasStr] = &tabletInfo{
				alias:  aliasStr,
				key:    TabletToMapKey(tablet.Tablet),
				tablet: tablet.Tablet,
			}
			tw.mu.Unlock()
		}(tAlias)
	}

	tw.mu.Unlock()
	wg.Wait()
	tw.mu.Lock()

	for alias, newVal := range newTablets {
		if val, ok := tw.tablets[alias]; !ok {
			// Check if there's a tablet with the same address key but a
			// different alias. If so, replace it and keep track of the
			// replaced alias to make sure it isn't removed later.
			found := false
			for _, otherVal := range tw.tablets {
				if newVal.key == otherVal.key {
					found = true
					hc.ReplaceTablet(otherVal.tablet, newVal.tablet)
					topologyWatcherOperations.Add(topologyWatcherOpReplaceTablet, 1)
					replacedTablets[otherVal.alias] = newVal
				}
			}
			if !found {
				hc.AddTablet(newVal.tablet)
				topologyWatcherOperations.Add(topologyWatcherOpAddTablet, 1)
			}

		} else if val.key != newVal.key {
			// Handle the case where the same tablet alias is now reporting
			// a different address key.
			replacedTablets[alias] = newVal
			hc.ReplaceTablet(val.tablet, newVal.tablet)
			topologyWatcherOperations.Add(topologyWatcherOpReplaceTablet, 1)
		}
	}

	for _, val := range tw.tablets {
		if _, ok := newTablets[val.alias]; !ok {
			if _, ok2 := replacedTablets[val.alias]; !ok2 {
				hc.RemoveTablet(val.tablet)
				topologyWatcherOperations.Add(topologyWatcherOpRemoveTablet, 1)
			}
		}
	}
	tw.tablets = newTablets
	if !tw.firstLoadDone {
		tw.firstLoadDone = true
		close(tw.firstLoadChan)
	}

	// iterate through the tablets in a stable order and compute a
	// checksum of the tablet map
	sort.Strings(tabletAliasStrs)
	var buf bytes.Buffer
	for _, alias := range tabletAliasStrs {
		tabletInfo, ok := tw.tablets[alias]
		if ok {
			buf.WriteString(alias)
			buf.WriteString(tabletInfo.key)
		}
	}
	tw.topoChecksum = crc32.ChecksumIEEE(buf.Bytes())
	tw.lastRefresh = time.Now()

	tw.mu.Unlock()

}

// RegisterStats registers the connection counts stats
func (hc *HealthCheckImpl) RegisterStats() {
	stats.NewGaugeDurationFunc(
		"TopologyWatcherMaxRefreshLag",
		"maximum time since the topology watcher refreshed a cell",
		hc.topologyWatcherMaxRefreshLag,
	)

	stats.NewGaugeFunc(
		"TopologyWatcherChecksum",
		"crc32 checksum of the topology watcher state",
		hc.topologyWatcherChecksum,
	)

	stats.NewGaugesFuncWithMultiLabels(
		"HealthcheckConnections",
		"the number of healthcheck connections registered",
		[]string{"Keyspace", "ShardName", "TabletType"},
		hc.servingConnStats)

	stats.NewGaugeFunc(
		"HealthcheckChecksum",
		"crc32 checksum of the current healthcheck state",
		hc.stateChecksum)
}

// ServeHTTP is part of the http.Handler interface. It renders the current state of the discovery gateway tablet cache into json.
func (hc *HealthCheckImpl) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	status := hc.cacheStatusMap()
	b, err := json.MarshalIndent(status, "", " ")
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	buf := bytes.NewBuffer(nil)
	json.HTMLEscape(buf, b)
	w.Write(buf.Bytes())
}

// servingConnStats returns the number of serving tablets per keyspace/shard/tablet type.
func (hc *HealthCheckImpl) servingConnStats() map[string]int64 {
	res := make(map[string]int64)
	hc.mu.Lock()
	defer hc.mu.Unlock()
	for key, ths := range hc.entries {
		for _, th := range ths {
			if !th.Up || !th.Serving || th.LastError != nil {
				continue
			}
			res[key]++
		}
	}
	return res
}

// stateChecksum returns a crc32 checksum of the healthcheck state
func (hc *HealthCheckImpl) stateChecksum() int64 {
	// CacheStatus is sorted so this should be stable across vtgates
	cacheStatus := hc.CacheStatus()
	var buf bytes.Buffer
	for _, st := range cacheStatus {
		fmt.Fprintf(&buf,
			"%v%v%v%v\n",
			st.Cell,
			st.Target.Keyspace,
			st.Target.Shard,
			st.Target.TabletType.String(),
		)
		sort.Sort(st.TabletsStats)
		for _, ts := range st.TabletsStats {
			fmt.Fprintf(&buf, "%v%v%v\n", ts.Up, ts.Serving, ts.MasterTermStartTime)
		}
	}

	return int64(crc32.ChecksumIEEE(buf.Bytes()))
}

// finalizeConn closes the health checking connection and sends the final
// notification about the tablet to downstream. To be called only on exit from
// checkConn().
func (hc *HealthCheckImpl) finalizeConn(hcc *healthCheckConn) {
	hcc.tabletHealth.mu.Lock()
	defer hcc.tabletHealth.mu.Unlock()
	hcc.tabletHealth.Up = false
	hcc.setServingState(false, "finalizeConn closing connection")
	// Note: checkConn() exits only when hcc.ctx.Done() is closed. Thus it's
	// safe to simply get Err() value here and assign to LastError.
	hcc.tabletHealth.LastError = hcc.ctx.Err()
	if hcc.tabletHealth.conn != nil {
		// Don't use hcc.ctx because it's already closed.
		// Use a separate context, and add a timeout to prevent unbounded waits.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		hcc.tabletHealth.conn.Close(ctx)
		hcc.tabletHealth.conn = nil
	}
}

// checkConn performs health checking on the given tablet.
func (hc *HealthCheckImpl) checkConn(hcc *healthCheckConn) {
	defer hc.connsWG.Done()
	defer hc.finalizeConn(hcc)

	retryDelay := hc.retryDelay
	for {
		streamCtx, streamCancel := context.WithCancel(hcc.ctx)

		// Setup a watcher that restarts the timer every time an update is received.
		// If a timeout occurs for a serving tablet, we make it non-serving and send
		// a status update. The stream is also terminated so it can be retried.
		// servingStatus feeds into the serving var, which keeps track of the serving
		// status transmitted by the tablet.
		servingStatus := make(chan bool, 1)
		// timedout is accessed atomically because there could be a race
		// between the goroutine that sets it and the check for its value
		// later.
		timedout := sync2.NewAtomicBool(false)
		go func() {
			for {
				select {
				case <-servingStatus:
					continue
				case <-time.After(hc.healthCheckTimeout):
					timedout.Set(true)
					streamCancel()
					return
				case <-streamCtx.Done():
					// If the stream is done, stop watching.
					return
				}
			}
		}()

		// Read stream health responses.
		hcc.stream(streamCtx, func(shr *querypb.StreamHealthResponse) error {
			// We received a message. Reset the back-off.
			retryDelay = hc.retryDelay
			// Don't block on send to avoid deadlocks.
			select {
			case servingStatus <- shr.Serving:
			default:
			}
			return hcc.processResponse(hc, shr)
		})

		// streamCancel to make sure the watcher goroutine terminates.
		streamCancel()

		// If there was a timeout send an error. We do this after stream has returned.
		// This will ensure that this update prevails over any previous message that
		// stream could have sent.
		if timedout.Get() {
			hcc.tabletHealth.mu.Lock()
			hcc.tabletHealth.LastError = fmt.Errorf("healthcheck timed out (latest %v)", hcc.lastResponseTimestamp)
			hcc.setServingState(false, hcc.tabletHealth.LastError.Error())
			hcErrorCounters.Add([]string{hcc.tabletHealth.Target.Keyspace, hcc.tabletHealth.Target.Shard, topoproto.TabletTypeLString(hcc.tabletHealth.Target.TabletType)}, 1)
			hcc.tabletHealth.mu.Unlock()
		}

		// Streaming RPC failed e.g. because vttablet was restarted or took too long.
		// Sleep until the next retry is up or the context is done/canceled.
		select {
		case <-hcc.ctx.Done():
			return
		case <-time.After(retryDelay):
			// Exponentially back-off to prevent tight-loop.
			retryDelay *= 2
			// Limit the retry delay backoff to the health check timeout
			if retryDelay > hc.healthCheckTimeout {
				retryDelay = hc.healthCheckTimeout
			}
		}
	}
}

// setServingState sets the tablet state to the given value.
//
// If the state changes, it logs the change so that failures
// from the health check connection are logged the first time,
// but don't continue to log if the connection stays down.
//
// hcc.tabletHealth.mu must be locked before calling this function
func (hcc *healthCheckConn) setServingState(serving bool, reason string) {
	if !hcc.loggedServingState || (serving != hcc.tabletHealth.Serving) {
		// Emit the log from a separate goroutine to avoid holding
		// the hcc lock while logging is happening
		go log.Infof("HealthCheckUpdate(Serving State): tablet: %v serving => %v for %v/%v (%v) reason: %s",
			topotools.TabletIdent(hcc.tabletHealth.Tablet),
			serving,
			hcc.tabletHealth.Tablet.GetKeyspace(),
			hcc.tabletHealth.Tablet.GetShard(),
			hcc.tabletHealth.Target.GetTabletType(),
			reason,
		)
		hcc.loggedServingState = true
	}
	hcc.tabletHealth.Serving = serving
}

// stream streams healthcheck responses to callback.
func (hcc *healthCheckConn) stream(ctx context.Context, callback func(*querypb.StreamHealthResponse) error) {
	hcc.tabletHealth.mu.Lock()
	if hcc.tabletHealth.conn == nil {
		conn, err := tabletconn.GetDialer()(hcc.tabletHealth.Tablet, grpcclient.FailFast(true))
		if err != nil {
			hcc.tabletHealth.LastError = err
			hcc.tabletHealth.mu.Unlock()
			return
		}
		hcc.tabletHealth.conn = conn
		hcc.tabletHealth.LastError = nil
	}
	conn := hcc.tabletHealth.conn
	hcc.tabletHealth.mu.Unlock()

	if err := conn.StreamHealth(ctx, callback); err != nil {
		hcc.tabletHealth.mu.Lock()
		log.Warningf("tablet %v healthcheck stream error: %v", hcc.tabletHealth.Tablet.Alias, err)
		hcc.setServingState(false, err.Error())
		hcc.tabletHealth.LastError = err
		hcc.tabletHealth.conn.Close(ctx)
		hcc.tabletHealth.conn = nil
		hcc.tabletHealth.mu.Unlock()
	}
}

// processResponse reads one health check response, and updates health
func (hcc *healthCheckConn) processResponse(hc *HealthCheckImpl, shr *querypb.StreamHealthResponse) error {
	select {
	case <-hcc.ctx.Done():
		return hcc.ctx.Err()
	default:
	}

	// Check for invalid data, better than panicking.
	if shr.Target == nil || shr.RealtimeStats == nil {
		return fmt.Errorf("health stats is not valid: %v", shr)
	}

	// an app-level error from tablet, force serving state.
	var healthErr error
	serving := shr.Serving
	if shr.RealtimeStats.HealthError != "" {
		healthErr = fmt.Errorf("vttablet error: %v", shr.RealtimeStats.HealthError)
		serving = false
	}

	if shr.TabletAlias != nil && !proto.Equal(shr.TabletAlias, hcc.tabletHealth.Tablet.Alias) {
		return fmt.Errorf("health stats mismatch, tablet %+v alias does not match response alias %v", hcc.tabletHealth.Tablet, shr.TabletAlias)
	}

	hcc.tabletHealth.mu.Lock()
	currentTablet := hcc.tabletHealth.Tablet
	hcc.tabletHealth.mu.Unlock()
	// In this case where a new tablet is initialized or a tablet type changes, we want to
	// initialize the counter so the rate can be calculated correctly.
	if currentTablet.Type != shr.Target.TabletType {
		hcErrorCounters.Add([]string{shr.Target.Keyspace, shr.Target.Shard, topoproto.TabletTypeLString(shr.Target.TabletType)}, 0)
		// hc still has this tabletHealth in the wrong target (because tabletType changed)
		oldTargetKey := hc.keyFromTablet(currentTablet)
		newTargetKey := hc.keyFromTarget(shr.Target)
		tabletAlias := topoproto.TabletAliasString(currentTablet.Alias)
		hc.mu.Lock()
		delete(hc.entries[oldTargetKey], tabletAlias)
		hc.entries[newTargetKey][tabletAlias] = hcc.tabletHealth
		hc.mu.Unlock()
	}

	// Update our record, and notify downstream for tabletType and
	// realtimeStats change.
	hcc.lastResponseTimestamp = time.Now()
	hcc.tabletHealth.mu.Lock()
	defer hcc.tabletHealth.mu.Unlock()
	hcc.tabletHealth.Target = shr.Target
	hcc.tabletHealth.MasterTermStartTime = shr.TabletExternallyReparentedTimestamp
	hcc.tabletHealth.Stats = shr.RealtimeStats
	hcc.tabletHealth.LastError = healthErr
	reason := "healthCheck update"
	if healthErr != nil {
		reason = "healthCheck update error: " + healthErr.Error()
	}
	hcc.setServingState(serving, reason)
	return nil
}

func (hc *HealthCheckImpl) deleteConn(tablet *topodatapb.Tablet) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	key := hc.keyFromTablet(tablet)
	tabletAlias := topoproto.TabletAliasString(tablet.Alias)
	ths, ok := hc.entries[key]
	if !ok {
		log.Warningf("Something is wrong, we have no health data for tablet: %v's target: %v", tabletAlias, key)
		return
	}
	th, ok := ths[tabletAlias]
	if !ok {
		log.Warningf("Something is wrong, we have no health data for tablet: %v", tabletAlias)
		return
	}
	th.deleteConnLocked()
	delete(ths, tabletAlias)
}

// AddTablet adds the tablet, and starts health check.
// It does not block on making connection.
// name is an optional tag for the tablet, e.g. an alternative address.
func (hc *HealthCheckImpl) AddTablet(tablet *topodatapb.Tablet) {
	hc.mu.Lock()
	if hc.entries == nil {
		// already closed.
		hc.mu.Unlock()
		return
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	target := &querypb.Target{
		Keyspace:   tablet.Keyspace,
		Shard:      tablet.Shard,
		TabletType: tablet.Type,
	}
	hcc := &healthCheckConn{
		ctx: ctx,
		tabletHealth: &tabletHealth{
			cancelFunc: cancelFunc,
			Tablet:     tablet,
			Target:     target,
			Up:         true,
		},
	}

	// add to our datastore
	key := hc.keyFromTarget(target)
	tabletAlias := topoproto.TabletAliasString(tablet.Alias)
	if ths, ok := hc.entries[key]; !ok {
		hc.entries[key] = make(map[string]*tabletHealth)
		hc.entries[key][tabletAlias] = hcc.tabletHealth
	} else {
		if _, ok := ths[tabletAlias]; !ok {
			ths[tabletAlias] = hcc.tabletHealth
		}
	}

	hc.connsWG.Add(1)
	hc.mu.Unlock()
	go hc.checkConn(hcc)
}

// RemoveTablet removes the tablet, and stops the health check.
// It does not block.
func (hc *HealthCheckImpl) RemoveTablet(tablet *topodatapb.Tablet) {
	hc.deleteConn(tablet)
}

// ReplaceTablet removes the old tablet and adds the new tablet.
func (hc *HealthCheckImpl) ReplaceTablet(old, new *topodatapb.Tablet) {
	hc.deleteConn(old)
	hc.AddTablet(new)
}

// getConnection returns the TabletConn of the given tablet.
func (hc *HealthCheckImpl) getConnection(key string) queryservice.QueryService {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	th := hc.findTabletHealthByAlias(key)
	if th == nil {
		return nil
	}
	return th.conn
}

func (hc *HealthCheckImpl) findTabletHealthByAlias(key string) *tabletHealth {
	for _, ths := range hc.entries {
		for _, th := range ths {
			if topoproto.TabletAliasString(th.Tablet.Alias) == key {
				return th
			}
		}
	}
	return nil
}

// CacheStatus returns a displayable version of the cache.
func (hc *HealthCheckImpl) CacheStatus() TabletsCacheStatusList {
	tcsMap := hc.cacheStatusMap()
	tcsl := make(TabletsCacheStatusList, 0, len(tcsMap))
	for _, tcs := range tcsMap {
		tcsl = append(tcsl, tcs)
	}
	sort.Sort(tcsl)
	return tcsl
}

func (hc *HealthCheckImpl) cacheStatusMap() map[string]*TabletsCacheStatus {
	tcsMap := make(map[string]*TabletsCacheStatus)
	hc.mu.Lock()
	defer hc.mu.Unlock()
	for _, ths := range hc.entries {
		for _, th := range ths {
			key := fmt.Sprintf("%v.%v.%v.%v", th.Tablet.Alias.Cell, th.Target.Keyspace, th.Target.Shard, th.Target.TabletType.String())
			var tcs *TabletsCacheStatus
			var ok bool
			if tcs, ok = tcsMap[key]; !ok {
				tcs = &TabletsCacheStatus{
					Cell:   th.Tablet.Alias.Cell,
					Target: th.Target,
				}
				tcsMap[key] = tcs
			}
			tcs.TabletsStats = append(tcs.TabletsStats, th)
		}
	}
	return tcsMap
}

// Close stops the healthcheck.
func (hc *HealthCheckImpl) Close() error {
	hc.mu.Lock()
	for _, ths := range hc.entries {
		for _, th := range ths {
			th.cancelFunc()
		}
	}
	hc.entries = nil
	for _, tw := range hc.topoWatchers {
		tw.Stop()
	}
	// Release the lock early or a pending checkHealthCheckTimeout
	// cannot get a read lock on it.
	hc.mu.Unlock()

	// Wait for the checkHealthCheckTimeout Go routine and each Go
	// routine per tablet.
	hc.connsWG.Wait()

	return nil
}

// topologyWatcherMaxRefreshLag returns the maximum lag since the watched
// cells were refreshed from the topo server
func (hc *HealthCheckImpl) topologyWatcherMaxRefreshLag() time.Duration {
	var lag time.Duration
	for _, tw := range hc.topoWatchers {
		cellLag := tw.RefreshLag()
		if cellLag > lag {
			lag = cellLag
		}
	}
	return lag
}

// topologyWatcherChecksum returns a checksum of the topology watcher state
func (hc *HealthCheckImpl) topologyWatcherChecksum() int64 {
	var checksum int64
	for _, tw := range hc.topoWatchers {
		checksum = checksum ^ int64(tw.TopoChecksum())
	}
	return checksum
}

// GetTabletAndConnection gets you a tablet connection and it's "Key" as produced by TabletToMapKey
// The Key is used by the caller to keep track of invalidTablets
func (hc *HealthCheckImpl) GetTabletAndConnection(target *querypb.Target, localCell string, invalidTablets map[string]bool) (string, queryservice.QueryService, error) {
	tablets := hc.getHealthyTabletStats(target)
	if len(tablets) == 0 {
		// fail fast if there is no tablet
		err := vterrors.New(vtrpcpb.Code_UNAVAILABLE, "no valid tablet")
		return "", nil, err
	}
	hc.shuffleTablets(localCell, tablets)

	// skip tablets we tried before
	for _, t := range tablets {
		tabletAlias := hc.keyFromTablet(t.Tablet)
		if _, ok := invalidTablets[tabletAlias]; !ok {
			conn := hc.getConnection(tabletAlias)
			if conn == nil {
				invalidTablets[tabletAlias] = true
			} else {
				return tabletAlias, conn, nil
			}
		}
	}
	err := vterrors.New(vtrpcpb.Code_UNAVAILABLE, "no available connection")
	return "", nil, err
}

// GetHealthyTabletStats returns only the healthy targets.
// The returned array is owned by the caller.
// For TabletType_MASTER, this will only return at most one entry,
// the most recent tablet of type master.
func (hc *HealthCheckImpl) getHealthyTabletStats(target *querypb.Target) []*tabletHealth {
	var result []*tabletHealth
	// we check all tablet types in all cells because of cellAliases
	key := hc.keyFromTarget(target)
	ths, ok := hc.entries[key]
	if !ok {
		log.Warningf("Healthcheck has no tablet health for target: %v", key)
		return result
	}
	if target.TabletType == topodatapb.TabletType_MASTER && len(ths) > 1 {
		log.Warningf("Can only have one master, program bug: %v", ths)
		return result
	}
	for _, th := range ths {
		if th.Tablet.Type == topodatapb.TabletType_MASTER {
			result = append(result, th)
			return result
		}
		if th.isHealthy() {
			result = append(result, th)
		}
	}
	return result
}

// GetHealthyTabletStats returns only the healthy targets.
// The returned array is owned by the caller.
// For TabletType_MASTER, this will only return at most one entry,
// the most recent tablet of type master.
func (hc *HealthCheckImpl) getTabletStats(target *querypb.Target) []*tabletHealth {
	var result []*tabletHealth
	// we check all tablet types in all cells because of cellAliases
	for _, ths := range hc.entries {
		for _, th := range ths {
			result = append(result, th)
		}
	}
	return result
}

func (hc *HealthCheckImpl) shuffleTablets(cell string, tablets []*tabletHealth) {
	sameCell, diffCell, sameCellMax := 0, 0, -1
	length := len(tablets)

	// move all same cell tablets to the front, this is O(n)
	for {
		sameCellMax = diffCell - 1
		sameCell = hc.nextTablet(cell, tablets, sameCell, length, true)
		diffCell = hc.nextTablet(cell, tablets, diffCell, length, false)
		// either no more diffs or no more same cells should stop the iteration
		if sameCell < 0 || diffCell < 0 {
			break
		}

		if sameCell < diffCell {
			// fast forward the `sameCell` lookup to `diffCell + 1`, `diffCell` unchanged
			sameCell = diffCell + 1
		} else {
			// sameCell > diffCell, swap needed
			tablets[sameCell], tablets[diffCell] = tablets[diffCell], tablets[sameCell]
			sameCell++
			diffCell++
		}
	}

	//shuffle in same cell tablets
	for i := sameCellMax; i > 0; i-- {
		swap := rand.Intn(i + 1)
		tablets[i], tablets[swap] = tablets[swap], tablets[i]
	}

	//shuffle in diff cell tablets
	for i, diffCellMin := length-1, sameCellMax+1; i > diffCellMin; i-- {
		swap := rand.Intn(i-sameCellMax) + diffCellMin
		tablets[i], tablets[swap] = tablets[swap], tablets[i]
	}
}

func (hc *HealthCheckImpl) nextTablet(cell string, tablets []*tabletHealth, offset, length int, sameCell bool) int {
	for ; offset < length; offset++ {
		if (tablets[offset].Tablet.Alias.Cell == cell) == sameCell {
			return offset
		}
	}
	return -1
}

func (hc *HealthCheckImpl) getAliasByCell(cell string) string {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if alias, ok := hc.cellAliases[cell]; ok {
		return alias
	}

	alias := topo.GetAliasByCell(context.Background(), hc.ts, cell)
	hc.cellAliases[cell] = alias

	return alias
}

func (hc *HealthCheckImpl) isTabletInCell(tablet *topodatapb.Tablet) bool {
	if tablet.Type == topodatapb.TabletType_MASTER {
		return true
	}
	if tablet.Alias.Cell == hc.cell {
		return true
	}
	if hc.getAliasByCell(tablet.Alias.Cell) == hc.getAliasByCell(hc.cell) {
		return true
	}
	return false
}

// WaitForTablets waits for at least one tablet in the given
// keyspace / shard / tablet type before returning. The tablets do not
// have to be healthy.  It will return ctx.Err() if the context is canceled.
func (hc *HealthCheckImpl) WaitForTablets(ctx context.Context, keyspace, shard string, tabletType topodatapb.TabletType) error {
	targets := []*querypb.Target{
		{
			Keyspace:   keyspace,
			Shard:      shard,
			TabletType: tabletType,
		},
	}
	return hc.waitForTablets(ctx, targets, false)
}

// WaitForAllServingTablets waits for at least one healthy serving tablet in
// each given target before returning.
// It will return ctx.Err() if the context is canceled.
// It will return an error if it can't read the necessary topology records.
func (hc *HealthCheckImpl) WaitForAllServingTablets(ctx context.Context, targets []*querypb.Target) error {
	return hc.waitForTablets(ctx, targets, true)
}

// waitForTablets is the internal method that polls for tablets.
func (hc *HealthCheckImpl) waitForTablets(ctx context.Context, targets []*querypb.Target, requireServing bool) error {
	for {
		// We nil targets as we find them.
		allPresent := true
		for i, target := range targets {
			if target == nil {
				continue
			}

			var stats []*tabletHealth
			if requireServing {
				stats = hc.getHealthyTabletStats(target)
			} else {
				stats = hc.getTabletStats(target)
			}
			if len(stats) == 0 {
				allPresent = false
			} else {
				targets[i] = nil
			}
		}

		if allPresent {
			// we found everything we needed
			return nil
		}

		// Unblock after the sleep or when the context has expired.
		timer := time.NewTimer(waitAvailableTabletInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Target includes cell which we ignore here
// because tabletStatsCache is intended to be per-cell
func (hc *HealthCheckImpl) keyFromTarget(target *querypb.Target) string {
	return fmt.Sprintf("%s.%s.%d", target.Keyspace, target.Shard, target.TabletType)
}

func (hc *HealthCheckImpl) keyFromTablet(tablet *topodatapb.Tablet) string {
	return fmt.Sprintf("%s.%s.%d", tablet.Keyspace, tablet.Shard, tablet.Type)
}
