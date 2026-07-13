package mml

import "testing"

// TestBuildMML verifies the MML command text format (对齐 Java buildMML):
// <CMD>:<NAME1>=<VAL1>,<NAME2>=<VAL2>;  values unquoted, keys sorted.
func TestBuildMML(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		params map[string]interface{}
		want   string
	}{
		{
			name:   "no params",
			cmd:    "LST CELL",
			params: nil,
			want:   "LST CELL;",
		},
		{
			name:   "single param",
			cmd:    "LST CELL",
			params: map[string]interface{}{"LOCALCELLID": "1"},
			want:   "LST CELL:LOCALCELLID=1;",
		},
		{
			name:   "multiple params sorted",
			cmd:    "SET NE",
			params: map[string]interface{}{"LOCAL": "1", "A": "2"},
			want:   "SET NE:A=2,LOCAL=1;",
		},
		{
			name:   "numeric value rendered without quotes",
			cmd:    "MOD CELL",
			params: map[string]interface{}{"CELLID": 5, "POWER": 30.5},
			want:   "MOD CELL:CELLID=5,POWER=30.5;",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildMML(c.cmd, c.params)
			if got != c.want {
				t.Errorf("buildMML(%q, %v) = %q, want %q", c.cmd, c.params, got, c.want)
			}
		})
	}
}
