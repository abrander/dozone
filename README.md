# dozone
Tool for downloading, uploading and syncing DNS zones to Digital Ocean.

# Usage

Setting the access token (get one [here](https://cloud.digitalocean.com/settings/api/tokens)):

`$ export DIGITALOCEAN_ACCESS_TOKEN=123456789abcdef123456789abcdef123456789abcdef123456789abcdef1234`

Retrieve a zone file for the zone example.com from Digital Ocean:

`$ dozone -download example.com > example.com.zone`

Synchronize local example.com-zone file to Digital Ocean:

`$ dozone example.com.zone`

Synchronize local zone without asking:

`$ dozone -yes example.com.zone`

# Limitations

Only `A`, `CNAME`, `MX`, `NS` and `TXT` records are supported for now. It's trivial to add more. Pull Requests for `AAAA` and `SRV` records would be appreciated :)

Due to a bug in the Digital Ocean API, this tool cannot compare CNAME records. CNAME records will always be deleted and rewritten for now. Digital Ocean is aware of this problem, and is working on a solution.
