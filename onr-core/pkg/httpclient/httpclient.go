package httpclient

import "net/http"

// HTTPDoer captures the subset of *http.Client the query packages rely on.
// The next-router codebase injects fake implementations of this interface
// so it can run offline verifications without making upstream requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
