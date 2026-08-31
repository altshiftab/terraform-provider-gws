package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/directory"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/directory/directory_config"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/directory/types/member"
	"github.com/altshiftab/utils_go/pkg/http/types/fetch_config"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	testGroupKey     = "privacy@example.com"
	testMemberEmail  = "external@other.example"
	testMemberId     = "103896343696236579037"
	testMemberEtag   = `"etag-value"`
	testMemberStatus = "ACTIVE"
)

// memberHandler answers the two Directory API calls the resource makes. get is
// the response to members.get for any member key; a nil get stands for the 404
// Google returns for a member key it will not resolve. list is the membership
// members.list reports.
type memberHandler struct {
	get         *member.Member
	getStatus   int
	list        []*member.Member
	listCalls   int
	insert      *member.Member
	insertCalls int
}

func (h *memberHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	notFound := func() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Resource Not Found: memberKey"}}`))
	}

	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/members"):
		h.insertCalls++
		writeJson(w, h.insert)
	case strings.HasSuffix(r.URL.Path, "/members"):
		h.listCalls++
		writeJson(w, map[string]any{"members": h.list})
	default:
		if h.getStatus != 0 && h.getStatus != http.StatusOK {
			w.WriteHeader(h.getStatus)
			_, _ = w.Write([]byte(`{"error":{"code":403,"message":"Not authorized."}}`))
			return
		}
		if h.get == nil {
			notFound()
			return
		}
		writeJson(w, h.get)
	}
}

func writeJson(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// testGroupMemberResource wires the resource to a server speaking the Directory
// API, so the tests exercise the real client and its error chain rather than a
// stand-in for them.
func testGroupMemberResource(t *testing.T, handler http.Handler) *groupMemberResource {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseUrl, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return &groupMemberResource{
		client:       directory.NewClient(directory_config.WithBaseUrl(baseUrl)),
		providerData: &gwsProviderData{fetchOption: fetch_config.WithHttpClient(server.Client())},
	}
}

func groupMemberSchema(t *testing.T) schema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	(&groupMemberResource{}).Schema(t.Context(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	return resp.Schema
}

// groupMemberObject builds a raw value for the resource's schema, with every
// attribute the caller does not name left null.
func groupMemberObject(t *testing.T, s schema.Schema, attributes map[string]string) tftypes.Value {
	t.Helper()

	objectType, ok := s.Type().TerraformType(t.Context()).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not an object")
	}

	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		if value, named := attributes[name]; named {
			values[name] = tftypes.NewValue(attributeType, value)
			continue
		}
		values[name] = tftypes.NewValue(attributeType, nil)
	}

	return tftypes.NewValue(objectType, values)
}

func stateAttribute(t *testing.T, state tfsdk.State, name string) string {
	t.Helper()

	var value types.String
	if diagnostics := state.GetAttribute(t.Context(), path.Root(name), &value); diagnostics.HasError() {
		t.Fatalf("get %s: %v", name, diagnostics)
	}

	return value.ValueString()
}

func TestGroupMemberResourceRead(t *testing.T) {
	t.Parallel()

	external := &member.Member{Id: testMemberId, Email: testMemberEmail, Role: "MEMBER", Type: "USER", Etag: testMemberEtag}

	testCases := []struct {
		name          string
		stateId       string
		handler       *memberHandler
		wantRemoved   bool
		wantError     bool
		wantId        string
		wantRole      string
		wantListCalls int
	}{
		{
			name:     "a member the ID resolves is read directly",
			stateId:  testMemberId,
			handler:  &memberHandler{get: &member.Member{Id: testMemberId, Email: testMemberEmail, Role: "OWNER", Type: "USER", Status: testMemberStatus}},
			wantId:   testMemberId,
			wantRole: "OWNER",
		},
		{
			// The state members.insert leaves behind for a member outside the
			// domain: no ID, so the read falls to the email, which 404s.
			name:          "a member without an ID is recovered from the member list",
			stateId:       "",
			handler:       &memberHandler{get: nil, list: []*member.Member{external}},
			wantId:        testMemberId,
			wantRole:      "MEMBER",
			wantListCalls: 1,
		},
		{
			name:          "an ID that no longer resolves is replaced from the member list",
			stateId:       "900000000000000000000",
			handler:       &memberHandler{get: nil, list: []*member.Member{external}},
			wantId:        testMemberId,
			wantRole:      "MEMBER",
			wantListCalls: 1,
		},
		{
			name:          "a membership that is gone is removed from state",
			stateId:       testMemberId,
			handler:       &memberHandler{get: nil, list: nil},
			wantRemoved:   true,
			wantListCalls: 1,
		},
		{
			name:      "a failure that is not a 404 is reported",
			stateId:   testMemberId,
			handler:   &memberHandler{getStatus: http.StatusForbidden},
			wantError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			r := testGroupMemberResource(t, testCase.handler)
			memberSchema := groupMemberSchema(t)
			raw := groupMemberObject(t, memberSchema, map[string]string{
				"group_key": testGroupKey,
				"email":     testMemberEmail,
				"id":        testCase.stateId,
				"role":      "MEMBER",
			})

			state := tfsdk.State{Schema: memberSchema, Raw: raw}
			resp := &resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, resp)

			if got := resp.Diagnostics.HasError(); got != testCase.wantError {
				t.Fatalf("HasError() = %v, want %v (%v)", got, testCase.wantError, resp.Diagnostics)
			}
			if testCase.handler.listCalls != testCase.wantListCalls {
				t.Errorf("members.list calls = %d, want %d", testCase.handler.listCalls, testCase.wantListCalls)
			}
			if testCase.wantError {
				return
			}

			if got := resp.State.Raw.IsNull(); got != testCase.wantRemoved {
				t.Fatalf("state removed = %v, want %v", got, testCase.wantRemoved)
			}
			if testCase.wantRemoved {
				return
			}

			if got := stateAttribute(t, resp.State, "id"); got != testCase.wantId {
				t.Errorf("id = %q, want %q", got, testCase.wantId)
			}
			if got := stateAttribute(t, resp.State, "role"); got != testCase.wantRole {
				t.Errorf("role = %q, want %q", got, testCase.wantRole)
			}
			if got := stateAttribute(t, resp.State, "email"); got != testMemberEmail {
				t.Errorf("email = %q, want %q", got, testMemberEmail)
			}
		})
	}
}

func TestGroupMemberResourceCreate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		insert        *member.Member
		list          []*member.Member
		wantId        string
		wantListCalls int
	}{
		{
			name:   "an ID the insert reports is kept",
			insert: &member.Member{Id: testMemberId, Email: testMemberEmail, Role: "MEMBER", Type: "USER", Status: testMemberStatus},
			wantId: testMemberId,
		},
		{
			// members.insert omits the ID for a member outside the domain, and
			// without it a later read has nothing that members.get will resolve.
			name:          "an ID the insert omits is taken from the member list",
			insert:        &member.Member{Email: testMemberEmail, Role: "MEMBER", Type: "USER", Status: testMemberStatus},
			list:          []*member.Member{{Id: testMemberId, Email: testMemberEmail, Role: "MEMBER", Type: "USER"}},
			wantId:        testMemberId,
			wantListCalls: 1,
		},
		{
			name:          "a membership is still recorded when the ID cannot be resolved",
			insert:        &member.Member{Email: testMemberEmail, Role: "MEMBER", Type: "USER", Status: testMemberStatus},
			list:          nil,
			wantId:        "",
			wantListCalls: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := &memberHandler{insert: testCase.insert, list: testCase.list}
			r := testGroupMemberResource(t, handler)
			memberSchema := groupMemberSchema(t)
			raw := groupMemberObject(t, memberSchema, map[string]string{
				"group_key": testGroupKey,
				"email":     testMemberEmail,
				"role":      "MEMBER",
			})

			resp := &resource.CreateResponse{State: tfsdk.State{Schema: memberSchema}}
			r.Create(
				t.Context(),
				resource.CreateRequest{Plan: tfsdk.Plan{Schema: memberSchema, Raw: raw}},
				resp,
			)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Create() diagnostics: %v", resp.Diagnostics)
			}
			if handler.insertCalls != 1 {
				t.Errorf("members.insert calls = %d, want 1", handler.insertCalls)
			}
			if handler.listCalls != testCase.wantListCalls {
				t.Errorf("members.list calls = %d, want %d", handler.listCalls, testCase.wantListCalls)
			}
			if got := stateAttribute(t, resp.State, "id"); got != testCase.wantId {
				t.Errorf("id = %q, want %q", got, testCase.wantId)
			}
			if got := stateAttribute(t, resp.State, "email"); got != testMemberEmail {
				t.Errorf("email = %q, want %q", got, testMemberEmail)
			}
		})
	}
}

func TestGroupMemberResourceFindMember(t *testing.T) {
	t.Parallel()

	members := []*member.Member{
		{Id: "1", Email: "First@example.com", Role: "OWNER"},
		nil,
		{Id: testMemberId, Email: testMemberEmail, Role: "MEMBER"},
	}

	testCases := []struct {
		name      string
		memberKey string
		wantId    string
		wantNil   bool
	}{
		{name: "by ID", memberKey: testMemberId, wantId: testMemberId},
		{name: "by email", memberKey: testMemberEmail, wantId: testMemberId},
		{name: "by email, case-insensitively", memberKey: "FIRST@EXAMPLE.COM", wantId: "1"},
		{name: "a member the group does not have", memberKey: "absent@example.com", wantNil: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			r := testGroupMemberResource(t, &memberHandler{list: members})

			found, err := r.findMember(t.Context(), testGroupKey, testCase.memberKey)
			if err != nil {
				t.Fatalf("findMember() error: %v", err)
			}
			if testCase.wantNil {
				if found != nil {
					t.Fatalf("findMember() = %v, want nil", found)
				}
				return
			}
			if found == nil {
				t.Fatal("findMember() = nil, want a member")
			}
			if found.Id != testCase.wantId {
				t.Errorf("Id = %q, want %q", found.Id, testCase.wantId)
			}
		})
	}
}

func TestGroupMemberResourceFindMemberError(t *testing.T) {
	t.Parallel()

	r := testGroupMemberResource(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"Not authorized."}}`))
	}))

	found, err := r.findMember(t.Context(), testGroupKey, testMemberEmail)
	if err == nil {
		t.Fatal("findMember() error = nil, want an error")
	}
	if found != nil {
		t.Errorf("findMember() = %v, want nil", found)
	}
	if detail := apiErrorDetail(err); !strings.Contains(detail, "Not authorized.") {
		t.Errorf("apiErrorDetail() = %q, missing the API response", detail)
	}
}

func TestMapMemberToState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		apiMember *member.Member
		expected  groupMemberResourceModel
	}{
		{
			name:      "nil member leaves state untouched",
			apiMember: nil,
			expected:  groupMemberResourceModel{Id: types.StringValue("existing")},
		},
		{
			name: "every field is carried over",
			apiMember: &member.Member{
				Id:               testMemberId,
				Email:            testMemberEmail,
				Role:             "MEMBER",
				Type:             "USER",
				Status:           testMemberStatus,
				Etag:             testMemberEtag,
				DeliverySettings: "ALL_MAIL",
			},
			expected: groupMemberResourceModel{
				Id:               types.StringValue(testMemberId),
				Email:            types.StringValue(testMemberEmail),
				Role:             types.StringValue("MEMBER"),
				Type:             types.StringValue("USER"),
				Status:           types.StringValue(testMemberStatus),
				Etag:             types.StringValue(testMemberEtag),
				DeliverySettings: types.StringValue("ALL_MAIL"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// The model is all types.String, so one comparison covers every
			// field — including the ones a partial mapping would leave behind.
			state := groupMemberResourceModel{Id: types.StringValue("existing")}
			mapMemberToState(testCase.apiMember, &state)

			if state != testCase.expected {
				t.Errorf("state = %+v, want %+v", state, testCase.expected)
			}
		})
	}
}
