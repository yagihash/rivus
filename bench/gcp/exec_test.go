package main

// What the cases do when they run, against fakes (no Google Cloud): where the text of the request arrives
// in what the client sends. The sink rows say that the SQL text is the dangerous field and the bind
// parameters are not; this checks that the client really puts the value of a V* case in the text, and the
// value of an F* or D* case in the parameters only.
//
//   - Cloud Spanner: the fake of the library (spanner/spannertest) is the server, and gRPC interceptors
//     record the requests (the fake does not implement everything: what it answers does not matter,
//     the request is recorded before).
//   - BigQuery: no fake in the library; an httptest server answers a finished job and records the request body.
//
// Run it with -tags gcpexec: the functions of the package opaque have no body without it (they are what the
// analysis does not read), and the test binary would not link.
//
//	go test -tags gcpexec ./...

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/spanner"
	sppb "cloud.google.com/go/spanner/apiv1/spannerpb"
	"cloud.google.com/go/spanner/spannertest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	bq "gcpsample/bigquery"
	sp "gcpsample/spanner"
)

const marker = "rivus'marker"

// sent is what a client sent: the SQL texts and the parameters (as text).
type sent struct {
	mu     sync.Mutex
	sql    []string
	params []string
}

func (s *sent) record(m any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	add := func(sql string, p proto.Message) {
		s.sql = append(s.sql, sql)
		if p != nil && !reflect.ValueOf(p).IsNil() {
			s.params = append(s.params, protojson.Format(p))
		}
	}
	switch r := m.(type) {
	case *sppb.ExecuteSqlRequest:
		add(r.Sql, r.Params)
	case *sppb.PartitionQueryRequest:
		add(r.Sql, r.Params)
	case *sppb.ExecuteBatchDmlRequest:
		for _, st := range r.Statements {
			add(st.Sql, st.Params)
		}
	}
}

func (s *sent) reset() { s.mu.Lock(); s.sql, s.params = nil, nil; s.mu.Unlock() }

func (s *sent) has() (inSQL, inParams bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.sql {
		inSQL = inSQL || strings.Contains(x, marker)
	}
	for _, x := range s.params {
		inParams = inParams || strings.Contains(x, marker)
	}
	return
}

type recStream struct {
	grpc.ClientStream
	s *sent
}

func (r recStream) SendMsg(m any) error { r.s.record(m); return r.ClientStream.SendMsg(m) }

// run calls every method of the server whose name starts with the prefix, and says for each: the value
// was in the text, the value was in the parameters.
func run(t *testing.T, srv any, rec *sent, reset func()) map[string][2]bool {
	t.Helper()
	out := map[string][2]bool{}
	v := reflect.ValueOf(srv)
	for i := 0; i < v.NumMethod(); i++ {
		name := v.Type().Method(i).Name
		h, ok := v.Method(i).Interface().(func(http.ResponseWriter, *http.Request))
		if !ok || (name[0] != 'V' && name[0] != 'F' && name[0] != 'D' && name[0] != 'U') {
			continue
		}
		reset()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		r := httptest.NewRequest("GET", "/?name="+marker, nil).WithContext(ctx)
		h(httptest.NewRecorder(), r)
		cancel()
		a, b := rec.has()
		out[name] = [2]bool{a, b}
	}
	return out
}

func check(t *testing.T, got map[string][2]bool) {
	t.Helper()
	if len(got) == 0 {
		t.Fatal("no case ran")
	}
	for name, g := range got {
		inSQL, inParams := g[0], g[1]
		switch name[0] {
		case 'V', 'U': // U: a true flow that the analysis does not report as a V* case (the function that is not read moves it)
			if !inSQL {
				t.Errorf("%s: the value is not in the text of the request that was sent", name)
			}
		case 'F':
			if inSQL || !inParams {
				t.Errorf("%s: want the value in the parameters only: text %v, parameters %v", name, inSQL, inParams)
			}
		case 'D':
			if inSQL {
				t.Errorf("%s: the value of a decoy is in the text", name)
			}
		}
		t.Logf("%-28s text=%-5v parameters=%v", name, inSQL, inParams)
	}
}

func TestSpannerCases(t *testing.T) {
	srv, err := spannertest.NewServer("localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	rec := &sent{}
	ctx := context.Background()
	client, err := spanner.NewClient(ctx, "projects/p/instances/i/databases/d",
		option.WithEndpoint(srv.Addr),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithGRPCDialOption(grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, inv grpc.UnaryInvoker, o ...grpc.CallOption) error {
			rec.record(req)
			if strings.HasSuffix(method, "/ExecuteBatchDml") || strings.HasSuffix(method, "/PartitionQuery") {
				// the fake does not implement these (it panics): the request is recorded and not sent
				return status.Error(codes.Unimplemented, "not in the fake")
			}
			return inv(ctx, method, req, reply, cc, o...)
		})),
		option.WithGRPCDialOption(grpc.WithStreamInterceptor(func(ctx context.Context, d *grpc.StreamDesc, cc *grpc.ClientConn, method string, st grpc.Streamer, o ...grpc.CallOption) (grpc.ClientStream, error) {
			cs, err := st(ctx, d, cc, method, o...)
			if err != nil {
				return nil, err
			}
			return recStream{cs, rec}, nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	check(t, run(t, &sp.Server{Client: client}, rec, rec.reset))
}

func TestBigQueryCases(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobReference":{"projectId":"p","jobId":"j"},"configuration":{"query":{}},"status":{"state":"DONE"}}`))
	}))
	defer srv.Close()
	client, err := bigquery.NewClient(context.Background(), "p", option.WithEndpoint(srv.URL), option.WithoutAuthentication(), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	// the request body is JSON: the text is configuration.query.query and the parameters are in queryParameters.
	rec := &bqSent{}
	reset := func() { mu.Lock(); bodies = nil; mu.Unlock() }
	got := map[string][2]bool{}
	v := reflect.ValueOf(&bq.Server{Client: client})
	for i := 0; i < v.NumMethod(); i++ {
		name := v.Type().Method(i).Name
		h, ok := v.Method(i).Interface().(func(http.ResponseWriter, *http.Request))
		if !ok {
			continue
		}
		reset()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		h(httptest.NewRecorder(), httptest.NewRequest("GET", "/?name="+marker, nil).WithContext(ctx))
		cancel()
		mu.Lock()
		got[name] = rec.where(bodies)
		mu.Unlock()
	}
	check(t, got)
}

// bqSent finds the value in the text of the query or elsewhere in the request.
type bqSent struct{}

func (bqSent) where(bodies []string) [2]bool {
	var inText, elsewhere bool
	for _, b := range bodies {
		total := strings.Count(b, marker)
		inQ := 0
		const k = `"query":"`
		for rest := b; ; {
			i := strings.Index(rest, k)
			if i < 0 {
				break
			}
			rest = rest[i+len(k):]
			j := strings.Index(rest, `"`)
			if j < 0 {
				break
			}
			inQ += strings.Count(rest[:j], marker)
		}
		inText = inText || inQ > 0
		elsewhere = elsewhere || total > inQ
	}
	return [2]bool{inText, elsewhere}
}
