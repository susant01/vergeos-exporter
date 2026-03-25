package collectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type TenantCollector struct {
	BaseCollector
	mutex sync.Mutex

	systemName string

	// Metrics
	tenantStatusDesc             *prometheus.Desc
	tenantTotalDesc              *prometheus.Desc
	tenantNodeTotalDesc          *prometheus.Desc
	tenantAllocatedCPUDesc       *prometheus.Desc
	tenantAllocatedRAMDesc       *prometheus.Desc
	tenantAllocatedStorageDesc   *prometheus.Desc
	tenantProvisionedStorageDesc *prometheus.Desc
	tenantUsedStorageDesc        *prometheus.Desc
	tenantNodeOnlineDesc         *prometheus.Desc
	tenantNodeTotalCPUDesc       *prometheus.Desc
	tenantNodeTotalRAMDesc       *prometheus.Desc
	tenantNodeTotalStorageDesc   *prometheus.Desc
	tenantNodeUsedCPUDesc        *prometheus.Desc
	tenantNodeUsedRAMDesc        *prometheus.Desc
}

func NewTenantCollector(url string, client *http.Client, username, password string) *TenantCollector {
	tc := &TenantCollector{
		BaseCollector: BaseCollector{
			url:        url,
			httpClient: client,
		},
		systemName: "unknown", // Will be updated in Collect
		tenantStatusDesc: prometheus.NewDesc(
			"vergeos_tenant_status",
			"Status of tenant",
			[]string{"system_name", "tenant_name"},
			nil,
		),
		tenantTotalDesc: prometheus.NewDesc(
			"vergeos_tenant_total",
			"Total number of tenants",
			[]string{"system_name"},
			nil,
		),
		tenantNodeOnlineDesc: prometheus.NewDesc(
			"vergeos_tenant_node_online",
			"Online Node  of tenants",
			[]string{"system_name", "tenant_name"},
			nil,
		),
		tenantNodeTotalDesc: prometheus.NewDesc(
			"vergeos_tenant_node_total",
			"Total number of nodes in tenant",
			[]string{"system_name", "tenant_name"},
			nil,
		),
		tenantAllocatedCPUDesc: prometheus.NewDesc(
			"vergeos_tenant_cpu_allocated",
			"Allocated CPU per tenant",
			[]string{"system_name", "tenant_name"},
			nil,
		),
		tenantAllocatedRAMDesc: prometheus.NewDesc(
			"vergeos_tenant_ram_allocated",
			"Allocated RAM per tenant",
			[]string{"system_name", "tenant_name"},
			nil,
		),
		tenantAllocatedStorageDesc: prometheus.NewDesc(
			"vergeos_tenant_storage_allocated",
			"Allocated Storage per tenant in MB",
			[]string{"system_name", "tenant_name", "tier"},
			nil,
		),
		tenantProvisionedStorageDesc: prometheus.NewDesc(
			"vergeos_tenant_strorage_provisioned",
			"Provisioned Storage ",
			[]string{"system_name", "tenant_name", "tier"},
			nil,
		),
		tenantUsedStorageDesc: prometheus.NewDesc(
			"vergeos_tenant_storage_usage",
			"Total storage Used",
			[]string{"system_name", "tenant_name", "tier"},
			nil,
		),
		tenantNodeTotalCPUDesc: prometheus.NewDesc(
			"vergeos_tenant_node_cpu_total",
			"Total CPU per node",
			[]string{"system_name", "tenant_name", "node_name", "cluster"},
			nil,
		),
		tenantNodeTotalRAMDesc: prometheus.NewDesc(
			"vergeos_tenant_node_ram_total",
			"Total RAM per node",
			[]string{"system_name", "tenant_name", "node_name", "cluster"},
			nil,
		),
		tenantNodeTotalStorageDesc: prometheus.NewDesc(
			"vergeos_tenant_node_storage_total",
			"Total Storage per tenant",
			[]string{"system_name", "tenant_name", "node_name", "cluster"},
			nil,
		),
		tenantNodeUsedCPUDesc: prometheus.NewDesc(
			"vergeos_tenant_node_cpu_usage",
			"CPU usage per node in Tenant",
			[]string{"system_name", "tenant_name", "node_name", "cluster"},
			nil,
		),
		tenantNodeUsedRAMDesc: prometheus.NewDesc(
			"vergeos_tenant_node_ram_usage",
			"RAM usage per node in tenant",
			[]string{"system_name", "tenant_name", "node_name", "cluster"},
			nil,
		),
	}
	// Authenticate with the API
	if err := tc.authenticate(username, password); err != nil {
		fmt.Printf("Error authenticating with VergeOS API: %v\n", err)
	}
	return tc
}

func (tc *TenantCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- tc.tenantStatusDesc
	ch <- tc.tenantNodeTotalDesc
	ch <- tc.tenantTotalDesc
	ch <- tc.tenantAllocatedCPUDesc
	ch <- tc.tenantAllocatedRAMDesc
	ch <- tc.tenantAllocatedStorageDesc
	ch <- tc.tenantProvisionedStorageDesc
	ch <- tc.tenantUsedStorageDesc
	ch <- tc.tenantNodeTotalCPUDesc
	ch <- tc.tenantNodeTotalRAMDesc
	ch <- tc.tenantNodeTotalStorageDesc
	ch <- tc.tenantNodeUsedCPUDesc
	ch <- tc.tenantNodeUsedRAMDesc
}

func (tc *TenantCollector) Collect(ch chan<- prometheus.Metric) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	// Get system name
	req, err := tc.makeRequest("GET", "/api/v4/settings?fields=most&filter=key%20eq%20%22cloud_name%22")
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error executing request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var settings []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		fmt.Printf("Error decoding settings response: %v\n", err)
		return
	}

	for _, setting := range settings {
		if setting.Key == "cloud_name" {
			tc.systemName = setting.Value
			break
		}
	}

	// Get Tenant detail
	req, err = tc.makeRequest("GET", "/api/v4/tenants?fields=dashboard&sort=%2Btimestamp")
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	resp, err = tc.httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error executing request: %v\n", err)
		return
	}
	defer resp.Body.Close()
	var tenants []TenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
		fmt.Printf("Error decoding response: %v\n", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(tc.tenantTotalDesc, prometheus.GaugeValue, float64(len(tenants)), tc.systemName)

	// tenantNodeCount := make(map[string]int)
	for _, tenant := range tenants {
		status := 2
		if tenant.Status.Status == "online" {
			status = 1
		}
		onlineNodes := len(tenant.NodesOnline)
		totalNodes := len(tenant.Nodes)

		ch <- prometheus.MustNewConstMetric(tc.tenantStatusDesc, prometheus.GaugeValue, float64(status), tc.systemName, tenant.Name)
		ch <- prometheus.MustNewConstMetric(tc.tenantAllocatedCPUDesc, prometheus.GaugeValue, float64(tenant.NodesTotals.CPUCores), tc.systemName, tenant.Name)
		ch <- prometheus.MustNewConstMetric(tc.tenantAllocatedRAMDesc, prometheus.GaugeValue, float64(tenant.NodesTotals.RAM), tc.systemName, tenant.Name)
		// Storage Stats
		for _, storage := range tenant.Storage {
			ch <- prometheus.MustNewConstMetric(tc.tenantAllocatedStorageDesc, prometheus.GaugeValue, float64(storage.Allocated), tc.systemName, tenant.Name, fmt.Sprintf("%d", storage.Tier))
			ch <- prometheus.MustNewConstMetric(tc.tenantProvisionedStorageDesc, prometheus.GaugeValue, float64(storage.Provisioned), tc.systemName, tenant.Name, fmt.Sprintf("%d", storage.Tier))
			ch <- prometheus.MustNewConstMetric(tc.tenantUsedStorageDesc, prometheus.GaugeValue, float64(storage.Used), tc.systemName, tenant.Name, fmt.Sprintf("%d", storage.Tier))
		}
		// NodeStats
		ch <- prometheus.MustNewConstMetric(tc.tenantNodeOnlineDesc, prometheus.GaugeValue, float64(onlineNodes), tc.systemName, tenant.Name)
		ch <- prometheus.MustNewConstMetric(tc.tenantNodeTotalDesc, prometheus.GaugeValue, float64(totalNodes), tc.systemName, tenant.Name)
		for _, node := range tenant.Nodes {
			ch <- prometheus.MustNewConstMetric(tc.tenantNodeTotalCPUDesc, prometheus.GaugeValue, float64(node.CPUCores), tc.systemName, tenant.Name, node.Name, node.Cluster)
			ch <- prometheus.MustNewConstMetric(tc.tenantNodeTotalRAMDesc, prometheus.GaugeValue, float64(node.RAM), tc.systemName, tenant.Name, node.Name, node.Cluster)
			ch <- prometheus.MustNewConstMetric(tc.tenantNodeUsedCPUDesc, prometheus.GaugeValue, float64(node.Machine.Stats.TotalCPU), tc.systemName, tenant.Name, node.Name, node.Cluster)
			ch <- prometheus.MustNewConstMetric(tc.tenantNodeUsedRAMDesc, prometheus.GaugeValue, float64(node.Machine.Stats.RAMUsed), tc.systemName, tenant.Name, node.Name, node.Cluster)
		}
	}
}
