package dhcp

import (
	"testing"
)

func TestIPv4_Parse_String_Next_Inc(t *testing.T) {
	i, err := ParseIPv4("1.2.3.254")
	assertTrue(t, err == nil)
	assertEqual(t, "1.2.3.254", i.String())
	i.Inc()
	assertEqual(t, "1.2.3.255", i.String())
	i.Inc()
	assertEqual(t, "1.2.4.0", i.String())
	assertEqual(t, "1.2.4.1", i.Next().String())
	i, err = ParseIPv4("1.2.3.300")
	assertTrue(t, err != nil)
}
