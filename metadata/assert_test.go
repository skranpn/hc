package metadata

import "testing"

func TestCompareEqual(t *testing.T) {
	testdata := []struct {
		name  string
		left  any
		right any
		want  bool
	}{
		{"string", "value", "value", true},
		{"int", 1, 1, true},
	}

	a := &Assertion{}
	for _, test := range testdata {
		got, _ := a.compareEqual(test.left, test.right)

		if test.want != got {
			t.Errorf("want: %v, got: %v", test.want, got)
		}
	}
}

func TestComapreNumeric(t *testing.T) {
	testdata := []struct {
		_type string
		left  any
		right any
		op    string
		want  bool
	}{
		{"string", "1", "1", "<", false},
		{"string", "1", "2", "<", true},
		{"string", "1", "0", "<", false},
		{"string", "1", "a", "<", false},
		{"string", "a", "1", "<", false},
		{"string", "1.1", "1.1", "<", false},
		{"string", "1.1", "2.1", "<", true},
		{"string", "1.1", "1.0", "<", false},
		{"int", 1, 1, "<", false},
		{"int", 1, 2, "<", true},
		{"int", 1, 0, "<", false},
		{"float", 1.1, 1.1, "<", false},
		{"float", 1.1, 2.1, "<", true},
		{"float", 1.1, 1.0, "<", false},

		{"string", "1", "1", ">", false},
		{"string", "1", "2", ">", false},
		{"string", "1", "0", ">", true},
		{"string", "1", "a", ">", false},
		{"string", "a", "1", ">", false},
		{"string", "1.1", "1.1", ">", false},
		{"string", "1.1", "2.1", ">", false},
		{"string", "1.1", "1.0", ">", true},
		{"int", 1, 1, ">", false},
		{"int", 1, 2, ">", false},
		{"int", 1, 0, ">", true},
		{"int", 1.1, 1.1, ">", false},
		{"int", 1.1, 2.1, ">", false},
		{"int", 1.1, 1.0, ">", true},

		{"string", "1", "1", "<=", true},
		{"string", "1", "2", "<=", true},
		{"string", "1", "0", "<=", false},
		{"string", "1", "a", "<=", false},
		{"string", "a", "1", "<=", false},
		{"string", "1.1", "1.1", "<=", true},
		{"string", "1.1", "2.1", "<=", true},
		{"string", "1.1", "1.0", "<=", false},
		{"int", 1, 1, "<=", true},
		{"int", 1, 2, "<=", true},
		{"int", 1, 0, "<=", false},
		{"int", 1.1, 1.1, "<=", true},
		{"int", 1.1, 2.1, "<=", true},
		{"int", 1.1, 1.0, "<=", false},

		{"string", "1", "1", ">=", true},
		{"string", "1", "2", ">=", false},
		{"string", "1", "0", ">=", true},
		{"string", "1", "a", ">=", false},
		{"string", "a", "1", ">=", false},
		{"string", "1.1", "1.1", ">=", true},
		{"string", "1.1", "2.1", ">=", false},
		{"string", "1.1", "1.0", ">=", true},
		{"int", 1, 1, ">=", true},
		{"int", 1, 2, ">=", false},
		{"int", 1, 0, ">=", true},
		{"int", 1.1, 1.1, ">=", true},
		{"int", 1.1, 2.1, ">=", false},
		{"int", 1.1, 1.0, ">=", true},
	}

	for _, test := range testdata {
		a := &Assertion{Operator: test.op}
		got, _ := a.compareNumeric(test.left, test.right)

		if test.want != got {
			t.Errorf("%s,%s %v %v, want: %v, got: %v", test.op, test._type, test.left, test.right, test.want, got)
		}
	}
}

func TestContains(t *testing.T) {
	testdata := []struct {
		name  string
		left  any
		right any
		want  bool
	}{
		{"string", "hello", "ll", true},
	}

	for _, test := range testdata {
		a := &Assertion{}
		got, _ := a.compareContains(test.left, test.right)

		if test.want != got {
			t.Errorf("%s, want: %v, got: %v", test.name, test.want, got)
		}
	}
}

func TestIs(t *testing.T) {
	testdata := []struct {
		name  string
		left  any
		right any
		want  bool
	}{
		{"array: []string", []string{"a", "b"}, "array", true},
		{"array: []map[string]any", []map[string]any{{"a": "hello", "b": 123}, {"c": true}}, "array", true},
		{"object: map", map[string]any{"a": "hello", "b": 123}, "object", true},
		{"object: []string", []string{"a", "hello", "b", "123"}, "object", false},
		{"object: string", "123", "object", false},
		{"object: int", 123, "object", false},
		{"object: nil", nil, "object", false},
		{"object: null", "null", "object", false},
		{"null", nil, "null", true},
		{"null: <nil>", "<nil>", "null", true},
		{"null: 0", 0, "null", false},
		{"null: empty string", "", "null", false},
		{"string", "hello", "string", true},
		{"string: empty", "", "string", true},
		{"number: int", 123, "number", true},
		{"number: float", 3.14, "number", true},
		{"bool: true", true, "bool", true},
		{"bool: false", false, "bool", true},
		{"bool: string 1", "1", "bool", true},
		{"bool: string 0", "0", "bool", true},
		{"bool: string 123", "123", "bool", false},
		{"bool: string a", "a", "bool", false},
		{"bool: int 1", 1, "bool", true},
		{"bool: int 0", 0, "bool", true},
		{"bool: int 123", 123, "bool", false},
	}

	for _, test := range testdata {
		a := &Assertion{}
		got, _ := a.compareIs(test.left, test.right)

		if test.want != got {
			t.Errorf("%s, want: %v, got: %v", test.name, test.want, got)
		}
	}
}
