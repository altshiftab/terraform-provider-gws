package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	altshiftErrors "github.com/altshiftab/utils_go/pkg/errors"
	altshiftHttpContext "github.com/altshiftab/utils_go/pkg/http/context"
	altshiftHttpTypes "github.com/altshiftab/utils_go/pkg/http/types"
)

// httpContextFromError walks the error chain for the HTTP context that the
// altshift fetch helpers attach to errors. It carries the request and response
// along with their bodies, which the API call sites otherwise discard.
func httpContextFromError(err error) *altshiftHttpTypes.HttpContext {
	var contextErr altshiftErrors.ContextErrorI
	if !errors.As(err, &contextErr) {
		return nil
	}

	ctxPtr := contextErr.GetContext()
	if ctxPtr == nil {
		return nil
	}

	httpContext, _ := (*ctxPtr).Value(altshiftHttpContext.HttpContextContextKey).(*altshiftHttpTypes.HttpContext)
	return httpContext
}

// isNotFound reports whether err came from a 404 response. The fetch helpers
// surface every non-2xx status as an error, so a resource that no longer exists
// arrives here as a failure rather than as an empty result — without this, a
// Read of something deleted out of band fails the whole plan instead of
// planning the resource back.
func isNotFound(err error) bool {
	httpContext := httpContextFromError(err)
	if httpContext == nil || httpContext.Response == nil {
		return false
	}

	return httpContext.Response.StatusCode == http.StatusNotFound
}

// apiErrorDetail renders err for a Terraform diagnostic. When the error carries
// an HTTP context it appends the response as a raw HTTP dump (status line,
// headers, then body), which for the Google APIs is a JSON document describing
// the actual failure.
func apiErrorDetail(err error) string {
	if err == nil {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(err.Error())

	httpContext := httpContextFromError(err)
	if httpContext == nil {
		return builder.String()
	}

	if response := httpContext.Response; response != nil {
		statusLine := strings.TrimSpace(response.Proto + " " + response.Status)
		if statusLine == "" {
			statusLine = fmt.Sprintf("%d %s", response.StatusCode, http.StatusText(response.StatusCode))
		}
		fmt.Fprintf(&builder, "\n\n%s", statusLine)

		keys := make([]string, 0, len(response.Header))
		for key := range response.Header {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			for _, value := range response.Header[key] {
				fmt.Fprintf(&builder, "\n%s: %s", key, value)
			}
		}
	}

	if body := bytes.TrimSpace(httpContext.ResponseBody); len(body) > 0 {
		var indented bytes.Buffer
		if json.Indent(&indented, body, "", "  ") == nil {
			body = indented.Bytes()
		}
		builder.WriteString("\n\n")
		builder.Write(body)
	}

	return builder.String()
}
