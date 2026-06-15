package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	motmedelHttpContext "github.com/Motmedel/utils_go/pkg/http/context"
	motmedelHttpTypes "github.com/Motmedel/utils_go/pkg/http/types"
)

// httpContextFromError walks the error chain for the HTTP context that the
// Motmedel fetch helpers attach to errors. It carries the request and response
// along with their bodies, which the API call sites otherwise discard.
func httpContextFromError(err error) *motmedelHttpTypes.HttpContext {
	var contextErr motmedelErrors.ContextErrorI
	if !errors.As(err, &contextErr) {
		return nil
	}

	ctxPtr := contextErr.GetContext()
	if ctxPtr == nil {
		return nil
	}

	httpContext, _ := (*ctxPtr).Value(motmedelHttpContext.HttpContextContextKey).(*motmedelHttpTypes.HttpContext)
	return httpContext
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
