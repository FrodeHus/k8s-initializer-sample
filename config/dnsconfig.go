package config

type DnsInitializerConfig struct {
	Dns   DnsConfig   `json:"dns"`
	Azure AzureConfig `json:"azure"`
}

type DnsConfig struct {
	Domain       string `json:"domain"`
	DefaultCName string `json:"defaultCName"`
}

type AzureConfig struct {
	Tenant        string `json:"tenant"`
	Subscription  string `json:"subscription"`
	ResourceGroup string `json:"resourceGroup"`
	Secret        string `json:"secret"`
}
