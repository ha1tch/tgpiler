-- ============================================================================
-- MoneySend Stored Procedures
-- Part 4: Transfer Status Transitions & Payment Processing
-- ============================================================================

-- ============================================================================
-- TRANSFER STATUS TRANSITIONS
-- ============================================================================

-- Internal helper to update transfer status
CREATE PROCEDURE usp_UpdateTransferStatus
    @TransferId     BIGINT,
    @NewStatus      NVARCHAR(30),
    @SubStatus      NVARCHAR(50) = NULL,
    @Reason         NVARCHAR(500) = NULL,
    @ChangedBy      NVARCHAR(100),
    @Notes          NVARCHAR(1000) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @PreviousStatus NVARCHAR(30);
    SELECT @PreviousStatus = Status FROM Transfers WHERE TransferId = @TransferId;
    
    IF @PreviousStatus IS NULL
    BEGIN
        SELECT 0 AS Success, 'TRANSFER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Update transfer
    UPDATE Transfers
    SET Status = @NewStatus,
        SubStatus = @SubStatus,
        StatusReason = @Reason,
        UpdatedAt = SYSUTCDATETIME(),
        ProcessingStartedAt = CASE WHEN @NewStatus = 'PROCESSING' AND ProcessingStartedAt IS NULL THEN SYSUTCDATETIME() ELSE ProcessingStartedAt END,
        SentToPartnerAt = CASE WHEN @NewStatus = 'SENT_TO_PARTNER' THEN SYSUTCDATETIME() ELSE SentToPartnerAt END,
        CompletedAt = CASE WHEN @NewStatus = 'COMPLETED' THEN SYSUTCDATETIME() ELSE CompletedAt END,
        CancelledAt = CASE WHEN @NewStatus IN ('CANCELLED', 'REFUNDED') THEN SYSUTCDATETIME() ELSE CancelledAt END
    WHERE TransferId = @TransferId;
    
    -- Log history
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, SubStatus, Reason, ChangedBy, Notes)
    VALUES (@TransferId, @PreviousStatus, @NewStatus, @SubStatus, @Reason, @ChangedBy, @Notes);
    
    SELECT 1 AS Success, @PreviousStatus AS PreviousStatus, @NewStatus AS NewStatus;
END;
GO

-- Confirm payment received (from payment gateway callback)
CREATE PROCEDURE usp_ConfirmPaymentReceived
    @TransferId             BIGINT,
    @GatewayTransactionId   NVARCHAR(100),
    @GatewayProvider        NVARCHAR(50),
    @Amount                 DECIMAL(18,2),
    @Currency               CHAR(3)
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    DECLARE @ExpectedAmount DECIMAL(18,2);
    DECLARE @FundingSourceId BIGINT;
    
    SELECT 
        @CurrentStatus = Status,
        @ExpectedAmount = TotalCharged
    FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus IS NULL
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'TRANSFER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @CurrentStatus NOT IN ('CREATED', 'PENDING_PAYMENT')
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode, 
               'Transfer is not awaiting payment' AS ErrorMessage;
        RETURN;
    END
    
    -- Verify amount (allow small variance for FX)
    IF ABS(@Amount - @ExpectedAmount) > 0.01
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'AMOUNT_MISMATCH' AS ErrorCode,
               'Expected: ' + CAST(@ExpectedAmount AS NVARCHAR) + ', Received: ' + CAST(@Amount AS NVARCHAR) AS ErrorMessage;
        RETURN;
    END
    
    -- Record payment transaction
    INSERT INTO PaymentTransactions (
        TransferId, PaymentType, Amount, Currency,
        Status, GatewayProvider, GatewayTransactionId, CompletedAt
    )
    VALUES (
        @TransferId, 'CHARGE', @Amount, @Currency,
        'COMPLETED', @GatewayProvider, @GatewayTransactionId, SYSUTCDATETIME()
    );
    
    -- Update transfer status
    UPDATE Transfers
    SET Status = 'PAYMENT_RECEIVED',
        PaymentReceivedAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, ChangedBy, Notes)
    VALUES (@TransferId, @CurrentStatus, 'PAYMENT_RECEIVED', 'PAYMENT_GATEWAY', 
            'Payment confirmed via ' + @GatewayProvider + ' - ' + @GatewayTransactionId);
    
    COMMIT;
    SELECT 1 AS Success;
END;
GO

-- Submit transfer for compliance review
CREATE PROCEDURE usp_SubmitForComplianceReview
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus != 'PAYMENT_RECEIVED'
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Transfers
    SET Status = 'COMPLIANCE_REVIEW',
        ComplianceStatus = 'PENDING',
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, ChangedBy)
    VALUES (@TransferId, @CurrentStatus, 'COMPLIANCE_REVIEW', 'SYSTEM');
    
    SELECT 1 AS Success;
END;
GO

-- Clear transfer for processing (compliance approved)
CREATE PROCEDURE usp_ClearTransferForProcessing
    @TransferId     BIGINT,
    @ClearedBy      NVARCHAR(100),
    @Notes          NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    DECLARE @ComplianceStatus NVARCHAR(20);
    
    SELECT @CurrentStatus = Status, @ComplianceStatus = ComplianceStatus 
    FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus NOT IN ('COMPLIANCE_REVIEW', 'ON_HOLD')
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Transfers
    SET Status = 'PROCESSING',
        ComplianceStatus = 'CLEARED',
        ComplianceReviewedAt = SYSUTCDATETIME(),
        ComplianceReviewedBy = @ClearedBy,
        ProcessingStartedAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, ChangedBy, Notes)
    VALUES (@TransferId, @CurrentStatus, 'PROCESSING', @ClearedBy, @Notes);
    
    -- Update compliance screening resolution
    UPDATE ComplianceScreenings
    SET ResolutionStatus = 'FALSE_POSITIVE',
        ResolvedBy = @ClearedBy,
        ResolvedAt = SYSUTCDATETIME(),
        ResolutionNotes = @Notes
    WHERE TransferId = @TransferId AND ResolutionStatus = 'PENDING_REVIEW';
    
    SELECT 1 AS Success;
END;
GO

-- Flag transfer for investigation
CREATE PROCEDURE usp_FlagTransferForInvestigation
    @TransferId     BIGINT,
    @FlaggedBy      NVARCHAR(100),
    @FlagReason     NVARCHAR(500),
    @RiskScore      DECIMAL(5,2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM Transfers WHERE TransferId = @TransferId;
    
    UPDATE Transfers
    SET Status = 'ON_HOLD',
        SubStatus = 'INVESTIGATION',
        ComplianceStatus = 'FLAGGED',
        StatusReason = @FlagReason,
        RiskScore = COALESCE(@RiskScore, RiskScore),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, SubStatus, Reason, ChangedBy)
    VALUES (@TransferId, @CurrentStatus, 'ON_HOLD', 'INVESTIGATION', @FlagReason, @FlaggedBy);
    
    SELECT 1 AS Success;
END;
GO

-- Block transfer (compliance rejection)
CREATE PROCEDURE usp_BlockTransfer
    @TransferId     BIGINT,
    @BlockedBy      NVARCHAR(100),
    @BlockReason    NVARCHAR(500),
    @InitiateRefund BIT = 1
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    DECLARE @CustomerId BIGINT;
    DECLARE @TotalCharged DECIMAL(18,2);
    
    SELECT 
        @CurrentStatus = Status,
        @CustomerId = CustomerId,
        @TotalCharged = TotalCharged
    FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus IN ('COMPLETED', 'CANCELLED', 'REFUNDED')
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode, 'Transfer already finalized' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE Transfers
    SET Status = 'CANCELLED',
        SubStatus = 'BLOCKED',
        ComplianceStatus = 'BLOCKED',
        StatusReason = @BlockReason,
        CancelledAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, SubStatus, Reason, ChangedBy)
    VALUES (@TransferId, @CurrentStatus, 'CANCELLED', 'BLOCKED', @BlockReason, @BlockedBy);
    
    -- Initiate refund if payment was received
    IF @InitiateRefund = 1 AND @CurrentStatus NOT IN ('CREATED', 'PENDING_PAYMENT')
    BEGIN
        INSERT INTO PaymentTransactions (TransferId, PaymentType, Amount, Currency, Status)
        SELECT @TransferId, 'REFUND', TotalCharged, SendCurrency, 'PENDING'
        FROM Transfers WHERE TransferId = @TransferId;
    END
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @BlockedBy, 'BLOCK_TRANSFER', 'TRANSFER', @TransferId, 
            '{"reason":"' + @BlockReason + '"}');
    
    COMMIT;
    SELECT 1 AS Success;
END;
GO

-- Send transfer to payout partner
CREATE PROCEDURE usp_SendToPayoutPartner
    @TransferId     BIGINT,
    @PartnerReference NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    DECLARE @PayoutMethod NVARCHAR(50);
    DECLARE @PartnerId INT;
    
    SELECT 
        @CurrentStatus = Status,
        @PayoutMethod = PayoutMethod,
        @PartnerId = PayoutPartnerId
    FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus != 'PROCESSING'
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    -- Generate cash pickup code if needed
    DECLARE @CashPickupCode NVARCHAR(20) = NULL;
    IF @PayoutMethod = 'CASH_PICKUP'
    BEGIN
        SET @CashPickupCode = UPPER(LEFT(NEWID(), 8));
    END
    
    UPDATE Transfers
    SET Status = 'SENT_TO_PARTNER',
        PayoutReference = @PartnerReference,
        CashPickupCode = @CashPickupCode,
        SentToPartnerAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, ChangedBy, Notes)
    VALUES (@TransferId, @CurrentStatus, 'SENT_TO_PARTNER', 'SYSTEM', 
            'Partner ref: ' + COALESCE(@PartnerReference, 'pending'));
    
    SELECT 1 AS Success, @CashPickupCode AS CashPickupCode;
END;
GO

-- Update payout status from partner
CREATE PROCEDURE usp_UpdatePayoutStatus
    @TransferId         BIGINT,
    @PartnerReference   NVARCHAR(100),
    @PayoutStatus       NVARCHAR(30),  -- PENDING, COMPLETED, FAILED, RETURNED
    @FailureReason      NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus NOT IN ('SENT_TO_PARTNER', 'PAYOUT_PENDING')
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    DECLARE @NewStatus NVARCHAR(30);
    SET @NewStatus = CASE @PayoutStatus
        WHEN 'PENDING' THEN 'PAYOUT_PENDING'
        WHEN 'COMPLETED' THEN 'COMPLETED'
        WHEN 'FAILED' THEN 'FAILED'
        WHEN 'RETURNED' THEN 'FAILED'
        ELSE @CurrentStatus
    END;
    
    UPDATE Transfers
    SET Status = @NewStatus,
        PayoutReference = @PartnerReference,
        StatusReason = @FailureReason,
        CompletedAt = CASE WHEN @PayoutStatus = 'COMPLETED' THEN SYSUTCDATETIME() ELSE CompletedAt END,
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, Reason, ChangedBy)
    VALUES (@TransferId, @CurrentStatus, @NewStatus, @FailureReason, 'PARTNER');
    
    SELECT 1 AS Success, @NewStatus AS Status;
END;
GO

-- Mark transfer as completed
CREATE PROCEDURE usp_CompleteTransfer
    @TransferId     BIGINT,
    @CompletedBy    NVARCHAR(100) = 'SYSTEM'
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus NOT IN ('SENT_TO_PARTNER', 'PAYOUT_PENDING')
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Transfers
    SET Status = 'COMPLETED',
        CompletedAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, ChangedBy)
    VALUES (@TransferId, @CurrentStatus, 'COMPLETED', @CompletedBy);
    
    -- Create notification
    INSERT INTO Notifications (CustomerId, Channel, Subject, Body, Status, RelatedEntityType, RelatedEntityId)
    SELECT CustomerId, 'EMAIL', 'Transfer Completed', 
           'Your transfer ' + TransferNumber + ' has been completed successfully.',
           'PENDING', 'TRANSFER', @TransferId
    FROM Transfers WHERE TransferId = @TransferId;
    
    SELECT 1 AS Success;
END;
GO

-- Cancel transfer (customer initiated)
CREATE PROCEDURE usp_CancelTransfer
    @TransferId         BIGINT,
    @CustomerId         BIGINT,
    @CancellationReason NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    DECLARE @TransferCustomerId BIGINT;
    DECLARE @PaymentReceivedAt DATETIME2;
    
    SELECT 
        @CurrentStatus = Status,
        @TransferCustomerId = CustomerId,
        @PaymentReceivedAt = PaymentReceivedAt
    FROM Transfers WHERE TransferId = @TransferId;
    
    -- Verify ownership
    IF @TransferCustomerId != @CustomerId
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'NOT_AUTHORIZED' AS ErrorCode;
        RETURN;
    END
    
    -- Check if cancellable
    IF @CurrentStatus NOT IN ('CREATED', 'PENDING_PAYMENT', 'PAYMENT_RECEIVED', 'COMPLIANCE_REVIEW')
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'NOT_CANCELLABLE' AS ErrorCode,
               'Transfer cannot be cancelled in current status: ' + @CurrentStatus AS ErrorMessage;
        RETURN;
    END
    
    UPDATE Transfers
    SET Status = 'CANCELLED',
        SubStatus = 'CUSTOMER_CANCELLED',
        StatusReason = @CancellationReason,
        CancelledAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, SubStatus, Reason, ChangedBy)
    VALUES (@TransferId, @CurrentStatus, 'CANCELLED', 'CUSTOMER_CANCELLED', @CancellationReason, CAST(@CustomerId AS NVARCHAR));
    
    -- Initiate refund if payment was received
    IF @PaymentReceivedAt IS NOT NULL
    BEGIN
        INSERT INTO PaymentTransactions (TransferId, PaymentType, Amount, Currency, Status)
        SELECT @TransferId, 'REFUND', TotalCharged, SendCurrency, 'PENDING'
        FROM Transfers WHERE TransferId = @TransferId;
        
        UPDATE Transfers SET SubStatus = 'REFUND_PENDING' WHERE TransferId = @TransferId;
    END
    
    COMMIT;
    SELECT 1 AS Success, 
           CASE WHEN @PaymentReceivedAt IS NOT NULL THEN 'REFUND_INITIATED' ELSE 'CANCELLED' END AS RefundStatus;
END;
GO

-- Process refund
CREATE PROCEDURE usp_ProcessRefund
    @TransferId             BIGINT,
    @GatewayTransactionId   NVARCHAR(100),
    @RefundStatus           NVARCHAR(20),  -- COMPLETED, FAILED
    @FailureReason          NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Update payment transaction
    UPDATE PaymentTransactions
    SET Status = @RefundStatus,
        GatewayTransactionId = @GatewayTransactionId,
        FailureReason = @FailureReason,
        CompletedAt = CASE WHEN @RefundStatus = 'COMPLETED' THEN SYSUTCDATETIME() ELSE NULL END
    WHERE TransferId = @TransferId AND PaymentType = 'REFUND' AND Status = 'PENDING';
    
    IF @RefundStatus = 'COMPLETED'
    BEGIN
        UPDATE Transfers
        SET Status = 'REFUNDED',
            SubStatus = NULL,
            UpdatedAt = SYSUTCDATETIME()
        WHERE TransferId = @TransferId;
        
        INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, ChangedBy, Notes)
        SELECT TransferId, Status, 'REFUNDED', 'PAYMENT_GATEWAY', 'Refund completed: ' + @GatewayTransactionId
        FROM Transfers WHERE TransferId = @TransferId;
    END
    
    SELECT 1 AS Success;
END;
GO

-- Retry failed transfer
CREATE PROCEDURE usp_RetryFailedTransfer
    @TransferId     BIGINT,
    @RetriedBy      NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM Transfers WHERE TransferId = @TransferId;
    
    IF @CurrentStatus != 'FAILED'
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Transfers
    SET Status = 'PROCESSING',
        SubStatus = 'RETRY',
        StatusReason = NULL,
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, SubStatus, ChangedBy)
    VALUES (@TransferId, 'FAILED', 'PROCESSING', 'RETRY', @RetriedBy);
    
    SELECT 1 AS Success;
END;
GO

-- ============================================================================
-- PAYMENT / FUNDING SOURCE MANAGEMENT
-- ============================================================================

-- Add funding source (card or bank account)
CREATE PROCEDURE usp_AddFundingSource
    @CustomerId         BIGINT,
    @SourceType         NVARCHAR(30),
    @CardLastFour       CHAR(4) = NULL,
    @CardBrand          NVARCHAR(20) = NULL,
    @CardExpiry         CHAR(7) = NULL,
    @CardHolderName     NVARCHAR(200) = NULL,
    @BankName           NVARCHAR(200) = NULL,
    @AccountLastFour    CHAR(4) = NULL,
    @RoutingNumber      NVARCHAR(20) = NULL,
    @TokenProvider      NVARCHAR(50),
    @PaymentToken       NVARCHAR(500),
    @SetAsPrimary       BIT = 0
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Customers WHERE CustomerId = @CustomerId AND Status = 'ACTIVE')
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_ACTIVE' AS ErrorCode;
        RETURN;
    END
    
    -- Check for duplicate token
    IF EXISTS (SELECT 1 FROM CustomerFundingSources WHERE CustomerId = @CustomerId AND PaymentToken = @PaymentToken AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_SOURCE' AS ErrorCode;
        RETURN;
    END
    
    IF @SetAsPrimary = 1
    BEGIN
        UPDATE CustomerFundingSources SET IsPrimary = 0 WHERE CustomerId = @CustomerId;
    END
    
    DECLARE @IsFirst BIT = 0;
    IF NOT EXISTS (SELECT 1 FROM CustomerFundingSources WHERE CustomerId = @CustomerId AND IsActive = 1)
        SET @IsFirst = 1;
    
    INSERT INTO CustomerFundingSources (
        CustomerId, SourceType, CardLastFour, CardBrand, CardExpiry, CardHolderName,
        BankName, AccountLastFour, RoutingNumber, TokenProvider, PaymentToken, IsPrimary
    )
    VALUES (
        @CustomerId, @SourceType, @CardLastFour, @CardBrand, @CardExpiry, @CardHolderName,
        @BankName, @AccountLastFour, @RoutingNumber, @TokenProvider, @PaymentToken,
        CASE WHEN @SetAsPrimary = 1 OR @IsFirst = 1 THEN 1 ELSE 0 END
    );
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS FundingSourceId;
END;
GO

-- Get funding sources by customer
CREATE PROCEDURE usp_GetFundingSourcesByCustomer
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        FundingSourceId,
        SourceType,
        CardLastFour,
        CardBrand,
        CardExpiry,
        CardHolderName,
        BankName,
        AccountLastFour,
        IsVerified,
        IsActive,
        IsPrimary,
        CreatedAt
    FROM CustomerFundingSources
    WHERE CustomerId = @CustomerId AND IsActive = 1
    ORDER BY IsPrimary DESC, CreatedAt DESC;
END;
GO

-- Remove funding source
CREATE PROCEDURE usp_RemoveFundingSource
    @FundingSourceId    BIGINT,
    @CustomerId         BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM CustomerFundingSources 
                   WHERE FundingSourceId = @FundingSourceId AND CustomerId = @CustomerId)
    BEGIN
        SELECT 0 AS Success, 'SOURCE_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE CustomerFundingSources
    SET IsActive = 0, UpdatedAt = SYSUTCDATETIME()
    WHERE FundingSourceId = @FundingSourceId;
    
    -- Set another as primary if needed
    IF NOT EXISTS (SELECT 1 FROM CustomerFundingSources 
                   WHERE CustomerId = @CustomerId AND IsActive = 1 AND IsPrimary = 1)
    BEGIN
        UPDATE TOP (1) CustomerFundingSources SET IsPrimary = 1 
        WHERE CustomerId = @CustomerId AND IsActive = 1;
    END
    
    SELECT 1 AS Success;
END;
GO

-- Set primary funding source
CREATE PROCEDURE usp_SetPrimaryFundingSource
    @FundingSourceId    BIGINT,
    @CustomerId         BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM CustomerFundingSources 
                   WHERE FundingSourceId = @FundingSourceId AND CustomerId = @CustomerId AND IsActive = 1)
    BEGIN
        SELECT 0 AS Success, 'SOURCE_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE CustomerFundingSources SET IsPrimary = 0 WHERE CustomerId = @CustomerId;
    UPDATE CustomerFundingSources SET IsPrimary = 1 WHERE FundingSourceId = @FundingSourceId;
    
    SELECT 1 AS Success;
END;
GO

-- Verify funding source
CREATE PROCEDURE usp_VerifyFundingSource
    @FundingSourceId    BIGINT,
    @VerificationCode   NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE CustomerFundingSources
    SET IsVerified = 1,
        VerifiedAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE FundingSourceId = @FundingSourceId;
    
    SELECT 1 AS Success;
END;
GO

-- Get payment transactions by transfer
CREATE PROCEDURE usp_GetPaymentTransactionsByTransfer
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        PaymentId,
        TransferId,
        FundingSourceId,
        PaymentType,
        Amount,
        Currency,
        Status,
        GatewayProvider,
        GatewayTransactionId,
        FailureCode,
        FailureReason,
        CreatedAt,
        CompletedAt
    FROM PaymentTransactions
    WHERE TransferId = @TransferId
    ORDER BY CreatedAt ASC;
END;
GO

-- Record chargeback
CREATE PROCEDURE usp_RecordChargeback
    @TransferId             BIGINT,
    @ChargebackAmount       DECIMAL(18,2),
    @ChargebackReason       NVARCHAR(500),
    @GatewayTransactionId   NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @CustomerId BIGINT;
    SELECT @CustomerId = CustomerId FROM Transfers WHERE TransferId = @TransferId;
    
    -- Record payment transaction
    INSERT INTO PaymentTransactions (
        TransferId, PaymentType, Amount, Currency, Status,
        GatewayTransactionId, FailureReason, CompletedAt
    )
    SELECT @TransferId, 'CHARGEBACK', @ChargebackAmount, SendCurrency, 'COMPLETED',
           @GatewayTransactionId, @ChargebackReason, SYSUTCDATETIME()
    FROM Transfers WHERE TransferId = @TransferId;
    
    -- Flag customer for review
    UPDATE Customers
    SET RiskLevel = 'HIGH',
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'RISK_ESCALATION', 'HIGH', 'Chargeback received on transfer ' + CAST(@TransferId AS NVARCHAR), 'SYSTEM');
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('SYSTEM', 'CHARGEBACK', 'RECORD_CHARGEBACK', 'TRANSFER', @TransferId,
            '{"amount":' + CAST(@ChargebackAmount AS NVARCHAR) + ',"reason":"' + @ChargebackReason + '"}');
    
    COMMIT;
    SELECT 1 AS Success;
END;
GO

-- ============================================================================
-- TRANSFER SEARCH & QUERIES
-- ============================================================================

-- Search transfers
CREATE PROCEDURE usp_SearchTransfers
    @TransferNumber     NVARCHAR(20) = NULL,
    @CustomerEmail      NVARCHAR(255) = NULL,
    @BeneficiaryName    NVARCHAR(200) = NULL,
    @Status             NVARCHAR(30) = NULL,
    @ComplianceStatus   NVARCHAR(20) = NULL,
    @CorridorId         INT = NULL,
    @MinAmount          DECIMAL(18,2) = NULL,
    @MaxAmount          DECIMAL(18,2) = NULL,
    @StartDate          DATETIME2 = NULL,
    @EndDate            DATETIME2 = NULL,
    @PageNumber         INT = 1,
    @PageSize           INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        t.TransferId,
        t.TransferNumber,
        t.SendAmount,
        t.SendCurrency,
        t.ReceiveAmount,
        t.ReceiveCurrency,
        t.Status,
        t.ComplianceStatus,
        t.PayoutMethod,
        t.CreatedAt,
        t.CompletedAt,
        c.Email AS CustomerEmail,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName,
        b.Country AS BeneficiaryCountry,
        cor.DisplayName AS CorridorName
    FROM Transfers t
    JOIN Customers c ON t.CustomerId = c.CustomerId
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    JOIN Corridors cor ON t.CorridorId = cor.CorridorId
    WHERE (@TransferNumber IS NULL OR t.TransferNumber LIKE '%' + @TransferNumber + '%')
    AND (@CustomerEmail IS NULL OR c.Email LIKE '%' + @CustomerEmail + '%')
    AND (@BeneficiaryName IS NULL OR b.FirstName + ' ' + b.LastName LIKE '%' + @BeneficiaryName + '%')
    AND (@Status IS NULL OR t.Status = @Status)
    AND (@ComplianceStatus IS NULL OR t.ComplianceStatus = @ComplianceStatus)
    AND (@CorridorId IS NULL OR t.CorridorId = @CorridorId)
    AND (@MinAmount IS NULL OR t.SendAmount >= @MinAmount)
    AND (@MaxAmount IS NULL OR t.SendAmount <= @MaxAmount)
    AND (@StartDate IS NULL OR t.CreatedAt >= @StartDate)
    AND (@EndDate IS NULL OR t.CreatedAt <= @EndDate)
    ORDER BY t.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Get transfers pending compliance review
CREATE PROCEDURE usp_GetTransfersPendingCompliance
    @PageNumber INT = 1,
    @PageSize   INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        t.TransferId,
        t.TransferNumber,
        t.SendAmount,
        t.SendCurrency,
        t.ReceiveAmount,
        t.ReceiveCurrency,
        t.Status,
        t.ComplianceStatus,
        t.RiskScore,
        t.Purpose,
        t.CreatedAt,
        DATEDIFF(MINUTE, t.CreatedAt, SYSUTCDATETIME()) AS MinutesPending,
        c.CustomerId,
        c.Email AS CustomerEmail,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName,
        c.RiskLevel AS CustomerRiskLevel,
        c.VerificationTier,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName,
        b.Country AS BeneficiaryCountry,
        b.ScreeningStatus AS BeneficiaryScreeningStatus,
        cor.DisplayName AS CorridorName
    FROM Transfers t
    JOIN Customers c ON t.CustomerId = c.CustomerId
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    JOIN Corridors cor ON t.CorridorId = cor.CorridorId
    WHERE t.Status = 'COMPLIANCE_REVIEW'
    ORDER BY t.RiskScore DESC, t.CreatedAt ASC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
    
    SELECT COUNT(*) AS TotalPending FROM Transfers WHERE Status = 'COMPLIANCE_REVIEW';
END;
GO

-- Get transfers on hold
CREATE PROCEDURE usp_GetTransfersOnHold
    @PageNumber INT = 1,
    @PageSize   INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        t.TransferId,
        t.TransferNumber,
        t.SendAmount,
        t.SendCurrency,
        t.Status,
        t.SubStatus,
        t.StatusReason,
        t.RiskScore,
        t.CreatedAt,
        t.UpdatedAt,
        c.Email AS CustomerEmail,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName,
        b.Country AS BeneficiaryCountry
    FROM Transfers t
    JOIN Customers c ON t.CustomerId = c.CustomerId
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    WHERE t.Status = 'ON_HOLD'
    ORDER BY t.UpdatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Get daily transfer summary
CREATE PROCEDURE usp_GetDailyTransferSummary
    @Date DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @Date = COALESCE(@Date, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedCount,
        SUM(CASE WHEN Status IN ('CREATED', 'PENDING_PAYMENT', 'PAYMENT_RECEIVED', 'COMPLIANCE_REVIEW', 'PROCESSING', 'SENT_TO_PARTNER', 'PAYOUT_PENDING') THEN 1 ELSE 0 END) AS PendingCount,
        SUM(CASE WHEN Status IN ('CANCELLED', 'REFUNDED', 'FAILED') THEN 1 ELSE 0 END) AS FailedCount,
        SUM(SendAmount) AS TotalSendVolume,
        SUM(TotalFees) AS TotalFeeRevenue,
        AVG(SendAmount) AS AvgTransferAmount,
        COUNT(DISTINCT CustomerId) AS UniqueCustomers,
        COUNT(DISTINCT CorridorId) AS CorridorsUsed
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) = @Date;
    
    -- By corridor
    SELECT 
        c.CorridorCode,
        c.DisplayName,
        COUNT(*) AS TransferCount,
        SUM(t.SendAmount) AS Volume,
        SUM(t.TotalFees) AS Fees
    FROM Transfers t
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE CAST(t.CreatedAt AS DATE) = @Date
    GROUP BY c.CorridorId, c.CorridorCode, c.DisplayName
    ORDER BY Volume DESC;
    
    -- By status
    SELECT 
        Status,
        COUNT(*) AS Count
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) = @Date
    GROUP BY Status;
END;
GO

-- Get customer transfer statistics
CREATE PROCEDURE usp_GetCustomerTransferStats
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Overall stats
    SELECT 
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(SendAmount) AS TotalSent,
        SUM(TotalFees) AS TotalFeesPaid,
        AVG(SendAmount) AS AvgTransferAmount,
        MIN(CreatedAt) AS FirstTransferDate,
        MAX(CreatedAt) AS LastTransferDate
    FROM Transfers
    WHERE CustomerId = @CustomerId;
    
    -- By corridor
    SELECT TOP 5
        c.DisplayName AS Corridor,
        COUNT(*) AS TransferCount,
        SUM(t.SendAmount) AS TotalSent
    FROM Transfers t
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE t.CustomerId = @CustomerId AND t.Status = 'COMPLETED'
    GROUP BY c.CorridorId, c.DisplayName
    ORDER BY TotalSent DESC;
    
    -- By beneficiary
    SELECT TOP 5
        b.FirstName + ' ' + b.LastName AS BeneficiaryName,
        b.Country,
        COUNT(*) AS TransferCount,
        SUM(t.SendAmount) AS TotalSent
    FROM Transfers t
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    WHERE t.CustomerId = @CustomerId AND t.Status = 'COMPLETED'
    GROUP BY b.BeneficiaryId, b.FirstName, b.LastName, b.Country
    ORDER BY TotalSent DESC;
    
    -- Recent activity (last 30 days)
    SELECT 
        CAST(CreatedAt AS DATE) AS TransferDate,
        COUNT(*) AS TransferCount,
        SUM(SendAmount) AS DailyVolume
    FROM Transfers
    WHERE CustomerId = @CustomerId
    AND CreatedAt >= DATEADD(DAY, -30, SYSUTCDATETIME())
    GROUP BY CAST(CreatedAt AS DATE)
    ORDER BY TransferDate DESC;
END;
GO
