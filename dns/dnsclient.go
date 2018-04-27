package dns

type DNSClient interface {
	LookupRecord(recordName string) (*string, error)
}
