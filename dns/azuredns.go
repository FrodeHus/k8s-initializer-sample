package dns

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/arm/dns"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/frodehus/dnsinit/auth"
	"github.com/frodehus/dnsinit/config"
	"github.com/golang/glog"
)

type AzureDNS struct {
	ClientId     string
	ClientSecret string
	AzureConfig  config.AzureConfig
	DnsConfig    config.DnsConfig
}

func NewDNSClient(servicePrincipal string, clientSecret string, azureConfig config.AzureConfig, dnsConfig config.DnsConfig) (*AzureDNS, error) {
	azuredns := &AzureDNS{
		ClientId:     servicePrincipal,
		ClientSecret: clientSecret,
		AzureConfig:  azureConfig,
		DnsConfig:    dnsConfig,
	}

	return azuredns, nil
}

func (d *AzureDNS) LookupRecord(recordName string) (*string, error) {
	glog.Infof("Retrieving record %s from Azure DNS", recordName)
	cname := d.DnsConfig.DefaultCName
	token, err := auth.NewServicePrincipalTokenFromCredentials(azure.PublicCloud.ResourceManagerEndpoint, d.AzureConfig.Tenant, d.ClientId, d.ClientSecret)
	if err != nil {
		return nil, err
	}
	rsc := dns.NewRecordSetsClient(d.AzureConfig.Subscription)
	rsc.Authorizer = autorest.NewBearerAuthorizer(token)
	recordType := dns.RecordType("CNAME")
	newRecord := dns.RecordSet{
		Name: &recordName,
		RecordSetProperties: &dns.RecordSetProperties{
			TTL: to.Int64Ptr(300),
			CnameRecord: &dns.CnameRecord{
				Cname: &cname,
			},
		},
	}
	recordSet, err := rsc.CreateOrUpdate(d.AzureConfig.ResourceGroup, d.DnsConfig.Domain, recordName, recordType, newRecord, "", "")
	if err != nil {
		fmt.Printf("Error retrieving record set: %s", err.Error())
		return nil, err
	}
	fmt.Printf("Found: %s", *recordSet.Name)
	return nil, nil
}

func createRecordSetBasedOnType(recordType string, recordName string, ttl int64) dns.RecordSet {
	switch recordType {
	case "CNAME":
		return dns.RecordSet{
			Name: &recordName,
			RecordSetProperties: &dns.RecordSetProperties{
				TTL: to.Int64Ptr(ttl),
			},
		}
	}
	return dns.RecordSet{}
}
