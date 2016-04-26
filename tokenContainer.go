package main

import (
	"strings"

	"github.com/digitalocean/godo"
	"github.com/miekg/dns"
)

type (
	// tokenContainer is a simple wrapper for keeping track of matched status.
	tokenContainer struct {
		*dns.Token
		matched bool
	}
)

// CreateRequest will create a godo.DomainRecordEditRequest based on the token.
func (t *tokenContainer) CreateRequest() *godo.DomainRecordEditRequest {
	req := &godo.DomainRecordEditRequest{}

	switch t.RR.(type) {
	case *dns.A:
		a := t.RR.(*dns.A)
		req.Type = "A"
		req.Name = a.Header().Name
		req.Data = a.A.String()
	case *dns.CNAME:
		cname := t.RR.(*dns.CNAME)
		req.Type = "CNAME"
		req.Name = cname.Header().Name
		req.Data = cname.Target
	case *dns.MX:
		mx := t.RR.(*dns.MX)
		req.Type = "MX"
		req.Name = mx.Header().Name
		req.Data = mx.Mx
		req.Priority = int(mx.Preference)
	case *dns.NS:
		ns := t.RR.(*dns.NS)
		req.Type = "NS"
		req.Name = ns.Header().Name
		req.Data = ns.Ns
	case *dns.TXT:
		txt := t.RR.(*dns.TXT)
		req.Type = "TXT"
		req.Name = txt.Header().Name
		req.Data = strings.Join(txt.Txt, " ")
	default:
		bail("%T added to zone reader, but not CreateRequest()", t.RR)
	}

	return req
}
