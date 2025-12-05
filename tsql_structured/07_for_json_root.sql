-- Build JSON with ROOT wrapper and options
CREATE PROCEDURE dbo.BuildEmployeeDirectory
    @DepartmentId INT
AS
BEGIN
    CREATE TABLE #Employees (
        EmployeeId INT,
        FirstName NVARCHAR(50),
        LastName NVARCHAR(50),
        Email NVARCHAR(100),
        DeptId INT,
        Salary DECIMAL(10,2)
    )
    
    INSERT INTO #Employees VALUES (1, 'John', 'Doe', 'john.doe@company.com', 1, 75000)
    INSERT INTO #Employees VALUES (2, 'Jane', 'Smith', 'jane.smith@company.com', 1, 82000)
    INSERT INTO #Employees VALUES (3, 'Bob', 'Wilson', 'bob.wilson@company.com', 2, 68000)
    INSERT INTO #Employees VALUES (4, 'Alice', 'Brown', 'alice.brown@company.com', 1, 91000)
    
    -- Build JSON with nested path and root element
    SELECT 
        EmployeeId AS 'id',
        FirstName AS 'name.first',
        LastName AS 'name.last',
        Email AS 'contact.email',
        Salary AS 'compensation.salary'
    FROM #Employees
    WHERE DeptId = @DepartmentId
    ORDER BY LastName, FirstName
    FOR JSON PATH, ROOT('employees')
    
    DROP TABLE #Employees
END
