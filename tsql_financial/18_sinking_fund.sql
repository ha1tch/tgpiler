-- Sinking Fund Calculator
-- Calculates periodic deposits needed to reach a future goal
-- Also calculates time to reach goal with fixed deposits
-- Payment = FV * r / [(1+r)^n - 1]

CREATE PROCEDURE dbo.SinkingFund
    @FutureValueGoal DECIMAL(18,4),
    @AnnualInterestRate DECIMAL(10,6),
    @PeriodsPerYear INT,
    @TotalYears INT,
    @FixedDeposit DECIMAL(18,4),  -- If provided, calculates time to goal
    @CalculationMode INT,  -- 1=Calculate payment, 2=Calculate time with fixed deposit
    @RequiredPayment DECIMAL(18,4) OUTPUT,
    @TotalDeposits DECIMAL(18,4) OUTPUT,
    @TotalInterestEarned DECIMAL(18,4) OUTPUT,
    @PeriodsToGoal INT OUTPUT,
    @YearsToGoal DECIMAL(10,2) OUTPUT,
    @FinalValue DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @PeriodicRate DECIMAL(18,10)
    DECLARE @TotalPeriods INT
    DECLARE @Multiplier DECIMAL(18,10)
    DECLARE @i INT
    DECLARE @Balance DECIMAL(18,4)
    DECLARE @Period INT
    
    -- Validate inputs
    IF @FutureValueGoal <= 0
    BEGIN
        SET @RequiredPayment = 0
        SET @TotalDeposits = 0
        SET @TotalInterestEarned = 0
        SET @PeriodsToGoal = 0
        SET @YearsToGoal = 0
        SET @FinalValue = 0
        RETURN
    END
    
    IF @PeriodsPerYear <= 0
        SET @PeriodsPerYear = 12  -- Default to monthly
    
    SET @PeriodicRate = @AnnualInterestRate / @PeriodsPerYear
    SET @TotalPeriods = @PeriodsPerYear * @TotalYears
    
    -- Mode 1: Calculate required payment to reach goal
    IF @CalculationMode = 1
    BEGIN
        IF @TotalYears <= 0
        BEGIN
            SET @RequiredPayment = @FutureValueGoal
            SET @TotalDeposits = @FutureValueGoal
            SET @TotalInterestEarned = 0
            SET @PeriodsToGoal = 1
            SET @YearsToGoal = 0
            SET @FinalValue = @FutureValueGoal
            RETURN
        END
        
        -- Calculate (1+r)^n
        SET @Multiplier = 1.0
        SET @i = 0
        WHILE @i < @TotalPeriods
        BEGIN
            SET @Multiplier = @Multiplier * (1.0 + @PeriodicRate)
            SET @i = @i + 1
        END
        
        -- Payment = FV * r / [(1+r)^n - 1]
        IF @Multiplier > 1.0 AND @PeriodicRate > 0
        BEGIN
            SET @RequiredPayment = @FutureValueGoal * @PeriodicRate / (@Multiplier - 1.0)
        END
        ELSE
        BEGIN
            -- Zero interest: simply divide
            SET @RequiredPayment = @FutureValueGoal / @TotalPeriods
        END
        
        SET @TotalDeposits = @RequiredPayment * @TotalPeriods
        SET @TotalInterestEarned = @FutureValueGoal - @TotalDeposits
        SET @PeriodsToGoal = @TotalPeriods
        SET @YearsToGoal = CAST(@TotalYears AS DECIMAL(10,2))
        SET @FinalValue = @FutureValueGoal
    END
    
    -- Mode 2: Calculate time to reach goal with fixed deposit
    ELSE IF @CalculationMode = 2
    BEGIN
        IF @FixedDeposit <= 0
        BEGIN
            SET @RequiredPayment = 0
            SET @TotalDeposits = 0
            SET @TotalInterestEarned = 0
            SET @PeriodsToGoal = 0
            SET @YearsToGoal = 0
            SET @FinalValue = 0
            RETURN
        END
        
        SET @RequiredPayment = @FixedDeposit
        SET @Balance = 0
        SET @Period = 0
        
        -- Simulate deposits until goal is reached (max 1200 periods = 100 years monthly)
        WHILE @Balance < @FutureValueGoal AND @Period < 1200
        BEGIN
            -- Add interest on existing balance
            SET @Balance = @Balance * (1.0 + @PeriodicRate)
            
            -- Add deposit
            SET @Balance = @Balance + @FixedDeposit
            
            SET @Period = @Period + 1
        END
        
        SET @PeriodsToGoal = @Period
        SET @YearsToGoal = CAST(@Period AS DECIMAL(10,2)) / @PeriodsPerYear
        SET @TotalDeposits = @FixedDeposit * @Period
        SET @FinalValue = @Balance
        SET @TotalInterestEarned = @Balance - @TotalDeposits
    END
    ELSE
    BEGIN
        -- Invalid mode
        SET @RequiredPayment = 0
        SET @TotalDeposits = 0
        SET @TotalInterestEarned = 0
        SET @PeriodsToGoal = 0
        SET @YearsToGoal = 0
        SET @FinalValue = 0
    END
    
    RETURN
END
