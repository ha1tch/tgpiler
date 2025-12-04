-- Calculate effective annual rate from nominal rate
CREATE PROCEDURE dbo.EffectiveAnnualRate
    @NominalRate DECIMAL(10,6),
    @CompoundingPeriodsPerYear INT,
    @EffectiveRate DECIMAL(10,6) OUTPUT
AS
BEGIN
    DECLARE @PeriodicRate DECIMAL(18,10)
    DECLARE @CompoundFactor DECIMAL(18,10)
    DECLARE @I INT
    
    IF @CompoundingPeriodsPerYear <= 0
    BEGIN
        SET @EffectiveRate = @NominalRate
        RETURN
    END
    
    SET @PeriodicRate = @NominalRate / 100.0 / @CompoundingPeriodsPerYear
    
    -- Calculate (1 + r/n)^n
    SET @CompoundFactor = 1.0
    SET @I = 0
    WHILE @I < @CompoundingPeriodsPerYear
    BEGIN
        SET @CompoundFactor = @CompoundFactor * (1.0 + @PeriodicRate)
        SET @I = @I + 1
    END
    
    -- EAR = (1 + r/n)^n - 1
    SET @EffectiveRate = (@CompoundFactor - 1.0) * 100.0
END
