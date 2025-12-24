-- ============================================================================
-- MoneySend Stored Procedures
-- Part 9: Notifications & Communications
-- ============================================================================

-- Create notification from template
CREATE PROCEDURE usp_CreateNotificationFromTemplate
    @TemplateCode       NVARCHAR(50),
    @CustomerId         BIGINT,
    @RelatedEntityType  NVARCHAR(50) = NULL,
    @RelatedEntityId    BIGINT = NULL,
    @Parameters         NVARCHAR(MAX) = NULL  -- JSON with substitution values
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @TemplateId INT;
    DECLARE @Channel NVARCHAR(20);
    DECLARE @Subject NVARCHAR(500);
    DECLARE @Body NVARCHAR(MAX);
    DECLARE @Language CHAR(2);
    
    -- Get customer's preferred language
    DECLARE @CustomerLang CHAR(2);
    DECLARE @CustomerEmail NVARCHAR(255);
    DECLARE @CustomerPhone NVARCHAR(50);
    
    SELECT 
        @CustomerLang = PreferredLanguage,
        @CustomerEmail = Email,
        @CustomerPhone = PhoneNumber
    FROM Customers WHERE CustomerId = @CustomerId;
    
    -- Get template (prefer customer's language, fall back to English)
    SELECT TOP 1
        @TemplateId = TemplateId,
        @Channel = Channel,
        @Subject = Subject,
        @Body = BodyTemplate
    FROM NotificationTemplates
    WHERE TemplateCode = @TemplateCode
    AND IsActive = 1
    AND Language IN (@CustomerLang, 'en')
    ORDER BY CASE WHEN Language = @CustomerLang THEN 0 ELSE 1 END;
    
    IF @TemplateId IS NULL
    BEGIN
        SELECT 0 AS Success, 'TEMPLATE_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Parameter substitution would happen here in production
    -- For now, just use the template as-is
    
    INSERT INTO Notifications (
        CustomerId, RecipientEmail, RecipientPhone, RelatedEntityType, RelatedEntityId,
        TemplateId, Channel, Subject, Body, Status
    )
    VALUES (
        @CustomerId,
        CASE WHEN @Channel = 'EMAIL' THEN @CustomerEmail ELSE NULL END,
        CASE WHEN @Channel IN ('SMS', 'PUSH') THEN @CustomerPhone ELSE NULL END,
        @RelatedEntityType, @RelatedEntityId,
        @TemplateId, @Channel, @Subject, @Body, 'PENDING'
    );
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS NotificationId;
END;
GO

-- Send transfer initiated notification
CREATE PROCEDURE usp_SendTransferInitiatedNotification
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @TransferNumber NVARCHAR(20);
    DECLARE @SendAmount DECIMAL(18,2);
    DECLARE @SendCurrency CHAR(3);
    DECLARE @ReceiveAmount DECIMAL(18,2);
    DECLARE @ReceiveCurrency CHAR(3);
    DECLARE @BeneficiaryName NVARCHAR(200);
    
    SELECT 
        @CustomerId = t.CustomerId,
        @TransferNumber = t.TransferNumber,
        @SendAmount = t.SendAmount,
        @SendCurrency = t.SendCurrency,
        @ReceiveAmount = t.ReceiveAmount,
        @ReceiveCurrency = t.ReceiveCurrency,
        @BeneficiaryName = b.FirstName + ' ' + b.LastName
    FROM Transfers t
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    WHERE t.TransferId = @TransferId;
    
    EXEC usp_CreateNotificationFromTemplate 
        @TemplateCode = 'TRANSFER_INITIATED',
        @CustomerId = @CustomerId,
        @RelatedEntityType = 'TRANSFER',
        @RelatedEntityId = @TransferId;
END;
GO

-- Send transfer completed notification
CREATE PROCEDURE usp_SendTransferCompletedNotification
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    SELECT @CustomerId = CustomerId FROM Transfers WHERE TransferId = @TransferId;
    
    EXEC usp_CreateNotificationFromTemplate 
        @TemplateCode = 'TRANSFER_COMPLETED',
        @CustomerId = @CustomerId,
        @RelatedEntityType = 'TRANSFER',
        @RelatedEntityId = @TransferId;
END;
GO

-- Send transfer failed notification
CREATE PROCEDURE usp_SendTransferFailedNotification
    @TransferId     BIGINT,
    @FailureReason  NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    SELECT @CustomerId = CustomerId FROM Transfers WHERE TransferId = @TransferId;
    
    EXEC usp_CreateNotificationFromTemplate 
        @TemplateCode = 'TRANSFER_FAILED',
        @CustomerId = @CustomerId,
        @RelatedEntityType = 'TRANSFER',
        @RelatedEntityId = @TransferId;
END;
GO

-- Send KYC reminder
CREATE PROCEDURE usp_SendKYCReminderNotification
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    EXEC usp_CreateNotificationFromTemplate 
        @TemplateCode = 'KYC_REMINDER',
        @CustomerId = @CustomerId,
        @RelatedEntityType = 'CUSTOMER',
        @RelatedEntityId = @CustomerId;
END;
GO

-- Send document approved notification
CREATE PROCEDURE usp_SendDocumentApprovedNotification
    @CustomerId     BIGINT,
    @DocumentId     BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    EXEC usp_CreateNotificationFromTemplate 
        @TemplateCode = 'DOCUMENT_APPROVED',
        @CustomerId = @CustomerId,
        @RelatedEntityType = 'DOCUMENT',
        @RelatedEntityId = @DocumentId;
END;
GO

-- Send document rejected notification
CREATE PROCEDURE usp_SendDocumentRejectedNotification
    @CustomerId     BIGINT,
    @DocumentId     BIGINT,
    @Reason         NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    EXEC usp_CreateNotificationFromTemplate 
        @TemplateCode = 'DOCUMENT_REJECTED',
        @CustomerId = @CustomerId,
        @RelatedEntityType = 'DOCUMENT',
        @RelatedEntityId = @DocumentId;
END;
GO

-- Get pending notifications
CREATE PROCEDURE usp_GetPendingNotifications
    @Channel    NVARCHAR(20) = NULL,
    @Limit      INT = 100
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT TOP (@Limit)
        NotificationId,
        CustomerId,
        RecipientEmail,
        RecipientPhone,
        Channel,
        Subject,
        Body,
        RelatedEntityType,
        RelatedEntityId,
        CreatedAt
    FROM Notifications
    WHERE Status = 'PENDING'
    AND (@Channel IS NULL OR Channel = @Channel)
    ORDER BY CreatedAt ASC;
END;
GO

-- Mark notification as sent
CREATE PROCEDURE usp_MarkNotificationSent
    @NotificationId     BIGINT,
    @ProviderName       NVARCHAR(50),
    @ProviderMessageId  NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Notifications
    SET Status = 'SENT',
        SentAt = SYSUTCDATETIME(),
        ProviderName = @ProviderName,
        ProviderMessageId = @ProviderMessageId
    WHERE NotificationId = @NotificationId;
    
    SELECT 1 AS Success;
END;
GO

-- Mark notification as delivered
CREATE PROCEDURE usp_MarkNotificationDelivered
    @NotificationId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Notifications
    SET Status = 'DELIVERED',
        DeliveredAt = SYSUTCDATETIME()
    WHERE NotificationId = @NotificationId;
    
    SELECT 1 AS Success;
END;
GO

-- Mark notification as failed
CREATE PROCEDURE usp_MarkNotificationFailed
    @NotificationId BIGINT,
    @FailureReason  NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Notifications
    SET Status = 'FAILED',
        FailureReason = @FailureReason
    WHERE NotificationId = @NotificationId;
    
    SELECT 1 AS Success;
END;
GO

-- Get notifications by customer
CREATE PROCEDURE usp_GetNotificationsByCustomer
    @CustomerId     BIGINT,
    @Status         NVARCHAR(20) = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 20
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        NotificationId,
        Channel,
        Subject,
        Body,
        Status,
        SentAt,
        DeliveredAt,
        RelatedEntityType,
        RelatedEntityId,
        CreatedAt
    FROM Notifications
    WHERE CustomerId = @CustomerId
    AND (@Status IS NULL OR Status = @Status)
    ORDER BY CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Create notification template
CREATE PROCEDURE usp_CreateNotificationTemplate
    @TemplateCode   NVARCHAR(50),
    @TemplateName   NVARCHAR(200),
    @Channel        NVARCHAR(20),
    @Subject        NVARCHAR(500) = NULL,
    @BodyTemplate   NVARCHAR(MAX),
    @Language       CHAR(2) = 'en'
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM NotificationTemplates WHERE TemplateCode = @TemplateCode AND Language = @Language)
    BEGIN
        SELECT 0 AS Success, 'TEMPLATE_EXISTS' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO NotificationTemplates (TemplateCode, TemplateName, Channel, Subject, BodyTemplate, Language)
    VALUES (@TemplateCode, @TemplateName, @Channel, @Subject, @BodyTemplate, @Language);
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS TemplateId;
END;
GO

-- Update notification template
CREATE PROCEDURE usp_UpdateNotificationTemplate
    @TemplateId     INT,
    @TemplateName   NVARCHAR(200) = NULL,
    @Subject        NVARCHAR(500) = NULL,
    @BodyTemplate   NVARCHAR(MAX) = NULL,
    @IsActive       BIT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE NotificationTemplates
    SET TemplateName = COALESCE(@TemplateName, TemplateName),
        Subject = COALESCE(@Subject, Subject),
        BodyTemplate = COALESCE(@BodyTemplate, BodyTemplate),
        IsActive = COALESCE(@IsActive, IsActive)
    WHERE TemplateId = @TemplateId;
    
    SELECT 1 AS Success;
END;
GO

-- List notification templates
CREATE PROCEDURE usp_ListNotificationTemplates
    @Channel    NVARCHAR(20) = NULL,
    @Language   CHAR(2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        TemplateId,
        TemplateCode,
        TemplateName,
        Channel,
        Subject,
        Language,
        IsActive
    FROM NotificationTemplates
    WHERE (@Channel IS NULL OR Channel = @Channel)
    AND (@Language IS NULL OR Language = @Language)
    ORDER BY TemplateCode, Language;
END;
GO

-- Get notification stats
CREATE PROCEDURE usp_GetNotificationStats
    @StartDate  DATETIME2,
    @EndDate    DATETIME2 = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    SELECT 
        Channel,
        Status,
        COUNT(*) AS NotificationCount
    FROM Notifications
    WHERE CreatedAt >= @StartDate AND CreatedAt <= @EndDate
    GROUP BY Channel, Status
    ORDER BY Channel, Status;
    
    -- Delivery rates
    SELECT 
        Channel,
        COUNT(*) AS TotalSent,
        SUM(CASE WHEN Status = 'DELIVERED' THEN 1 ELSE 0 END) AS Delivered,
        SUM(CASE WHEN Status = 'FAILED' THEN 1 ELSE 0 END) AS Failed,
        CAST(SUM(CASE WHEN Status = 'DELIVERED' THEN 1 ELSE 0 END) AS DECIMAL) / 
            NULLIF(COUNT(*), 0) * 100 AS DeliveryRate
    FROM Notifications
    WHERE CreatedAt >= @StartDate AND CreatedAt <= @EndDate
    AND Status IN ('SENT', 'DELIVERED', 'FAILED')
    GROUP BY Channel;
END;
GO

-- Retry failed notifications
CREATE PROCEDURE usp_RetryFailedNotifications
    @MaxRetries INT = 3
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Reset failed notifications to pending (would track retry count in production)
    UPDATE Notifications
    SET Status = 'PENDING'
    WHERE Status = 'FAILED'
    AND CreatedAt >= DATEADD(HOUR, -24, SYSUTCDATETIME());
    
    SELECT @@ROWCOUNT AS RetriedCount;
END;
GO
