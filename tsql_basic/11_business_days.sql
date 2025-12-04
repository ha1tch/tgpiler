-- Calculate business days between two dates
-- Excludes weekends (Saturday and Sunday)
CREATE PROCEDURE dbo.BusinessDaysBetween
    @StartDate DATE,
    @EndDate DATE,
    @Days INT OUTPUT
AS
BEGIN
    DECLARE @CurrentDate DATE
    DECLARE @DayOfWeek INT
    
    SET @Days = 0
    SET @CurrentDate = @StartDate
    
    -- Swap if dates are reversed
    IF @StartDate > @EndDate
    BEGIN
        SET @CurrentDate = @EndDate
        SET @EndDate = @StartDate
    END
    
    WHILE @CurrentDate <= @EndDate
    BEGIN
        SET @DayOfWeek = DATEPART(WEEKDAY, @CurrentDate)
        
        -- Skip Saturday (7) and Sunday (1)
        IF @DayOfWeek <> 1 AND @DayOfWeek <> 7
            SET @Days = @Days + 1
        
        SET @CurrentDate = DATEADD(DAY, 1, @CurrentDate)
    END
END
