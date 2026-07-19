package api

import "testing"

func TestLaplace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		success int
		failure int
		want    int
	}{
		// Prior neutre : (0+1)*100/(0+0+2) = 100/2 = 50
		{name: "Laplace(0,0) = 50 (prior neutre)", success: 0, failure: 0, want: 50},
		// (1+1)*100/(1+0+2) = 200/3 = 66 (division entiere)
		{name: "Laplace(1,0) = 66", success: 1, failure: 0, want: 66},
		// (10+1)*100/(10+0+2) = 1100/12 = 91
		{name: "Laplace(10,0) = 91", success: 10, failure: 0, want: 91},
		// (0+1)*100/(0+10+2) = 100/12 = 8
		{name: "Laplace(0,10) = 8", success: 0, failure: 10, want: 8},
		// (5+1)*100/(5+5+2) = 600/12 = 50
		{name: "Laplace(5,5) = 50", success: 5, failure: 5, want: 50},
		// Monotonie : plus de succes → score plus eleve
		{name: "Laplace(2,0) > Laplace(1,0)", success: 2, failure: 0, want: 75}, // (3*100)/4=75
		{name: "Laplace(3,0) > Laplace(2,0)", success: 3, failure: 0, want: 80}, // (4*100)/5=80
		// Monotonie decroissante sur les echecs
		{name: "Laplace(1,1) < Laplace(1,0)", success: 1, failure: 1, want: 50}, // (2*100)/4=50
		// Entrees negatives bornees a 0 → identique a (0,0)
		{name: "Laplace(-1,0) = 50 (borne a 0)", success: -1, failure: 0, want: 50},
		{name: "Laplace(0,-5) = 50 (borne a 0)", success: 0, failure: -5, want: 50},
		// Grand nombre : (100+1)*100/(100+0+2) = 10100/102 = 99
		{name: "Laplace(100,0) = 99", success: 100, failure: 0, want: 99},
		// (0+1)*100/(0+100+2) = 100/102 = 0
		{name: "Laplace(0,100) = 0", success: 0, failure: 100, want: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Laplace(tc.success, tc.failure)
			if got != tc.want {
				t.Errorf("Laplace(%d, %d) = %d, voulu %d",
					tc.success, tc.failure, got, tc.want)
			}
		})
	}
}
