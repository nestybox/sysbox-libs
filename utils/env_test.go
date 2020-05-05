package utils

import "testing"

func TestGetEnvVarInfo(t *testing.T) {

	test := []string{"a=b", "var=1", "other-var=hello", "var2="}
	name := []string{"a", "var", "other-var", "var2"}
	val := []string{"b", "1", "hello", ""}

	for i, _ := range test {
		n, v, err := GetEnvVarInfo(test[i])
		if err != nil {
			t.Errorf("GetEnvVarInfo(%s) failed: returned unexpected error %v", test[i], err)
		}
		if n != name[i] || v != val[i] {
			t.Errorf("GetEnvVarInfo(%s) failed: want %s, %s; got %s, %s", test[i], name[i], val[i], n, v)
		}
	}

	if _, _, err := GetEnvVarInfo("a=b=c"); err == nil {
		t.Errorf("GetEnvVarInfo(%s) failed: expected error, got no error.", "a=b=c")
	}
}
