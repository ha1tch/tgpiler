-- Calculate letter grade from percentage
-- Uses standard grading scale
CREATE PROCEDURE dbo.CalculateGrade
    @Percentage DECIMAL(5,2),
    @LetterGrade CHAR(2) OUTPUT,
    @GradePoints DECIMAL(3,2) OUTPUT,
    @PassFail VARCHAR(4) OUTPUT
AS
BEGIN
    -- Validate input
    IF @Percentage < 0
        SET @Percentage = 0
    IF @Percentage > 100
        SET @Percentage = 100
    
    -- Determine letter grade
    IF @Percentage >= 93
    BEGIN
        SET @LetterGrade = 'A'
        SET @GradePoints = 4.00
    END
    ELSE IF @Percentage >= 90
    BEGIN
        SET @LetterGrade = 'A-'
        SET @GradePoints = 3.70
    END
    ELSE IF @Percentage >= 87
    BEGIN
        SET @LetterGrade = 'B+'
        SET @GradePoints = 3.30
    END
    ELSE IF @Percentage >= 83
    BEGIN
        SET @LetterGrade = 'B'
        SET @GradePoints = 3.00
    END
    ELSE IF @Percentage >= 80
    BEGIN
        SET @LetterGrade = 'B-'
        SET @GradePoints = 2.70
    END
    ELSE IF @Percentage >= 77
    BEGIN
        SET @LetterGrade = 'C+'
        SET @GradePoints = 2.30
    END
    ELSE IF @Percentage >= 73
    BEGIN
        SET @LetterGrade = 'C'
        SET @GradePoints = 2.00
    END
    ELSE IF @Percentage >= 70
    BEGIN
        SET @LetterGrade = 'C-'
        SET @GradePoints = 1.70
    END
    ELSE IF @Percentage >= 67
    BEGIN
        SET @LetterGrade = 'D+'
        SET @GradePoints = 1.30
    END
    ELSE IF @Percentage >= 60
    BEGIN
        SET @LetterGrade = 'D'
        SET @GradePoints = 1.00
    END
    ELSE
    BEGIN
        SET @LetterGrade = 'F'
        SET @GradePoints = 0.00
    END
    
    -- Pass/Fail determination
    IF @Percentage >= 60
        SET @PassFail = 'PASS'
    ELSE
        SET @PassFail = 'FAIL'
END
