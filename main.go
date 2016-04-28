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

	if !yes {
		// Present work for the user.
		list := []string{}
		for _, rec := range dom.Records {
			if !rec.Matched {
				list = append(list, rec.String())
			}
		}

		if len(list) > 0 {
			fmt.Printf("Records to delete:\n")
			for _, r := range list {
				fmt.Printf("%s\n", r)
			}
			fmt.Printf("\n")
		}

		numChanges := len(list)

		list = nil
		for _, token := range zoneTokens {
			if !token.matched {
				list = append(list, token.String())
			}
		}
		if len(list) > 0 {
			fmt.Printf("Records to add:\n")
			for _, r := range list {
				fmt.Printf("%s\n", r)
			}
			fmt.Printf("\n")
		}

		numChanges += len(list)

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
	for _, rec := range dom.Records {
		if !rec.Matched {
			fmt.Printf("Deleting %+v\n", rec)
			err = rec.Delete(client)
			bailIfError(err)
		}
	}

	// Add zone records missing from DO.
	for _, token := range zoneTokens {
		if !token.matched {
			req := token.CreateRequest()

			fmt.Printf("Adding %+v\n", req)
			_, _, err := client.Domains.CreateRecord(zoneName.String(""), req)
			bailIfError(err)
		}
	}

	fmt.Printf("Zone synced.\n")
}
