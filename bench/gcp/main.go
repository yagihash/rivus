// Command gcpsample registers the handlers of the cases (nothing connects to Google Cloud: the clients
// are nil unless a test gives them a fake).
package main

import (
	"net/http"

	bq "gcpsample/bigquery"
	sp "gcpsample/spanner"
)

func main() {
	s := &sp.Server{}
	b := &bq.Server{}
	for p, h := range map[string]http.HandlerFunc{
		"/sp/query": s.VQuery, "/sp/fquery": s.FQuery, "/sp/dquery": s.DQuery,
		"/sp/newstmt": s.VNewStatement, "/sp/fnewstmt": s.FNewStatement,
		"/sp/qwo": s.VQueryWithOptions, "/sp/rot": s.VReadOnlyTransaction, "/sp/analyze": s.VAnalyze,
		"/sp/update": s.VUpdate, "/sp/fupdate": s.FUpdate, "/sp/dupdate": s.DUpdate,
		"/sp/txquery": s.VTxQuery, "/sp/batch": s.VBatchUpdate, "/sp/dbatch": s.DBatchUpdate,
		"/sp/pdml": s.VPartitionedUpdate, "/sp/dpdml": s.DPartitionedUpdate,
		"/sp/stats": s.VQueryWithStats, "/sp/updopt": s.VUpdateWithOptions, "/sp/batchopt": s.VBatchUpdateWithOptions,
		"/sp/pdmlopt": s.VPartitionedUpdateWithOptions, "/sp/part": s.VPartitionQuery, "/sp/dpart": s.DPartitionQuery,
		"/sp/partopt": s.VPartitionQueryWithOptions,
		"/sp/columns": s.ReadColumns, "/sp/byname": s.ReadColumnByName, "/sp/tostruct": s.ReadToStruct,
		"/bq/run": b.VRun, "/bq/read": b.VRead, "/bq/field": b.VField, "/bq/frun": b.FRun,
		"/bq/drun": b.DRun, "/bq/dlabels": b.DLabels,
	} {
		http.HandleFunc(p, h)
	}
	_ = http.ListenAndServe(":0", nil)
}
