package proxy

import (
	"errors"

	"github.com/digitalocean/godo"
)

type (
	// Domain represents a DNS domain from Digital Ocean.
	Domain struct {
		*godo.Domain
		ZoneName ZoneName
		Records  []*Record
	}
)

var (
	// ErrZoneNotFound will be returned if the zone cannot be found at Digital Ocean.
	ErrZoneNotFound = errors.New("Zone not found")
)

// NewDomain will instantiate a new Domain from the specified zoneName.
func NewDomain(zoneName ZoneName) *Domain {
	return &Domain{
		ZoneName: zoneName,
	}
}

// Find will try to find the matching domain at Digital Ocean.
func (d *Domain) Find(client *godo.Client) error {
	// Check if the domain is registered at Digital Ocean.
	domains, _, err := client.Domains.List(nil)
	if err != nil {
		return err
	}

	for _, domain := range domains {
		if d.ZoneName == NewZoneName(domain.Name) {
			return nil
		}
	}

	return ErrZoneNotFound
}

// Add will add a new zone/domain to Digital Ocean.
func (d *Domain) Add(client *godo.Client) error {
	req := godo.DomainCreateRequest{
		Name:      d.ZoneName.String(""),
		IPAddress: "127.0.0.1", // FIXME: Try to add something that makes more sense
	}

	domain, _, err := client.Domains.Create(&req)
	if err != nil {
		return err
	}

	d.Domain = domain

	return nil
}

// FindOrAdd will search for the named zone at Digital Ocean - and add it if not
// found.
func (d *Domain) FindOrAdd(client *godo.Client) error {
	err := d.Find(client)
	if err == ErrZoneNotFound {
		return d.Add(client)
	}

	return nil
}

// RefreshRecords retrieves all records from Digital Ocean.
func (d *Domain) RefreshRecords(client *godo.Client) error {
	// create options. initially, these will be blank.
	opt := &godo.ListOptions{
		// DO doesn't support 10.000 entries per page (yet). It will be clamped
		// to 200 (for now), but we set it anyway. Maybe they will some time in
		// the future.
		PerPage: 10000,
	}

	d.Records = nil
	for {
		records, resp, err := client.Domains.Records(d.ZoneName.String(""), opt)
		if err != nil {
			return err
		}

		for _, dr := range records {
			record := NewRecord(dr, d.ZoneName)
			d.Records = append(d.Records, record)
		}

		// if we are at the last page, break out of the for loop.
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return err
		}

		// set the page we want for the next request
		opt.Page = page + 1
	}

	// If we arrived here, everything must be good :)
	return nil
}
