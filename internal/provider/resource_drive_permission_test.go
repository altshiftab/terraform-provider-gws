package provider

import (
	"testing"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/drive/types/permission"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMapDrivePermissionToState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		apiPermission *permission.Permission
		expected      drivePermissionResourceModel
	}{
		{
			name:          "nil permission leaves state untouched",
			apiPermission: nil,
			expected: drivePermissionResourceModel{
				Id: types.StringValue("existing"),
			},
		},
		{
			name: "user permission",
			apiPermission: &permission.Permission{
				Id:           "permission-1",
				Type:         permission.TypeUser,
				EmailAddress: "sa@project.iam.gserviceaccount.com",
				Role:         permission.RoleReader,
				DisplayName:  "Service Account",
			},
			expected: drivePermissionResourceModel{
				Id:                 types.StringValue("permission-1"),
				Type:               types.StringValue("user"),
				EmailAddress:       types.StringValue("sa@project.iam.gserviceaccount.com"),
				Domain:             types.StringNull(),
				Role:               types.StringValue("reader"),
				AllowFileDiscovery: types.BoolValue(false),
				DisplayName:        types.StringValue("Service Account"),
			},
		},
		{
			name: "domain permission",
			apiPermission: &permission.Permission{
				Id:                 "permission-2",
				Type:               permission.TypeDomain,
				Domain:             "example.com",
				Role:               permission.RoleWriter,
				AllowFileDiscovery: true,
			},
			expected: drivePermissionResourceModel{
				Id:                 types.StringValue("permission-2"),
				Type:               types.StringValue("domain"),
				EmailAddress:       types.StringNull(),
				Domain:             types.StringValue("example.com"),
				Role:               types.StringValue("writer"),
				AllowFileDiscovery: types.BoolValue(true),
				DisplayName:        types.StringValue(""),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			state := drivePermissionResourceModel{Id: types.StringValue("existing")}
			mapDrivePermissionToState(testCase.apiPermission, &state)

			if state.Id != testCase.expected.Id {
				t.Errorf("Id = %v, want %v", state.Id, testCase.expected.Id)
			}
			if testCase.apiPermission == nil {
				return
			}
			if state.Type != testCase.expected.Type {
				t.Errorf("Type = %v, want %v", state.Type, testCase.expected.Type)
			}
			if state.EmailAddress != testCase.expected.EmailAddress {
				t.Errorf("EmailAddress = %v, want %v", state.EmailAddress, testCase.expected.EmailAddress)
			}
			if state.Domain != testCase.expected.Domain {
				t.Errorf("Domain = %v, want %v", state.Domain, testCase.expected.Domain)
			}
			if state.Role != testCase.expected.Role {
				t.Errorf("Role = %v, want %v", state.Role, testCase.expected.Role)
			}
			if state.AllowFileDiscovery != testCase.expected.AllowFileDiscovery {
				t.Errorf("AllowFileDiscovery = %v, want %v", state.AllowFileDiscovery, testCase.expected.AllowFileDiscovery)
			}
			if state.DisplayName != testCase.expected.DisplayName {
				t.Errorf("DisplayName = %v, want %v", state.DisplayName, testCase.expected.DisplayName)
			}
		})
	}
}
