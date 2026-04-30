package graphql

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	gql "github.com/graphql-go/graphql"
)

// safeID guards against SQL injection in ClickHouse queries via stationId arg.
var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type chRow struct {
	StationID string  `json:"station_id"`
	VIN       string  `json:"vin"`
	Firmware  string  `json:"firmware"`
	Torque    float64 `json:"torque"`
	TS        int64   `json:"ts"`
}

// BuildSchema constructs the GraphQL schema.
// The telemetry query reads from ClickHouse via its HTTP interface (port 8123).
func BuildSchema(clickhouseHTTP string) (gql.Schema, error) {
	telemetryType := gql.NewObject(gql.ObjectConfig{
		Name: "TelemetryRecord",
		Fields: gql.Fields{
			"stationId": &gql.Field{Type: gql.String},
			"vin":       &gql.Field{Type: gql.String},
			"firmware":  &gql.Field{Type: gql.String},
			"torqueNm":  &gql.Field{Type: gql.Float},
			"ts":        &gql.Field{Type: gql.Int},
		},
	})

	return gql.NewSchema(gql.SchemaConfig{
		Query: gql.NewObject(gql.ObjectConfig{
			Name: "Query",
			Fields: gql.Fields{
				"telemetry": &gql.Field{
					Type:        gql.NewList(telemetryType),
					Description: "Recent enriched PLC telemetry for a station from ClickHouse.",
					Args: gql.FieldConfigArgument{
						"stationId": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
						"limit":     &gql.ArgumentConfig{Type: gql.Int, DefaultValue: 100},
					},
					Resolve: func(p gql.ResolveParams) (interface{}, error) {
						stationID, _ := p.Args["stationId"].(string)
						limit, _ := p.Args["limit"].(int)
						if !safeID.MatchString(stationID) {
							return nil, fmt.Errorf("invalid stationId")
						}
						return fetchTelemetry(clickhouseHTTP, stationID, limit)
					},
				},
			},
		}),
	})
}

func fetchTelemetry(chHTTP, stationID string, limit int) ([]map[string]interface{}, error) {
	q := fmt.Sprintf(
		"SELECT station_id, vin, firmware, torque, ts FROM aegis.enriched_telemetry WHERE station_id='%s' ORDER BY ts DESC LIMIT %d FORMAT JSONEachRow",
		stationID, limit,
	)
	resp, err := http.Get(chHTTP + "/?query=" + url.QueryEscape(q))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []map[string]interface{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var row chRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"stationId": row.StationID,
			"vin":       row.VIN,
			"firmware":  row.Firmware,
			"torqueNm":  row.Torque,
			"ts":        row.TS,
		})
	}
	return results, scanner.Err()
}

// Handler returns an HTTP handler for GraphQL POST and GET requests.
func Handler(schema gql.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var queryStr string
		switch r.Method {
		case http.MethodGet:
			queryStr = r.URL.Query().Get("query")
		case http.MethodPost:
			var body struct {
				Query string `json:"query"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			queryStr = body.Query
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result := gql.Do(gql.Params{
			Schema:        schema,
			RequestString: queryStr,
			Context:       r.Context(),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
