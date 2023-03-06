//nolint:funlen,lll // ok for param tests
package state

import (
	"reflect"
	"testing"
)

func Test_computeCarChanges(t *testing.T) {
	type args struct {
		ref [][]interface{}
		cur [][]interface{}
	}
	tests := []struct {
		name string
		args args
		want [][3]any
	}{
		{name: "empty", args: args{}, want: [][3]any{}},
		{
			name: "standard",
			args: args{
				ref: [][]interface{}{
					{1, 2, 3, 8.1, "ref", []any{1.22, "ob"}, ""},
				},
				cur: [][]interface{}{
					{5, 2, 7, 8.2, "cur", 1.23, []any{"4.55", "pb"}},
				},
			},
			want: [][3]any{
				{0, 0, 5},
				{0, 2, 7},
				{0, 3, 8.2},
				{0, 4, "cur"},
				{0, 5, 1.23},
				{0, 6, []any{"4.55", "pb"}},
			},
		},
		{
			name: "no change",
			args: args{
				ref: [][]interface{}{
					{1, 2, 3},
				},
				cur: [][]interface{}{
					{1, 2, 3},
				},
			},
			want: [][3]any{},
		},
		{
			name: "interfaces",
			args: args{
				ref: [][]interface{}{
					{[]interface{}{1.22, "ob"}},
				},
				cur: [][]interface{}{
					{[]interface{}{1.22, "ob"}},
				},
			},
			want: [][3]any{},
		},
		{
			name: "additional", // only additional, omitting columns in cur not supported
			args: args{
				ref: [][]interface{}{
					{1, 2, 3},
				},
				cur: [][]interface{}{
					{1, 2, 3, 4},
				},
			},
			want: [][3]any{{0, 3, 4}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeCarChanges(tt.args.ref, tt.args.cur); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeCarChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_computeSessionChanges(t *testing.T) {
	type args struct {
		ref []interface{}
		cur []interface{}
	}
	tests := []struct {
		name string
		args args
		want [][2]any
	}{
		{name: "empty", args: args{}, want: [][2]any{}},
		{
			name: "standard",
			args: args{
				ref: []any{1, 2, "a", "b"},
				cur: []any{5, 2, "x", "y"},
			},
			want: [][2]any{{0, 5}, {2, "x"}, {3, "y"}},
		},
		{
			name: "no change",
			args: args{
				ref: []any{1, 2},
				cur: []any{1, 2},
			},
			want: [][2]any{},
		},
		{
			name: "additional", // only additional, omitting columns in cur not supported
			args: args{
				ref: []any{1, 2},
				cur: []any{1, 2, 3},
			},
			want: [][2]any{{2, 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeSessionChanges(tt.args.ref, tt.args.cur); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeSessionChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}
