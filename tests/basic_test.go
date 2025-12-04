// Integration tests for tsql_basic examples (non-decimal functions)
// Run with: go test -v ./tests/...
package tests

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Suppress unused import errors
var _ = math.Sqrt
var _ = strconv.Atoi
var _ = strings.Contains
var _ = time.Now
var _ = utf8.RuneCountInString

// =============================================================================
// 01_simple_add.sql - AddNumbers
// =============================================================================

func AddNumbers(a int32, b int32) (result int32) {
	result = (a + b)
	return result
}

func TestAddNumbers(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int32
		expected int32
	}{
		{"positive numbers", 3, 5, 8},
		{"zero values", 0, 0, 0},
		{"negative numbers", -10, -5, -15},
		{"mixed signs", 10, -3, 7},
		{"large numbers", 1000000, 2000000, 3000000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AddNumbers(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("AddNumbers(%d, %d) = %d; want %d", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 02_factorial.sql - Factorial
// =============================================================================

func Factorial(n int32) (result int64) {
	var counter int32 = 1
	_ = counter
	result = 1
	for counter <= n {
		result = (result * int64(counter))
		counter = (counter + 1)
	}
	return result
}

func TestFactorial(t *testing.T) {
	tests := []struct {
		name     string
		n        int32
		expected int64
	}{
		{"factorial of 0", 0, 1},
		{"factorial of 1", 1, 1},
		{"factorial of 5", 5, 120},
		{"factorial of 10", 10, 3628800},
		{"factorial of 12", 12, 479001600},
		{"factorial of 20", 20, 2432902008176640000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Factorial(tc.n)
			if result != tc.expected {
				t.Errorf("Factorial(%d) = %d; want %d", tc.n, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 04_gcd.sql - GCD (Greatest Common Divisor)
// =============================================================================

func Gcd(a int32, b int32) (result int32) {
	var temp int32
	_ = temp
	if a < 0 {
		a = (-a)
	}
	if b < 0 {
		b = (-b)
	}
	for b != 0 {
		temp = b
		b = (a % b)
		a = temp
	}
	result = a
	return result
}

func TestGcd(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int32
		expected int32
	}{
		{"GCD of 48 and 18", 48, 18, 6},
		{"GCD of 100 and 25", 100, 25, 25},
		{"GCD of 17 and 13 (coprimes)", 17, 13, 1},
		{"GCD with zero", 42, 0, 42},
		{"GCD of negatives", -48, -18, 6},
		{"GCD equal numbers", 7, 7, 7},
		{"GCD of 1 and any", 1, 999, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Gcd(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("Gcd(%d, %d) = %d; want %d", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 05_is_prime.sql - IsPrime
// =============================================================================

func IsPrime(n int32) (isPrime bool) {
	var divisor int32 = 2
	_ = divisor
	var limit int32
	_ = limit
	if n <= 1 {
		isPrime = false
		return isPrime
	}
	if n <= 3 {
		isPrime = true
		return isPrime
	}
	if (n % 2) == 0 {
		isPrime = false
		return isPrime
	}
	limit = int32(math.Sqrt(float64(float64(n))))
	divisor = 3
	for divisor <= limit {
		if (n % divisor) == 0 {
			isPrime = false
			return isPrime
		}
		divisor = (divisor + 2)
	}
	isPrime = true
	return isPrime
}

func TestIsPrime(t *testing.T) {
	tests := []struct {
		name     string
		n        int32
		expected bool
	}{
		{"0 is not prime", 0, false},
		{"1 is not prime", 1, false},
		{"2 is prime", 2, true},
		{"3 is prime", 3, true},
		{"4 is not prime", 4, false},
		{"17 is prime", 17, true},
		{"18 is not prime", 18, false},
		{"97 is prime", 97, true},
		{"100 is not prime", 100, false},
		{"997 is prime", 997, true},
		{"negative is not prime", -5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPrime(tc.n)
			if result != tc.expected {
				t.Errorf("IsPrime(%d) = %v; want %v", tc.n, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 06_fibonacci.sql - Fibonacci
// =============================================================================

func Fibonacci(n int32) (result int64) {
	var prev1 int64 = 0
	_ = prev1
	var prev2 int64 = 1
	_ = prev2
	var counter int32 = 2
	_ = counter
	var temp int64
	_ = temp
	if n <= 0 {
		result = 0
		return result
	}
	if n == 1 {
		result = 1
		return result
	}
	for counter <= n {
		temp = (prev1 + prev2)
		prev1 = prev2
		prev2 = temp
		counter = (counter + 1)
	}
	result = prev2
	return result
}

func TestFibonacci(t *testing.T) {
	tests := []struct {
		name     string
		n        int32
		expected int64
	}{
		{"F(0)", 0, 0},
		{"F(1)", 1, 1},
		{"F(2)", 2, 1},
		{"F(3)", 3, 2},
		{"F(4)", 4, 3},
		{"F(5)", 5, 5},
		{"F(10)", 10, 55},
		{"F(20)", 20, 6765},
		{"F(30)", 30, 832040},
		{"F(negative)", -1, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Fibonacci(tc.n)
			if result != tc.expected {
				t.Errorf("Fibonacci(%d) = %d; want %d", tc.n, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 08_count_words.sql - CountWords
// =============================================================================

func CountWords(text string) (wordCount int32) {
	var inWord bool = false
	_ = inWord
	var i int32
	_ = i
	var textLen int32
	_ = textLen
	var currentChar string
	_ = currentChar
	wordCount = 0
	textLen = int32(utf8.RuneCountInString(text))
	i = 1
	for i <= textLen {
		currentChar = (text)[(i)-1 : (i)-1+(1)]
		if (currentChar == " ") || (currentChar == "\t") || (currentChar == "\n") || (currentChar == "\r") {
			inWord = false
		} else {
			if !inWord {
				wordCount = (wordCount + 1)
				inWord = true
			}
		}
		i = (i + 1)
	}
	return wordCount
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int32
	}{
		{"empty string", "", 0},
		{"single word", "hello", 1},
		{"two words", "hello world", 2},
		{"multiple spaces", "hello    world", 2},
		{"leading/trailing spaces", "  hello world  ", 2},
		{"tabs and newlines", "hello\tworld\ntest", 3},
		{"sentence", "The quick brown fox jumps over the lazy dog", 9},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CountWords(tc.text)
			if result != tc.expected {
				t.Errorf("CountWords(%q) = %d; want %d", tc.text, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 09_validate_email.sql - ValidateEmail
// =============================================================================

func ValidateEmail(email string) (isValid bool, errorMessage string) {
	var atPos int32
	_ = atPos
	var dotPos int32
	_ = dotPos
	var emailLen int32
	_ = emailLen
	isValid = false
	errorMessage = ""
	emailLen = int32(utf8.RuneCountInString(email))
	if emailLen == 0 {
		errorMessage = "Email cannot be empty"
		return isValid, errorMessage
	}
	atPos = int32(strings.Index(email, "@") + 1)
	if atPos == 0 {
		errorMessage = "Missing @ symbol"
		return isValid, errorMessage
	}
	if atPos == 1 {
		errorMessage = "Missing local part before @"
		return isValid, errorMessage
	}
	dotPos = int32(strings.Index((email)[atPos:], ".") + 1)
	if dotPos == 0 {
		errorMessage = "Missing domain extension"
		return isValid, errorMessage
	}
	dotPos = (dotPos + atPos)
	if (dotPos - atPos) <= 1 {
		errorMessage = "Missing domain name"
		return isValid, errorMessage
	}
	if dotPos >= emailLen {
		errorMessage = "Missing extension after dot"
		return isValid, errorMessage
	}
	isValid = true
	errorMessage = "Valid email format"
	return isValid, errorMessage
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		expectValid   bool
		expectContain string
	}{
		{"valid email", "test@example.com", true, "Valid"},
		{"empty email", "", false, "empty"},
		{"no @ symbol", "testexample.com", false, "@"},
		{"no local part", "@example.com", false, "local"},
		{"no domain", "test@.com", false, "domain"},
		{"no extension", "test@example", false, "extension"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isValid, msg := ValidateEmail(tc.email)
			if isValid != tc.expectValid {
				t.Errorf("ValidateEmail(%q) isValid = %v; want %v", tc.email, isValid, tc.expectValid)
			}
			if !strings.Contains(strings.ToLower(msg), strings.ToLower(tc.expectContain)) {
				t.Errorf("ValidateEmail(%q) message = %q; want to contain %q", tc.email, msg, tc.expectContain)
			}
		})
	}
}

// =============================================================================
// 14_password_check.sql - CheckPasswordStrength
// =============================================================================

func CheckPasswordStrength(password string) (score int32, feedback string) {
	var hasUpper bool = false
	_ = hasUpper
	var hasLower bool = false
	_ = hasLower
	var hasDigit bool = false
	_ = hasDigit
	var hasSpecial bool = false
	_ = hasSpecial
	var i int32
	_ = i
	var passLen int32
	_ = passLen
	var currentChar string
	_ = currentChar
	score = 0
	feedback = ""
	passLen = int32(utf8.RuneCountInString(password))
	if passLen == 0 {
		feedback = "Password cannot be empty"
		return score, feedback
	}
	if passLen >= 8 {
		score = (score + 1)
	}
	if passLen >= 12 {
		score = (score + 1)
	}
	if passLen >= 16 {
		score = (score + 1)
	}
	i = 1
	for i <= passLen {
		currentChar = (password)[(i)-1 : (i)-1+(1)]
		if (currentChar >= "A") && (currentChar <= "Z") {
			hasUpper = true
		}
		if (currentChar >= "a") && (currentChar <= "z") {
			hasLower = true
		}
		if (currentChar >= "0") && (currentChar <= "9") {
			hasDigit = true
		}
		if (currentChar == "!") || (currentChar == "@") || (currentChar == "#") || (currentChar == "$") || (currentChar == "%") || (currentChar == "^") || (currentChar == "&") || (currentChar == "*") {
			hasSpecial = true
		}
		i = (i + 1)
	}
	if hasUpper {
		score = (score + 1)
	}
	if hasLower {
		score = (score + 1)
	}
	if hasDigit {
		score = (score + 1)
	}
	if hasSpecial {
		score = (score + 2)
	}
	if score <= 2 {
		feedback = "Weak password"
	} else {
		if score <= 4 {
			feedback = "Moderate password"
		} else {
			if score <= 6 {
				feedback = "Strong password"
			} else {
				feedback = "Very strong password"
			}
		}
	}
	return score, feedback
}

func TestCheckPasswordStrength(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		expectMinScore int32
		expectMaxScore int32
		expectContain  string
	}{
		{"empty password", "", 0, 0, "empty"},
		{"short weak", "abc", 1, 2, "Weak"},
		{"medium length", "abcdefgh", 2, 3, ""},
		{"with upper", "Abcdefgh", 3, 4, ""},
		{"with digit", "Abcdefg1", 4, 5, ""},
		{"with special", "Abcdef1!", 6, 8, "Strong"},
		{"very strong", "AbCdEfGh12!@#$%^", 8, 8, "Very strong"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, feedback := CheckPasswordStrength(tc.password)
			if score < tc.expectMinScore || score > tc.expectMaxScore {
				t.Errorf("CheckPasswordStrength(%q) score = %d; want between %d and %d",
					tc.password, score, tc.expectMinScore, tc.expectMaxScore)
			}
			if tc.expectContain != "" && !strings.Contains(feedback, tc.expectContain) {
				t.Errorf("CheckPasswordStrength(%q) feedback = %q; want to contain %q",
					tc.password, feedback, tc.expectContain)
			}
		})
	}
}

// =============================================================================
// 17_roman_numerals.sql - ToRomanNumeral
// =============================================================================

func ToRomanNumeral(number int32) (roman string) {
	roman = ""
	if (number <= 0) || (number > 3999) {
		roman = "Invalid"
		return roman
	}
	for number >= 1000 {
		roman = (roman + "M")
		number = (number - 1000)
	}
	for number >= 900 {
		roman = (roman + "CM")
		number = (number - 900)
	}
	for number >= 500 {
		roman = (roman + "D")
		number = (number - 500)
	}
	for number >= 400 {
		roman = (roman + "CD")
		number = (number - 400)
	}
	for number >= 100 {
		roman = (roman + "C")
		number = (number - 100)
	}
	for number >= 90 {
		roman = (roman + "XC")
		number = (number - 90)
	}
	for number >= 50 {
		roman = (roman + "L")
		number = (number - 50)
	}
	for number >= 40 {
		roman = (roman + "XL")
		number = (number - 40)
	}
	for number >= 10 {
		roman = (roman + "X")
		number = (number - 10)
	}
	for number >= 9 {
		roman = (roman + "IX")
		number = (number - 9)
	}
	for number >= 5 {
		roman = (roman + "V")
		number = (number - 5)
	}
	for number >= 4 {
		roman = (roman + "IV")
		number = (number - 4)
	}
	for number >= 1 {
		roman = (roman + "I")
		number = (number - 1)
	}
	return roman
}

func TestToRomanNumeral(t *testing.T) {
	tests := []struct {
		name     string
		number   int32
		expected string
	}{
		{"1", 1, "I"},
		{"4", 4, "IV"},
		{"5", 5, "V"},
		{"9", 9, "IX"},
		{"10", 10, "X"},
		{"40", 40, "XL"},
		{"50", 50, "L"},
		{"90", 90, "XC"},
		{"100", 100, "C"},
		{"400", 400, "CD"},
		{"500", 500, "D"},
		{"900", 900, "CM"},
		{"1000", 1000, "M"},
		{"1984", 1984, "MCMLXXXIV"},
		{"2024", 2024, "MMXXIV"},
		{"3999", 3999, "MMMCMXCIX"},
		{"zero invalid", 0, "Invalid"},
		{"negative invalid", -1, "Invalid"},
		{"too large invalid", 4000, "Invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ToRomanNumeral(tc.number)
			if result != tc.expected {
				t.Errorf("ToRomanNumeral(%d) = %q; want %q", tc.number, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// 18_luhn_validation.sql - ValidateCreditCard
// =============================================================================

func ValidateCreditCard(cardNumber string) (isValid bool, cardType string) {
	var cleanNumber string
	_ = cleanNumber
	var i int32
	_ = i
	var length int32
	_ = length
	var sum int32 = 0
	_ = sum
	var digit int32
	_ = digit
	var isDouble bool
	_ = isDouble
	var prefix2 string
	_ = prefix2
	var prefix4 string
	_ = prefix4
	var index int32
	_ = index
	var currentChar string
	_ = currentChar
	isValid = false
	cardType = "Unknown"
	cleanNumber = ""
	i = 1
	length = int32(utf8.RuneCountInString(cardNumber))
	for i <= length {
		currentChar = (cardNumber)[(i)-1 : (i)-1+(1)]
		if (currentChar >= "0") && (currentChar <= "9") {
			cleanNumber = (cleanNumber + currentChar)
		}
		i = (i + 1)
	}
	length = int32(utf8.RuneCountInString(cleanNumber))
	if (length < 13) || (length > 19) {
		return isValid, cardType
	}
	prefix2 = (cleanNumber)[(1)-1 : (1)-1+(2)]
	prefix4 = (cleanNumber)[(1)-1 : (1)-1+(4)]
	if (prefix2 == "34") || (prefix2 == "37") {
		cardType = "American Express"
	}
	if (prefix2 >= "51") && (prefix2 <= "55") {
		cardType = "MasterCard"
	}
	if (cleanNumber)[(1)-1:(1)-1+(1)] == "4" {
		cardType = "Visa"
	}
	if prefix4 == "6011" {
		cardType = "Discover"
	}
	index = length
	isDouble = false
	for index >= 1 {
		digit = func() int32 { v, _ := strconv.ParseInt((cleanNumber)[(index)-1:(index)-1+(1)], 10, 32); return int32(v) }()
		if isDouble {
			digit = (digit * 2)
			if digit > 9 {
				digit = (digit - 9)
			}
		}
		sum = (sum + digit)
		isDouble = !isDouble
		index = (index - 1)
	}
	if (sum % 10) == 0 {
		isValid = true
	}
	return isValid, cardType
}

func TestValidateCreditCard(t *testing.T) {
	tests := []struct {
		name        string
		cardNumber  string
		expectValid bool
		expectType  string
	}{
		// Valid test card numbers (Luhn-valid)
		{"valid Visa", "4111111111111111", true, "Visa"},
		{"valid MasterCard", "5500000000000004", true, "MasterCard"},
		{"valid Amex", "340000000000009", true, "American Express"},
		{"valid Discover", "6011000000000004", true, "Discover"},
		// Invalid numbers
		{"invalid checksum", "4111111111111112", false, "Visa"},
		{"too short", "411111111111", false, "Unknown"},
		{"too long", "41111111111111111111", false, "Unknown"},
		// With formatting
		{"formatted valid", "4111-1111-1111-1111", true, "Visa"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isValid, cardType := ValidateCreditCard(tc.cardNumber)
			if isValid != tc.expectValid {
				t.Errorf("ValidateCreditCard(%q) isValid = %v; want %v", tc.cardNumber, isValid, tc.expectValid)
			}
			if cardType != tc.expectType {
				t.Errorf("ValidateCreditCard(%q) cardType = %q; want %q", tc.cardNumber, cardType, tc.expectType)
			}
		})
	}
}

// =============================================================================
// 11_business_days.sql - BusinessDaysBetween
// =============================================================================

func BusinessDaysBetween(startDate time.Time, endDate time.Time) (days int32) {
	var currentDate time.Time
	_ = currentDate
	var dayOfWeek int32
	_ = dayOfWeek
	days = 0
	currentDate = startDate
	if startDate.After(endDate) {
		currentDate = endDate
		endDate = startDate
	}
	for !currentDate.After(endDate) {
		dayOfWeek = int32((currentDate).Weekday() + 1)
		if (dayOfWeek != 1) && (dayOfWeek != 7) {
			days = (days + 1)
		}
		currentDate = (currentDate).AddDate(0, 0, 1)
	}
	return days
}

func TestBusinessDaysBetween(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected int32
	}{
		{
			"one week Mon-Fri",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // Monday
			time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), // Friday
			5,
		},
		{
			"full week Mon-Sun",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // Monday
			time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC), // Sunday
			5,
		},
		{
			"same day weekday",
			time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), // Wednesday
			time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), // Wednesday
			1,
		},
		{
			"same day weekend",
			time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC), // Saturday
			time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC), // Saturday
			0,
		},
		{
			"two weeks",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),  // Monday
			time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), // Sunday
			10,
		},
		{
			"reversed dates",
			time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), // Friday
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // Monday
			5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BusinessDaysBetween(tc.start, tc.end)
			if result != tc.expected {
				t.Errorf("BusinessDaysBetween(%v, %v) = %d; want %d",
					tc.start.Format("2006-01-02"), tc.end.Format("2006-01-02"), result, tc.expected)
			}
		})
	}
}
