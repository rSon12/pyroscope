package querybackend

import (
	"sync"

	"github.com/grafana/dskit/runutil"

	querybackendv1 "github.com/grafana/pyroscope/api/gen/proto/go/querybackend/v1"
	"github.com/grafana/pyroscope/pkg/experiment/querybackend/block"
	"github.com/grafana/pyroscope/pkg/model"
	parquetquery "github.com/grafana/pyroscope/pkg/phlaredb/query"
	v1 "github.com/grafana/pyroscope/pkg/phlaredb/schemas/v1"
	"github.com/grafana/pyroscope/pkg/phlaredb/symdb"
)

func init() {
	registerQueryType(
		querybackendv1.QueryType_QUERY_TREE,
		querybackendv1.ReportType_REPORT_TREE,
		queryTree,
		newTreeAggregator,
		[]block.Section{
			block.SectionTSDB,
			block.SectionProfiles,
			block.SectionSymbols,
		}...,
	)
}

func queryTree(q *queryContext, query *querybackendv1.Query) (*querybackendv1.Report, error) {
	entries, err := profileEntryIterator(q)
	if err != nil {
		return nil, err
	}
	defer runutil.CloseWithErrCapture(&err, entries, "failed to close profile entry iterator")

	var columns v1.SampleColumns
	if err = columns.Resolve(q.ds.Profiles().Schema()); err != nil {
		return nil, err
	}

	profiles := parquetquery.NewRepeatedRowIterator(q.ctx, entries, q.ds.Profiles().RowGroups(),
		columns.StacktraceID.ColumnIndex,
		columns.Value.ColumnIndex)
	defer runutil.CloseWithErrCapture(&err, profiles, "failed to close profile stream")

	resolver := symdb.NewResolver(q.ctx, q.ds.Symbols())
	defer resolver.Release()
	for profiles.Next() {
		p := profiles.At()
		resolver.AddSamplesFromParquetRow(p.Row.Partition, p.Values[0], p.Values[1])
	}
	if err = profiles.Err(); err != nil {
		return nil, err
	}

	tree, err := resolver.Tree()
	if err != nil {
		return nil, err
	}

	resp := &querybackendv1.Report{
		Tree: &querybackendv1.TreeReport{
			Query: query.Tree.CloneVT(),
			Tree:  tree.Bytes(query.Tree.GetMaxNodes()),
		},
	}
	return resp, nil
}

type treeAggregator struct {
	init  sync.Once
	query *querybackendv1.TreeQuery
	tree  *model.TreeMerger
}

func newTreeAggregator(*querybackendv1.InvokeRequest) aggregator { return new(treeAggregator) }

func (a *treeAggregator) aggregate(report *querybackendv1.Report) error {
	r := report.Tree
	a.init.Do(func() {
		a.tree = model.NewTreeMerger()
		a.query = r.Query.CloneVT()
	})
	return a.tree.MergeTreeBytes(r.Tree)
}

func (a *treeAggregator) build() *querybackendv1.Report {
	return &querybackendv1.Report{
		Tree: &querybackendv1.TreeReport{
			Query: a.query,
			Tree:  a.tree.Tree().Bytes(a.query.GetMaxNodes()),
		},
	}
}
