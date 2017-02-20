package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/digitalocean/godo"
	"github.com/miekg/dns"
	"golang.org/x/oauth2"

	"github.com/abrander/dozone/proxy"
)

const (
	patEnvName = "DIGITALOCEAN_ACCESS_TOKEN"
)

var (
	pat tokenSource

	client    *godo.Client
	debugHTTP = true
	yes       = true
	download  string
)

func init() {
	pat = tokenSource(os.Getenv(patEnvName))

	if pat == "" {
		bail("Please set the environment variable: %s\n", patEnvName)
	}

	flag.BoolVar(&debugHTTP, "debugHTTP", false, "Output Digital Ocean API requests for debugging")
	flag.BoolVar(&yes, "yes", false, "Don't ask before committing")
	flag.StringVar(&download, "download", "", "Download zone")

	flag.Parse()
}

func getClient() *godo.Client {
	if client != nil {
		return client
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, pat)
	client = godo.NewClient(oauthClient)

	if debugHTTP {
		client.OnRequestCompleted(func(req *http.Request, resp *http.Response) {
			data, err := httputil.DumpRequestOut(req, true)
			if err == nil {
				fmt.Printf("\033[33m%s\033[0m\n", data)
			} else {
				log.Fatalf("\033[33mERROR: %s\033[0m\n", err.Error())
			}

			data, err = httputil.DumpResponse(resp, true)
			if err == nil {
				fmt.Printf("\033[32m%s\033[0m\n", data)
			} else {
				log.Fatalf("\033[32mERROR: %s\033[0m\n", err.Error())
			}
		})
	}

	return client
}

func bail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func bailIfError(err error) {
	if err != nil {
		bail("Error: %s\n", err.Error())
	}
}

func main() {
	client := getClient()

	if download != "" {
		dom := proxy.NewDomain(proxy.NewZoneName(download))

		err := dom.Find(client)
		bailIfError(err)

		fmt.Printf("%s", dom.ZoneFile)

		os.Exit(0)
	}

	filename := flag.Arg(0)
	r, err := os.Open(filename)
	bailIfError(err)
	defer r.Close()

	var zoneTokens []*tokenContainer
	var zoneName proxy.ZoneName

	for token := range dns.ParseZone(r, "", "") {
		if token.Error != nil {
			fmt.Printf("%s\n", token.Error.Error())
		} else {
			switch token.RR.(type) {
			case *dns.A:
				zoneTokens = append(zoneTokens, &tokenContainer{Token: token})
			case *dns.CNAME:
				zoneTokens = append(zoneTokens, &tokenContainer{Token: token})
			case *dns.MX:
				zoneTokens = append(zoneTokens, &tokenContainer{Token: token})
			case *dns.NS:
				zoneTokens = append(zoneTokens, &tokenContainer{Token: token})
			case *dns.SOA:
				soa := token.RR.(*dns.SOA)
				trimmed := strings.Trim(soa.Header().Name, ".")
				if trimmed != "" {
					zoneName = proxy.NewZoneName(soa.Header().Name)
				}
			case *dns.TXT:
				zoneTokens = append(zoneTokens, &tokenContainer{Token: token})
			default:
				fmt.Printf("%T not supported yet\n", token.RR)
			}
		}
	}

	if zoneName.String("") == "" {
		bail("Could not derive zone name from zone file")
	}

	dom := proxy.NewDomain(zoneName)

	err = dom.FindOrAdd(client)
	bailIfError(err)

	err = dom.RefreshRecords(client)
	bailIfError(err)

	// Match the zone with records retrieved from DO.
	for _, rec := range dom.Records {
		for _, token := range zoneTokens {
			if !token.matched && rec.Matches(token.Token) {
				rec.Matched = true
				token.matched = true
			}
		}
	}

	// Find records to delete.
	toDelete := make(map[godo.DomainRecord]bool)
	for _, rec := range dom.Records {
		if !rec.Matched {
			toDelete[rec.DomainRecord] = true
		}
	}

	// Find records to add.
	toAdd := make(map[*tokenContainer]*godo.DomainRecordEditRequest)
	for _, token := range zoneTokens {
		if !token.matched {
			toAdd[token] = token.CreateRequest()
		}
	}

	// Find records to edit
	toEdit := make(map[godo.DomainRecord]*godo.DomainRecordEditRequest)
	for rec := range toDelete {
		for token := range toAdd {
			req := token.CreateRequest()
			if req.Name == zoneName.FQDN(rec.Name) {
				toEdit[rec] = token.CreateRequest()

				// Remove from add/delete maps.
				delete(toAdd, token)
				delete(toDelete, rec)
			}
		}
	}

	if !yes {
		// Present work for the user.
		if len(toDelete) > 0 {
			fmt.Printf("Records to delete:\n")
			for r := range toDelete {
				fmt.Printf("%s\n", r.String())
			}
			fmt.Printf("\n")
		}

		numChanges := len(toDelete)

		if len(toAdd) > 0 {
			fmt.Printf("Records to add:\n")
			for _, r := range toAdd {
				fmt.Printf("%s\n", r)
			}
			fmt.Printf("\n")
		}

		numChanges += len(toAdd)

		if len(toEdit) > 0 {
			fmt.Printf("Records to update:\n")
			for from, to := range toEdit {
				fmt.Printf("%s -> %s\n", from, to)
			}
			fmt.Printf("\n")
		}

		numChanges += len(toEdit)

		if numChanges > 0 {
			fmt.Printf("%d change(s). Continue (y/N)? ", numChanges)

			bio := bufio.NewReader(os.Stdin)
			line, _, _ := bio.ReadLine()

			// If we get anything but y/Y we abort.
			if !(string(line) == "y" || string(line) == "Y") {
				bail("Aborting")
			}
		}
	}

	// Delete DO records not present in the zone.
	for rec := range toDelete {
		fmt.Printf("Deleting %+v\n", rec)
		_, err = client.Domains.DeleteRecord(string(zoneName), rec.ID)
		bailIfError(err)
	}

	// Add zone records missing from DO.
	for _, req := range toAdd {
		fmt.Printf("Adding %+v\n", req)
		_, _, err := client.Domains.CreateRecord(zoneName.String(""), req)
		bailIfError(err)
	}

	// Edit records.
	for from, to := range toEdit {
		fmt.Printf("Updating %+v\n", from)
		_, _, err := client.Domains.EditRecord(zoneName.String(""), from.ID, to)
		bailIfError(err)
	}

	fmt.Printf("Zone synced.\n")
}
