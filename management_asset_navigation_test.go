package main

import (
	"os"
	"strings"
	"testing"
)

func TestConfigAssetUsesManageRouteForAPIKeyLink(t *testing.T) {
	data, err := os.ReadFile("assets/ConfigPage-B5oeh-Kj.js")
	if err != nil {
		t.Fatalf("read config asset: %v", err)
	}
	content := string(data)

	if strings.Contains(content, `href:"#/api-keys"`) {
		t.Fatalf("config asset still uses a hash API key link that BrowserRouter ignores")
	}
	if !strings.Contains(content, `href:"/manage/api-keys"`) {
		t.Fatalf("config asset missing /manage/api-keys link")
	}
}

func TestManagementAssetsExposeTotalCostSummaries(t *testing.T) {
	usageData, err := os.ReadFile("assets/usage-C3O4ka5r.js")
	if err != nil {
		t.Fatalf("read usage asset: %v", err)
	}
	if !strings.Contains(string(usageData), `total_cost:e?.stats?.total_cost??0`) {
		t.Fatalf("usage service asset does not preserve request-log total_cost stats")
	}

	dashboardData, err := os.ReadFile("assets/DashboardPage-BqPddJLs.js")
	if err != nil {
		t.Fatalf("read dashboard asset: %v", err)
	}
	if !strings.Contains(string(dashboardData), `c?.total_cost??0`) {
		t.Fatalf("dashboard asset does not render kpi.total_cost")
	}

	requestLogsData, err := os.ReadFile("assets/RequestLogsPage-B62Ow4Oo.js")
	if err != nil {
		t.Fatalf("read request logs asset: %v", err)
	}
	if !strings.Contains(string(requestLogsData), `O.total_cost`) {
		t.Fatalf("request logs stats strip does not render total_cost")
	}
}
