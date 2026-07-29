package config

import (
	"reflect"
	"testing"
)

func TestStringListCleansTrustedProxyConfiguration(t *testing.T) {
	actual := stringList(" 127.0.0.1, 172.16.0.0/12 ,, ::1 ")
	expected := []string{"127.0.0.1", "172.16.0.0/12", "::1"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}
