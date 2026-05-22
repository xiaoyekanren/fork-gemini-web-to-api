package providers

import "testing"

func TestCookieStoreFillFromExtraCookies(t *testing.T) {
	store := &CookieStore{
		ExtraCookies: "__Secure-1PSID=psid-value; __Secure-1PSIDCC=psidcc-value; COMPASS=gemini-pd=abc",
	}

	store.FillFromExtraCookies()

	if store.Secure1PSID != "psid-value" {
		t.Fatalf("Secure1PSID was not filled: %q", store.Secure1PSID)
	}
	if store.Secure1PSIDCC != "psidcc-value" {
		t.Fatalf("Secure1PSIDCC was not filled: %q", store.Secure1PSIDCC)
	}
}

func TestCookieStoreToHTTPCookiesUsesExtraCookies(t *testing.T) {
	store := &CookieStore{
		Secure1PSID:  "explicit-psid",
		ExtraCookies: "__Secure-1PSID=extra-psid; APISID=apisid-value; COMPASS=gemini-pd=abc",
	}

	cookies := store.ToHTTPCookies()
	got := map[string]string{}
	for _, cookie := range cookies {
		got[cookie.Name] = cookie.Value
	}

	if got["__Secure-1PSID"] != "extra-psid" {
		t.Fatalf("extra cookie should override duplicate cookie values: %q", got["__Secure-1PSID"])
	}
	if got["APISID"] != "apisid-value" {
		t.Fatalf("APISID was not included: %q", got["APISID"])
	}
	if got["COMPASS"] != "gemini-pd=abc" {
		t.Fatalf("cookie value containing '=' was parsed incorrectly: %q", got["COMPASS"])
	}
}

func TestMergeCookiesKeepsValuesContainingEquals(t *testing.T) {
	merged := mergeCookieHeader("COMPASS=gemini-pd=abc; SID=old", "SID=new; NID=531=value")
	pairs := parseCookiePairs(merged)

	if pairs["COMPASS"] != "gemini-pd=abc" {
		t.Fatalf("COMPASS was parsed incorrectly: %q", pairs["COMPASS"])
	}
	if pairs["SID"] != "new" {
		t.Fatalf("SID was not overridden: %q", pairs["SID"])
	}
	if pairs["NID"] != "531=value" {
		t.Fatalf("NID was parsed incorrectly: %q", pairs["NID"])
	}
}
