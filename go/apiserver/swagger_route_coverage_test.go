package apiserver_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/services"
)

// TestSwaggerRouteCoverage walks the chi router that APIServer returns and
// asserts every /api/v1/... route has a matching swagger operation in
// docs/swagger.json — and conversely, that no documented operation
// references a path that isn't registered. The shape comes from the
// prototype originally drafted on feat/1422-openapi-drift-ci and embedded
// in #1442.
func TestSwaggerRouteCoverage(t *testing.T) {
	t.Parallel()

	params, _, _ := newParams()
	// Mount every conditionally-gated route so the bidirectional check sees
	// the same surface the swagger annotations document. The public scan
	// endpoint (#1988) is only mounted when PublicScanEnabled is true AND a
	// scan service is wired; without both, its documented operation would
	// be (incorrectly) reported as stale.
	params.PublicScanEnabled = true
	params.CommodityScanService = services.NewCommodityScanService(nil, params.FactorySet.CommodityScanAuditRegistry, services.CommodityScanConfig{})
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	router, ok := handler.(chi.Router)
	if !ok {
		t.Fatalf("APIServer should return a chi.Router-typed handler, got %T", handler)
	}

	registered, err := walkAPIRoutes(router)
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	documented, err := loadSwaggerOperations()
	if err != nil {
		t.Fatalf("load swagger.json: %v", err)
	}

	var missing, stale []string
	for _, op := range registered {
		if _, ok := documented[op]; !ok {
			missing = append(missing, op.String())
		}
	}
	for op := range documented {
		if !slices.Contains(registered, op) {
			stale = append(stale, op.String())
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)

	if len(missing) > 0 {
		t.Errorf("\n%d route(s) under /api/v1 are not documented in docs/swagger.json:\n  %s\n\nAdd a swag-style annotation to the handler and run `make swagger`.", len(missing), strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("\n%d swagger operation(s) reference paths that are no longer registered:\n  %s\n\nEither restore the route or remove the stale annotation, then run `make swagger`.", len(stale), strings.Join(stale, "\n  "))
	}
}

type routeOp struct {
	Method string
	Path   string
}

func (r routeOp) String() string { return r.Method + " " + r.Path }

func walkAPIRoutes(router chi.Router) ([]routeOp, error) {
	const apiPrefix = "/api/v1"
	var ops []routeOp
	walker := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, apiPrefix) {
			return nil
		}
		if strings.HasSuffix(route, "/*") {
			return nil
		}
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			return nil
		}
		stripped := strings.TrimPrefix(route, apiPrefix)
		if stripped != "/" {
			stripped = strings.TrimRight(stripped, "/")
		}
		if stripped == "" {
			stripped = "/"
		}
		ops = append(ops, routeOp{Method: method, Path: stripped})
		return nil
	}
	if err := chi.Walk(router, walker); err != nil {
		return nil, fmt.Errorf("chi.Walk: %w", err)
	}
	return ops, nil
}

func loadSwaggerOperations() (map[routeOp]struct{}, error) {
	path := filepath.Join("..", "docs", "swagger.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		BasePath string                    `json:"basePath"`
		Paths    map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode swagger.json: %w", err)
	}
	ops := make(map[routeOp]struct{}, len(doc.Paths))
	for p, methods := range doc.Paths {
		for method := range methods {
			switch strings.ToLower(method) {
			case "get", "post", "put", "patch", "delete":
			default:
				continue
			}
			ops[routeOp{Method: strings.ToUpper(method), Path: p}] = struct{}{}
		}
	}
	return ops, nil
}
