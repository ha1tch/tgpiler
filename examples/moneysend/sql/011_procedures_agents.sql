-- ============================================================================
-- MoneySend Stored Procedures
-- Part 11: Agent & Staff Management
-- ============================================================================

-- Create agent
CREATE PROCEDURE usp_CreateAgent
    @EmployeeId     NVARCHAR(50),
    @Email          NVARCHAR(255),
    @FirstName      NVARCHAR(100),
    @LastName       NVARCHAR(100),
    @Role           NVARCHAR(50),
    @Department     NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM Agents WHERE EmployeeId = @EmployeeId)
    BEGIN
        SELECT 0 AS Success, 'EMPLOYEE_EXISTS' AS ErrorCode;
        RETURN;
    END
    
    IF EXISTS (SELECT 1 FROM Agents WHERE Email = @Email)
    BEGIN
        SELECT 0 AS Success, 'EMAIL_EXISTS' AS ErrorCode;
        RETURN;
    END
    
    -- Set default permissions based on role
    DECLARE @CanReviewKYC BIT = 0;
    DECLARE @CanApproveTransfers BIT = 0;
    DECLARE @CanManageCustomers BIT = 0;
    DECLARE @CanFileSARs BIT = 0;
    DECLARE @CanManagePartners BIT = 0;
    DECLARE @CanViewReports BIT = 0;
    
    IF @Role = 'AGENT'
    BEGIN
        SET @CanManageCustomers = 1;
    END
    ELSE IF @Role = 'SUPERVISOR'
    BEGIN
        SET @CanReviewKYC = 1;
        SET @CanApproveTransfers = 1;
        SET @CanManageCustomers = 1;
        SET @CanViewReports = 1;
    END
    ELSE IF @Role = 'COMPLIANCE_OFFICER'
    BEGIN
        SET @CanReviewKYC = 1;
        SET @CanApproveTransfers = 1;
        SET @CanFileSARs = 1;
        SET @CanViewReports = 1;
    END
    ELSE IF @Role = 'ADMIN'
    BEGIN
        SET @CanReviewKYC = 1;
        SET @CanApproveTransfers = 1;
        SET @CanManageCustomers = 1;
        SET @CanFileSARs = 1;
        SET @CanManagePartners = 1;
        SET @CanViewReports = 1;
    END
    
    INSERT INTO Agents (
        EmployeeId, Email, FirstName, LastName, Role, Department,
        CanReviewKYC, CanApproveTransfers, CanManageCustomers,
        CanFileSARs, CanManagePartners, CanViewReports
    )
    VALUES (
        @EmployeeId, @Email, @FirstName, @LastName, @Role, @Department,
        @CanReviewKYC, @CanApproveTransfers, @CanManageCustomers,
        @CanFileSARs, @CanManagePartners, @CanViewReports
    );
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS AgentId;
END;
GO

-- Get agent by ID
CREATE PROCEDURE usp_GetAgentById
    @AgentId INT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        AgentId,
        EmployeeId,
        Email,
        FirstName,
        LastName,
        Role,
        Department,
        CanReviewKYC,
        CanApproveTransfers,
        CanManageCustomers,
        CanFileSARs,
        CanManagePartners,
        CanViewReports,
        Status,
        CreatedAt,
        LastLoginAt
    FROM Agents
    WHERE AgentId = @AgentId;
END;
GO

-- Get agent by employee ID
CREATE PROCEDURE usp_GetAgentByEmployeeId
    @EmployeeId NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        AgentId,
        EmployeeId,
        Email,
        FirstName,
        LastName,
        Role,
        Department,
        CanReviewKYC,
        CanApproveTransfers,
        CanManageCustomers,
        CanFileSARs,
        CanManagePartners,
        CanViewReports,
        Status,
        CreatedAt,
        LastLoginAt
    FROM Agents
    WHERE EmployeeId = @EmployeeId;
END;
GO

-- List agents
CREATE PROCEDURE usp_ListAgents
    @Role       NVARCHAR(50) = NULL,
    @Department NVARCHAR(50) = NULL,
    @Status     NVARCHAR(20) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        AgentId,
        EmployeeId,
        Email,
        FirstName,
        LastName,
        Role,
        Department,
        Status,
        CreatedAt,
        LastLoginAt
    FROM Agents
    WHERE (@Role IS NULL OR Role = @Role)
    AND (@Department IS NULL OR Department = @Department)
    AND (@Status IS NULL OR Status = @Status)
    ORDER BY LastName, FirstName;
END;
GO

-- Update agent
CREATE PROCEDURE usp_UpdateAgent
    @AgentId        INT,
    @FirstName      NVARCHAR(100) = NULL,
    @LastName       NVARCHAR(100) = NULL,
    @Role           NVARCHAR(50) = NULL,
    @Department     NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Agents WHERE AgentId = @AgentId)
    BEGIN
        SELECT 0 AS Success, 'AGENT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Agents
    SET FirstName = COALESCE(@FirstName, FirstName),
        LastName = COALESCE(@LastName, LastName),
        Role = COALESCE(@Role, Role),
        Department = COALESCE(@Department, Department)
    WHERE AgentId = @AgentId;
    
    SELECT 1 AS Success;
END;
GO

-- Update agent permissions
CREATE PROCEDURE usp_UpdateAgentPermissions
    @AgentId            INT,
    @CanReviewKYC       BIT = NULL,
    @CanApproveTransfers BIT = NULL,
    @CanManageCustomers BIT = NULL,
    @CanFileSARs        BIT = NULL,
    @CanManagePartners  BIT = NULL,
    @CanViewReports     BIT = NULL,
    @UpdatedBy          NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Agents WHERE AgentId = @AgentId)
    BEGIN
        SELECT 0 AS Success, 'AGENT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Agents
    SET CanReviewKYC = COALESCE(@CanReviewKYC, CanReviewKYC),
        CanApproveTransfers = COALESCE(@CanApproveTransfers, CanApproveTransfers),
        CanManageCustomers = COALESCE(@CanManageCustomers, CanManageCustomers),
        CanFileSARs = COALESCE(@CanFileSARs, CanFileSARs),
        CanManagePartners = COALESCE(@CanManagePartners, CanManagePartners),
        CanViewReports = COALESCE(@CanViewReports, CanViewReports)
    WHERE AgentId = @AgentId;
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, Description)
    SELECT AgentId, 'PERMISSIONS_UPDATED', 'Permissions updated by ' + @UpdatedBy
    FROM Agents WHERE EmployeeId = @UpdatedBy;
    
    SELECT 1 AS Success;
END;
GO

-- Suspend agent
CREATE PROCEDURE usp_SuspendAgent
    @AgentId        INT,
    @SuspendedBy    NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Agents SET Status = 'SUSPENDED' WHERE AgentId = @AgentId;
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, Description)
    VALUES (@AgentId, 'SUSPENDED', 'Suspended by ' + @SuspendedBy);
    
    SELECT 1 AS Success;
END;
GO

-- Reactivate agent
CREATE PROCEDURE usp_ReactivateAgent
    @AgentId        INT,
    @ReactivatedBy  NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Agents SET Status = 'ACTIVE' WHERE AgentId = @AgentId;
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, Description)
    VALUES (@AgentId, 'REACTIVATED', 'Reactivated by ' + @ReactivatedBy);
    
    SELECT 1 AS Success;
END;
GO

-- Record agent login
CREATE PROCEDURE usp_RecordAgentLogin
    @EmployeeId NVARCHAR(50),
    @IPAddress  NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @AgentId INT;
    SELECT @AgentId = AgentId FROM Agents WHERE EmployeeId = @EmployeeId AND Status = 'ACTIVE';
    
    IF @AgentId IS NULL
    BEGIN
        SELECT 0 AS Success, 'AGENT_NOT_ACTIVE' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Agents SET LastLoginAt = SYSUTCDATETIME() WHERE AgentId = @AgentId;
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, Description, IPAddress)
    VALUES (@AgentId, 'LOGIN', 'Agent logged in', @IPAddress);
    
    SELECT 1 AS Success, @AgentId AS AgentId;
END;
GO

-- Log agent activity
CREATE PROCEDURE usp_LogAgentActivity
    @AgentId        INT,
    @ActivityType   NVARCHAR(100),
    @EntityType     NVARCHAR(50) = NULL,
    @EntityId       BIGINT = NULL,
    @Description    NVARCHAR(500),
    @IPAddress      NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, EntityType, EntityId, Description, IPAddress)
    VALUES (@AgentId, @ActivityType, @EntityType, @EntityId, @Description, @IPAddress);
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS LogId;
END;
GO

-- Get agent activity log
CREATE PROCEDURE usp_GetAgentActivityLog
    @AgentId        INT = NULL,
    @ActivityType   NVARCHAR(100) = NULL,
    @StartDate      DATETIME2 = NULL,
    @EndDate        DATETIME2 = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    SET @StartDate = COALESCE(@StartDate, DATEADD(DAY, -30, SYSUTCDATETIME()));
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    SELECT 
        l.LogId,
        l.AgentId,
        a.EmployeeId,
        a.FirstName + ' ' + a.LastName AS AgentName,
        l.ActivityType,
        l.EntityType,
        l.EntityId,
        l.Description,
        l.IPAddress,
        l.LoggedAt
    FROM AgentActivityLog l
    JOIN Agents a ON l.AgentId = a.AgentId
    WHERE (@AgentId IS NULL OR l.AgentId = @AgentId)
    AND (@ActivityType IS NULL OR l.ActivityType = @ActivityType)
    AND l.LoggedAt >= @StartDate AND l.LoggedAt <= @EndDate
    ORDER BY l.LoggedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Get agent performance stats
CREATE PROCEDURE usp_GetAgentPerformanceStats
    @AgentId    INT,
    @StartDate  DATE = NULL,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @StartDate = COALESCE(@StartDate, DATEADD(MONTH, -1, CAST(SYSUTCDATETIME() AS DATE)));
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    DECLARE @EmployeeId NVARCHAR(50);
    SELECT @EmployeeId = EmployeeId FROM Agents WHERE AgentId = @AgentId;
    
    -- KYC reviews
    SELECT 
        COUNT(*) AS TotalDocumentsReviewed,
        SUM(CASE WHEN VerificationStatus = 'VERIFIED' THEN 1 ELSE 0 END) AS Approved,
        SUM(CASE WHEN VerificationStatus = 'REJECTED' THEN 1 ELSE 0 END) AS Rejected,
        AVG(DATEDIFF(MINUTE, SubmittedAt, VerifiedAt)) AS AvgReviewMinutes
    FROM CustomerDocuments
    WHERE VerifiedBy = @EmployeeId
    AND CAST(VerifiedAt AS DATE) BETWEEN @StartDate AND @EndDate;
    
    -- Transfer reviews
    SELECT 
        COUNT(*) AS TotalTransfersReviewed,
        SUM(CASE WHEN ComplianceStatus = 'CLEARED' THEN 1 ELSE 0 END) AS Cleared,
        SUM(CASE WHEN ComplianceStatus = 'BLOCKED' THEN 1 ELSE 0 END) AS Blocked
    FROM Transfers
    WHERE ComplianceReviewedBy = @EmployeeId
    AND CAST(ComplianceReviewedAt AS DATE) BETWEEN @StartDate AND @EndDate;
    
    -- Screening resolutions
    SELECT 
        COUNT(*) AS TotalScreeningsResolved,
        SUM(CASE WHEN ResolutionStatus = 'TRUE_POSITIVE' THEN 1 ELSE 0 END) AS TruePositives,
        SUM(CASE WHEN ResolutionStatus = 'FALSE_POSITIVE' THEN 1 ELSE 0 END) AS FalsePositives
    FROM ComplianceScreenings
    WHERE ResolvedBy = @EmployeeId
    AND CAST(ResolvedAt AS DATE) BETWEEN @StartDate AND @EndDate;
    
    -- Activity summary
    SELECT 
        ActivityType,
        COUNT(*) AS Count
    FROM AgentActivityLog
    WHERE AgentId = @AgentId
    AND CAST(LoggedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY ActivityType
    ORDER BY Count DESC;
END;
GO

-- Check agent permission
CREATE PROCEDURE usp_CheckAgentPermission
    @EmployeeId     NVARCHAR(50),
    @Permission     NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @HasPermission BIT = 0;
    
    SELECT @HasPermission = 
        CASE @Permission
            WHEN 'REVIEW_KYC' THEN CanReviewKYC
            WHEN 'APPROVE_TRANSFERS' THEN CanApproveTransfers
            WHEN 'MANAGE_CUSTOMERS' THEN CanManageCustomers
            WHEN 'FILE_SARS' THEN CanFileSARs
            WHEN 'MANAGE_PARTNERS' THEN CanManagePartners
            WHEN 'VIEW_REPORTS' THEN CanViewReports
            ELSE 0
        END
    FROM Agents
    WHERE EmployeeId = @EmployeeId AND Status = 'ACTIVE';
    
    SELECT COALESCE(@HasPermission, 0) AS HasPermission;
END;
GO

-- Get agents by permission
CREATE PROCEDURE usp_GetAgentsByPermission
    @Permission NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        AgentId,
        EmployeeId,
        Email,
        FirstName,
        LastName,
        Role,
        Department
    FROM Agents
    WHERE Status = 'ACTIVE'
    AND (
        (@Permission = 'REVIEW_KYC' AND CanReviewKYC = 1) OR
        (@Permission = 'APPROVE_TRANSFERS' AND CanApproveTransfers = 1) OR
        (@Permission = 'MANAGE_CUSTOMERS' AND CanManageCustomers = 1) OR
        (@Permission = 'FILE_SARS' AND CanFileSARs = 1) OR
        (@Permission = 'MANAGE_PARTNERS' AND CanManagePartners = 1) OR
        (@Permission = 'VIEW_REPORTS' AND CanViewReports = 1)
    )
    ORDER BY LastName, FirstName;
END;
GO

-- Get agent workload
CREATE PROCEDURE usp_GetAgentWorkload
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Current queue sizes by agent (for compliance reviews)
    SELECT 
        a.AgentId,
        a.EmployeeId,
        a.FirstName + ' ' + a.LastName AS AgentName,
        a.Role,
        (SELECT COUNT(*) FROM CustomerDocuments d 
         WHERE d.VerificationStatus = 'PENDING') AS PendingKYCDocuments,
        (SELECT COUNT(*) FROM Transfers t 
         WHERE t.ComplianceStatus = 'FLAGGED' AND t.Status = 'COMPLIANCE_REVIEW') AS FlaggedTransfers,
        (SELECT COUNT(*) FROM ComplianceScreenings s 
         WHERE s.ResolutionStatus = 'PENDING_REVIEW') AS PendingScreenings
    FROM Agents a
    WHERE a.Status = 'ACTIVE'
    AND (a.CanReviewKYC = 1 OR a.CanApproveTransfers = 1)
    ORDER BY a.Role, a.LastName;
END;
GO
