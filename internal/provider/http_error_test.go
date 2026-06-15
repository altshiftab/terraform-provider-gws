package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	motmedelHttpContext "github.com/Motmedel/utils_go/pkg/http/context"
	motmedelHttpErrors "github.com/Motmedel/utils_go/pkg/http/errors"
	motmedelHttpTypes "github.com/Motmedel/utils_go/pkg/http/types"
)

// fetchLikeError reproduces the error chain the Motmedel fetch helpers build for
// a non-2xx response: the innermost error carries the HTTP context, and the
// client layers wrap it with plain fmt-wrapped messages.
func fetchLikeError(status string, statusCode int, header http.Header, body string) error {
	httpContext := &motmedelHttpTypes.HttpContext{
		Response:     &http.Response{Proto: "HTTP/2.0", Status: status, StatusCode: statusCode, Header: header},
		ResponseBody: []byte(body),
	}
	ctx := motmedelHttpContext.WithHttpContextValue(context.Background(), httpContext)
	inner := motmedelErrors.NewWithTraceCtx(ctx, &motmedelHttpErrors.Non2xxStatusCodeError{StatusCode: statusCode})
	wrapped := fmt.Errorf("fetch: %w", fmt.Errorf("fetch: %w", inner))
	return motmedelErrors.New(fmt.Errorf("fetch json with body: %w", wrapped), "https://example/url")
}

func TestApiErrorDetailIncludesResponse(t *testing.T) {
	header := http.Header{
		"Content-Type": {"application/json; charset=UTF-8"},
		"Www-Authenticate": {`Bearer realm="example"`},
	}
	err := fetchLikeError("409 Conflict", 409, header, `{"error":{"code":409,"message":"Member already exists."}}`)

	detail := apiErrorDetail(err)

	for _, want := range []string{
		"non-2xx status code",
		"HTTP/2.0 409 Conflict",
		"Content-Type: application/json; charset=UTF-8",
		`Www-Authenticate: Bearer realm="example"`,
		`"message": "Member already exists."`, // indented JSON
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("apiErrorDetail() = %q, missing %q", detail, want)
		}
	}
}

func TestApiErrorDetailWithoutHttpContext(t *testing.T) {
	err := fmt.Errorf("plain error")

	if got := apiErrorDetail(err); got != "plain error" {
		t.Errorf("apiErrorDetail() = %q, want %q", got, "plain error")
	}
}
