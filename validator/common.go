package validator

import (
	"net/url"
	"regexp"
)

var (
	// emailRegex is a simple email validation regex
	// This is a basic pattern - for production use, consider a more comprehensive regex
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// IsEmail returns true if the string is a valid email address
func IsEmail(value string) bool {
	if Blank(value) {
		return false
	}
	return emailRegex.MatchString(value)
}

// IsURL returns true if the string is a valid URL
func IsURL(value string) bool {
	if Blank(value) {
		return false
	}
	u, err := url.Parse(value)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// IsHTTPURL returns true if the string is a valid HTTP or HTTPS URL
func IsHTTPURL(value string) bool {
	if Blank(value) {
		return false
	}
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// InRange returns true if the integer value is between min and max (inclusive)
func InRange(value, min, max int) bool {
	return value >= min && value <= max
}

// InRangeFloat returns true if the float value is between min and max (inclusive)
func InRangeFloat(value, min, max float64) bool {
	return value >= min && value <= max
}

// Between returns true if the integer value is between min and max (inclusive)
// Alias for InRange for better readability in some contexts
func Between(value, min, max int) bool {
	return InRange(value, min, max)
}

// BetweenFloat returns true if the float value is between min and max (inclusive)
// Alias for InRangeFloat for better readability in some contexts
func BetweenFloat(value, min, max float64) bool {
	return InRangeFloat(value, min, max)
}

// IsAlpha returns true if the string contains only alphabetic characters
func IsAlpha(value string) bool {
	if Blank(value) {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// IsAlphanumeric returns true if the string contains only alphanumeric characters
func IsAlphanumeric(value string) bool {
	if Blank(value) {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// IsNumeric returns true if the string contains only numeric characters
func IsNumeric(value string) bool {
	if Blank(value) {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
