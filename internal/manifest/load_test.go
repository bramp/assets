//nolint:testpackage // Load helper tests exercise unexported merge helpers.
package manifest

import (
	"reflect"
	"testing"
)

func TestMergeYAMLMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		want     map[string]any
	}{
		{
			name: "adds_new_top_level_keys",
			base: map[string]any{
				"meta": map[string]any{"project": "x"},
			},
			override: map[string]any{
				"assets": []any{},
			},
			want: map[string]any{
				"meta":   map[string]any{"project": "x"},
				"assets": []any{},
			},
		},
		{
			name: "recursively_merges_maps",
			base: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"tools": map[string]any{
							"resvg": map[string]any{
								"accepts": []any{".svg"},
								"command": "resvg {input} {output}",
							},
						},
					},
				},
			},
			override: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"tools": map[string]any{
							"resvg": map[string]any{
								"command": "custom-resvg {input} {output}",
							},
						},
					},
				},
			},
			want: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"tools": map[string]any{
							"resvg": map[string]any{
								"accepts": []any{".svg"},
								"command": "custom-resvg {input} {output}",
							},
						},
					},
				},
			},
		},
		{
			name: "override_scalar_replaces_map",
			base: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"defaults": map[string]any{"tools": []any{"resvg"}},
					},
				},
			},
			override: map[string]any{
				"meta": "disabled",
			},
			want: map[string]any{
				"meta": "disabled",
			},
		},
		{
			name: "override_sequence_replaces_sequence",
			base: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"defaults": map[string]any{
							"tools": []any{"resvg", "inkscape"},
						},
					},
				},
			},
			override: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"defaults": map[string]any{
							"tools": []any{"inkscape"},
						},
					},
				},
			},
			want: map[string]any{
				"meta": map[string]any{
					"render": map[string]any{
						"defaults": map[string]any{
							"tools": []any{"inkscape"},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			baseBefore, ok := cloneYAMLValue(tc.base).(map[string]any)
			if !ok {
				t.Fatal("clone of base was not a map")
			}
			overrideBefore, ok := cloneYAMLValue(tc.override).(map[string]any)
			if !ok {
				t.Fatal("clone of override was not a map")
			}

			got := mergeYAMLMaps(tc.base, tc.override)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mergeYAMLMaps() mismatch\n got: %#v\nwant: %#v", got, tc.want)
			}

			if !reflect.DeepEqual(tc.base, baseBefore) {
				t.Fatalf("mergeYAMLMaps() mutated base input")
			}
			if !reflect.DeepEqual(tc.override, overrideBefore) {
				t.Fatalf("mergeYAMLMaps() mutated override input")
			}
		})
	}
}
