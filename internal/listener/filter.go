package listener

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/APRSCN/aprsutils"
	"github.com/APRSCN/aprsutils/parser"
)

// Filter checks if the packet matches the given APRS-IS filter specifications
func Filter(filter string, packet parser.Parsed) bool {
	if filter == "" || filter == "default" {
		return false // Default filter passes nothing additional
	}

	// Split multiple filters
	filterParts := strings.Fields(filter)

	// Separate include and exclude filters
	var includeFilters, excludeFilters []string

	for _, part := range filterParts {
		if strings.HasPrefix(part, "-") {
			excludeFilters = append(excludeFilters, part[1:])
		} else {
			includeFilters = append(includeFilters, part)
		}
	}

	// No include filters means no additional data is passed
	if len(includeFilters) == 0 {
		return false
	}

	// Check if packet matches any include filter
	matched := false
	for _, filterSpec := range includeFilters {
		if matchFilter(filterSpec, packet) {
			matched = true
			break
		}
	}

	if !matched {
		return false
	}

	// Check if packet is excluded by any exclude filter
	for _, filterSpec := range excludeFilters {
		if matchFilter(filterSpec, packet) {
			return false
		}
	}

	return true
}

// matchFilter checks if a packet matches a single filter specification
func matchFilter(filterSpec string, packet parser.Parsed) bool {
	if len(filterSpec) < 2 {
		return false
	}

	filterType := filterSpec[0:1]
	params := strings.Split(filterSpec[2:], "/")

	switch filterType {
	case "r": // Range filter - r/lat/lon/dist
		return matchRangeFilter(params, packet)
	case "p": // Prefix filter - p/aa/bb/cc...
		return matchPrefixFilter(params, packet)
	case "b": // Buddy list filter - b/call1/call2...
		return matchBuddyFilter(params, packet)
	case "o": // Object filter - o/obj1/obj2...
		return matchObjectFilter(params, packet)
	case "os": // Strict object filter - os/obj1/obj2...
		return matchStrictObjectFilter(params, packet)
	case "t": // Type filter - t/poimqstunw or t/poimqstuw/call/km
		return matchTypeFilter(params, packet)
	case "s": // Symbol filter - s/pri/alt/over
		return matchSymbolFilter(params, packet)
	case "d": // Digipeater filter - d/digi1/digi2...
		return matchDigipeaterFilter(params, packet)
	case "a": // Area filter - a/latN/lonW/latS/lonE
		return matchAreaFilter(params, packet)
	case "e": // Entry station filter - e/call1/call2...
		return matchEntryFilter(params, packet)
	case "g": // Group message filter - g/call1/call2...
		return matchGroupFilter(params, packet)
	case "u": // Unproto filter - u/unproto1/unproto2...
		return matchUnprotoFilter(params, packet)
	case "q": // q Construct filter - q/con/I
		return matchQConstructFilter(params, packet)
	case "m": // My Range filter - m/dist
		return matchMyRangeFilter(params, packet)
	case "f": // Friend Range filter - f/call/dist
		return matchFriendRangeFilter(params, packet)
	default:
		return false
	}
}

// Range filter - passes packets within specified distance from coordinates
func matchRangeFilter(params []string, packet parser.Parsed) bool {
	if len(params) < 3 || packet.Lat == 0 || packet.Lon == 0 {
		return false
	}

	lat, err := strconv.ParseFloat(params[0], 64)
	if err != nil {
		return false
	}

	lon, err := strconv.ParseFloat(params[1], 64)
	if err != nil {
		return false
	}

	dist, err := strconv.ParseFloat(params[2], 64)
	if err != nil {
		return false
	}

	// Calculate distance using Haversine formula
	distance := aprsutils.CalculateDistance(lat, lon, packet.Lat, packet.Lon)
	return distance <= dist
}

// Prefix filter - passes packets with callsign starting with specified prefixes
func matchPrefixFilter(params []string, packet parser.Parsed) bool {
	for _, prefix := range params {
		if strings.HasPrefix(packet.From, prefix) {
			return true
		}
	}
	return false
}

// Buddy list filter - passes packets from exact callsigns (supports wildcards)
func matchBuddyFilter(params []string, packet parser.Parsed) bool {
	for _, call := range params {
		if wildcardMatch(call, packet.From) {
			return true
		}
	}
	return false
}

// Object filter - passes objects with specified names (supports wildcards)
func matchObjectFilter(params []string, packet parser.Parsed) bool {
	if packet.ObjectName == "" {
		return false
	}

	for _, obj := range params {
		obj = strings.ReplaceAll(obj, "|", "/")
		obj = strings.ReplaceAll(obj, "~", "*")
		if wildcardMatch(obj, packet.ObjectName) {
			return true
		}
	}
	return false
}

// Strict object filter - passes objects with exact names (supports wildcards)
func matchStrictObjectFilter(params []string, packet parser.Parsed) bool {
	if packet.ObjectName == "" {
		return false
	}

	for _, obj := range params {
		obj = strings.ReplaceAll(obj, "|", "/")
		obj = strings.ReplaceAll(obj, "~", "*")
		if wildcardMatch(obj, packet.ObjectName) {
			return true
		}
	}
	return false
}

// Type filter - passes packets based on packet type
func matchTypeFilter(params []string, packet parser.Parsed) bool {
	if len(params) == 0 {
		return false
	}

	typeStr := params[0]

	// Check packet type against each character in type string
	for _, char := range typeStr {
		switch char {
		case 'p': // Position packets
			if packet.Format == "position" {
				return true
			}
		case 'o': // Objects
			if packet.ObjectFormat != "" {
				return true
			}
		case 'i': // Items
			// TODO: Implement item type detection
			return true
		case 'm': // Message
			if packet.MessageText != "" {
				return true
			}
		case 'q': // Query
			if packet.Format == "query" {
				return true
			}
		case 's': // Status
			if packet.Status != "" {
				return true
			}
		case 't': // Telemetry
			if len(packet.TelemetryMicE) > 0 || len(packet.TPARM) > 0 {
				return true
			}
		case 'u': // User-defined
			// TODO: Implement user-defined type detection
			return true
		case 'n': // NWS format messages and objects
			if strings.Contains(packet.Comment, "NWS") {
				return true
			}
		case 'w': // Weather
			if len(packet.Weather) > 0 {
				return true
			}
		}
	}

	return false
}

// Symbol filter - passes packets based on symbol table and overlay
func matchSymbolFilter(params []string, packet parser.Parsed) bool {
	if len(packet.Symbol) < 2 {
		return false
	}

	if len(params) > 0 && params[0] != "" {
		// Check primary symbol table
		if !strings.Contains(params[0], packet.Symbol[0]) {
			return false
		}
	}

	if len(params) > 1 && params[1] != "" {
		// Check alternate symbol table
		if !strings.Contains(params[1], packet.Symbol[1]) {
			return false
		}
	}

	return true
}

// Digipeater filter - passes packets digipeated by specified stations
func matchDigipeaterFilter(params []string, packet parser.Parsed) bool {
	for _, digi := range params {
		for _, path := range packet.Path {
			if wildcardMatch(digi, path) {
				return true
			}
		}
	}
	return false
}

// Area filter - passes packets within specified bounding box
func matchAreaFilter(params []string, packet parser.Parsed) bool {
	if len(params) < 4 || packet.Lat == 0 || packet.Lon == 0 {
		return false
	}

	latN, _ := strconv.ParseFloat(params[0], 64)
	lonW, _ := strconv.ParseFloat(params[1], 64)
	latS, _ := strconv.ParseFloat(params[2], 64)
	lonE, _ := strconv.ParseFloat(params[3], 64)

	return packet.Lat <= latN && packet.Lat >= latS &&
		packet.Lon >= lonW && packet.Lon <= lonE
}

// Entry station filter - passes packets received by specified IGates
func matchEntryFilter(params []string, packet parser.Parsed) bool {
	// Check first digipeater in path (receiving IGate)
	if len(packet.Path) == 0 {
		return false
	}

	firstDigi := packet.Path[0]
	for _, call := range params {
		if wildcardMatch(call, firstDigi) {
			return true
		}
	}
	return false
}

// Group message filter - passes messages addressed to specified call signs
func matchGroupFilter(params []string, packet parser.Parsed) bool {
	if packet.Addressee == "" {
		return false
	}

	for _, call := range params {
		if wildcardMatch(call, packet.Addressee) {
			return true
		}
	}
	return false
}

// Unproto filter - passes packets with specified destination call signs
func matchUnprotoFilter(params []string, packet parser.Parsed) bool {
	for _, unproto := range params {
		if wildcardMatch(unproto, packet.To) {
			return true
		}
	}
	return false
}

// q Construct filter - passes packets with specified q constructs
func matchQConstructFilter(params []string, packet parser.Parsed) bool {
	// Check for q constructs in raw packet data
	if len(params) > 0 && params[0] != "" {
		// Check for specified q constructs
		if strings.Contains(packet.Raw, "q"+params[0]) {
			return true
		}
	}

	if len(params) > 1 && params[1] == "I" {
		// Check for IGate identification constructs
		if strings.Contains(packet.Raw, "qAr") ||
			strings.Contains(packet.Raw, "qAo") ||
			strings.Contains(packet.Raw, "qAR") {
			return true
		}
	}

	return false
}

// My Range filter - passes packets within distance from client's position
func matchMyRangeFilter(params []string, packet parser.Parsed) bool {
	// TODO: Requires client position information not available in packet struct
	return false
}

// Friend Range filter - passes packets within distance from friend's position
func matchFriendRangeFilter(params []string, packet parser.Parsed) bool {
	// TODO: Requires friend position information not available in packet struct
	return false
}

// wildcardMatch performs wildcard pattern matching with support for *
func wildcardMatch(pattern, text string) bool {
	if pattern == "*" {
		return true
	}

	// Convert wildcard pattern to regex
	if strings.Contains(pattern, "*") {
		regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
		matched, _ := regexp.MatchString(regexPattern, text)
		return matched
	}

	return pattern == text
}
