package utils

import (
	"context"
	"fmt"
	"net/http"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // Register all browser implementations
)

// CookieMap holds simple key-value cookies
type CookieMap map[string]string

// GetGeminiCookies attempts to find Gemini cookies (__Secure-1PSID, __Secure-1PSIDTS)
// from all available browsers.
func GetGeminiCookies() (CookieMap, error) {
	// Read cookies from all standard browsers
	// kooky v2 requires context
	cookies, err := kooky.ReadCookies(context.Background(), kooky.Valid, kooky.DomainHasSuffix("google.com"))
	if err != nil {
		return nil, fmt.Errorf("failed to read browser cookies: %w", err)
	}

	foundCookies := make(CookieMap)
	
	// Prioritize finding the specific auth cookies
	for _, cookie := range cookies {
		// Verify domain match loosely if kooky's filter wasn't strict enough
		// (kooky handles this, but good to be sure)
		
		if cookie.Name == "__Secure-1PSID" || cookie.Name == "__Secure-1PSIDTS" {
			// We might have multiple cookies with the same name for different domains or paths.
			// Usually the one for .google.com is essential.
			// In a real usage, we might need to filter by specific domain priority.
			// For now, simple overwrite or keep first valid.
			
			// Simple heuristics: prefer .google.com
			if cookie.Domain == ".google.com" || cookie.Domain == "gemini.google.com" {
				foundCookies[cookie.Name] = cookie.Value
			}
		}
	}

	if foundCookies["__Secure-1PSID"] == "" {
		return nil, fmt.Errorf("could not find __Secure-1PSID in any browser")
	}

	return foundCookies, nil
}

// ConvertToHttpCookies converts our simple map to http.Cookie slice for the client
func (cm CookieMap) ToHttpCookies() []*http.Cookie {
	var cookies []*http.Cookie
	for k, v := range cm {
		cookies = append(cookies, &http.Cookie{
			Name:   k,
			Value:  v,
			Domain: ".google.com",
			Path:   "/",
		})
	}
	return cookies
}
