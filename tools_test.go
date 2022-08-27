package toolkit

import "testing"

func TestTools_RandomString(t *testing.T) {
    var testTools Tools
    const l = 10

    s := testTools.RandomString(l)
    if len(s) != l {
        t.Error("wrong length string returned")
    }
}
