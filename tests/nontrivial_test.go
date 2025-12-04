// Integration tests for tsql_nontrivial examples (non-decimal functions)
// Run with: go test -v ./tests/...
package tests

import (
	"encoding/base64"
	"testing"
	"unicode/utf8"
)

// =============================================================================
// 01_levenshtein.sql - LevenshteinDistance
// =============================================================================

func LevenshteinDistance(source string, target string) (distance int32) {
	var sourceLen int32 = int32(utf8.RuneCountInString(source))
	_ = sourceLen
	var targetLen int32 = int32(utf8.RuneCountInString(target))
	_ = targetLen
	if sourceLen == 0 {
		distance = targetLen
		return distance
	}
	if targetLen == 0 {
		distance = sourceLen
		return distance
	}
	var prevRow string
	_ = prevRow
	var currRow string
	_ = currRow
	var i int32
	_ = i
	var j int32
	_ = j
	var cost int32
	_ = cost
	var insertCost int32
	_ = insertCost
	var deleteCost int32
	_ = deleteCost
	var replaceCost int32
	_ = replaceCost
	var minCost int32
	_ = minCost
	var prevDiag int32
	_ = prevDiag
	var prevRowJ int32
	_ = prevRowJ
	prevRow = ""
	i = 0
	for i <= targetLen {
		prevRow = (prevRow + string(rune(i)))
		i = (i + 1)
	}
	i = 1
	for i <= sourceLen {
		currRow = string(rune(i))
		prevDiag = (i - 1)
		j = 1
		for j <= targetLen {
			if (source)[(i)-1:(i)-1+(1)] == (target)[(j)-1:(j)-1+(1)] {
				cost = 0
			} else {
				cost = 1
			}
			prevRowJ = int32([]rune((prevRow)[((j + 1))-1:((j+1))-1+(1)])[0])
			insertCost = (int32([]rune((currRow)[(j)-1:(j)-1+(1)])[0]) + 1)
			deleteCost = (prevRowJ + 1)
			replaceCost = (prevDiag + cost)
			minCost = insertCost
			if deleteCost < minCost {
				minCost = deleteCost
			}
			if replaceCost < minCost {
				minCost = replaceCost
			}
			currRow = (currRow + string(rune(minCost)))
			prevDiag = prevRowJ
			j = (j + 1)
		}
		prevRow = currRow
		i = (i + 1)
	}
	distance = int32([]rune((currRow)[((targetLen + 1))-1:((targetLen+1))-1+(1)])[0])
	return distance
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		target   string
		expected int32
	}{
		{"identical strings", "hello", "hello", 0},
		{"empty source", "", "hello", 5},
		{"empty target", "hello", "", 5},
		{"both empty", "", "", 0},
		{"one char diff", "cat", "bat", 1},
		{"insertion", "cat", "cart", 1},
		{"deletion", "cart", "cat", 1},
		{"kitten to sitting", "kitten", "sitting", 3},
		{"Saturday to Sunday", "Saturday", "Sunday", 3},
		{"completely different", "abc", "xyz", 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := LevenshteinDistance(tc.source, tc.target)
			if result != tc.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d", tc.source, tc.target, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 02_extended_euclidean.sql - ExtendedEuclidean
// =============================================================================

func ExtendedEuclidean(a int64, b int64) (gcd int64, x int64, y int64) {
	var originalA int64 = a
	_ = originalA
	var originalB int64 = b
	_ = originalB
	if a < 0 {
		a = (-a)
	}
	if b < 0 {
		b = (-b)
	}
	if a == 0 {
		gcd = b
		x = 0
		y = 1
		return gcd, x, y
	}
	if b == 0 {
		gcd = a
		x = 1
		y = 0
		return gcd, x, y
	}
	var x0 int64 = 1
	_ = x0
	var x1 int64 = 0
	_ = x1
	var y0 int64 = 0
	_ = y0
	var y1 int64 = 1
	_ = y1
	var quotient int64
	_ = quotient
	var remainder int64
	_ = remainder
	var tempX int64
	_ = tempX
	var tempY int64
	_ = tempY
	var tempVal int64
	_ = tempVal
	for b != 0 {
		quotient = (a / b)
		remainder = (a % b)
		tempX = (x0 - (quotient * x1))
		x0 = x1
		x1 = tempX
		tempY = (y0 - (quotient * y1))
		y0 = y1
		y1 = tempY
		a = b
		b = remainder
	}
	gcd = a
	x = x0
	y = y0
	if originalA < 0 {
		x = (-x)
	}
	if originalB < 0 {
		y = (-y)
	}
	return gcd, x, y
}

func TestExtendedEuclidean(t *testing.T) {
	tests := []struct {
		name        string
		a, b        int64
		expectedGcd int64
	}{
		{"GCD of 48 and 18", 48, 18, 6},
		{"GCD of 35 and 15", 35, 15, 5},
		{"GCD of 101 and 103 (coprimes)", 101, 103, 1},
		{"GCD with zero", 42, 0, 42},
		{"GCD of 0 and 42", 0, 42, 42},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gcd, x, y := ExtendedEuclidean(tc.a, tc.b)
			if gcd != tc.expectedGcd {
				t.Errorf("ExtendedEuclidean(%d, %d) gcd = %d; want %d", tc.a, tc.b, gcd, tc.expectedGcd)
			}
			// Verify Bézout's identity: ax + by = gcd(a, b)
			if tc.a != 0 && tc.b != 0 {
				computed := tc.a*x + tc.b*y
				if computed != gcd {
					t.Errorf("Bézout identity failed: %d*%d + %d*%d = %d; want %d",
						tc.a, x, tc.b, y, computed, gcd)
				}
			}
		})
	}
}

// =============================================================================
// 03_base64_encode.sql - Base64Encode
// =============================================================================

func Base64Encode(input string) (output string) {
	var inputLen int32 = int32(utf8.RuneCountInString(input))
	_ = inputLen
	var base64Chars string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	_ = base64Chars
	var pos int32 = 1
	_ = pos
	var byte1 int32
	_ = byte1
	var byte2 int32
	_ = byte2
	var byte3 int32
	_ = byte3
	var triple int32
	_ = triple
	var index1 int32
	_ = index1
	var index2 int32
	_ = index2
	var index3 int32
	_ = index3
	var index4 int32
	_ = index4
	output = ""
	for pos <= inputLen {
		byte1 = int32(((input)[(pos)-1 : (pos)-1+(1)])[0])
		if (pos + 1) <= inputLen {
			byte2 = int32(((input)[((pos + 1))-1 : ((pos+1))-1+(1)])[0])
		} else {
			byte2 = 0
		}
		if (pos + 2) <= inputLen {
			byte3 = int32(((input)[((pos + 2))-1 : ((pos+2))-1+(1)])[0])
		} else {
			byte3 = 0
		}
		triple = ((byte1 * 65536) + (byte2 * 256)) + byte3
		index1 = ((triple / 262144) % 64)
		index2 = ((triple / 4096) % 64)
		index3 = ((triple / 64) % 64)
		index4 = (triple % 64)
		output = (output + (base64Chars)[(index1+1)-1:(index1+1)-1+(1)])
		output = (output + (base64Chars)[(index2+1)-1:(index2+1)-1+(1)])
		if (pos + 1) <= inputLen {
			output = (output + (base64Chars)[(index3+1)-1:(index3+1)-1+(1)])
		} else {
			output = (output + "=")
		}
		if (pos + 2) <= inputLen {
			output = (output + (base64Chars)[(index4+1)-1:(index4+1)-1+(1)])
		} else {
			output = (output + "=")
		}
		pos = (pos + 3)
	}
	return output
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single char", "M", "TQ=="},
		{"two chars", "Ma", "TWE="},
		{"three chars", "Man", "TWFu"},
		{"hello", "Hello", "SGVsbG8="},
		{"hello world", "Hello, World!", "SGVsbG8sIFdvcmxkIQ=="},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Base64Encode(tc.input)
			// Verify against Go's standard base64
			expected := base64.StdEncoding.EncodeToString([]byte(tc.input))
			if result != expected {
				t.Errorf("Base64Encode(%q) = %q; want %q", tc.input, result, expected)
			}
		})
	}
}

// =============================================================================
// 04_run_length_encoding.sql - RunLengthEncode
// =============================================================================

func RunLengthEncode(input string) (encoded string) {
	var inputLen int32 = int32(utf8.RuneCountInString(input))
	_ = inputLen
	var pos int32 = 1
	_ = pos
	var currentChar string
	_ = currentChar
	var runLength int32
	_ = runLength
	var nextChar string
	_ = nextChar
	var counting bool
	_ = counting
	encoded = ""
	if inputLen == 0 {
		return encoded
	}
	for pos <= inputLen {
		currentChar = (input)[(pos)-1 : (pos)-1+(1)]
		runLength = 1
		counting = true
		for counting && ((pos + runLength) <= inputLen) {
			nextChar = (input)[((pos + runLength))-1 : ((pos+runLength))-1+(1)]
			if nextChar == currentChar {
				runLength = (runLength + 1)
			} else {
				counting = false
			}
		}
		// Use fmt.Sprintf equivalent - simplified
		if runLength < 10 {
			encoded = encoded + string(rune('0'+runLength)) + currentChar
		} else {
			// For runs >= 10, use simple conversion
			r := runLength
			s := ""
			for r > 0 {
				s = string(rune('0'+r%10)) + s
				r = r / 10
			}
			encoded = encoded + s + currentChar
		}
		pos = (pos + runLength)
	}
	return encoded
}

func TestRunLengthEncode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single char", "A", "1A"},
		{"no repeats", "ABC", "1A1B1C"},
		{"all same", "AAAA", "4A"},
		{"mixed", "AAABBC", "3A2B1C"},
		{"alternating", "ABABAB", "1A1B1A1B1A1B"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RunLengthEncode(tc.input)
			if result != tc.expected {
				t.Errorf("RunLengthEncode(%q) = %q; want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 06_easter_computus.sql - CalculateEasterDate
// =============================================================================

func CalculateEasterDate(year int32) (easterMonth int32, easterDay int32) {
	var a int32
	_ = a
	var b int32
	_ = b
	var c int32
	_ = c
	var d int32
	_ = d
	var e int32
	_ = e
	var f int32
	_ = f
	var g int32
	_ = g
	var h int32
	_ = h
	var i int32
	_ = i
	var k int32
	_ = k
	var l int32
	_ = l
	var m int32
	_ = m
	a = (year % 19)
	b = (year / 100)
	c = (year % 100)
	d = (b / 4)
	e = (b % 4)
	f = ((b + 8) / 25)
	g = (((b - f) + 1) / 3)
	h = ((((((19 * a) + b) - d) - g) + 15) % 30)
	i = (c / 4)
	k = (c % 4)
	l = (((((32 + (2 * e)) + (2 * i)) - h) - k) % 7)
	m = (((a + (11 * h)) + (22 * l)) / 451)
	easterMonth = ((((h + l) - (7 * m)) + 114) / 31)
	easterDay = (((((h + l) - (7 * m)) + 114) % 31) + 1)
	return easterMonth, easterDay
}

func TestCalculateEasterDate(t *testing.T) {
	// Known Easter dates
	tests := []struct {
		name          string
		year          int32
		expectedMonth int32
		expectedDay   int32
	}{
		{"Easter 2020", 2020, 4, 12},
		{"Easter 2021", 2021, 4, 4},
		{"Easter 2022", 2022, 4, 17},
		{"Easter 2023", 2023, 4, 9},
		{"Easter 2024", 2024, 3, 31},
		{"Easter 2025", 2025, 4, 20},
		{"Easter 2000", 2000, 4, 23},
		{"Easter 1999", 1999, 4, 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			month, day := CalculateEasterDate(tc.year)
			if month != tc.expectedMonth || day != tc.expectedDay {
				t.Errorf("CalculateEasterDate(%d) = %d/%d; want %d/%d",
					tc.year, month, day, tc.expectedMonth, tc.expectedDay)
			}
		})
	}
}

// =============================================================================
// 07_modular_arithmetic.sql - ModularExponentiation
// =============================================================================

func ModularExponentiation(base int64, exponent int64, modulus int64) (result int64) {
	var currentBase int64
	_ = currentBase
	var currentExp int64
	_ = currentExp
	if modulus == 1 {
		result = 0
		return result
	}
	if exponent == 0 {
		result = 1
		return result
	}
	if exponent < 0 {
		result = 0
		return result
	}
	currentBase = (base % modulus)
	if currentBase < 0 {
		currentBase = (currentBase + modulus)
	}
	result = 1
	currentExp = exponent
	for currentExp > 0 {
		if (currentExp % 2) == 1 {
			result = ((result * currentBase) % modulus)
		}
		currentBase = ((currentBase * currentBase) % modulus)
		currentExp = (currentExp / 2)
	}
	return result
}

func TestModularExponentiation(t *testing.T) {
	tests := []struct {
		name     string
		base     int64
		exp      int64
		mod      int64
		expected int64
	}{
		{"2^10 mod 1000", 2, 10, 1000, 24},
		{"3^7 mod 13", 3, 7, 13, 3},
		{"5^3 mod 13", 5, 3, 13, 8},
		{"7^256 mod 13", 7, 256, 13, 9},
		{"anything mod 1", 12345, 67890, 1, 0},
		{"x^0 mod m", 999, 0, 100, 1},
		{"2^20 mod 1000000007", 2, 20, 1000000007, 1048576},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ModularExponentiation(tc.base, tc.exp, tc.mod)
			if result != tc.expected {
				t.Errorf("ModularExponentiation(%d, %d, %d) = %d; want %d",
					tc.base, tc.exp, tc.mod, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 07b_modular_inverse.sql - ModularInverse
// =============================================================================

func ModularInverse(a int64, modulus int64) (inverse int64, exists bool) {
	var m0 int64 = modulus
	_ = m0
	var x0 int64 = 0
	_ = x0
	var x1 int64 = 1
	_ = x1
	var quotient int64
	_ = quotient
	var temp int64
	_ = temp
	var tempA int64 = a
	_ = tempA
	exists = false
	inverse = 0
	if modulus == 1 {
		return inverse, exists
	}
	for tempA > 1 {
		if modulus == 0 {
			return inverse, exists
		}
		quotient = (tempA / modulus)
		temp = modulus
		modulus = (tempA % modulus)
		tempA = temp
		temp = x0
		x0 = (x1 - (quotient * x0))
		x1 = temp
	}
	if tempA == 1 {
		exists = true
		if x1 < 0 {
			x1 = (x1 + m0)
		}
		inverse = x1
	}
	return inverse, exists
}

func TestModularInverse(t *testing.T) {
	tests := []struct {
		name         string
		a            int64
		modulus      int64
		expectExists bool
		expectValue  int64
	}{
		{"3 mod 11", 3, 11, true, 4},       // 3*4 = 12 ≡ 1 (mod 11)
		{"10 mod 17", 10, 17, true, 12},    // 10*12 = 120 ≡ 1 (mod 17)
		{"7 mod 26", 7, 26, true, 15},      // 7*15 = 105 ≡ 1 (mod 26)
		{"no inverse: 2 mod 4", 2, 4, false, 0},
		{"no inverse: 6 mod 9", 6, 9, false, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inverse, exists := ModularInverse(tc.a, tc.modulus)
			if exists != tc.expectExists {
				t.Errorf("ModularInverse(%d, %d) exists = %v; want %v",
					tc.a, tc.modulus, exists, tc.expectExists)
			}
			if exists && inverse != tc.expectValue {
				t.Errorf("ModularInverse(%d, %d) = %d; want %d",
					tc.a, tc.modulus, inverse, tc.expectValue)
			}
			// Verify: a * inverse ≡ 1 (mod modulus)
			if exists {
				product := (tc.a * inverse) % tc.modulus
				if product != 1 {
					t.Errorf("Verification failed: %d * %d mod %d = %d; want 1",
						tc.a, inverse, tc.modulus, product)
				}
			}
		})
	}
}

// =============================================================================
// 08_lcs.sql - LongestCommonSubsequenceLength
// =============================================================================

func LongestCommonSubsequenceLength(string1 string, string2 string) (length int32) {
	var len1 int32 = int32(utf8.RuneCountInString(string1))
	_ = len1
	var len2 int32 = int32(utf8.RuneCountInString(string2))
	_ = len2
	if len1 == 0 {
		length = 0
		return length
	}
	if len2 == 0 {
		length = 0
		return length
	}
	var prevRow string
	_ = prevRow
	var currRow string
	_ = currRow
	var i int32
	_ = i
	var j int32
	_ = j
	var val int32
	_ = val
	var diagVal int32
	_ = diagVal
	var leftVal int32
	_ = leftVal
	var upVal int32
	_ = upVal
	prevRow = ""
	i = 0
	for i <= len2 {
		prevRow = (prevRow + string(rune(0)))
		i = (i + 1)
	}
	i = 1
	for i <= len1 {
		currRow = string(rune(0))
		j = 1
		for j <= len2 {
			if (string1)[(i)-1:(i)-1+(1)] == (string2)[(j)-1:(j)-1+(1)] {
				diagVal = int32([]rune((prevRow)[(j)-1:(j)-1+(1)])[0])
				val = (diagVal + 1)
			} else {
				leftVal = int32([]rune((currRow)[(j)-1:(j)-1+(1)])[0])
				upVal = int32([]rune((prevRow)[((j + 1))-1:((j+1))-1+(1)])[0])
				if leftVal > upVal {
					val = leftVal
				} else {
					val = upVal
				}
			}
			currRow = (currRow + string(rune(val)))
			j = (j + 1)
		}
		prevRow = currRow
		i = (i + 1)
	}
	length = int32([]rune((currRow)[((len2 + 1))-1:((len2+1))-1+(1)])[0])
	return length
}

func TestLongestCommonSubsequenceLength(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int32
	}{
		{"identical strings", "ABC", "ABC", 3},
		{"empty first", "", "ABC", 0},
		{"empty second", "ABC", "", 0},
		{"both empty", "", "", 0},
		{"no common", "ABC", "XYZ", 0},
		{"ABCD and ACD", "ABCD", "ACD", 3},
		{"classic example", "AGGTAB", "GXTXAYB", 4}, // GTAB
		{"interleaved", "ABCBDAB", "BDCABA", 4},     // BCBA or BDAB
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := LongestCommonSubsequenceLength(tc.s1, tc.s2)
			if result != tc.expected {
				t.Errorf("LongestCommonSubsequenceLength(%q, %q) = %d; want %d",
					tc.s1, tc.s2, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 10_checksums.sql - CRC16_CCITT
// =============================================================================

func CrC16Ccitt(data string, initialValue int32) (checksum int32) {
	var dataLen int32 = int32(utf8.RuneCountInString(data))
	_ = dataLen
	var polynomial int32 = 4129
	_ = polynomial
	var crc int32 = initialValue
	_ = crc
	var i int32
	_ = i
	var j int32
	_ = j
	var byteVal int32
	_ = byteVal
	var msb int32
	_ = msb
	i = 1
	for i <= dataLen {
		byteVal = int32(((data)[(i)-1 : (i)-1+(1)])[0])
		crc = (crc ^ (byteVal * 256))
		j = 0
		for j < 8 {
			msb = (crc / 32768)
			crc = ((crc * 2) & 65535)
			if msb == 1 {
				crc = (crc ^ polynomial)
			}
			j = (j + 1)
		}
		i = (i + 1)
	}
	checksum = crc
	return checksum
}

func TestCrC16Ccitt(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		initial  int32
		expected int32
	}{
		{"empty string", "", 0xFFFF, 0xFFFF},
		{"single A", "A", 0xFFFF, 0xB915}, // Corrected value for this implementation
		{"123456789", "123456789", 0xFFFF, 0x29B1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CrC16Ccitt(tc.data, tc.initial)
			if result != tc.expected {
				t.Errorf("CrC16Ccitt(%q, 0x%X) = 0x%X; want 0x%X",
					tc.data, tc.initial, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 10b_adler32.sql - Adler32
// =============================================================================

func Adler32(data string) (checksum int64) {
	var modAdler int64 = 65521
	_ = modAdler
	var a int64 = 1
	_ = a
	var b int64 = 0
	_ = b
	var i int32 = 1
	_ = i
	var dataLen int32 = int32(utf8.RuneCountInString(data))
	_ = dataLen
	var byteVal int32
	_ = byteVal
	for i <= dataLen {
		byteVal = int32(((data)[(i)-1 : (i)-1+(1)])[0])
		a = ((a + int64(byteVal)) % modAdler)
		b = ((b + a) % modAdler)
		i = (i + 1)
	}
	checksum = ((b * 65536) + a)
	return checksum
}

func TestAdler32(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected int64
	}{
		{"empty string", "", 1},
		{"Wikipedia", "Wikipedia", 0x11E60398},
		{"a", "a", 0x00620062},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Adler32(tc.data)
			if result != tc.expected {
				t.Errorf("Adler32(%q) = 0x%X; want 0x%X", tc.data, result, tc.expected)
			}
		})
	}
}
