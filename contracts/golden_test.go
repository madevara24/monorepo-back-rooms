package contracts

import (
	"encoding/json"
	"testing"
	"time"
)

// TestHeartbeatGolden pins the exact JSON bytes json.Marshal emits for each
// heartbeat wire type: field names, omitempty, and the time format. A
// field rename must fail this test.
func TestHeartbeatGolden(t *testing.T) {
	collectedAt := time.Date(2026, 1, 2, 15, 4, 5, 123456789, time.UTC)

	cases := []struct {
		name string
		v    any
		want string
	}{
		{
			name: "Capacity",
			v: Capacity{
				PortRangeMin: 21000,
				PortRangeMax: 21999,
				PortsTotal:   1000,
				PortsUsed:    1,
			},
			want: `{"port_range_min":21000,"port_range_max":21999,"ports_total":1000,"ports_used":1}`,
		},
		{
			name: "HeartbeatConnectionMetrics",
			v: HeartbeatConnectionMetrics{
				Active:                       71,
				Idle:                         2,
				IdleInTransaction:            0,
				MaxConnections:               100,
				SuperuserReservedConnections: 3,
				MemoryLimitBytes:             536870912,
				CollectedAt:                  collectedAt,
			},
			want: `{"active":71,"idle":2,"idle_in_transaction":0,"max_connections":100,"superuser_reserved_connections":3,"memory_limit_bytes":536870912,"collected_at":"2026-01-02T15:04:05.123456789Z"}`,
		},
		{
			name: "HeartbeatContainer with metrics",
			v: HeartbeatContainer{
				ID:     "db-a",
				Host:   "10.0.0.5",
				Port:   21000,
				Status: "running",
				Metrics: &HeartbeatConnectionMetrics{
					Active:                       71,
					Idle:                         2,
					IdleInTransaction:            0,
					MaxConnections:               100,
					SuperuserReservedConnections: 3,
					MemoryLimitBytes:             536870912,
					CollectedAt:                  collectedAt,
				},
			},
			want: `{"id":"db-a","host":"10.0.0.5","port":21000,"status":"running","metrics":{"active":71,"idle":2,"idle_in_transaction":0,"max_connections":100,"superuser_reserved_connections":3,"memory_limit_bytes":536870912,"collected_at":"2026-01-02T15:04:05.123456789Z"}}`,
		},
		{
			name: "HeartbeatContainer without metrics omits the field",
			v: HeartbeatContainer{
				ID:     "db-b",
				Host:   "10.0.0.5",
				Port:   21001,
				Status: "exited",
			},
			want: `{"id":"db-b","host":"10.0.0.5","port":21001,"status":"exited"}`,
		},
		{
			name: "HeartbeatRequest",
			v: HeartbeatRequest{
				NodeID: "node-1",
				Host:   "10.0.0.5",
				Capacity: Capacity{
					PortRangeMin: 21000,
					PortRangeMax: 21999,
					PortsTotal:   1000,
					PortsUsed:    1,
				},
				Containers: []HeartbeatContainer{
					{
						ID:     "db-a",
						Host:   "10.0.0.5",
						Port:   21000,
						Status: "running",
						Metrics: &HeartbeatConnectionMetrics{
							Active:                       71,
							Idle:                         2,
							IdleInTransaction:            0,
							MaxConnections:               100,
							SuperuserReservedConnections: 3,
							MemoryLimitBytes:             536870912,
							CollectedAt:                  collectedAt,
						},
					},
				},
			},
			want: `{"node_id":"node-1","host":"10.0.0.5","capacity":{"port_range_min":21000,"port_range_max":21999,"ports_total":1000,"ports_used":1},"containers":[{"id":"db-a","host":"10.0.0.5","port":21000,"status":"running","metrics":{"active":71,"idle":2,"idle_in_transaction":0,"max_connections":100,"superuser_reserved_connections":3,"memory_limit_bytes":536870912,"collected_at":"2026-01-02T15:04:05.123456789Z"}}]}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != c.want {
				t.Fatalf("got  %s\nwant %s", b, c.want)
			}
		})
	}
}
