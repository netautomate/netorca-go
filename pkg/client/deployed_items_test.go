package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/netautomate/netorca-go/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	deployedItemsHost    = "http://api-aws.demo.netorca.io"
	deployedItemsRoot    = deployedItemsHost + "/v1/orcabase/serviceowner/deployed_items"
	deployedItemDetail   = deployedItemsRoot + "/7412/"
	deployedItemOrphan   = deployedItemsRoot + "/7413/"
	deployedItemConsumer = deployedItemsHost + "/v1/orcabase/consumer/deployed_items/"
	// serviceItemLink is the hyperlink form the API uses for the parent relation. Ids are
	// rejected on the way in, so this exact string is what a create has to carry.
	serviceItemLink    = deployedItemsHost + "/v1/orcabase/serviceowner/service_items/389/"
	changeInstanceLink = deployedItemsHost + "/v1/orcabase/serviceowner/change_instances/900/"
)

// oneDeployedItem mirrors a real response: both relations are hyperlinks rather than nested
// objects or ids, and change_instance is null for an item written against a service item.
const oneDeployedItem = `{
  "id": 7412,
  "url": "` + deployedItemsRoot + `/7412/",
  "created": "2026-07-19T10:11:12Z",
  "modified": "2026-07-19T11:12:13Z",
  "data": {"vip": "10.0.0.7", "partition": "demo"},
  "version": 3,
  "service_item": "` + serviceItemLink + `",
  "change_instance": null,
  "consumer_team": {"id": 12, "name": "core-networks"},
  "service_owner_team": {"id": 4, "name": "loadbalancer-team"}
}`

// newDeployedItemTestClient builds a client pointed at the mocked base URL.
func newDeployedItemTestClient(t *testing.T) *client.Client {
	t.Helper()
	nc, err := client.NewClient(deployedItemsHost, "test-api-key", "v1", 5*time.Second)
	require.NoError(t, err)
	return nc
}

func TestDeployedItemWriteValidate(t *testing.T) {
	data := json.RawMessage(`{"vip":"10.0.0.7"}`)

	t.Run("rejects a body with both parents set", func(t *testing.T) {
		body := &client.DeployedItemWrite{Data: data, ServiceItemID: 389, ChangeInstanceID: 900}

		err := body.Validate()

		require.Error(t, err)
		require.ErrorContains(t, err, "not both")
	})

	t.Run("rejects a body with neither parent set", func(t *testing.T) {
		body := &client.DeployedItemWrite{Data: data}

		err := body.Validate()

		require.Error(t, err)
		require.ErrorContains(t, err, "needs a parent")
	})

	t.Run("accepts a service item parent on its own", func(t *testing.T) {
		body := &client.DeployedItemWrite{Data: data, ServiceItemID: 389}

		require.NoError(t, body.Validate())
	})

	t.Run("accepts a change instance parent on its own", func(t *testing.T) {
		body := &client.DeployedItemWrite{Data: data, ChangeInstanceID: 900}

		require.NoError(t, body.Validate())
	})

	t.Run("accepts empty data, which is sent as an empty object", func(t *testing.T) {
		body := &client.DeployedItemWrite{ServiceItemID: 389}

		require.NoError(t, body.Validate())
	})

	t.Run("rejects data that is not JSON", func(t *testing.T) {
		body := &client.DeployedItemWrite{
			Data:          json.RawMessage(`{"vip": `),
			ServiceItemID: 389,
		}

		err := body.Validate()

		require.Error(t, err)
		require.ErrorContains(t, err, "not valid JSON")
	})
}

func TestDeployedItemLinks(t *testing.T) {
	t.Run("reads the parent ids out of their hyperlinks", func(t *testing.T) {
		var item client.DeployedItem
		require.NoError(t, json.Unmarshal([]byte(oneDeployedItem), &item))

		serviceItemID, err := item.ServiceItemID()
		require.NoError(t, err)
		assert.Equal(t, 389, serviceItemID)

		// change_instance was null, which is the normal shape for an item written
		// straight against a service item.
		assert.Empty(t, item.ChangeInstance)
		changeInstanceID, err := item.ChangeInstanceID()
		require.NoError(t, err)
		assert.Equal(t, 0, changeInstanceID)
	})

	t.Run("tolerates a format suffix on the hyperlink", func(t *testing.T) {
		item := client.DeployedItem{ServiceItem: serviceItemLink + "?format=json"}

		serviceItemID, err := item.ServiceItemID()

		require.NoError(t, err)
		assert.Equal(t, 389, serviceItemID)
	})

	t.Run("errors on a hyperlink that does not end in an id", func(t *testing.T) {
		item := client.DeployedItem{ID: 7412, ServiceItem: "https://example.com/nonsense/"}

		_, err := item.ServiceItemID()

		require.Error(t, err)
		require.ErrorContains(t, err, "does not end in an id")
	})
}

func TestListDeployedItemsToQueryParams(t *testing.T) {
	t.Run("renders every supported filter", func(t *testing.T) {
		filters := &client.ListDeployedItemsRequest{
			ApplicationID:      []int{1, 2},
			ConsumerTeamID:     []int{12},
			ServiceOwnerTeamID: []int{4},
			ServiceID:          []int{7},
			ServiceItemID:      []int{389, 412},
			Declaration:        map[string]any{"partition": "demo"},
			Limit:              50,
			Offset:             20,
			Ordering:           "-version",
		}

		query, err := filters.ToQueryParams()

		require.NoError(t, err)
		// Lists become comma-joined "in" lookups.
		assert.Contains(t, query, "service_item_id=389%2C412")
		assert.Contains(t, query, "application_id=1%2C2")
		assert.Contains(t, query, "consumer_team_id=12")
		assert.Contains(t, query, "service_owner_team_id=4")
		assert.Contains(t, query, "service_id=7")
		assert.Contains(t, query, "limit=50")
		assert.Contains(t, query, "offset=20")
		assert.Contains(t, query, "ordering=-version")
		assert.Contains(t, query, "declaration=%7B%22partition%22%3A%22demo%22%7D")
	})

	t.Run("renders nothing for the zero value", func(t *testing.T) {
		query, err := (&client.ListDeployedItemsRequest{}).ToQueryParams()

		require.NoError(t, err)
		assert.Empty(t, query)
	})
}

func TestListDeployedItems(t *testing.T) {
	t.Run("decodes the paginated envelope", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemsRoot+"/",
			httpmock.NewStringResponder(200,
				`{"count":1,"next":null,"previous":null,"results":[`+oneDeployedItem+`]}`))

		nc := newDeployedItemTestClient(t)
		resp, err := nc.ListDeployedItems(context.Background(), nil)

		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		require.Len(t, resp.Results, 1)
		assert.Equal(t, 7412, resp.Results[0].ID)
		assert.Equal(t, 3, resp.Results[0].Version)
		assert.JSONEq(t, `{"vip":"10.0.0.7","partition":"demo"}`, string(resp.Results[0].Data))
		require.NotNil(t, resp.Results[0].ServiceOwnerTeam)
		assert.Equal(t, "loadbalancer-team", resp.Results[0].ServiceOwnerTeam.Name)
	})

	t.Run("sends the filters and honours the point of view", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET",
			deployedItemConsumer,
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200, `{"count":0,"results":[]}`), nil
			})

		nc := newDeployedItemTestClient(t)
		_, err := nc.ListDeployedItems(context.Background(), &client.ListDeployedItemsRequest{
			POV:           client.POVConsumer,
			ServiceItemID: []int{389},
		})

		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "service_item_id=389")
	})

	t.Run("surfaces a 403 as ErrForbidden", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemsRoot+"/",
			httpmock.NewStringResponder(403,
				`{"detail":"You do not have permission to perform this action."}`))

		nc := newDeployedItemTestClient(t)
		resp, err := nc.ListDeployedItems(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, resp)
		require.ErrorIs(t, err, client.ErrForbidden)
	})
}

func TestGetDeployedItem(t *testing.T) {
	t.Run("returns the item on success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemDetail,
			httpmock.NewStringResponder(200, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		item, err := nc.GetDeployedItem(context.Background(), client.POVServiceOwner, 7412)

		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, 7412, item.ID)
		assert.Equal(t, serviceItemLink, item.ServiceItem)
		require.NotNil(t, item.ConsumerTeam)
		assert.Equal(t, 12, item.ConsumerTeam.ID)
	})

	t.Run("defaults an empty POV to serviceowner", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemDetail,
			httpmock.NewStringResponder(200, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		_, err := nc.GetDeployedItem(context.Background(), "", 7412)

		require.NoError(t, err)
	})

	t.Run("returns ErrNotFound on a 404", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemDetail,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newDeployedItemTestClient(t)
		item, err := nc.GetDeployedItem(context.Background(), client.POVServiceOwner, 7412)

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorIs(t, err, client.ErrNotFound)
	})
}

func TestFindDeployedItemForServiceItem(t *testing.T) { //nolint:funlen
	t.Run("returns the highest version when the service item has a history", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Deliberately out of order: the current version is the highest one, not the
		// first the server happened to send.
		older := `{"id":7000,"version":1,"data":{"vip":"10.0.0.1"},
			"service_item":"` + serviceItemLink + `"}`
		newer := `{"id":7412,"version":3,"data":{"vip":"10.0.0.7"},
			"service_item":"` + serviceItemLink + `"}`
		httpmock.RegisterResponder("GET", deployedItemsRoot+"/",
			httpmock.NewStringResponder(200,
				`{"count":2,"results":[`+older+`,`+newer+`]}`))

		nc := newDeployedItemTestClient(t)
		item, err := nc.FindDeployedItemForServiceItem(
			context.Background(), client.POVServiceOwner, 389,
		)

		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, 7412, item.ID)
		assert.Equal(t, 3, item.Version)
	})

	t.Run("filters by service item", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedQuery string
		httpmock.RegisterResponder("GET", deployedItemsRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				capturedQuery = req.URL.RawQuery
				return httpmock.NewStringResponse(200,
					`{"count":1,"results":[`+oneDeployedItem+`]}`), nil
			})

		nc := newDeployedItemTestClient(t)
		_, err := nc.FindDeployedItemForServiceItem(
			context.Background(), client.POVServiceOwner, 389,
		)

		require.NoError(t, err)
		assert.Contains(t, capturedQuery, "service_item_id=389")
	})

	t.Run("finds the highest version when the history spans more than one page", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// The history is longer than one page and, deliberately, unordered: the highest version
		// sits on the second page. Taking the max over the first page alone - the bug this test
		// guards - would return the stale v20 as current.
		var page1 []string
		for v := 1; v <= 100; v++ {
			page1 = append(page1, fmt.Sprintf(
				`{"id":%d,"version":%d,"service_item":"%s"}`, 7000+v, v, serviceItemLink))
		}
		current := fmt.Sprintf(
			`{"id":9999,"version":132,"service_item":"%s"}`, serviceItemLink)

		httpmock.RegisterResponder("GET", deployedItemsRoot+"/",
			func(req *http.Request) (*http.Response, error) {
				body := `{"count":101,"results":[` + current + `]}`
				if req.URL.Query().Get("offset") == "" || req.URL.Query().Get("offset") == "0" {
					body = `{"count":101,"results":[` + joinJSON(page1) + `]}`
				}
				return httpmock.NewStringResponse(200, body), nil
			})

		nc := newDeployedItemTestClient(t)
		item, err := nc.FindDeployedItemForServiceItem(
			context.Background(), client.POVServiceOwner, 389,
		)

		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, 132, item.Version, "the current version is on the second page")
		assert.Equal(t, 9999, item.ID)
	})

	t.Run("returns ErrNotFound when nothing is deployed", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemsRoot+"/",
			httpmock.NewStringResponder(200, `{"count":0,"next":null,"previous":null,"results":[]}`))

		nc := newDeployedItemTestClient(t)
		item, err := nc.FindDeployedItemForServiceItem(
			context.Background(), client.POVServiceOwner, 389,
		)

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorIs(t, err, client.ErrNotFound)
		require.ErrorContains(t, err, "389")
	})
}

func TestCreateDeployedItem(t *testing.T) { //nolint:funlen
	t.Run("sends the service item as a hyperlink, not an id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", deployedItemsRoot+"/",
			captureBodyResponder(&capturedBody, 201, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		item, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{
				Data:          json.RawMessage(`{"vip":"10.0.0.7","partition":"demo"}`),
				ServiceItemID: 389,
			})

		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, 7412, item.ID)
		// The relation is a HyperlinkedRelatedField: 389 on its own would be rejected,
		// and the unused change_instance key must not appear at all because neither
		// relation accepts null.
		assert.JSONEq(t,
			`{"data":{"vip":"10.0.0.7","partition":"demo"},"service_item":"`+serviceItemLink+`"}`,
			capturedBody)
	})

	t.Run("sends the change instance as a hyperlink when that is the parent", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", deployedItemsRoot+"/",
			captureBodyResponder(&capturedBody, 201, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		_, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{
				Data:             json.RawMessage(`{"vip":"10.0.0.7"}`),
				ChangeInstanceID: 900,
			})

		require.NoError(t, err)
		assert.JSONEq(t,
			`{"data":{"vip":"10.0.0.7"},"change_instance":"`+changeInstanceLink+`"}`,
			capturedBody)
	})

	t.Run("builds the link from the point of view of the request", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST",
			deployedItemConsumer,
			captureBodyResponder(&capturedBody, 201, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		_, err := nc.CreateDeployedItem(context.Background(), client.POVConsumer,
			&client.DeployedItemWrite{ServiceItemID: 389})

		require.NoError(t, err)
		// The server resolves the URL through its own URL configuration, and the pov is
		// part of that configuration, so the link has to match the request it travels in.
		assert.Contains(t, capturedBody, "/v1/orcabase/consumer/service_items/389/")
	})

	t.Run("sends an empty object rather than null for absent data", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var capturedBody string
		httpmock.RegisterResponder("POST", deployedItemsRoot+"/",
			captureBodyResponder(&capturedBody, 201, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		_, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{ServiceItemID: 389})

		require.NoError(t, err)
		assert.JSONEq(t, `{"data":{},"service_item":"`+serviceItemLink+`"}`, capturedBody)
	})

	t.Run("rejects both parents without touching the network", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newDeployedItemTestClient(t)
		item, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{
				Data:             json.RawMessage(`{"vip":"10.0.0.7"}`),
				ServiceItemID:    389,
				ChangeInstanceID: 900,
			})

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorContains(t, err, "not both")
		// The platform would accept this and quietly discard the service item, so the
		// whole point is that the request is never made.
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("rejects neither parent without touching the network", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newDeployedItemTestClient(t)
		item, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{Data: json.RawMessage(`{"vip":"10.0.0.7"}`)})

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorContains(t, err, "needs a parent")
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("rejects a nil body", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newDeployedItemTestClient(t)
		item, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner, nil)

		require.Error(t, err)
		assert.Nil(t, item)
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})

	t.Run("returns the existing item when the API detects no change", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Repeating the previous version's data is answered with 200 and the item that
		// already exists, carrying an extra "message" key this package ignores.
		noChange := `{"id":7412,"version":3,"data":{"vip":"10.0.0.7"},
			"service_item":"` + serviceItemLink + `","message":"no change detected"}`
		httpmock.RegisterResponder("POST", deployedItemsRoot+"/",
			httpmock.NewStringResponder(200, noChange))

		nc := newDeployedItemTestClient(t)
		item, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{
				Data:          json.RawMessage(`{"vip":"10.0.0.7"}`),
				ServiceItemID: 389,
			})

		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, 3, item.Version)
	})

	t.Run("surfaces a 400 as ErrBadRequest with the server's explanation", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("POST", deployedItemsRoot+"/",
			httpmock.NewStringResponder(400, `{"service_item":["Invalid hyperlink - No URL match."]}`))

		nc := newDeployedItemTestClient(t)
		item, err := nc.CreateDeployedItem(context.Background(), client.POVServiceOwner,
			&client.DeployedItemWrite{ServiceItemID: 389})

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorIs(t, err, client.ErrBadRequest)
		require.ErrorContains(t, err, "Invalid hyperlink")
	})
}

func TestUpdateDeployedItem(t *testing.T) { //nolint:funlen
	t.Run("writes only data and echoes the existing parent back", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", deployedItemDetail,
			httpmock.NewStringResponder(200, oneDeployedItem))

		var capturedBody string
		httpmock.RegisterResponder("PATCH", deployedItemDetail,
			captureBodyResponder(&capturedBody, 200, oneDeployedItem))

		nc := newDeployedItemTestClient(t)
		item, err := nc.UpdateDeployedItem(context.Background(), client.POVServiceOwner, 7412,
			json.RawMessage(`{"vip":"10.0.0.9"}`))

		require.NoError(t, err)
		require.NotNil(t, item)
		// The parent is echoed rather than changed: the serializer insists on one in every
		// write, a partial one included, but it must stay the parent it already had.
		assert.JSONEq(t,
			`{"data":{"vip":"10.0.0.9"},"service_item":"`+serviceItemLink+`"}`,
			capturedBody)
	})

	t.Run("falls back to the change instance when there is no service item", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		orphan := `{"id":7413,"version":1,"data":{},"service_item":null,
			"change_instance":"` + changeInstanceLink + `"}`
		httpmock.RegisterResponder("GET", deployedItemOrphan,
			httpmock.NewStringResponder(200, orphan))

		var capturedBody string
		httpmock.RegisterResponder("PATCH", deployedItemOrphan,
			captureBodyResponder(&capturedBody, 200, orphan))

		nc := newDeployedItemTestClient(t)
		_, err := nc.UpdateDeployedItem(context.Background(), client.POVServiceOwner, 7413,
			json.RawMessage(`{"vip":"10.0.0.9"}`))

		require.NoError(t, err)
		assert.JSONEq(t,
			`{"data":{"vip":"10.0.0.9"},"change_instance":"`+changeInstanceLink+`"}`,
			capturedBody)
	})

	t.Run("returns ErrNotFound when the item has been removed", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("GET", deployedItemDetail,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newDeployedItemTestClient(t)
		item, err := nc.UpdateDeployedItem(context.Background(), client.POVServiceOwner, 7412,
			json.RawMessage(`{"vip":"10.0.0.9"}`))

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorIs(t, err, client.ErrNotFound)
	})

	t.Run("rejects data that is not JSON before touching the network", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		nc := newDeployedItemTestClient(t)
		item, err := nc.UpdateDeployedItem(context.Background(), client.POVServiceOwner, 7412,
			json.RawMessage(`{"vip": `))

		require.Error(t, err)
		assert.Nil(t, item)
		require.ErrorContains(t, err, "not valid JSON")
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})
}

func TestDeleteDeployedItem(t *testing.T) {
	t.Run("returns no error on a 204", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("DELETE", deployedItemDetail,
			httpmock.NewStringResponder(204, ""))

		nc := newDeployedItemTestClient(t)
		err := nc.DeleteDeployedItem(context.Background(), client.POVServiceOwner, 7412)

		require.NoError(t, err)
	})

	t.Run("returns ErrNotFound when the item is already gone", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()
		httpmock.RegisterResponder("DELETE", deployedItemDetail,
			httpmock.NewStringResponder(404, `{"detail":"Not found."}`))

		nc := newDeployedItemTestClient(t)
		err := nc.DeleteDeployedItem(context.Background(), client.POVServiceOwner, 7412)

		require.Error(t, err)
		require.ErrorIs(t, err, client.ErrNotFound)
	})
}

// joinJSON joins pre-rendered JSON object literals with commas, for building a results array.
func joinJSON(parts []string) string {
	return strings.Join(parts, ",")
}
