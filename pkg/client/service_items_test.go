package client_test

import (
	"testing"

	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		request  *client.GetServiceItemsRequest
		expected string
	}{
		{
			name: "All fields set",
			request: &client.GetServiceItemsRequest{
				Name:                "test",
				RuntimeState:        "running",
				ChangeState:         "changed",
				Declaration:         "declaration",
				ApplicationID:       "app-id",
				ConsumerTeamID:      "team-id",
				DeclarationContains: "contains",
				DeclarationRegex:    "regex",
				ServiceID:           "service-id",
				ServiceName:         "service-name",
				ServiceOwnerID:      "owner-id",
				ServiceOwnerTeamID:  "team-owner-id",
				Limit:               10,
				Offset:              0,
				Ordering:            "-created_at",
			},
			expected: "application_id=app-id&change_state=changed&consumer_team_id=team-id&declaration=declaration&declaration_contains=contains&declaration_regex=regex&limit=10&name=test&ordering=-created_at&runtime_state=running&service_id=service-id&service_name=service-name&service_owner_id=owner-id&service_owner_team_id=team-owner-id", //nolint
		},
		{
			name:     "No fields set",
			request:  &client.GetServiceItemsRequest{},
			expected: "",
		},
		{
			name: "Some fields set",
			request: &client.GetServiceItemsRequest{
				Name:         "test",
				RuntimeState: "running",
				Limit:        5,
				Offset:       10,
				Ordering:     "name",
			},
			expected: "limit=5&name=test&offset=10&ordering=name&runtime_state=running",
		},
		{
			name: "Only limit and offset set",
			request: &client.GetServiceItemsRequest{
				Limit:  20,
				Offset: 5,
			},
			expected: "limit=20&offset=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := tt.request.ToQueryParams()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, query)
		})
	}
}
