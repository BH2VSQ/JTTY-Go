package macro

import "testing"

func TestRenderMacros(t *testing.T) {
	c := Context{MyCall: "N0CALL", MyGrid: "PM95", DXCall: "W1ABC", Exchange: "599 001", QueueCall: "K1XYZ"}
	got := Render("%M %G %H %E %Q", c)
	want := "N0CALL PM95 W1ABC 599 001 K1XYZ"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
