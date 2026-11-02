package livephoto

import "testing"

func TestAspect(t *testing.T) {
	if a := (Photo{Width: 3024, Height: 4032}).Aspect(); a != 0.75 {
		t.Fatalf("aspect = %v", a)
	}
	if a := (Photo{}).Aspect(); a != 1 {
		t.Fatalf("unknown aspect = %v", a)
	}
}
