package scraper

import (
	"testing"

	"github.com/prometheus/prometheus/promql/parser"
)

func TestVectorIsEqual(t *testing.T) {
	t.Parallel()
	type testcaseType struct {
		name        string
		expectEqual bool
		a           string
		b           string
	}
	testcases := []testcaseType{
		{
			name:        "equal",
			expectEqual: true,
			a:           "metric{label=\"value\", label2=\"value2\"}",
			b:           "metric{label=\"value\", label2=\"value2\"}",
		},
		{
			name:        "not equal but same label keys",
			expectEqual: false,
			a:           "metric{label=\"value\", label2=\"value2\"}",
			b:           "metric{label=\"value\", label2=\"value3\"}",
		},
		{
			name:        "not equal but different label keys",
			expectEqual: false,
			a:           "metric{label1=\"value\", label2=\"value2\"}",
			b:           "metric{label=\"value\", label2=\"value2\"}",
		},
	}
	for _, testcase := range testcases {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := parser.ParseExpr(tc.a)
			if err != nil {
				t.Fatalf("could not parse expression: %v", err)
			}
			b, err := parser.ParseExpr(tc.b)
			if err != nil {
				t.Fatalf("could not parse expression: %v", err)
			}
			got, err := vectorIsEqual(a.(*parser.VectorSelector), b.(*parser.VectorSelector))
			if got != tc.expectEqual {
				if err != nil && !tc.expectEqual {
					t.Fatalf("expected no error, got %v", err)
				}
				t.Errorf("expected %v, got %v", tc.expectEqual, got)
			}
		})
	}
}
