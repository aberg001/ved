package main
import "testing"
func mkrows(n int) []rline {
	rows := make([]rline, n)
	for i := range rows { rows[i].lineNum = i + 1 }
	return rows
}
func kinds(rows []rline) []int {
	out := []int{}
	for _, r := range rows { out = append(out, r.kind) }
	return out
}
func eqi(a, b []int) bool { if len(a)!=len(b) {return false}; for i:=range a {if a[i]!=b[i] {return false}}; return true }
func TestOverlayMove(t *testing.T) {
	cases := []struct{ name string; mv MoveInfo; want []int }{
		// 1,3m4 on 6 lines: rows 2-4 teal (moved-to)
		{"1,3m4", MoveInfo{1,3,4,false}, []int{0,4,4,4,0,0}},
		// 1,3m5: rows 3-5 teal
		{"1,3m5", MoveInfo{1,3,5,false}, []int{0,0,4,4,4,0}},
		// 4,6m1: rows 2-4 teal (no shift when dest before block)
		{"4,6m1", MoveInfo{4,6,1,false}, []int{0,4,4,4,0,0}},
		// copy: 1,2t4 → inserted block at rows 5-6
		{"1,2t4", MoveInfo{1,2,4,true}, []int{0,0,0,0,4,4}},
	}
	for _, c := range cases {
		rows := mkrows(6)
		overlayMove(rows, &c.mv)
		if !eqi(kinds(rows), c.want) {
			t.Errorf("%s: got %v want %v", c.name, kinds(rows), c.want)
		}
	}
}
