package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	altshiftErrors "github.com/altshiftab/utils_go/pkg/errors"
	altshiftHttpContext "github.com/altshiftab/utils_go/pkg/http/context"
	altshiftHttpErrors "github.com/altshiftab/utils_go/pkg/http/errors"
	altshiftHttpTypes "github.com/altshiftab/utils_go/pkg/http/types"
)

// errTransport stands for a failure that never reached a response, and so
// carries no HTTP context to read a status from.
var errTransport = errors.New("dial tcp: connection refused")

// fetchLikeError reproduces the error chain the altshift fetch helpers build for
// a non-2xx response: the innermost error carries the HTTP context, and the
// client layers wrap it with plain fmt-wrapped messages.
func fetchLikeError(status string, statusCode int, header http.Header, body string) error {
	httpContext := &altshiftHttpTypes.HttpContext{
		Response:     &http.Response{Proto: "HTTP/2.0", Status: status, StatusCode: statusCode, Header: header},
		ResponseBody: []byte(body),
	}
	ctx := altshiftHttpContext.WithHttpContextValue(context.Background(), httpContext)
	inner := altshiftErrors.NewWithTraceCtx(ctx, &altshiftHttpErrors.Non2xxStatusCodeError{StatusCode: statusCode})
	wrapped := fmt.Errorf("fetch: %w", fmt.Errorf("fetch: %w", inner))
	return altshiftErrors.New(fmt.Errorf("fetch json with body: %w", wrapped), "https://example/url")
}

func TestApiErrorDetailIncludesResponse(t *testing.T) {
	header := http.Header{
		"Content-Type":     {"application/json; charset=UTF-8"},
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

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "a 404 is a missing resource",
			err:  fetchLikeError("404 Not Found", http.StatusNotFound, nil, `{"error":{"code":404}}`),
			want: true,
		},
		{
			name: "another status is not",
			err:  fetchLikeError("403 Forbidden", http.StatusForbidden, nil, `{"error":{"code":403}}`),
		},
		{
			name: "an error carrying no response is not",
			err:  errTransport,
		},
		{
			name: "no error is not",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := isNotFound(testCase.err); got != testCase.want {
				t.Errorf("isNotFound() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestApiErrorDetailWithoutHttpContext(t *testing.T) {
	err := fmt.Errorf("plain error")

	if got := apiErrorDetail(err); got != "plain error" {
		t.Errorf("apiErrorDetail() = %q, want %q", got, "plain error")
	}
}
