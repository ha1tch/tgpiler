-- Recursive CTE: Employee hierarchy (org chart)
CREATE PROCEDURE GetEmployeeHierarchy
    @RootEmployeeID INT
AS
BEGIN
    WITH EmployeeHierarchy AS (
        -- Anchor member: the root employee
        SELECT 
            EmployeeID, 
            ManagerID, 
            Name, 
            Title,
            1 AS Level,
            CAST(Name AS NVARCHAR(1000)) AS HierarchyPath
        FROM Employees
        WHERE EmployeeID = @RootEmployeeID
        
        UNION ALL
        
        -- Recursive member: employees who report to someone in the CTE
        SELECT 
            e.EmployeeID, 
            e.ManagerID, 
            e.Name, 
            e.Title,
            h.Level + 1,
            CAST(h.HierarchyPath + ' > ' + e.Name AS NVARCHAR(1000))
        FROM Employees e
        INNER JOIN EmployeeHierarchy h ON e.ManagerID = h.EmployeeID
    )
    SELECT 
        EmployeeID, 
        ManagerID, 
        Name, 
        Title,
        Level,
        HierarchyPath
    FROM EmployeeHierarchy
    ORDER BY Level, Name
END
