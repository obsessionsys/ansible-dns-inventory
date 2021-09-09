package datasource

import (
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

const (
	// DNS TXT record type.
	dnsRrTxtType uint16 = 16
	// Number of the field that contains the TXT record value.
	dnsRrTxtField int = 1
)

type (
	// A DNS datasource implementation.
	DNS struct {
		// DNS client.
		Client *dns.Client
		// DNS zone transfer parameters.
		Transfer *dns.Transfer
		// Inventory configuration.
		Config types.Config
	}
)

// Process a single DNS resource record.
func (d *DNS) processRecord(rr dns.RR) *types.Record {
	var name, attrs string

	if d.Config.GetBool("dns.notransfer.enabled") {
		name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), d.Config.GetString("dns.notransfer.separator"))[0], ".")
		attrs = strings.Split(dns.Field(rr, dnsRrTxtField), d.Config.GetString("dns.notransfer.separator"))[1]
	} else {
		name = strings.TrimSuffix(rr.Header().Name, ".")
		attrs = dns.Field(rr, dnsRrTxtField)
	}

	return &types.Record{
		Hostname:   name,
		Attributes: attrs,
	}
}

// Process several DNS resource records.
func (d *DNS) processRecords(rrs []dns.RR) []*types.Record {
	records := make([]*types.Record, 0)

	for _, rr := range rrs {
		records = append(records, d.processRecord(rr))
	}

	return records
}

// Produce a fully qualified host name for use in DNS requests.
func (d *DNS) makeFQDN(host string, zone string) string {
	name := strings.TrimPrefix(host, ".")
	domain := strings.TrimPrefix(zone, ".")

	if len(domain) == 0 {
		return dns.Fqdn(name)
	}

	return strings.TrimPrefix(dns.Fqdn(name+"."+domain), ".")
}

// getZone acquires TXT records for all hosts in a specific zone.
func (d *DNS) getZone(zone string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)

	msg := new(dns.Msg)
	msg.SetAxfr(dns.Fqdn(zone))

	if d.Config.GetBool("dns.tsig.enabled") {
		d.Transfer.TsigSecret = map[string]string{d.Config.GetString("dns.tsig.key"): d.Config.GetString("dns.tsig.secret")}
		msg.SetTsig(d.Config.GetString("dns.tsig.key"), d.Config.GetString("dns.tsig.algo"), 300, time.Now().Unix())
	}

	// Perform the transfer.
	c, err := d.Transfer.In(msg, d.Config.GetString("dns.server"))
	if err != nil {
		return nil, errors.Wrap(err, "zone transfer failed")
	}

	// Process transferred records. Ignore anything that is not a TXT recordd. Ignore the special inventory record as well.
	for e := range c {
		for _, rr := range e.RR {
			if rr.Header().Rrtype == dnsRrTxtType && rr.Header().Name != d.makeFQDN(d.Config.GetString("dns.notransfer.host"), zone) {
				records = append(records, rr)
			}
		}
	}

	return records, nil
}

// getHost acquires all TXT records for a specific host.
func (d *DNS) getHost(host string) ([]dns.RR, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(host, dns.TypeTXT)

	rx, _, err := d.Client.Exchange(msg, d.Config.GetString("dns.server"))
	if err != nil {
		return nil, errors.Wrap(err, "dns request failed")
	}

	return rx.Answer, nil
}

// GetAllRecords acquires all available host records.
func (d *DNS) GetAllRecords() ([]*types.Record, error) {
	records := make([]*types.Record, 0)

	for _, zone := range d.Config.GetStringSlice("dns.zones") {
		var rrs []dns.RR
		var err error

		if d.Config.GetBool("dns.notransfer.enabled") {
			rrs, err = d.getHost(d.makeFQDN(d.Config.GetString("dns.notransfer.host"), zone))
		} else {
			rrs, err = d.getZone(d.makeFQDN("", zone))
		}
		if err != nil {
			// log.Printf("[%s] skipping zone: %v", zone, err)
			continue
		}

		records = append(records, d.processRecords(rrs)...)
	}

	return records, nil
}

// GetHostRecords acquires all available records for a specific host.
func (d *DNS) GetHostRecords(host string) ([]*types.Record, error) {
	records := make([]*types.Record, 0)
	var err error

	if d.Config.GetBool("dns.notransfer.enabled") {
		// No-transfer mode is enabled.
		var zone string
		var rrs []dns.RR

		// Determine which zone we are working with.
		for _, z := range d.Config.GetStringSlice("dns.zones") {
			if strings.HasSuffix(dns.Fqdn(host), dns.Fqdn(z)) {
				zone = z
				break
			}
		}

		if len(zone) == 0 {
			return nil, errors.New("failed to determine zone from hostname")
		}

		// Get no-transfer host records.
		rrs, err = d.getHost(d.makeFQDN(d.Config.GetString("dns.notransfer.host"), zone))
		if err != nil {
			return nil, err
		}

		// Filter out the irrelevant records.
		for _, rr := range rrs {
			name := strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), d.Config.GetString("dns.notransfer.separator"))[0], ".")
			if host == name {
				records = append(records, d.processRecord(rr))
			}
		}
	} else {
		// No-transfer mode is disabled, no special logic is needed.
		rrs, err := d.getHost(d.makeFQDN(host, ""))
		if err != nil {
			return nil, err
		}

		records = append(records, d.processRecords(rrs)...)
	}

	return records, nil
}

// Close datasource and perform housekeeping.
func (d *DNS) Close() {}
