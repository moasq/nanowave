package appleauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

var (
	srpN, _ = new(big.Int).SetString(
		"AC6BDB41324A9A9BF166DE5E1389582FAF72B6651987EE07FC3192943DB56050"+
			"A37329CBB4A099ED8193E0757767A13DD52312AB4B03310DCD7F48A9DA04FD50"+
			"E8083969EDB767B0CF6095179A163AB3661A05FBD5FAAAE82918A9962F0B93B8"+
			"55F97993EC975EEAA80D740ADBF4FF747359D041D5C33EA71D281E446B14773B"+
			"CA97B43A23FB801676BD207A436C6481F1D2B9078717461A5B9D32E688F87748"+
			"544523B524B0D57D5EA77A2775D2ECFA032CFBDBF52FB3786160279004E57AE6"+
			"AF874E7303CE53299CCC041C7BC308D82A5698F3A8D0C38271AE35F8E9DBFBB6"+
			"94B5C803D89F7AE435DE236D525F54759B65E372FCD68EF20FA7111F9E4AFF73", 16)
	srpG = big.NewInt(2)
)

func srpGenerateClientKeyPair() (a, A *big.Int) {
	aBytes := make([]byte, 256)
	if _, err := rand.Read(aBytes); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	a = new(big.Int).SetBytes(aBytes)
	A = new(big.Int).Exp(srpG, a, srpN)
	return a, A
}

func computeSRP(a, A, B *big.Int, salt []byte, username, password, protocol string, iterations int) (m1, m2 []byte, err error) {
	if new(big.Int).Mod(B, srpN).Sign() == 0 {
		return nil, nil, fmt.Errorf("invalid server public key B")
	}

	var derived []byte
	switch protocol {
	case "s2k":
		pwHash := sha256.Sum256([]byte(password))
		derived = pbkdf2.Key(pwHash[:], salt, iterations, 32, sha256.New)
	case "s2k_fo":
		pwHash := sha256.Sum256([]byte(password))
		hexPW := []byte(hex.EncodeToString(pwHash[:]))
		derived = pbkdf2.Key(hexPW, salt, iterations, 32, sha256.New)
	default:
		return nil, nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	inner := sha256.Sum256(append([]byte{0x3a}, derived...))
	outer := sha256.Sum256(append(salt, inner[:]...))
	x := new(big.Int).SetBytes(outer[:])

	nHexStr := fmt.Sprintf("%x", srpN)
	nlenHexChars := 2 * ((len(nHexStr)*4 + 7) >> 3)

	srpPadHex := func(v *big.Int) string {
		h := fmt.Sprintf("%x", v)
		if len(h) < nlenHexChars {
			h = strings.Repeat("0", nlenHexChars-len(h)) + h
		}
		return h
	}

	srpH := func(args ...*big.Int) *big.Int {
		var buf string
		for _, arg := range args {
			buf += srpPadHex(arg)
		}
		decoded, _ := hex.DecodeString(buf)
		h := sha256.Sum256(decoded)
		result := new(big.Int).SetBytes(h[:])
		result.Mod(result, srpN)
		return result
	}

	u := srpH(A, B)
	k := srpH(srpN, srpG)

	gx := new(big.Int).Exp(srpG, x, srpN)
	kgx := new(big.Int).Mul(k, gx)
	kgx.Mod(kgx, srpN)
	diff := new(big.Int).Sub(B, kgx)
	if diff.Sign() < 0 {
		diff.Add(diff, srpN)
	}
	ux := new(big.Int).Mul(u, x)
	exp := new(big.Int).Add(a, ux)
	S := new(big.Int).Exp(diff, exp, srpN)

	sHex := fmt.Sprintf("%x", S)
	if len(sHex)%2 != 0 {
		sHex = "0" + sHex
	}
	sBytes, _ := hex.DecodeString(sHex)
	K := sha256.Sum256(sBytes)

	shaHex := func(hexStr string) string {
		b, _ := hex.DecodeString(hexStr)
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}

	hNHex := shaHex(srpPadHex(srpN))
	hGHex := shaHex(srpPadHex(srpG))

	hNInt := new(big.Int)
	hNInt.SetString(hNHex, 16)
	hGInt := new(big.Int)
	hGInt.SetString(hGHex, 16)
	hXorInt := new(big.Int).Xor(hNInt, hGInt)
	hXorHex := fmt.Sprintf("%x", hXorInt)
	if len(hXorHex)%2 != 0 {
		hXorHex = "0" + hXorHex
	}

	hUserHex := fmt.Sprintf("%x", sha256.Sum256([]byte(username)))
	saltHex := hex.EncodeToString(salt)

	aHex := fmt.Sprintf("%x", A)
	if len(aHex)%2 != 0 {
		aHex = "0" + aHex
	}
	bHex := fmt.Sprintf("%x", B)
	if len(bHex)%2 != 0 {
		bHex = "0" + bHex
	}

	kHex := hex.EncodeToString(K[:])

	m1HexBuf := hXorHex + hUserHex + saltHex + aHex + bHex + kHex
	m1Bytes, _ := hex.DecodeString(m1HexBuf)
	m1Hash := sha256.Sum256(m1Bytes)
	m1Hex := hex.EncodeToString(m1Hash[:])

	m2HexBuf := aHex + m1Hex + kHex
	m2Bytes, _ := hex.DecodeString(m2HexBuf)
	m2Hash := sha256.Sum256(m2Bytes)

	return m1Hash[:], m2Hash[:], nil
}

// Helper functions for base64 encoding/decoding.
func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func newBigIntFromBase64(s string) *big.Int {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return new(big.Int).SetBytes(b)
}
