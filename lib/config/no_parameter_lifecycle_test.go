package config

import "testing"

func TestNoParameterLifecycleSwitches(t *testing.T) {
 tests := []struct {
	name string
	values NoParameterConfig
	wantError bool
 }{
	{"default update", NoParameterConfig{"update"}, false},
	{"skip bootstrap", NoParameterConfig{"update", "no-bootstrap"}, false},
	{"skip install", NoParameterConfig{"update", "no-install"}, false},
	{"skip both", NoParameterConfig{"update", "no-bootstrap", "no-install"}, false},
	{"requires update", NoParameterConfig{"no-bootstrap"}, true},
	{"duplicate", NoParameterConfig{"update", "no-install", "no-install"}, true},
 }
 for _, tc := range tests {
	t.Run(tc.name, func(t *testing.T) {
	 _, err := normalizedNoParameter(tc.values)
	 if (err != nil) != tc.wantError {
	  t.Fatalf("normalizedNoParameter(%v): error=%v, wantError=%v", tc.values, err, tc.wantError)
	 }
	})
 }
}
