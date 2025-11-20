package beatport

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	unicode2 "unicode"

	"github.com/mozillazg/go-unidecode"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type SanitizedString string
type Duration int
type NamingPreferences struct {
	Template           string
	Whitespace         string
	ArtistsLimit       int
	ArtistsShortForm   string
	TrackNumberPadding int
	KeySystem          string
	AsciiOnly          bool
}

func (d *Duration) Display() string {
	seconds := *d / 1000
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainingSeconds := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d-%02d-%02d", hours, minutes, remainingSeconds)
	}
	return fmt.Sprintf("%02d-%02d", minutes, remainingSeconds)
}

func (s *SanitizedString) UnmarshalJSON(data []byte) error {
	rawValue := string(bytes.Trim(data, `"`))
	r := strings.NewReplacer(
		"\\n", "",
		"\\r", "",
		"\\t", "",
	)
	sanitized := r.Replace(rawValue)
	*s = SanitizedString(strings.Join(strings.Fields(sanitized), " "))
	return nil
}

func (s *SanitizedString) String() string {
	return string(*s)
}

func SanitizeForPath(s string, sanitize bool) string {

	r := strings.NewReplacer(
		"\\", "",
		"/", "",
	)
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}

func SanitizePath(name string, whitespace string, asciiOnly bool) string {

	if len(name) > 250 {
		name = name[:250]
	}

	// Convert to ASCII if requested
	if asciiOnly {
		name = toASCII(name)
	}

	oldnew := []string{
		"<", "",
		">", "",
		":", "",
		"\"", "",
		"|", "",
		"?", "",
		"*", "",
	}

	if whitespace != "" {
		oldnew = append(oldnew, " ", whitespace)
	}

	r := strings.NewReplacer(oldnew...)
	name = r.Replace(name)

	return strings.Join(strings.Fields(name), " ")
}

// toASCII converts a string to pure ASCII by using robust Unicode normalization and transliteration
func toASCII(s string) string {
	if s == "" {
		return s
	}

	// Step 1: Unicode normalization (NFKD) to decompose characters
	normalized := norm.NFKD.String(s)

	// Step 2: Remove combining marks (diacritics) - category Mn (Mark, nonspacing)
	// This removes accents, umlauts, etc.
	removeMarks := runes.Remove(runes.In(unicode2.Mn))
	noMarks, _, err := transform.String(removeMarks, normalized)
	if err != nil {
		// Fallback: if transformation fails, use the normalized string
		noMarks = normalized
	}

	// Step 3: Use unidecode library for comprehensive transliteration
	// This handles characters that aren't covered by normalization alone
	ascii := unidecode.Unidecode(noMarks)

	// Step 4: Final cleanup - ensure only ASCII characters remain
	// Remove any remaining non-ASCII characters (though unidecode should handle most)
	var result strings.Builder
	for _, r := range ascii {
		if r <= unicode2.MaxASCII && unicode2.IsPrint(r) {
			result.WriteRune(r)
		}
		// Skip non-printable characters and any remaining non-ASCII
	}

	return result.String()
}

// ExtractFirstArtist extracts the first artist from a formatted artist string
func ExtractFirstArtist(artistString string) string {
	// Use 'Unknown' if artistString is empty
	if artistString == "" {
		artistString = "Unknown"
	}

	// Replace '&' with ','
	artistString = strings.ReplaceAll(artistString, "&", ",")

	// Split on comma and take first part
	parts := strings.SplitN(artistString, ",", 2)
	firstArtist := strings.TrimSpace(parts[0])

	// Remove leading digits, spaces, dots, dashes, underscores, parentheses, brackets
	charsToStrip := " -_()[]"
	firstArtist = strings.TrimLeft(firstArtist, charsToStrip)

	// Use 'Unknown' if result is empty
	if firstArtist == "" {
		firstArtist = "Unknown"
	}

	// Title case the result
	caser := cases.Title(language.Und, cases.NoLower)
	return caser.String(firstArtist)
}

func NumberWithPadding(value, total, padding int) string {
	if padding == 0 {
		padding = len(strconv.Itoa(total))
	}
	return fmt.Sprintf("%0*d", padding, value)
}

func ParseTemplate(template string, values map[string]string) string {
	re := regexp.MustCompile(`\{(\w+)}`)
	result := re.ReplaceAllStringFunc(template, func(placeholder string) string {
		key := strings.Trim(placeholder, "{}")
		if value, found := values[key]; found {
			return value
		}
		return placeholder
	})
	return result
}

func storeUrl(id int64, entity, slug string, store Store) string {
	var domain string
	switch store {
	default:
		domain = "beatport.com"
	case StoreBeatsource:
		domain = "beatsource.com"
	}
	return fmt.Sprintf("https://www.%s/%s/%s/%d", domain, entity, slug, id)
}
