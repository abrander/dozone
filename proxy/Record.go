package proxy

import (
	"context"
	"fmt"
	"net"

	"github.com/digitalocean/godo"
	"github.com/miekg/dns"
)

type (
	// Record represents a domain record from Digital Ocean with extra
	// properties to keep track of sync state.
	Record struct {
		godo.DomainRecord
		zoneName ZoneName
		Matched  bool
	}
)

// NewRecord will instantiate a new Record from a DomainRecord and a zoneName.
func NewRecord(record godo.DomainRecord, zoneName ZoneName) *Record {
	return &Record{
		DomainRecord: record,
		zoneName:     zoneName,
	}
}

// Matches will check if the record from DO matches a token from the zone file.
func (r *Record) Matches(token *dns.Token) bool {
	if r.zoneName.FQDN(r.Name) != token.Header().Name {
		return false
	}

	switch r.Type {
	case "A":
		a, ok := token.RR.(*dns.A)
		if !ok {
			return false
		}

		if net.ParseIP(r.Data).Equal(a.A) {
			return true
		}

	case "AAAA":
		aaaa, ok := token.RR.(*dns.AAAA)
		if !ok {
			return false
		}

		if net.ParseIP(r.Data).Equal(aaaa.AAAA) {
			return true
		}

	case "CNAME":
		cname, ok := token.RR.(*dns.CNAME)
		if !ok {
			return false
		}

		if cname.Target == r.zoneName.FQDN(r.Data) {
			return true
		}

	case "NS":
		ns, ok := token.RR.(*dns.NS)
		if !ok {
			return false
		}

		if ns.Ns == r.Data+"." {
			return true
		}

	case "MX":
		mx, ok := token.RR.(*dns.MX)
		if !ok {
			return false
		}

		if int(mx.Preference) != r.Priority {
			return false
		}

		if mx.Mx == r.Data+"." {
			return true
		}

		return false

	case "SRV":
		srv, ok := token.RR.(*dns.SRV)
		if !ok {
			return false
		}

		if int(srv.Priority) != r.Priority {
			return false
		}

		if int(srv.Port) != r.Port {
			return false
		}

		if srv.Target == r.Data+"." {
			return true
		}

		return false

	case "TXT":
		txt, ok := token.RR.(*dns.TXT)
		if !ok {
			return false
		}

		if txt.Txt[0] == r.Data {
			return true
		}

	default:
		fmt.Printf("Unknown type: %s\n", r.Type)

		return false
	}

	return false
}

// Delete will delete the record at Digital Ocean
func (r *Record) Delete(client *godo.Client) error {
	_, err := client.Domains.DeleteRecord(context.TODO(), r.zoneName.String(""), r.ID)
	return err
}
