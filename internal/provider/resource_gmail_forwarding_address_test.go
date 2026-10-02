package provider

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail/gmail_config"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail/types/forwarding_address"
	"github.com/altshiftab/utils_go/pkg/http/types/fetch_config"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

const testForwardingEmail = "inbox.ver.1@arkivplats.example"

// forwardingAddressHandler answers the forwardingAddresses calls. A nil get stands for the 404 Gmail
// returns for an address the user does not have.
type forwardingAddressHandler struct {
	get          *forwarding_address.ForwardingAddress
	createStatus int
	deleteStatus int
	createCalls  int
	deleteCalls  int
}

func (h *forwardingAddressHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createCalls++
		if h.createStatus != 0 && h.createStatus != http.StatusOK {
			w.WriteHeader(h.createStatus)
			_, _ = w.Write([]byte(`{"error":{"message":"Forwarding address already exists."}}`))
			return
		}
		writeJson(w, &forwarding_address.ForwardingAddress{
			ForwardingEmail:    testForwardingEmail,
			VerificationStatus: forwarding_address.VerificationStatusPending,
		})
	case http.MethodDelete:
		h.deleteCalls++
		w.WriteHeader(h.deleteStatus)
	default:
		if h.get == nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Not Found"}}`))
			return
		}
		writeJson(w, h.get)
	}
}

func testForwardingAddressResource(t *testing.T, handler http.Handler) *gmailForwardingAddressResource {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseUrl, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return &gmailForwardingAddressResource{
		client:       gmail.NewClient(gmail_config.WithBaseUrl(baseUrl)),
		providerData: &gwsProviderData{fetchOption: fetch_config.WithHttpClient(server.Client())},
	}
}

func forwardingAddressSchema(t *testing.T) schema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	(&gmailForwardingAddressResource{}).Schema(t.Context(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func hasWarning(diagnostics diag.Diagnostics, summary string) bool {
	for _, warning := range diagnostics.Warnings() {
		if strings.Contains(warning.Summary(), summary) {
			return true
		}
	}
	return false
}

func TestGmailForwardingAddressResourceCreate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		handler     *forwardingAddressHandler
		wantError   bool
		wantStatus  string
		wantPending bool
		wantCreates int
	}{
		{
			name:        "a new external address is pending and says so",
			handler:     &forwardingAddressHandler{},
			wantStatus:  "pending",
			wantPending: true,
			wantCreates: 1,
		},
		{
			// Added by hand before Terraform knew of it: an error, which import resolves, rather than
			// a silent adoption that a later destroy would act on.
			name:        "an address that already exists is reported",
			handler:     &forwardingAddressHandler{createStatus: http.StatusConflict},
			wantError:   true,
			wantCreates: 1,
		},
		{
			name:        "a refusal is reported",
			handler:     &forwardingAddressHandler{createStatus: http.StatusForbidden},
			wantError:   true,
			wantCreates: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			r := testForwardingAddressResource(t, testCase.handler)
			addressSchema := forwardingAddressSchema(t)
			raw := groupMemberObject(t, addressSchema, map[string]string{
				"user_id":          "me",
				"forwarding_email": testForwardingEmail,
			})

			resp := &resource.CreateResponse{State: tfsdk.State{Schema: addressSchema}}
			r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: addressSchema, Raw: raw}}, resp)

			if testCase.handler.createCalls != testCase.wantCreates {
				t.Errorf("create calls = %d, want %d", testCase.handler.createCalls, testCase.wantCreates)
			}
			if got := resp.Diagnostics.HasError(); got != testCase.wantError {
				t.Fatalf("HasError() = %v, want %v (%v)", got, testCase.wantError, resp.Diagnostics)
			}
			if testCase.wantError {
				return
			}
			if got := stateAttribute(t, resp.State, "verification_status"); got != testCase.wantStatus {
				t.Errorf("verification_status = %q, want %q", got, testCase.wantStatus)
			}
			if got := hasWarning(resp.Diagnostics, "awaits confirmation"); got != testCase.wantPending {
				t.Errorf("pending warning = %v, want %v", got, testCase.wantPending)
			}
		})
	}
}

func TestGmailForwardingAddressResourceRead(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		configured  string
		get         *forwarding_address.ForwardingAddress
		wantRemoved bool
		wantEmail   string
		wantStatus  string
		wantPending bool
	}{
		{
			name:       "a confirmed address reads as accepted",
			configured: testForwardingEmail,
			get: &forwarding_address.ForwardingAddress{
				ForwardingEmail:    testForwardingEmail,
				VerificationStatus: forwarding_address.VerificationStatusAccepted,
			},
			wantEmail:  testForwardingEmail,
			wantStatus: "accepted",
		},
		{
			name:       "an unconfirmed address warns on every plan",
			configured: testForwardingEmail,
			get: &forwarding_address.ForwardingAddress{
				ForwardingEmail:    testForwardingEmail,
				VerificationStatus: forwarding_address.VerificationStatusPending,
			},
			wantEmail:   testForwardingEmail,
			wantStatus:  "pending",
			wantPending: true,
		},
		{
			name:       "the configured spelling survives Gmail answering in another case",
			configured: "Inbox.Ver.1@Arkivplats.example",
			get: &forwarding_address.ForwardingAddress{
				ForwardingEmail:    testForwardingEmail,
				VerificationStatus: forwarding_address.VerificationStatusAccepted,
			},
			wantEmail:  "Inbox.Ver.1@Arkivplats.example",
			wantStatus: "accepted",
		},
		{
			name:        "an address removed out of band is removed from state",
			configured:  testForwardingEmail,
			wantRemoved: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			r := testForwardingAddressResource(t, &forwardingAddressHandler{get: testCase.get})
			addressSchema := forwardingAddressSchema(t)
			raw := groupMemberObject(t, addressSchema, map[string]string{
				"user_id":             "me",
				"forwarding_email":    testCase.configured,
				"verification_status": "pending",
			})

			state := tfsdk.State{Schema: addressSchema, Raw: raw}
			resp := &resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Read() diagnostics: %v", resp.Diagnostics)
			}
			if got := resp.State.Raw.IsNull(); got != testCase.wantRemoved {
				t.Fatalf("state removed = %v, want %v", got, testCase.wantRemoved)
			}
			if testCase.wantRemoved {
				return
			}
			if got := stateAttribute(t, resp.State, "forwarding_email"); got != testCase.wantEmail {
				t.Errorf("forwarding_email = %q, want %q", got, testCase.wantEmail)
			}
			if got := stateAttribute(t, resp.State, "verification_status"); got != testCase.wantStatus {
				t.Errorf("verification_status = %q, want %q", got, testCase.wantStatus)
			}
			if got := hasWarning(resp.Diagnostics, "awaits confirmation"); got != testCase.wantPending {
				t.Errorf("pending warning = %v, want %v", got, testCase.wantPending)
			}
		})
	}
}

func TestGmailForwardingAddressResourceDelete(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		deleteStatus int
		wantError    bool
	}{
		{name: "deleted", deleteStatus: http.StatusNoContent},
		{name: "already gone", deleteStatus: http.StatusNotFound},
		{name: "refused", deleteStatus: http.StatusForbidden, wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := &forwardingAddressHandler{deleteStatus: testCase.deleteStatus}
			r := testForwardingAddressResource(t, handler)
			addressSchema := forwardingAddressSchema(t)
			raw := groupMemberObject(t, addressSchema, map[string]string{
				"user_id":             "me",
				"forwarding_email":    testForwardingEmail,
				"verification_status": "accepted",
			})

			state := tfsdk.State{Schema: addressSchema, Raw: raw}
			resp := &resource.DeleteResponse{State: state}
			r.Delete(t.Context(), resource.DeleteRequest{State: state}, resp)

			if handler.deleteCalls != 1 {
				t.Errorf("delete calls = %d, want 1", handler.deleteCalls)
			}
			if got := resp.Diagnostics.HasError(); got != testCase.wantError {
				t.Errorf("HasError() = %v, want %v (%v)", got, testCase.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestGmailForwardingAddressResourceImportState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		id        string
		wantError bool
		wantUser  string
		wantEmail string
	}{
		{name: "user and address", id: "me/" + testForwardingEmail, wantUser: "me", wantEmail: testForwardingEmail},
		{name: "a user given by address", id: "viktor.persson@altshift.se/" + testForwardingEmail, wantUser: "viktor.persson@altshift.se", wantEmail: testForwardingEmail},
		{name: "no separator", id: testForwardingEmail, wantError: true},
		{name: "no user", id: "/" + testForwardingEmail, wantError: true},
		{name: "no address", id: "me/", wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			addressSchema := forwardingAddressSchema(t)
			resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: addressSchema, Raw: groupMemberObject(t, addressSchema, nil)}}
			(&gmailForwardingAddressResource{}).ImportState(t.Context(), resource.ImportStateRequest{ID: testCase.id}, resp)

			if got := resp.Diagnostics.HasError(); got != testCase.wantError {
				t.Fatalf("HasError() = %v, want %v (%v)", got, testCase.wantError, resp.Diagnostics)
			}
			if testCase.wantError {
				return
			}
			if got := stateAttribute(t, resp.State, "user_id"); got != testCase.wantUser {
				t.Errorf("user_id = %q, want %q", got, testCase.wantUser)
			}
			if got := stateAttribute(t, resp.State, "forwarding_email"); got != testCase.wantEmail {
				t.Errorf("forwarding_email = %q, want %q", got, testCase.wantEmail)
			}
		})
	}
}
