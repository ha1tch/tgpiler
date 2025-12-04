// Integration tests for tsql_financial examples
// Run with: go test -v ./tests/... -run TestFinancial
package tests

import (
	"github.com/shopspring/decimal"
	"testing"
)

// Helper to create decimal from float
func d(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

// Helper to compare decimals with tolerance
func decimalClose(a, b decimal.Decimal, tolerance float64) bool {
	diff := a.Sub(b).Abs()
	return diff.LessThan(decimal.NewFromFloat(tolerance))
}

// =============================================================================
// 01_future_value.sql - FutureValue
// =============================================================================

func FutureValue(presentValue decimal.Decimal, annualRate decimal.Decimal, compoundsPerYear int32, years int32) (futureValue decimal.Decimal, totalInterest decimal.Decimal) {
	var ratePerPeriod decimal.Decimal
	_ = ratePerPeriod
	var totalPeriods int32
	_ = totalPeriods
	var multiplier decimal.Decimal
	_ = multiplier
	var i int32
	_ = i
	if presentValue.LessThanOrEqual(decimal.NewFromInt(0)) {
		futureValue = decimal.NewFromInt(0)
		totalInterest = decimal.NewFromInt(0)
		return futureValue, totalInterest
	}
	if compoundsPerYear <= 0 {
		compoundsPerYear = 1
	}
	if years < 0 {
		years = 0
	}
	ratePerPeriod = annualRate.Div(decimal.NewFromInt(int64(compoundsPerYear)))
	totalPeriods = (compoundsPerYear * years)
	multiplier = decimal.NewFromFloat(1)
	i = 0
	for i < totalPeriods {
		multiplier = multiplier.Mul(decimal.NewFromFloat(1).Add(ratePerPeriod))
		i = (i + 1)
	}
	futureValue = presentValue.Mul(multiplier)
	totalInterest = futureValue.Sub(presentValue)
	return futureValue, totalInterest
}

func TestFutureValue(t *testing.T) {
	tests := []struct {
		name       string
		pv         float64
		rate       float64
		compounds  int32
		years      int32
		expectedFV float64
		tolerance  float64
	}{
		{"10k at 5% annual for 10 years", 10000, 0.05, 1, 10, 16288.95, 1.0},
		{"10k at 5% monthly for 10 years", 10000, 0.05, 12, 10, 16470.09, 1.0},
		{"zero principal", 0, 0.05, 12, 10, 0, 0.01},
		{"zero rate", 10000, 0, 12, 10, 10000, 0.01},
		{"zero years", 10000, 0.05, 12, 0, 10000, 0.01},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fv, _ := FutureValue(d(tc.pv), d(tc.rate), tc.compounds, tc.years)
			if !decimalClose(fv, d(tc.expectedFV), tc.tolerance) {
				t.Errorf("FutureValue(%v, %v, %d, %d) = %v; want ~%v",
					tc.pv, tc.rate, tc.compounds, tc.years, fv, tc.expectedFV)
			}
		})
	}
}

// =============================================================================
// 04_loan_payment.sql - LoanPayment
// =============================================================================

func LoanPayment(principal decimal.Decimal, annualRate decimal.Decimal, paymentsPerYear int32, loanTermYears int32) (monthlyPayment decimal.Decimal, totalPayments decimal.Decimal, totalInterest decimal.Decimal) {
	var periodicRate decimal.Decimal
	_ = periodicRate
	var numPayments int32
	_ = numPayments
	var multiplier decimal.Decimal
	_ = multiplier
	var i int32
	_ = i
	var numerator decimal.Decimal
	_ = numerator
	var denominator decimal.Decimal
	_ = denominator
	if principal.LessThanOrEqual(decimal.NewFromInt(0)) {
		monthlyPayment = decimal.NewFromInt(0)
		totalPayments = decimal.NewFromInt(0)
		totalInterest = decimal.NewFromInt(0)
		return monthlyPayment, totalPayments, totalInterest
	}
	if paymentsPerYear <= 0 {
		paymentsPerYear = 12
	}
	if loanTermYears <= 0 {
		monthlyPayment = principal
		totalPayments = principal
		totalInterest = decimal.NewFromInt(0)
		return monthlyPayment, totalPayments, totalInterest
	}
	periodicRate = annualRate.Div(decimal.NewFromInt(int64(paymentsPerYear)))
	numPayments = (paymentsPerYear * loanTermYears)
	if annualRate.Equal(decimal.NewFromInt(0)) || periodicRate.Equal(decimal.NewFromInt(0)) {
		monthlyPayment = principal.Div(decimal.NewFromInt(int64(numPayments)))
		totalPayments = principal
		totalInterest = decimal.NewFromInt(0)
		return monthlyPayment, totalPayments, totalInterest
	}
	multiplier = decimal.NewFromFloat(1)
	i = 0
	for i < numPayments {
		multiplier = multiplier.Mul(decimal.NewFromFloat(1).Add(periodicRate))
		i = (i + 1)
	}
	numerator = periodicRate.Mul(multiplier)
	denominator = multiplier.Sub(decimal.NewFromFloat(1))
	if denominator.Equal(decimal.NewFromInt(0)) {
		monthlyPayment = principal.Div(decimal.NewFromInt(int64(numPayments)))
	} else {
		monthlyPayment = principal.Mul(numerator.Div(denominator))
	}
	totalPayments = monthlyPayment.Mul(decimal.NewFromInt(int64(numPayments)))
	totalInterest = totalPayments.Sub(principal)
	return monthlyPayment, totalPayments, totalInterest
}

func TestLoanPayment(t *testing.T) {
	tests := []struct {
		name            string
		principal       float64
		rate            float64
		paymentsPerYear int32
		years           int32
		expectedPayment float64
		tolerance       float64
	}{
		{"250k mortgage at 6.5% for 30 years", 250000, 0.065, 12, 30, 1580.17, 1.0},
		{"100k at 5% for 15 years", 100000, 0.05, 12, 15, 790.79, 1.0},
		{"zero rate loan", 12000, 0, 12, 1, 1000, 0.01},
		{"zero principal", 0, 0.05, 12, 30, 0, 0.01},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payment, _, _ := LoanPayment(d(tc.principal), d(tc.rate), tc.paymentsPerYear, tc.years)
			if !decimalClose(payment, d(tc.expectedPayment), tc.tolerance) {
				t.Errorf("LoanPayment(%v, %v, %d, %d) = %v; want ~%v",
					tc.principal, tc.rate, tc.paymentsPerYear, tc.years, payment, tc.expectedPayment)
			}
		})
	}
}

// =============================================================================
// 06_progressive_tax.sql - ProgressiveTax
// =============================================================================

func ProgressiveTax(taxableIncome decimal.Decimal, bracket1 decimal.Decimal, bracket2 decimal.Decimal, bracket3 decimal.Decimal, bracket4 decimal.Decimal, bracket5 decimal.Decimal, rate1 decimal.Decimal, rate2 decimal.Decimal, rate3 decimal.Decimal, rate4 decimal.Decimal, rate5 decimal.Decimal, rate6 decimal.Decimal) (totalTax decimal.Decimal, effectiveRate decimal.Decimal, marginalRate decimal.Decimal) {
	var tax decimal.Decimal
	_ = tax
	var remaining decimal.Decimal
	_ = remaining
	var bracketTax decimal.Decimal
	_ = bracketTax
	var prevBracket decimal.Decimal
	_ = prevBracket
	var bracketWidth decimal.Decimal
	_ = bracketWidth
	tax = decimal.NewFromInt(0)
	remaining = taxableIncome
	prevBracket = decimal.NewFromInt(0)
	marginalRate = rate1
	if taxableIncome.LessThanOrEqual(decimal.NewFromInt(0)) {
		totalTax = decimal.NewFromInt(0)
		effectiveRate = decimal.NewFromInt(0)
		marginalRate = rate1
		return totalTax, effectiveRate, marginalRate
	}
	// Bracket 1
	if remaining.GreaterThan(decimal.NewFromInt(0)) && bracket1.GreaterThan(decimal.NewFromInt(0)) {
		bracketWidth = bracket1.Sub(prevBracket)
		if remaining.GreaterThanOrEqual(bracketWidth) {
			bracketTax = bracketWidth.Mul(rate1)
			remaining = remaining.Sub(bracketWidth)
		} else {
			bracketTax = remaining.Mul(rate1)
			remaining = decimal.NewFromInt(0)
			marginalRate = rate1
		}
		tax = tax.Add(bracketTax)
		prevBracket = bracket1
	}
	// Bracket 2
	if remaining.GreaterThan(decimal.NewFromInt(0)) && bracket2.GreaterThan(bracket1) {
		bracketWidth = bracket2.Sub(prevBracket)
		if remaining.GreaterThanOrEqual(bracketWidth) {
			bracketTax = bracketWidth.Mul(rate2)
			remaining = remaining.Sub(bracketWidth)
		} else {
			bracketTax = remaining.Mul(rate2)
			remaining = decimal.NewFromInt(0)
			marginalRate = rate2
		}
		tax = tax.Add(bracketTax)
		prevBracket = bracket2
	}
	// Bracket 3
	if remaining.GreaterThan(decimal.NewFromInt(0)) && bracket3.GreaterThan(bracket2) {
		bracketWidth = bracket3.Sub(prevBracket)
		if remaining.GreaterThanOrEqual(bracketWidth) {
			bracketTax = bracketWidth.Mul(rate3)
			remaining = remaining.Sub(bracketWidth)
		} else {
			bracketTax = remaining.Mul(rate3)
			remaining = decimal.NewFromInt(0)
			marginalRate = rate3
		}
		tax = tax.Add(bracketTax)
		prevBracket = bracket3
	}
	// Bracket 4
	if remaining.GreaterThan(decimal.NewFromInt(0)) && bracket4.GreaterThan(bracket3) {
		bracketWidth = bracket4.Sub(prevBracket)
		if remaining.GreaterThanOrEqual(bracketWidth) {
			bracketTax = bracketWidth.Mul(rate4)
			remaining = remaining.Sub(bracketWidth)
		} else {
			bracketTax = remaining.Mul(rate4)
			remaining = decimal.NewFromInt(0)
			marginalRate = rate4
		}
		tax = tax.Add(bracketTax)
		prevBracket = bracket4
	}
	// Bracket 5
	if remaining.GreaterThan(decimal.NewFromInt(0)) && bracket5.GreaterThan(bracket4) {
		bracketWidth = bracket5.Sub(prevBracket)
		if remaining.GreaterThanOrEqual(bracketWidth) {
			bracketTax = bracketWidth.Mul(rate5)
			remaining = remaining.Sub(bracketWidth)
		} else {
			bracketTax = remaining.Mul(rate5)
			remaining = decimal.NewFromInt(0)
			marginalRate = rate5
		}
		tax = tax.Add(bracketTax)
	}
	// Bracket 6 (above bracket5)
	if remaining.GreaterThan(decimal.NewFromInt(0)) {
		bracketTax = remaining.Mul(rate6)
		tax = tax.Add(bracketTax)
		marginalRate = rate6
	}
	totalTax = tax
	if taxableIncome.GreaterThan(decimal.NewFromInt(0)) {
		effectiveRate = totalTax.Div(taxableIncome)
	} else {
		effectiveRate = decimal.NewFromInt(0)
	}
	return totalTax, effectiveRate, marginalRate
}

func TestProgressiveTax(t *testing.T) {
	// US 2024 tax brackets (single filer, approximate)
	b1 := d(11000)
	b2 := d(44725)
	b3 := d(95375)
	b4 := d(182100)
	b5 := d(231250)
	r1 := d(0.10)
	r2 := d(0.12)
	r3 := d(0.22)
	r4 := d(0.24)
	r5 := d(0.32)
	r6 := d(0.35)

	tests := []struct {
		name           string
		income         float64
		expectedTax    float64
		expectedMarg   float64
		tolerance      float64
	}{
		{"50k income", 50000, 6307, 0.22, 50},
		{"100k income", 100000, 17400, 0.24, 50},
		{"150k income", 150000, 29400, 0.24, 50},
		{"zero income", 0, 0, 0.10, 0.01},
		{"10k income (first bracket)", 10000, 1000, 0.10, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tax, _, marginal := ProgressiveTax(d(tc.income), b1, b2, b3, b4, b5, r1, r2, r3, r4, r5, r6)
			if !decimalClose(tax, d(tc.expectedTax), tc.tolerance) {
				t.Errorf("ProgressiveTax(%v) tax = %v; want ~%v", tc.income, tax, tc.expectedTax)
			}
			if !decimalClose(marginal, d(tc.expectedMarg), 0.001) {
				t.Errorf("ProgressiveTax(%v) marginal = %v; want %v", tc.income, marginal, tc.expectedMarg)
			}
		})
	}
}

// =============================================================================
// 10_break_even.sql - BreakEvenAnalysis
// =============================================================================

func BreakEvenAnalysis(fixedCosts decimal.Decimal, variableCostPerUnit decimal.Decimal, sellingPricePerUnit decimal.Decimal, actualSalesUnits int32) (breakEvenUnits int32, breakEvenRevenue decimal.Decimal, contributionMarginPerUnit decimal.Decimal, contributionMarginRatio decimal.Decimal, marginOfSafetyUnits int32, marginOfSafetyPercent decimal.Decimal, profitAtActualSales decimal.Decimal) {
	var contributionMargin decimal.Decimal
	_ = contributionMargin
	var breakEvenExact decimal.Decimal
	_ = breakEvenExact
	if fixedCosts.LessThan(decimal.NewFromInt(0)) {
		fixedCosts = decimal.NewFromInt(0)
	}
	if variableCostPerUnit.LessThan(decimal.NewFromInt(0)) {
		variableCostPerUnit = decimal.NewFromInt(0)
	}
	if sellingPricePerUnit.LessThanOrEqual(decimal.NewFromInt(0)) {
		breakEvenUnits = 0
		breakEvenRevenue = decimal.NewFromInt(0)
		contributionMarginPerUnit = decimal.NewFromInt(0)
		contributionMarginRatio = decimal.NewFromInt(0)
		marginOfSafetyUnits = 0
		marginOfSafetyPercent = decimal.NewFromInt(0)
		profitAtActualSales = fixedCosts.Neg()
		return
	}
	if actualSalesUnits < 0 {
		actualSalesUnits = 0
	}
	contributionMargin = sellingPricePerUnit.Sub(variableCostPerUnit)
	contributionMarginPerUnit = contributionMargin
	contributionMarginRatio = contributionMargin.Div(sellingPricePerUnit)
	if contributionMargin.LessThanOrEqual(decimal.NewFromInt(0)) {
		breakEvenUnits = 0
		breakEvenRevenue = decimal.NewFromInt(0)
		marginOfSafetyUnits = 0
		marginOfSafetyPercent = decimal.NewFromInt(0)
		profitAtActualSales = contributionMargin.Mul(decimal.NewFromInt(int64(actualSalesUnits))).Sub(fixedCosts)
		return
	}
	breakEvenExact = fixedCosts.Div(contributionMargin)
	breakEvenUnits = int32(breakEvenExact.IntPart())
	if breakEvenExact.GreaterThan(decimal.NewFromInt(int64(breakEvenUnits))) {
		breakEvenUnits = breakEvenUnits + 1
	}
	breakEvenRevenue = decimal.NewFromInt(int64(breakEvenUnits)).Mul(sellingPricePerUnit)
	marginOfSafetyUnits = actualSalesUnits - breakEvenUnits
	if actualSalesUnits > 0 {
		marginOfSafetyPercent = decimal.NewFromInt(int64(marginOfSafetyUnits)).Div(decimal.NewFromInt(int64(actualSalesUnits))).Mul(decimal.NewFromFloat(100))
	} else {
		marginOfSafetyPercent = decimal.NewFromInt(0)
	}
	profitAtActualSales = contributionMargin.Mul(decimal.NewFromInt(int64(actualSalesUnits))).Sub(fixedCosts)
	return
}

func TestBreakEvenAnalysis(t *testing.T) {
	tests := []struct {
		name           string
		fixed          float64
		variable       float64
		price          float64
		sales          int32
		expectedBE     int32
		expectedProfit float64
	}{
		{"standard case", 50000, 25, 75, 1500, 1000, 25000},
		{"at break-even", 50000, 25, 75, 1000, 1000, 0},
		{"below break-even", 50000, 25, 75, 500, 1000, -25000},
		{"zero fixed costs", 0, 25, 75, 100, 0, 5000},
		{"high margin", 10000, 10, 100, 200, 112, 8000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			beUnits, _, _, _, _, _, profit := BreakEvenAnalysis(
				d(tc.fixed), d(tc.variable), d(tc.price), tc.sales)
			if beUnits != tc.expectedBE {
				t.Errorf("BreakEvenAnalysis break-even units = %d; want %d", beUnits, tc.expectedBE)
			}
			if !decimalClose(profit, d(tc.expectedProfit), 1.0) {
				t.Errorf("BreakEvenAnalysis profit = %v; want %v", profit, tc.expectedProfit)
			}
		})
	}
}

// =============================================================================
// 14_bond_price.sql - BondPrice
// =============================================================================

func BondPrice(faceValue decimal.Decimal, couponRate decimal.Decimal, marketYield decimal.Decimal, yearsToMaturity int32, paymentsPerYear int32) (bondPrice decimal.Decimal, currentYield decimal.Decimal, annualCoupon decimal.Decimal, totalCouponPayments decimal.Decimal, priceType string) {
	var couponPayment decimal.Decimal
	_ = couponPayment
	var periodicYield decimal.Decimal
	_ = periodicYield
	var totalPeriods int32
	_ = totalPeriods
	var pvCoupons decimal.Decimal
	_ = pvCoupons
	var pvFaceValue decimal.Decimal
	_ = pvFaceValue
	var discountFactor decimal.Decimal
	_ = discountFactor
	var i int32
	_ = i
	if faceValue.LessThanOrEqual(decimal.NewFromInt(0)) {
		bondPrice = decimal.NewFromInt(0)
		currentYield = decimal.NewFromInt(0)
		annualCoupon = decimal.NewFromInt(0)
		totalCouponPayments = decimal.NewFromInt(0)
		priceType = "Invalid"
		return
	}
	if paymentsPerYear <= 0 {
		paymentsPerYear = 2
	}
	if yearsToMaturity <= 0 {
		bondPrice = faceValue
		annualCoupon = faceValue.Mul(couponRate)
		currentYield = couponRate
		totalCouponPayments = decimal.NewFromInt(0)
		priceType = "Par"
		return
	}
	couponPayment = faceValue.Mul(couponRate).Div(decimal.NewFromInt(int64(paymentsPerYear)))
	periodicYield = marketYield.Div(decimal.NewFromInt(int64(paymentsPerYear)))
	totalPeriods = yearsToMaturity * paymentsPerYear
	annualCoupon = faceValue.Mul(couponRate)
	pvCoupons = decimal.NewFromInt(0)
	i = 1
	for i <= totalPeriods {
		discountFactor = decimal.NewFromFloat(1)
		var j int32
		j = 0
		for j < i {
			discountFactor = discountFactor.Div(decimal.NewFromFloat(1).Add(periodicYield))
			j = j + 1
		}
		pvCoupons = pvCoupons.Add(couponPayment.Mul(discountFactor))
		i = i + 1
	}
	discountFactor = decimal.NewFromFloat(1)
	i = 0
	for i < totalPeriods {
		discountFactor = discountFactor.Div(decimal.NewFromFloat(1).Add(periodicYield))
		i = i + 1
	}
	pvFaceValue = faceValue.Mul(discountFactor)
	bondPrice = pvCoupons.Add(pvFaceValue)
	if bondPrice.GreaterThan(decimal.NewFromInt(0)) {
		currentYield = annualCoupon.Div(bondPrice)
	} else {
		currentYield = decimal.NewFromInt(0)
	}
	totalCouponPayments = couponPayment.Mul(decimal.NewFromInt(int64(totalPeriods)))
	if bondPrice.GreaterThan(faceValue.Mul(decimal.NewFromFloat(1.001))) {
		priceType = "Premium"
	} else if bondPrice.LessThan(faceValue.Mul(decimal.NewFromFloat(0.999))) {
		priceType = "Discount"
	} else {
		priceType = "Par"
	}
	return
}

func TestBondPrice(t *testing.T) {
	tests := []struct {
		name          string
		face          float64
		coupon        float64
		yield         float64
		years         int32
		payments      int32
		expectedPrice float64
		expectedType  string
		tolerance     float64
	}{
		{"premium bond", 1000, 0.05, 0.04, 10, 2, 1081.76, "Premium", 1.0},
		{"discount bond", 1000, 0.04, 0.05, 10, 2, 922.78, "Discount", 1.0},
		{"par bond", 1000, 0.05, 0.05, 10, 2, 1000.00, "Par", 1.0},
		{"zero coupon", 1000, 0, 0.05, 10, 2, 610.27, "Discount", 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			price, _, _, _, priceType := BondPrice(d(tc.face), d(tc.coupon), d(tc.yield), tc.years, tc.payments)
			if !decimalClose(price, d(tc.expectedPrice), tc.tolerance) {
				t.Errorf("BondPrice = %v; want ~%v", price, tc.expectedPrice)
			}
			if priceType != tc.expectedType {
				t.Errorf("BondPrice type = %s; want %s", priceType, tc.expectedType)
			}
		})
	}
}

// =============================================================================
// 16_cagr.sql - CompoundAnnualGrowthRate
// =============================================================================

func CompoundAnnualGrowthRate(beginningValue decimal.Decimal, endingValue decimal.Decimal, numYears int32) (cagr decimal.Decimal, totalGrowth decimal.Decimal, totalGrowthPercent decimal.Decimal, futureValue5Years decimal.Decimal, futureValue10Years decimal.Decimal) {
	var ratio decimal.Decimal
	_ = ratio
	var guess decimal.Decimal
	_ = guess
	var newGuess decimal.Decimal
	_ = newGuess
	var power decimal.Decimal
	_ = power
	var i int32
	_ = i
	var j int32
	_ = j
	var multiplier decimal.Decimal
	_ = multiplier
	if beginningValue.LessThanOrEqual(decimal.NewFromInt(0)) || endingValue.LessThanOrEqual(decimal.NewFromInt(0)) {
		cagr = decimal.NewFromInt(0)
		totalGrowth = decimal.NewFromInt(0)
		totalGrowthPercent = decimal.NewFromInt(0)
		futureValue5Years = endingValue
		futureValue10Years = endingValue
		return
	}
	if numYears <= 0 {
		cagr = decimal.NewFromInt(0)
		totalGrowth = endingValue.Sub(beginningValue)
		totalGrowthPercent = endingValue.Sub(beginningValue).Div(beginningValue).Mul(decimal.NewFromFloat(100))
		futureValue5Years = endingValue
		futureValue10Years = endingValue
		return
	}
	ratio = endingValue.Div(beginningValue)
	totalGrowth = endingValue.Sub(beginningValue)
	totalGrowthPercent = totalGrowth.Div(beginningValue).Mul(decimal.NewFromFloat(100))
	if numYears == 1 {
		cagr = ratio.Sub(decimal.NewFromFloat(1))
		multiplier = decimal.NewFromFloat(1).Add(cagr)
		futureValue5Years = endingValue
		i = 1
		for i < 5 {
			futureValue5Years = futureValue5Years.Mul(multiplier)
			i = i + 1
		}
		futureValue10Years = futureValue5Years
		i = 5
		for i < 10 {
			futureValue10Years = futureValue10Years.Mul(multiplier)
			i = i + 1
		}
		return
	}
	// Newton-Raphson to find nth root
	guess = decimal.NewFromFloat(1).Add(totalGrowthPercent.Div(decimal.NewFromFloat(100)).Div(decimal.NewFromInt(int64(numYears))))
	for iteration := 0; iteration < 50; iteration++ {
		power = decimal.NewFromFloat(1)
		j = 0
		for j < (numYears - 1) {
			power = power.Mul(guess)
			j = j + 1
		}
		guessToN := power.Mul(guess)
		fOfX := guessToN.Sub(ratio)
		derivative := decimal.NewFromInt(int64(numYears)).Mul(power)
		if derivative.Abs().LessThan(decimal.NewFromFloat(0.0000001)) {
			derivative = decimal.NewFromFloat(0.0000001)
		}
		newGuess = guess.Sub(fOfX.Div(derivative))
		if newGuess.LessThanOrEqual(decimal.NewFromInt(0)) {
			newGuess = guess.Mul(decimal.NewFromFloat(0.5))
		}
		if newGuess.Sub(guess).Abs().LessThan(decimal.NewFromFloat(0.0000001)) {
			break
		}
		guess = newGuess
	}
	cagr = guess.Sub(decimal.NewFromFloat(1))
	multiplier = decimal.NewFromFloat(1).Add(cagr)
	futureValue5Years = endingValue
	i = 0
	for i < 5 {
		futureValue5Years = futureValue5Years.Mul(multiplier)
		i = i + 1
	}
	futureValue10Years = endingValue
	i = 0
	for i < 10 {
		futureValue10Years = futureValue10Years.Mul(multiplier)
		i = i + 1
	}
	return
}

func TestCompoundAnnualGrowthRate(t *testing.T) {
	tests := []struct {
		name         string
		begin        float64
		end          float64
		years        int32
		expectedCAGR float64
		tolerance    float64
	}{
		{"double in 5 years", 10000, 20000, 5, 0.1487, 0.01},
		{"2.5x in 5 years", 10000, 25000, 5, 0.2011, 0.01},
		{"no growth", 10000, 10000, 5, 0, 0.001},
		{"1 year", 10000, 12000, 1, 0.20, 0.001},
		{"decline", 10000, 8000, 2, -0.1056, 0.01},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cagr, _, _, _, _ := CompoundAnnualGrowthRate(d(tc.begin), d(tc.end), tc.years)
			if !decimalClose(cagr, d(tc.expectedCAGR), tc.tolerance) {
				t.Errorf("CAGR(%v->%v in %d years) = %v; want ~%v",
					tc.begin, tc.end, tc.years, cagr, tc.expectedCAGR)
			}
		})
	}
}

// =============================================================================
// 07_straight_line_depreciation.sql - StraightLineDepreciation
// =============================================================================

func StraightLineDepreciation(assetCost decimal.Decimal, salvageValue decimal.Decimal, usefulLifeYears int32, currentYear int32) (annualDepreciation decimal.Decimal, accumulatedDepreciation decimal.Decimal, bookValue decimal.Decimal, depreciationRate decimal.Decimal) {
	var depreciableBase decimal.Decimal
	_ = depreciableBase
	if assetCost.LessThanOrEqual(decimal.NewFromInt(0)) {
		annualDepreciation = decimal.NewFromInt(0)
		accumulatedDepreciation = decimal.NewFromInt(0)
		bookValue = decimal.NewFromInt(0)
		depreciationRate = decimal.NewFromInt(0)
		return
	}
	if usefulLifeYears <= 0 {
		usefulLifeYears = 1
	}
	if salvageValue.LessThan(decimal.NewFromInt(0)) {
		salvageValue = decimal.NewFromInt(0)
	}
	if salvageValue.GreaterThan(assetCost) {
		salvageValue = assetCost
	}
	if currentYear < 0 {
		currentYear = 0
	}
	depreciableBase = assetCost.Sub(salvageValue)
	annualDepreciation = depreciableBase.Div(decimal.NewFromInt(int64(usefulLifeYears)))
	if assetCost.GreaterThan(decimal.NewFromInt(0)) {
		depreciationRate = annualDepreciation.Div(assetCost)
	} else {
		depreciationRate = decimal.NewFromInt(0)
	}
	if currentYear >= usefulLifeYears {
		accumulatedDepreciation = depreciableBase
		bookValue = salvageValue
		annualDepreciation = decimal.NewFromInt(0)
	} else if currentYear > 0 {
		accumulatedDepreciation = annualDepreciation.Mul(decimal.NewFromInt(int64(currentYear)))
		bookValue = assetCost.Sub(accumulatedDepreciation)
	} else {
		accumulatedDepreciation = decimal.NewFromInt(0)
		bookValue = assetCost
	}
	return
}

func TestStraightLineDepreciation(t *testing.T) {
	tests := []struct {
		name             string
		cost             float64
		salvage          float64
		life             int32
		year             int32
		expectedAnnual   float64
		expectedBook     float64
	}{
		{"year 1 of 5", 10000, 2000, 5, 1, 1600, 8400},
		{"year 3 of 5", 10000, 2000, 5, 3, 1600, 5200},
		{"year 5 of 5", 10000, 2000, 5, 5, 0, 2000},
		{"year 0", 10000, 2000, 5, 0, 1600, 10000},
		{"fully depreciated", 10000, 2000, 5, 10, 0, 2000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			annual, _, bookValue, _ := StraightLineDepreciation(d(tc.cost), d(tc.salvage), tc.life, tc.year)
			if !decimalClose(annual, d(tc.expectedAnnual), 0.01) {
				t.Errorf("Annual depreciation = %v; want %v", annual, tc.expectedAnnual)
			}
			if !decimalClose(bookValue, d(tc.expectedBook), 0.01) {
				t.Errorf("Book value = %v; want %v", bookValue, tc.expectedBook)
			}
		})
	}
}

// =============================================================================
// 09_markup_margin.sql - MarkupMargin (mode 1: calculate from cost/price)
// =============================================================================

func MarkupMargin(cost decimal.Decimal, sellingPrice decimal.Decimal, desiredMarkupPercent decimal.Decimal, desiredMarginPercent decimal.Decimal, calculationMode int32) (actualMarkupPercent decimal.Decimal, actualMarginPercent decimal.Decimal, calculatedSellingPrice decimal.Decimal, grossProfit decimal.Decimal) {
	var profit decimal.Decimal
	_ = profit
	if cost.LessThan(decimal.NewFromInt(0)) {
		cost = decimal.NewFromInt(0)
	}
	if sellingPrice.LessThan(decimal.NewFromInt(0)) {
		sellingPrice = decimal.NewFromInt(0)
	}
	// Mode 1: Calculate from cost and price
	if calculationMode == 1 {
		if cost.LessThanOrEqual(decimal.NewFromInt(0)) {
			actualMarkupPercent = decimal.NewFromInt(0)
			actualMarginPercent = decimal.NewFromInt(0)
			calculatedSellingPrice = sellingPrice
			grossProfit = decimal.NewFromInt(0)
			return
		}
		profit = sellingPrice.Sub(cost)
		grossProfit = profit
		actualMarkupPercent = profit.Div(cost).Mul(decimal.NewFromFloat(100))
		if sellingPrice.GreaterThan(decimal.NewFromInt(0)) {
			actualMarginPercent = profit.Div(sellingPrice).Mul(decimal.NewFromFloat(100))
		} else {
			actualMarginPercent = decimal.NewFromInt(0)
		}
		calculatedSellingPrice = sellingPrice
	} else if calculationMode == 2 {
		// Mode 2: Calculate from markup
		if cost.LessThanOrEqual(decimal.NewFromInt(0)) {
			return
		}
		calculatedSellingPrice = cost.Mul(decimal.NewFromFloat(1).Add(desiredMarkupPercent.Div(decimal.NewFromFloat(100))))
		profit = calculatedSellingPrice.Sub(cost)
		grossProfit = profit
		actualMarkupPercent = desiredMarkupPercent
		if calculatedSellingPrice.GreaterThan(decimal.NewFromInt(0)) {
			actualMarginPercent = profit.Div(calculatedSellingPrice).Mul(decimal.NewFromFloat(100))
		}
	} else if calculationMode == 3 {
		// Mode 3: Calculate from margin
		if cost.LessThanOrEqual(decimal.NewFromInt(0)) {
			return
		}
		if desiredMarginPercent.GreaterThanOrEqual(decimal.NewFromFloat(100)) {
			desiredMarginPercent = decimal.NewFromFloat(99.99)
		}
		calculatedSellingPrice = cost.Div(decimal.NewFromFloat(1).Sub(desiredMarginPercent.Div(decimal.NewFromFloat(100))))
		profit = calculatedSellingPrice.Sub(cost)
		grossProfit = profit
		actualMarginPercent = desiredMarginPercent
		if cost.GreaterThan(decimal.NewFromInt(0)) {
			actualMarkupPercent = profit.Div(cost).Mul(decimal.NewFromFloat(100))
		}
	}
	return
}

func TestMarkupMargin(t *testing.T) {
	// Mode 1: Calculate from cost and price
	markup, margin, _, profit := MarkupMargin(d(100), d(150), d(0), d(0), 1)
	if !decimalClose(markup, d(50), 0.1) {
		t.Errorf("Markup = %v; want 50", markup)
	}
	if !decimalClose(margin, d(33.33), 0.1) {
		t.Errorf("Margin = %v; want ~33.33", margin)
	}
	if !decimalClose(profit, d(50), 0.01) {
		t.Errorf("Profit = %v; want 50", profit)
	}

	// Mode 2: Calculate from markup
	_, _, price, _ := MarkupMargin(d(100), d(0), d(50), d(0), 2)
	if !decimalClose(price, d(150), 0.01) {
		t.Errorf("Price from 50%% markup = %v; want 150", price)
	}

	// Mode 3: Calculate from margin
	_, _, price, _ = MarkupMargin(d(100), d(0), d(0), d(25), 3)
	if !decimalClose(price, d(133.33), 0.1) {
		t.Errorf("Price from 25%% margin = %v; want ~133.33", price)
	}
}
