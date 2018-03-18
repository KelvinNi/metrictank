// The idx package provides a metadata index for metrics

package idx

import (
	"errors"
	"time"

	"github.com/grafana/metrictank/msg"

	schema "gopkg.in/raintank/schema.v1"
)

var OrgIdPublic = 0

var (
	BothBranchAndLeaf = errors.New("node can't be both branch and leaf")
	BranchUnderLeaf   = errors.New("can't add branch under leaf")
)

//go:generate msgp
type Node struct {
	Path        string
	Leaf        bool
	Defs        []Archive
	HasChildren bool
}

type Archive struct {
	schema.MetricDefinition
	SchemaId uint16 // index in mdata.schemas (not persisted)
	AggId    uint16 // index in mdata.aggregations (not persisted)
	LastSave uint32 // last time the metricDefinition was saved to a backend store (cassandra)
}

// used primarily by tests, for convenience
func NewArchiveBare(name string) Archive {
	return Archive{
		MetricDefinition: schema.MetricDefinition{
			Name: name,
		},
	}
}

// The MetricIndex interface supports Graphite style queries.
// Note:
// * metrictank is a multi-tenant system where different orgs cannot see each
//   other's data, and any given metric name may appear multiple times,
//   under different organisations
//
// * Each metric path can be mapped to multiple metricDefinitions in the case that
//   fields other then the Name vary.  The most common occurrence of this is when
//   the Interval at which the metric is being collected has changed.
type MetricIndex interface {
	// Init initializes the index at startup and
	// blocks until the index is ready for use.
	Init() error

	// Stop shuts down the index.
	Stop()

	// AddOrUpdate makes sure a metric is known in the index,
	// and should be called for every received metric.
	AddOrUpdate(point msg.Point, partition int32) Archive

	// Get returns the archive for the requested id.
	Get(key schema.MKey) (Archive, bool)

	// GetPath returns the archives under the given path.
	GetPath(orgId int, path string) []Archive

	// Delete deletes items from the index
	// If the pattern matches a branch node, then
	// all leaf nodes on that branch are deleted. So if the pattern is
	// "*", all items in the index are deleted.
	// It returns a copy of all of the Archives deleted.
	Delete(orgId int, pattern string) ([]Archive, error)

	// Find searches the index for matching nodes.
	// * orgId describes the org to search in (public data in orgIdPublic is automatically included)
	// * pattern is handled like graphite does. see https://graphite.readthedocs.io/en/latest/render_api.html#paths-and-wildcards
	// * from is a unix timestamp. series not updated since then are excluded.
	Find(orgId int, pattern string, from int64) ([]Node, error)

	// List returns all Archives for the passed OrgId and the public orgId
	List(orgId int) []Archive

	// Prune deletes all metrics that haven't been seen since the given timestamp.
	// It returns all Archives deleted and any error encountered.
	Prune(oldest time.Time) ([]Archive, error)

	// FindByTag takes a list of expressions in the format key<operator>value.
	// The allowed operators are: =, !=, =~, !=~.
	// It returns a slice of Node structs that match the given conditions, the
	// conditions are logically AND-ed.
	// If the third argument is > 0 then the results will be filtered and only those
	// where the LastUpdate time is >= from will be returned as results.
	// The returned results are not deduplicated and in certain cases it is possible
	// that duplicate entries will be returned.
	FindByTag(orgId int, expressions []string, from int64) ([]Node, error)

	// Tags returns a list of all tag keys associated with the metrics of a given
	// organization. The return values are filtered by the regex in the second parameter.
	// If the third parameter is >0 then only metrics will be accounted of which the
	// LastUpdate time is >= the given value.
	Tags(orgId int, filter string, from int64) ([]string, error)

	// FindTags generates a list of possible tags that could complete a
	// given prefix. It also accepts additional tag conditions to further narrow
	// down the result set in the format of graphite's tag queries
	FindTags(orgId int, prefix string, expressions []string, from int64, limit uint) ([]string, error)

	// FindTagValues generates a list of possible values that could
	// complete a given value prefix. It requires a tag to be specified and only values
	// of the given tag will be returned. It also accepts additional conditions to
	// further narrow down the result set in the format of graphite's tag queries
	FindTagValues(orgId int, tag string, prefix string, expressions []string, from int64, limit uint) ([]string, error)

	// TagDetails returns a list of all values associated with a given tag key in the
	// given org. The occurrences of each value is counted and the count is referred to by
	// the metric names in the returned map.
	// If the third parameter is not "" it will be used as a regular expression to filter
	// the values before accounting for them.
	// If the fourth parameter is > 0 then only those metrics of which the LastUpdate
	// time is >= the from timestamp will be included.
	TagDetails(orgId int, key string, filter string, from int64) (map[string]uint64, error)

	// DeleteTagged deletes the specified series from the tag index and also the
	// DefById index.
	DeleteTagged(orgId int, paths []string) ([]Archive, error)
}
