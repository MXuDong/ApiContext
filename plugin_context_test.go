package api

import (
	"testing"
)

func TestApiContext_BoolValue(t *testing.T) {
	context := NewContext()
	context.SetValue("testBoolRight", true)
	context.SetValue("testBoolFalse", "testFalseBool")

	resBool1, errBool1 := context.BoolValue("testBoolRight")
	_, errBool2 := context.BoolValue("testBoolFalse")
	resString1, errStr1 := context.StringValue("testBoolFalse")
	if errBool1 != nil || resBool1 != true {
		t.Fail()
	}
	if errBool2 == nil {
		t.Fail()
	}
	if len(resString1) == 0 || errStr1 != nil {
		t.Fail()
	}
}
