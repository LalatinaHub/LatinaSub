package sandbox

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/region"
)

var orgCleanRegex = regexp.MustCompile(`[^a-zA-Z0-9\s\.\-]+`)

// CountryToEmoji converts an ISO 3166-1 alpha-2 country code to its corresponding Unicode flag emoji.
// Falls back to "🌐" if invalid.
func CountryToEmoji(countryCode string) string {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if len(code) != 2 {
		return "🌐"
	}
	r1 := rune(code[0])
	r2 := rune(code[1])
	if r1 < 'A' || r1 > 'Z' || r2 < 'A' || r2 > 'Z' {
		return "🌐"
	}
	return string(rune(0x1F1E6+(r1-'A'))) + string(rune(0x1F1E6+(r2-'A')))
}

// SanitizeOrg cleans the AsOrganization string of any unusual characters or excessive spacing.
func SanitizeOrg(org string) string {
	cleaned := orgCleanRegex.ReplaceAllString(org, " ")
	return strings.Join(strings.Fields(cleaned), " ")
}

// LookupRegion attempts to find the city or region name using IATA airport code lookup.
func LookupRegion(iataCode string) string {
	if reg, ok := region.Lookup(iataCode); ok && reg != "" {
		return reg
	}
	return strings.ToUpper(strings.TrimSpace(iataCode))
}

// FormatRemark generates a standardized, informative remark string for a tested proxy node.
// Format: "[Index] [Flag] [Org] [Transport] [Mode] [TLS]"
func FormatRemark(index int, node *model.ProxyNode, countryCode, org, connMode string) string {
	var parts []string

	if index > 0 {
		parts = append(parts, fmt.Sprintf("[%d]", index))
	}

	flag := CountryToEmoji(countryCode)
	if flag != "" {
		parts = append(parts, flag)
	}

	cleanOrg := SanitizeOrg(org)
	if cleanOrg != "" {
		parts = append(parts, cleanOrg)
	}

	transport := strings.ToUpper(strings.TrimSpace(node.Transport))
	if transport == "" {
		transport = strings.ToUpper(strings.TrimSpace(node.VPN))
	}
	if transport != "" {
		parts = append(parts, transport)
	}

	if connMode != "" {
		parts = append(parts, strings.ToUpper(connMode))
	}

	if node.TLS {
		parts = append(parts, "TLS")
	}

	return strings.Join(parts, " ")
}

// EnrichNode populates geolocation, organization, region, connection mode, and formatted remark into node.
func EnrichNode(index int, node *model.ProxyNode, geo GeoIPResult, passedModes []string, iataCode string) {
	if node == nil {
		return
	}

	node.IP = geo.IP
	node.CountryCode = strings.ToUpper(strings.TrimSpace(geo.Country))
	node.Org = SanitizeOrg(geo.AsOrganization)

	if iataCode != "" {
		node.Region = LookupRegion(iataCode)
	} else if node.CountryCode != "" {
		node.Region = node.CountryCode
	}

	modeStr := strings.Join(passedModes, ",")
	node.ConnMode = modeStr
	node.Remark = FormatRemark(index, node, node.CountryCode, node.Org, modeStr)
}
