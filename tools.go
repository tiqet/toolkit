package toolkit

import (
    "crypto/rand"
    "math/big"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is a type used to instantiate this module. Any variable of this type will have access
// to all the methods with the receiver *Tools
type Tools struct{}

// RandomString returns a string of random characters of length n, using randomStringSource
// as a source of characters to generate random string
func (t *Tools) RandomString(n int) string {
    var (
        p    *big.Int
        x, y uint64
    )
    s, r := make([]rune, n), []rune(randomStringSource)
    for i := range s {
        p, _ = rand.Prime(rand.Reader, len(r))
        x, y = p.Uint64(), uint64(len(r))
        s[i] = r[x%y]
    }
    return string(s)
}
