package golib

import (
	"math"
	"testing"
)

// near reports whether a and b are the same vector, give or take rounding.
func near(a, b Vector2) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-4 && math.Abs(float64(a.Y-b.Y)) < 1e-4
}

func TestVector2(t *testing.T) {
	v := Vector2{X: 3, Y: 4}
	tests := []struct {
		name      string
		got, want Vector2
	}{
		{"Add", v.Add(Vector2{X: 1, Y: -1}), Vector2{X: 4, Y: 3}},
		{"Sub", v.Sub(Vector2{X: 1, Y: -1}), Vector2{X: 2, Y: 5}},
		{"Scale", v.Scale(2), Vector2{X: 6, Y: 8}},
		{"Normalize", v.Normalize(), Vector2{X: 0.6, Y: 0.8}},
		{"Normalize zero", Vector2{}.Normalize(), Vector2{}},
		{"ClampLength longer", v.ClampLength(2.5), Vector2{X: 1.5, Y: 2}},
		{"ClampLength shorter", v.ClampLength(10), v},
		{"ClampLength zero", Vector2{}.ClampLength(0), Vector2{}},
		{"diagonal move", Vector2{X: 1, Y: 1}.ClampLength(1), Vector2{X: float32(math.Sqrt2) / 2, Y: float32(math.Sqrt2) / 2}},
		{"Rotate 90 is clockwise", Vector2{X: 1}.Rotate(90), Vector2{Y: 1}},
		{"Rotate -90", Vector2{X: 1}.Rotate(-90), Vector2{Y: -1}},
		{"Rotate 180", v.Rotate(180), Vector2{X: -3, Y: -4}},
		{"FromAngle 0", Vector2FromAngle(0), Vector2{X: 1}},
		{"FromAngle 90 is down", Vector2FromAngle(90), Vector2{Y: 1}},
		{"FromAngle 180", Vector2FromAngle(180), Vector2{X: -1}},
		{"MoveTowards part way", Vector2{}.MoveTowards(v, 2.5), Vector2{X: 1.5, Y: 2}},
		{"MoveTowards arrives", Vector2{}.MoveTowards(v, 6), v},
		{"MoveTowards itself", v.MoveTowards(v, 1), v},
		{"Lerp halfway", Vector2{}.Lerp(v, 0.5), Vector2{X: 1.5, Y: 2}},
		{"Lerp start", v.Lerp(Vector2{}, 0), v},
		{"Rectangle.Center", Rectangle{X: 10, Y: 20, Width: 4, Height: 6}.Center(), Vector2{X: 12, Y: 23}},
	}
	for _, test := range tests {
		if !near(test.got, test.want) {
			t.Errorf("%s = %v, want %v", test.name, test.got, test.want)
		}
	}

	numbers := []struct {
		name      string
		got, want float32
	}{
		{"Length", v.Length(), 5},
		{"Distance", v.Distance(Vector2{X: 3, Y: 1}), 3},
		{"Dot", v.Dot(Vector2{X: 2, Y: -1}), 2},
		{"Angle right", Vector2{X: 2}.Angle(), 0},
		{"Angle down", Vector2{Y: 2}.Angle(), 90},
		{"Angle up", Vector2{Y: -2}.Angle(), -90},
		{"Angle left", Vector2{X: -2}.Angle(), 180},
		{"Angle zero", Vector2{}.Angle(), 0},
		{"Angle of FromAngle", Vector2FromAngle(-30).Angle(), -30},
	}
	for _, test := range numbers {
		if math.Abs(float64(test.got-test.want)) > 1e-4 {
			t.Errorf("%s = %v, want %v", test.name, test.got, test.want)
		}
	}
}
