-- ============================================================================
-- MoneySend Stored Procedures
-- Part 2: KYC Documents & Beneficiary Management
-- ============================================================================

-- ============================================================================
-- KYC DOCUMENT MANAGEMENT
-- ============================================================================

-- Submit a new document for KYC
CREATE PROCEDURE usp_SubmitDocument
    @CustomerId         BIGINT,
    @DocumentType       NVARCHAR(50),
    @DocumentNumber     NVARCHAR(100) = NULL,
    @IssuingCountry     CHAR(2) = NULL,
    @IssueDate          DATE = NULL,
    @ExpiryDate         DATE = NULL,
    @FrontImagePath     NVARCHAR(500),
    @BackImagePath      NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Validate customer exists
    IF NOT EXISTS (SELECT 1 FROM Customers WHERE CustomerId = @CustomerId)
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for existing pending document of same type
    IF EXISTS (SELECT 1 FROM CustomerDocuments 
               WHERE CustomerId = @CustomerId 
               AND DocumentType = @DocumentType 
               AND VerificationStatus = 'PENDING')
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_PENDING' AS ErrorCode, 
               'A document of this type is already pending review' AS ErrorMessage;
        RETURN;
    END
    
    -- Check document expiry
    IF @ExpiryDate IS NOT NULL AND @ExpiryDate < DATEADD(MONTH, 1, GETDATE())
    BEGIN
        SELECT 0 AS Success, 'DOCUMENT_EXPIRING' AS ErrorCode, 
               'Document must be valid for at least 1 month' AS ErrorMessage;
        RETURN;
    END
    
    INSERT INTO CustomerDocuments (
        CustomerId, DocumentType, DocumentNumber, IssuingCountry,
        IssueDate, ExpiryDate, FrontImagePath, BackImagePath,
        VerificationStatus, SubmittedAt
    )
    VALUES (
        @CustomerId, @DocumentType, @DocumentNumber, @IssuingCountry,
        @IssueDate, @ExpiryDate, @FrontImagePath, @BackImagePath,
        'PENDING', SYSUTCDATETIME()
    );
    
    DECLARE @DocumentId BIGINT = SCOPE_IDENTITY();
    
    -- Update customer KYC status if needed
    UPDATE Customers
    SET KYCStatus = 'PENDING',
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId
    AND KYCStatus IN ('PENDING', 'REJECTED');
    
    SELECT 1 AS Success, @DocumentId AS DocumentId;
END;
GO

-- Get documents by customer
CREATE PROCEDURE usp_GetDocumentsByCustomer
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        DocumentId,
        DocumentType,
        DocumentNumber,
        IssuingCountry,
        IssueDate,
        ExpiryDate,
        FrontImagePath,
        BackImagePath,
        VerificationStatus,
        VerifiedAt,
        VerifiedBy,
        RejectionReason,
        SubmittedAt,
        UpdatedAt
    FROM CustomerDocuments
    WHERE CustomerId = @CustomerId
    ORDER BY SubmittedAt DESC;
END;
GO

-- Get document by ID
CREATE PROCEDURE usp_GetDocumentById
    @DocumentId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        d.DocumentId,
        d.CustomerId,
        d.DocumentType,
        d.DocumentNumber,
        d.IssuingCountry,
        d.IssueDate,
        d.ExpiryDate,
        d.FrontImagePath,
        d.BackImagePath,
        d.VerificationStatus,
        d.VerifiedAt,
        d.VerifiedBy,
        d.RejectionReason,
        d.SubmittedAt,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName,
        c.Email AS CustomerEmail
    FROM CustomerDocuments d
    JOIN Customers c ON d.CustomerId = c.CustomerId
    WHERE d.DocumentId = @DocumentId;
END;
GO

-- Approve document
CREATE PROCEDURE usp_ApproveDocument
    @DocumentId     BIGINT,
    @ApprovedBy     NVARCHAR(100),
    @Notes          NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @CurrentStatus NVARCHAR(20);
    
    SELECT @CustomerId = CustomerId, @CurrentStatus = VerificationStatus
    FROM CustomerDocuments WHERE DocumentId = @DocumentId;
    
    IF @CustomerId IS NULL
    BEGIN
        SELECT 0 AS Success, 'DOCUMENT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @CurrentStatus != 'PENDING'
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode, 
               'Document is not pending review' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE CustomerDocuments
    SET VerificationStatus = 'VERIFIED',
        VerifiedAt = SYSUTCDATETIME(),
        VerifiedBy = @ApprovedBy,
        UpdatedAt = SYSUTCDATETIME()
    WHERE DocumentId = @DocumentId;
    
    -- Check if customer has required documents verified for KYC
    DECLARE @VerifiedDocCount INT;
    SELECT @VerifiedDocCount = COUNT(*) FROM CustomerDocuments 
    WHERE CustomerId = @CustomerId AND VerificationStatus = 'VERIFIED';
    
    IF @VerifiedDocCount >= 1
    BEGIN
        UPDATE Customers
        SET KYCStatus = 'VERIFIED',
            KYCVerifiedAt = SYSUTCDATETIME(),
            KYCExpiresAt = DATEADD(YEAR, 1, SYSUTCDATETIME()),
            UpdatedAt = SYSUTCDATETIME()
        WHERE CustomerId = @CustomerId;
        
        INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
        VALUES (@CustomerId, 'KYC_APPROVED', 'VERIFIED', @Notes, @ApprovedBy);
    END
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, EntityType, EntityId, Description)
    SELECT AgentId, 'APPROVE_DOCUMENT', 'DOCUMENT', @DocumentId, 'Approved document for customer ' + CAST(@CustomerId AS NVARCHAR)
    FROM Agents WHERE EmployeeId = @ApprovedBy;
    
    SELECT 1 AS Success;
END;
GO

-- Reject document
CREATE PROCEDURE usp_RejectDocument
    @DocumentId     BIGINT,
    @RejectedBy     NVARCHAR(100),
    @RejectionReason NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @CurrentStatus NVARCHAR(20);
    
    SELECT @CustomerId = CustomerId, @CurrentStatus = VerificationStatus
    FROM CustomerDocuments WHERE DocumentId = @DocumentId;
    
    IF @CustomerId IS NULL
    BEGIN
        SELECT 0 AS Success, 'DOCUMENT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @CurrentStatus != 'PENDING'
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE CustomerDocuments
    SET VerificationStatus = 'REJECTED',
        VerifiedBy = @RejectedBy,
        RejectionReason = @RejectionReason,
        UpdatedAt = SYSUTCDATETIME()
    WHERE DocumentId = @DocumentId;
    
    -- Update customer KYC status if no other verified documents
    IF NOT EXISTS (SELECT 1 FROM CustomerDocuments 
                   WHERE CustomerId = @CustomerId AND VerificationStatus = 'VERIFIED')
    BEGIN
        UPDATE Customers SET KYCStatus = 'REJECTED', UpdatedAt = SYSUTCDATETIME()
        WHERE CustomerId = @CustomerId;
        
        INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
        VALUES (@CustomerId, 'KYC_REJECTED', 'REJECTED', @RejectionReason, @RejectedBy);
    END
    
    SELECT 1 AS Success;
END;
GO

-- List pending KYC reviews
CREATE PROCEDURE usp_ListPendingKYCReviews
    @PageNumber INT = 1,
    @PageSize   INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        d.DocumentId,
        d.CustomerId,
        d.DocumentType,
        d.DocumentNumber,
        d.IssuingCountry,
        d.SubmittedAt,
        c.FirstName,
        c.LastName,
        c.Email,
        c.CountryOfResidence,
        c.RiskLevel,
        DATEDIFF(HOUR, d.SubmittedAt, SYSUTCDATETIME()) AS HoursPending
    FROM CustomerDocuments d
    JOIN Customers c ON d.CustomerId = c.CustomerId
    WHERE d.VerificationStatus = 'PENDING'
    ORDER BY d.SubmittedAt ASC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
    
    SELECT COUNT(*) AS TotalPending FROM CustomerDocuments WHERE VerificationStatus = 'PENDING';
END;
GO

-- List expiring/expired documents
CREATE PROCEDURE usp_ListExpiringDocuments
    @DaysUntilExpiry INT = 30
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        d.DocumentId,
        d.CustomerId,
        d.DocumentType,
        d.ExpiryDate,
        DATEDIFF(DAY, SYSUTCDATETIME(), d.ExpiryDate) AS DaysUntilExpiry,
        c.FirstName,
        c.LastName,
        c.Email
    FROM CustomerDocuments d
    JOIN Customers c ON d.CustomerId = c.CustomerId
    WHERE d.VerificationStatus = 'VERIFIED'
    AND d.ExpiryDate IS NOT NULL
    AND d.ExpiryDate <= DATEADD(DAY, @DaysUntilExpiry, SYSUTCDATETIME())
    ORDER BY d.ExpiryDate ASC;
END;
GO

-- Request document resubmission
CREATE PROCEDURE usp_RequestDocumentResubmission
    @CustomerId     BIGINT,
    @DocumentType   NVARCHAR(50),
    @Reason         NVARCHAR(500),
    @RequestedBy    NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Mark existing document as expired
    UPDATE CustomerDocuments
    SET VerificationStatus = 'EXPIRED',
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId
    AND DocumentType = @DocumentType
    AND VerificationStatus = 'VERIFIED';
    
    -- Log the request
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'DOC_RESUBMIT_REQUEST', @DocumentType, @Reason, @RequestedBy);
    
    -- Create notification (would trigger email/SMS)
    INSERT INTO Notifications (CustomerId, Channel, Subject, Body, Status, RelatedEntityType, RelatedEntityId)
    VALUES (@CustomerId, 'EMAIL', 'Document Resubmission Required', 
            'Please resubmit your ' + @DocumentType + '. Reason: ' + @Reason,
            'PENDING', 'CUSTOMER', @CustomerId);
    
    SELECT 1 AS Success;
END;
GO

-- ============================================================================
-- BENEFICIARY MANAGEMENT
-- ============================================================================

-- Create a new beneficiary
CREATE PROCEDURE usp_CreateBeneficiary
    @CustomerId             BIGINT,
    @FirstName              NVARCHAR(100),
    @LastName               NVARCHAR(100),
    @MiddleName             NVARCHAR(100) = NULL,
    @Relationship           NVARCHAR(50) = NULL,
    @Email                  NVARCHAR(255) = NULL,
    @PhoneNumber            NVARCHAR(50) = NULL,
    @Country                CHAR(2),
    @City                   NVARCHAR(100) = NULL,
    @AddressLine1           NVARCHAR(255) = NULL,
    @AddressLine2           NVARCHAR(255) = NULL,
    @StateProvince          NVARCHAR(100) = NULL,
    @PostalCode             NVARCHAR(20) = NULL,
    @PreferredPayoutMethod  NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Validate customer
    IF NOT EXISTS (SELECT 1 FROM Customers WHERE CustomerId = @CustomerId AND Status = 'ACTIVE')
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_ACTIVE' AS ErrorCode;
        RETURN;
    END
    
    -- Validate country is enabled for receiving
    IF NOT EXISTS (SELECT 1 FROM CountryConfiguration WHERE CountryCode = @Country AND IsReceivingEnabled = 1)
    BEGIN
        SELECT 0 AS Success, 'COUNTRY_NOT_SUPPORTED' AS ErrorCode,
               'This country is not currently supported for receiving' AS ErrorMessage;
        RETURN;
    END
    
    -- Check for duplicate beneficiary
    IF EXISTS (SELECT 1 FROM Beneficiaries 
               WHERE CustomerId = @CustomerId 
               AND FirstName = @FirstName AND LastName = @LastName
               AND Country = @Country AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_BENEFICIARY' AS ErrorCode,
               'A beneficiary with this name already exists for this country' AS ErrorMessage;
        RETURN;
    END
    
    INSERT INTO Beneficiaries (
        CustomerId, FirstName, LastName, MiddleName, Relationship,
        Email, PhoneNumber, Country, City, AddressLine1, AddressLine2,
        StateProvince, PostalCode, PreferredPayoutMethod, ScreeningStatus
    )
    VALUES (
        @CustomerId, @FirstName, @LastName, @MiddleName, @Relationship,
        @Email, @PhoneNumber, @Country, @City, @AddressLine1, @AddressLine2,
        @StateProvince, @PostalCode, @PreferredPayoutMethod, 'PENDING'
    );
    
    DECLARE @BeneficiaryId BIGINT = SCOPE_IDENTITY();
    DECLARE @ExternalId UNIQUEIDENTIFIER;
    SELECT @ExternalId = ExternalId FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId;
    
    SELECT 1 AS Success, @BeneficiaryId AS BeneficiaryId, @ExternalId AS ExternalId;
END;
GO

-- Get beneficiary by ID
CREATE PROCEDURE usp_GetBeneficiaryById
    @BeneficiaryId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        b.BeneficiaryId,
        b.ExternalId,
        b.CustomerId,
        b.FirstName,
        b.MiddleName,
        b.LastName,
        b.Relationship,
        b.Email,
        b.PhoneNumber,
        b.Country,
        b.City,
        b.AddressLine1,
        b.AddressLine2,
        b.StateProvince,
        b.PostalCode,
        b.PreferredPayoutMethod,
        b.ScreeningStatus,
        b.LastScreenedAt,
        b.IsActive,
        b.IsFavorite,
        b.CreatedAt,
        b.UpdatedAt
    FROM Beneficiaries b
    WHERE b.BeneficiaryId = @BeneficiaryId;
END;
GO

-- Get beneficiary by external ID
CREATE PROCEDURE usp_GetBeneficiaryByExternalId
    @ExternalId UNIQUEIDENTIFIER
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        b.BeneficiaryId,
        b.ExternalId,
        b.CustomerId,
        b.FirstName,
        b.MiddleName,
        b.LastName,
        b.Relationship,
        b.Email,
        b.PhoneNumber,
        b.Country,
        b.City,
        b.PreferredPayoutMethod,
        b.ScreeningStatus,
        b.IsActive,
        b.IsFavorite,
        b.CreatedAt
    FROM Beneficiaries b
    WHERE b.ExternalId = @ExternalId;
END;
GO

-- List beneficiaries by customer
CREATE PROCEDURE usp_ListBeneficiariesByCustomer
    @CustomerId     BIGINT,
    @IncludeInactive BIT = 0
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        b.BeneficiaryId,
        b.ExternalId,
        b.FirstName,
        b.MiddleName,
        b.LastName,
        b.Relationship,
        b.Country,
        b.City,
        b.PreferredPayoutMethod,
        b.ScreeningStatus,
        b.IsActive,
        b.IsFavorite,
        b.CreatedAt,
        (SELECT COUNT(*) FROM Transfers t WHERE t.BeneficiaryId = b.BeneficiaryId) AS TransferCount
    FROM Beneficiaries b
    WHERE b.CustomerId = @CustomerId
    AND (@IncludeInactive = 1 OR b.IsActive = 1)
    ORDER BY b.IsFavorite DESC, b.CreatedAt DESC;
END;
GO

-- Update beneficiary
CREATE PROCEDURE usp_UpdateBeneficiary
    @BeneficiaryId      BIGINT,
    @CustomerId         BIGINT,  -- For ownership verification
    @FirstName          NVARCHAR(100) = NULL,
    @LastName           NVARCHAR(100) = NULL,
    @MiddleName         NVARCHAR(100) = NULL,
    @Relationship       NVARCHAR(50) = NULL,
    @Email              NVARCHAR(255) = NULL,
    @PhoneNumber        NVARCHAR(50) = NULL,
    @City               NVARCHAR(100) = NULL,
    @AddressLine1       NVARCHAR(255) = NULL,
    @AddressLine2       NVARCHAR(255) = NULL,
    @StateProvince      NVARCHAR(100) = NULL,
    @PostalCode         NVARCHAR(20) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Verify ownership
    IF NOT EXISTS (SELECT 1 FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND CustomerId = @CustomerId)
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for pending transfers
    IF EXISTS (SELECT 1 FROM Transfers 
               WHERE BeneficiaryId = @BeneficiaryId 
               AND Status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED', 'FAILED'))
    BEGIN
        SELECT 0 AS Success, 'PENDING_TRANSFERS' AS ErrorCode,
               'Cannot update beneficiary with pending transfers' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE Beneficiaries
    SET FirstName = COALESCE(@FirstName, FirstName),
        LastName = COALESCE(@LastName, LastName),
        MiddleName = COALESCE(@MiddleName, MiddleName),
        Relationship = COALESCE(@Relationship, Relationship),
        Email = COALESCE(@Email, Email),
        PhoneNumber = COALESCE(@PhoneNumber, PhoneNumber),
        City = COALESCE(@City, City),
        AddressLine1 = COALESCE(@AddressLine1, AddressLine1),
        AddressLine2 = COALESCE(@AddressLine2, AddressLine2),
        StateProvince = COALESCE(@StateProvince, StateProvince),
        PostalCode = COALESCE(@PostalCode, PostalCode),
        ScreeningStatus = 'PENDING',  -- Re-screen after update
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    SELECT 1 AS Success;
END;
GO

-- Deactivate beneficiary
CREATE PROCEDURE usp_DeactivateBeneficiary
    @BeneficiaryId  BIGINT,
    @CustomerId     BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND CustomerId = @CustomerId)
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for pending transfers
    IF EXISTS (SELECT 1 FROM Transfers 
               WHERE BeneficiaryId = @BeneficiaryId 
               AND Status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED', 'FAILED'))
    BEGIN
        SELECT 0 AS Success, 'PENDING_TRANSFERS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Beneficiaries
    SET IsActive = 0,
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    SELECT 1 AS Success;
END;
GO

-- Reactivate beneficiary
CREATE PROCEDURE usp_ReactivateBeneficiary
    @BeneficiaryId  BIGINT,
    @CustomerId     BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND CustomerId = @CustomerId)
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Beneficiaries
    SET IsActive = 1,
        ScreeningStatus = 'PENDING',  -- Re-screen on reactivation
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    SELECT 1 AS Success;
END;
GO

-- Set beneficiary as favorite
CREATE PROCEDURE usp_SetFavoriteBeneficiary
    @BeneficiaryId  BIGINT,
    @CustomerId     BIGINT,
    @IsFavorite     BIT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND CustomerId = @CustomerId)
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Beneficiaries
    SET IsFavorite = @IsFavorite,
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    SELECT 1 AS Success;
END;
GO

-- Add bank account to beneficiary
CREATE PROCEDURE usp_AddBeneficiaryBankAccount
    @BeneficiaryId      BIGINT,
    @BankName           NVARCHAR(200),
    @BankCode           NVARCHAR(50) = NULL,
    @BranchCode         NVARCHAR(50) = NULL,
    @AccountNumber      NVARCHAR(50),
    @AccountType        NVARCHAR(20) = 'SAVINGS',
    @Currency           CHAR(3),
    @IBAN               NVARCHAR(50) = NULL,
    @RoutingNumber      NVARCHAR(50) = NULL,
    @CLABE              NVARCHAR(50) = NULL,
    @IFSC               NVARCHAR(50) = NULL,
    @AccountHolderName  NVARCHAR(200),
    @SetAsPrimary       BIT = 0
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Validate beneficiary exists
    IF NOT EXISTS (SELECT 1 FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for duplicate account
    IF EXISTS (SELECT 1 FROM BeneficiaryBankAccounts 
               WHERE BeneficiaryId = @BeneficiaryId 
               AND AccountNumber = @AccountNumber 
               AND BankCode = @BankCode
               AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_ACCOUNT' AS ErrorCode;
        RETURN;
    END
    
    -- If setting as primary, unset others
    IF @SetAsPrimary = 1
    BEGIN
        UPDATE BeneficiaryBankAccounts
        SET IsPrimary = 0
        WHERE BeneficiaryId = @BeneficiaryId;
    END
    
    -- If first account, make it primary
    DECLARE @IsFirst BIT = 0;
    IF NOT EXISTS (SELECT 1 FROM BeneficiaryBankAccounts WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1)
        SET @IsFirst = 1;
    
    INSERT INTO BeneficiaryBankAccounts (
        BeneficiaryId, BankName, BankCode, BranchCode, AccountNumber,
        AccountType, Currency, IBAN, RoutingNumber, CLABE, IFSC,
        AccountHolderName, IsPrimary
    )
    VALUES (
        @BeneficiaryId, @BankName, @BankCode, @BranchCode, @AccountNumber,
        @AccountType, @Currency, @IBAN, @RoutingNumber, @CLABE, @IFSC,
        @AccountHolderName, CASE WHEN @SetAsPrimary = 1 OR @IsFirst = 1 THEN 1 ELSE 0 END
    );
    
    DECLARE @BankAccountId BIGINT = SCOPE_IDENTITY();
    
    SELECT 1 AS Success, @BankAccountId AS BankAccountId;
END;
GO

-- Update bank account
CREATE PROCEDURE usp_UpdateBeneficiaryBankAccount
    @BankAccountId      BIGINT,
    @BankName           NVARCHAR(200) = NULL,
    @BankCode           NVARCHAR(50) = NULL,
    @BranchCode         NVARCHAR(50) = NULL,
    @AccountNumber      NVARCHAR(50) = NULL,
    @AccountHolderName  NVARCHAR(200) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM BeneficiaryBankAccounts WHERE BankAccountId = @BankAccountId)
    BEGIN
        SELECT 0 AS Success, 'ACCOUNT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE BeneficiaryBankAccounts
    SET BankName = COALESCE(@BankName, BankName),
        BankCode = COALESCE(@BankCode, BankCode),
        BranchCode = COALESCE(@BranchCode, BranchCode),
        AccountNumber = COALESCE(@AccountNumber, AccountNumber),
        AccountHolderName = COALESCE(@AccountHolderName, AccountHolderName),
        IsVerified = 0,  -- Require re-verification after update
        UpdatedAt = SYSUTCDATETIME()
    WHERE BankAccountId = @BankAccountId;
    
    SELECT 1 AS Success;
END;
GO

-- Remove bank account
CREATE PROCEDURE usp_RemoveBeneficiaryBankAccount
    @BankAccountId  BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @BeneficiaryId BIGINT;
    SELECT @BeneficiaryId = BeneficiaryId FROM BeneficiaryBankAccounts WHERE BankAccountId = @BankAccountId;
    
    IF @BeneficiaryId IS NULL
    BEGIN
        SELECT 0 AS Success, 'ACCOUNT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for pending transfers
    IF EXISTS (SELECT 1 FROM Transfers 
               WHERE PayoutBankAccountId = @BankAccountId 
               AND Status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED', 'FAILED'))
    BEGIN
        SELECT 0 AS Success, 'PENDING_TRANSFERS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE BeneficiaryBankAccounts
    SET IsActive = 0,
        UpdatedAt = SYSUTCDATETIME()
    WHERE BankAccountId = @BankAccountId;
    
    -- If this was primary, set another as primary
    IF NOT EXISTS (SELECT 1 FROM BeneficiaryBankAccounts 
                   WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1 AND IsPrimary = 1)
    BEGIN
        UPDATE TOP (1) BeneficiaryBankAccounts
        SET IsPrimary = 1
        WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1;
    END
    
    SELECT 1 AS Success;
END;
GO

-- Set primary bank account
CREATE PROCEDURE usp_SetPrimaryBankAccount
    @BankAccountId  BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @BeneficiaryId BIGINT;
    SELECT @BeneficiaryId = BeneficiaryId 
    FROM BeneficiaryBankAccounts 
    WHERE BankAccountId = @BankAccountId AND IsActive = 1;
    
    IF @BeneficiaryId IS NULL
    BEGIN
        SELECT 0 AS Success, 'ACCOUNT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Unset all, then set this one
    UPDATE BeneficiaryBankAccounts SET IsPrimary = 0 WHERE BeneficiaryId = @BeneficiaryId;
    UPDATE BeneficiaryBankAccounts SET IsPrimary = 1 WHERE BankAccountId = @BankAccountId;
    
    SELECT 1 AS Success;
END;
GO

-- Get bank accounts by beneficiary
CREATE PROCEDURE usp_GetBankAccountsByBeneficiary
    @BeneficiaryId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        BankAccountId,
        BankName,
        BankCode,
        BranchCode,
        AccountNumber,
        AccountType,
        Currency,
        IBAN,
        RoutingNumber,
        CLABE,
        IFSC,
        AccountHolderName,
        IsVerified,
        VerifiedAt,
        IsActive,
        IsPrimary,
        CreatedAt
    FROM BeneficiaryBankAccounts
    WHERE BeneficiaryId = @BeneficiaryId
    AND IsActive = 1
    ORDER BY IsPrimary DESC, CreatedAt DESC;
END;
GO

-- Add mobile wallet to beneficiary
CREATE PROCEDURE usp_AddBeneficiaryMobileWallet
    @BeneficiaryId      BIGINT,
    @ProviderCode       NVARCHAR(50),
    @WalletNumber       NVARCHAR(50),
    @WalletHolderName   NVARCHAR(200),
    @Currency           CHAR(3),
    @SetAsPrimary       BIT = 0
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for duplicate
    IF EXISTS (SELECT 1 FROM BeneficiaryMobileWallets 
               WHERE BeneficiaryId = @BeneficiaryId 
               AND ProviderCode = @ProviderCode 
               AND WalletNumber = @WalletNumber
               AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_WALLET' AS ErrorCode;
        RETURN;
    END
    
    IF @SetAsPrimary = 1
    BEGIN
        UPDATE BeneficiaryMobileWallets SET IsPrimary = 0 WHERE BeneficiaryId = @BeneficiaryId;
    END
    
    DECLARE @IsFirst BIT = 0;
    IF NOT EXISTS (SELECT 1 FROM BeneficiaryMobileWallets WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1)
        SET @IsFirst = 1;
    
    INSERT INTO BeneficiaryMobileWallets (
        BeneficiaryId, ProviderCode, WalletNumber, WalletHolderName, Currency, IsPrimary
    )
    VALUES (
        @BeneficiaryId, @ProviderCode, @WalletNumber, @WalletHolderName, @Currency,
        CASE WHEN @SetAsPrimary = 1 OR @IsFirst = 1 THEN 1 ELSE 0 END
    );
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS WalletId;
END;
GO

-- Remove mobile wallet
CREATE PROCEDURE usp_RemoveBeneficiaryMobileWallet
    @WalletId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @BeneficiaryId BIGINT;
    SELECT @BeneficiaryId = BeneficiaryId FROM BeneficiaryMobileWallets WHERE WalletId = @WalletId;
    
    IF @BeneficiaryId IS NULL
    BEGIN
        SELECT 0 AS Success, 'WALLET_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Check for pending transfers
    IF EXISTS (SELECT 1 FROM Transfers 
               WHERE PayoutWalletId = @WalletId 
               AND Status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED', 'FAILED'))
    BEGIN
        SELECT 0 AS Success, 'PENDING_TRANSFERS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE BeneficiaryMobileWallets SET IsActive = 0, UpdatedAt = SYSUTCDATETIME() WHERE WalletId = @WalletId;
    
    -- Set another as primary if needed
    IF NOT EXISTS (SELECT 1 FROM BeneficiaryMobileWallets 
                   WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1 AND IsPrimary = 1)
    BEGIN
        UPDATE TOP (1) BeneficiaryMobileWallets SET IsPrimary = 1 
        WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1;
    END
    
    SELECT 1 AS Success;
END;
GO

-- Get mobile wallets by beneficiary
CREATE PROCEDURE usp_GetMobileWalletsByBeneficiary
    @BeneficiaryId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        WalletId,
        ProviderCode,
        WalletNumber,
        WalletHolderName,
        Currency,
        IsActive,
        IsPrimary,
        CreatedAt
    FROM BeneficiaryMobileWallets
    WHERE BeneficiaryId = @BeneficiaryId
    AND IsActive = 1
    ORDER BY IsPrimary DESC, CreatedAt DESC;
END;
GO

-- Screen beneficiary against watchlists
CREATE PROCEDURE usp_ScreenBeneficiary
    @BeneficiaryId      BIGINT,
    @ScreeningProvider  NVARCHAR(50) = 'INTERNAL',
    @ScreenedBy         NVARCHAR(100) = 'SYSTEM'
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @FirstName NVARCHAR(100), @LastName NVARCHAR(100), @Country CHAR(2);
    
    SELECT @FirstName = FirstName, @LastName = LastName, @Country = Country
    FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId;
    
    IF @FirstName IS NULL
    BEGIN
        SELECT 0 AS Success, 'BENEFICIARY_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- This would integrate with real screening provider
    -- For now, simulate a clear result
    DECLARE @Result NVARCHAR(20) = 'CLEAR';
    DECLARE @MatchScore DECIMAL(5,2) = 0;
    
    INSERT INTO ComplianceScreenings (
        EntityType, EntityId, ScreeningType, ScreeningProvider,
        Result, MatchScore, ResolutionStatus, ScreenedAt
    )
    VALUES (
        'BENEFICIARY', @BeneficiaryId, 'SANCTIONS', @ScreeningProvider,
        @Result, @MatchScore, 
        CASE WHEN @Result = 'CLEAR' THEN NULL ELSE 'PENDING_REVIEW' END,
        SYSUTCDATETIME()
    );
    
    -- Update beneficiary screening status
    UPDATE Beneficiaries
    SET ScreeningStatus = CASE WHEN @Result = 'CLEAR' THEN 'CLEARED' ELSE 'FLAGGED' END,
        LastScreenedAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    SELECT 1 AS Success, @Result AS ScreeningResult, @MatchScore AS MatchScore;
END;
GO

-- Get beneficiary screening status
CREATE PROCEDURE usp_GetBeneficiaryScreeningStatus
    @BeneficiaryId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Current status
    SELECT 
        b.BeneficiaryId,
        b.FirstName,
        b.LastName,
        b.Country,
        b.ScreeningStatus,
        b.LastScreenedAt
    FROM Beneficiaries b
    WHERE b.BeneficiaryId = @BeneficiaryId;
    
    -- Screening history
    SELECT 
        ScreeningId,
        ScreeningType,
        ScreeningProvider,
        Result,
        MatchScore,
        MatchDetails,
        ResolutionStatus,
        ResolvedBy,
        ResolvedAt,
        ScreenedAt
    FROM ComplianceScreenings
    WHERE EntityType = 'BENEFICIARY' AND EntityId = @BeneficiaryId
    ORDER BY ScreenedAt DESC;
END;
GO

-- Search beneficiaries (for compliance)
CREATE PROCEDURE usp_SearchBeneficiaries
    @SearchTerm     NVARCHAR(100),
    @Country        CHAR(2) = NULL,
    @ScreeningStatus NVARCHAR(20) = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        b.BeneficiaryId,
        b.ExternalId,
        b.CustomerId,
        b.FirstName,
        b.LastName,
        b.Country,
        b.ScreeningStatus,
        b.IsActive,
        b.CreatedAt,
        c.Email AS CustomerEmail,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName
    FROM Beneficiaries b
    JOIN Customers c ON b.CustomerId = c.CustomerId
    WHERE (
        b.FirstName LIKE '%' + @SearchTerm + '%'
        OR b.LastName LIKE '%' + @SearchTerm + '%'
        OR b.Email LIKE '%' + @SearchTerm + '%'
        OR b.PhoneNumber LIKE '%' + @SearchTerm + '%'
    )
    AND (@Country IS NULL OR b.Country = @Country)
    AND (@ScreeningStatus IS NULL OR b.ScreeningStatus = @ScreeningStatus)
    ORDER BY b.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- List flagged beneficiaries
CREATE PROCEDURE usp_ListFlaggedBeneficiaries
    @PageNumber INT = 1,
    @PageSize   INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        b.BeneficiaryId,
        b.FirstName,
        b.LastName,
        b.Country,
        b.ScreeningStatus,
        b.LastScreenedAt,
        c.CustomerId,
        c.Email AS CustomerEmail,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName,
        cs.MatchScore,
        cs.ListsMatched
    FROM Beneficiaries b
    JOIN Customers c ON b.CustomerId = c.CustomerId
    LEFT JOIN ComplianceScreenings cs ON cs.EntityType = 'BENEFICIARY' 
        AND cs.EntityId = b.BeneficiaryId 
        AND cs.ResolutionStatus = 'PENDING_REVIEW'
    WHERE b.ScreeningStatus = 'FLAGGED'
    ORDER BY cs.MatchScore DESC, b.LastScreenedAt ASC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Block beneficiary
CREATE PROCEDURE usp_BlockBeneficiary
    @BeneficiaryId  BIGINT,
    @Reason         NVARCHAR(500),
    @BlockedBy      NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Beneficiaries
    SET ScreeningStatus = 'BLOCKED',
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @BlockedBy, 'BLOCK_BENEFICIARY', 'BENEFICIARY', @BeneficiaryId,
            '{"reason":"' + @Reason + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Unblock beneficiary
CREATE PROCEDURE usp_UnblockBeneficiary
    @BeneficiaryId  BIGINT,
    @Reason         NVARCHAR(500),
    @UnblockedBy    NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Beneficiaries
    SET ScreeningStatus = 'CLEARED',
        UpdatedAt = SYSUTCDATETIME()
    WHERE BeneficiaryId = @BeneficiaryId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @UnblockedBy, 'UNBLOCK_BENEFICIARY', 'BENEFICIARY', @BeneficiaryId,
            '{"reason":"' + @Reason + '"}');
    
    SELECT 1 AS Success;
END;
GO
