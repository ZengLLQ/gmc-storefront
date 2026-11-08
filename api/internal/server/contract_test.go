package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gmc/api/internal/admin/rbac"
	"gmc/api/internal/gen/openapi"
)

func TestPublicSchemasNeverLeakCostIdr(t *testing.T) {
	cardJSON, _ := json.Marshal(openapi.ProductCard{})
	if strings.Contains(string(cardJSON), "costIdr") {
		t.Fatalf("ProductCard leaked costIdr: %s", string(cardJSON))
	}
	detailJSON, _ := json.Marshal(openapi.ProductDetail{})
	if strings.Contains(string(detailJSON), "costIdr") {
		t.Fatalf("ProductDetail leaked costIdr: %s", string(detailJSON))
	}
	variantJSON, _ := json.Marshal(openapi.ProductVariant{})
	if strings.Contains(string(variantJSON), "costIdr") {
		t.Fatalf("ProductVariant leaked costIdr: %s", string(variantJSON))
	}
	orderJSON, _ := json.Marshal(openapi.Order{})
	if strings.Contains(string(orderJSON), "costIdr") {
		t.Fatalf("Order leaked costIdr: %s", string(orderJSON))
	}
}

func TestOpenAPIAdminRoutesHavePermissions(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "openapi.yaml"))
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	spec := string(raw)
	for _, req := range rbac.RoutePermissions {
		if !strings.Contains(spec, req.Pattern+":") {
			t.Errorf("route permission %s %s does not exist in openapi.yaml", req.Method, req.Pattern)
		}
	}
}
