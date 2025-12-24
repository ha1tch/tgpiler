-- ============================================================================
-- MoneySend Stored Procedures
-- Part 10: Reporting & Analytics
-- ============================================================================

-- Daily transfer summary
CREATE PROCEDURE usp_GetDailyTransferSummary
    @Date DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @Date = COALESCE(@Date, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        @Date AS ReportDate,
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN Status = 'CANCELLED' THEN 1 ELSE 0 END) AS CancelledTransfers,
        SUM(CASE WHEN Status = 'FAILED' THEN 1 ELSE 0 END) AS FailedTransfers,
        SUM(CASE WHEN ComplianceStatus = 'FLAGGED' THEN 1 ELSE 0 END) AS FlaggedTransfers,
        SUM(SendAmount) AS TotalSendVolume,
        SUM(ReceiveAmount) AS TotalReceiveVolume,
        SUM(TotalFees) AS TotalFeeRevenue,
        SUM(FXMargin) AS TotalFXRevenue,
        AVG(SendAmount) AS AvgTransferAmount,
        COUNT(DISTINCT CustomerId) AS UniqueCustomers,
        COUNT(DISTINCT BeneficiaryId) AS UniqueBeneficiaries
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) = @Date;
END;
GO

-- Weekly transfer summary
CREATE PROCEDURE usp_GetWeeklyTransferSummary
    @WeekStartDate DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Default to current week start (Monday)
    SET @WeekStartDate = COALESCE(@WeekStartDate, DATEADD(DAY, 1-DATEPART(WEEKDAY, SYSUTCDATETIME()), CAST(SYSUTCDATETIME() AS DATE)));
    
    DECLARE @WeekEndDate DATE = DATEADD(DAY, 6, @WeekStartDate);
    
    SELECT 
        @WeekStartDate AS WeekStart,
        @WeekEndDate AS WeekEnd,
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(SendAmount) AS TotalSendVolume,
        SUM(TotalFees) AS TotalFeeRevenue,
        SUM(FXMargin) AS TotalFXRevenue,
        COUNT(DISTINCT CustomerId) AS UniqueCustomers
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @WeekStartDate AND @WeekEndDate;
    
    -- Daily breakdown
    SELECT 
        CAST(CreatedAt AS DATE) AS TransferDate,
        COUNT(*) AS Transfers,
        SUM(SendAmount) AS Volume,
        SUM(TotalFees) AS Fees
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @WeekStartDate AND @WeekEndDate
    GROUP BY CAST(CreatedAt AS DATE)
    ORDER BY TransferDate;
END;
GO

-- Monthly transfer summary
CREATE PROCEDURE usp_GetMonthlyTransferSummary
    @Year   INT = NULL,
    @Month  INT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @Year = COALESCE(@Year, YEAR(SYSUTCDATETIME()));
    SET @Month = COALESCE(@Month, MONTH(SYSUTCDATETIME()));
    
    DECLARE @StartDate DATE = DATEFROMPARTS(@Year, @Month, 1);
    DECLARE @EndDate DATE = EOMONTH(@StartDate);
    
    SELECT 
        @Year AS Year,
        @Month AS Month,
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN Status IN ('CANCELLED', 'FAILED', 'REFUNDED') THEN 1 ELSE 0 END) AS UnsuccessfulTransfers,
        SUM(SendAmount) AS TotalSendVolume,
        SUM(ReceiveAmount) AS TotalReceiveVolume,
        SUM(TotalFees) AS TotalFeeRevenue,
        SUM(FXMargin) AS TotalFXRevenue,
        SUM(TotalFees) + SUM(FXMargin) AS TotalRevenue,
        AVG(SendAmount) AS AvgTransferAmount,
        COUNT(DISTINCT CustomerId) AS UniqueCustomers,
        COUNT(DISTINCT BeneficiaryId) AS UniqueBeneficiaries,
        COUNT(DISTINCT CorridorId) AS ActiveCorridors
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate;
END;
GO

-- Corridor performance report
CREATE PROCEDURE usp_GetCorridorPerformanceReport
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        c.CorridorId,
        c.CorridorCode,
        c.DisplayName,
        c.OriginCountry,
        c.DestinationCountry,
        COUNT(t.TransferId) AS TotalTransfers,
        SUM(CASE WHEN t.Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(t.SendAmount) AS TotalVolume,
        SUM(t.TotalFees) AS TotalFeeRevenue,
        SUM(t.FXMargin) AS TotalFXRevenue,
        AVG(t.SendAmount) AS AvgTransferAmount,
        COUNT(DISTINCT t.CustomerId) AS UniqueCustomers,
        AVG(DATEDIFF(MINUTE, t.CreatedAt, t.CompletedAt)) AS AvgCompletionMinutes
    FROM Corridors c
    LEFT JOIN Transfers t ON c.CorridorId = t.CorridorId
        AND CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    WHERE c.IsActive = 1
    GROUP BY c.CorridorId, c.CorridorCode, c.DisplayName, c.OriginCountry, c.DestinationCountry
    ORDER BY TotalVolume DESC;
END;
GO

-- Payout method analysis
CREATE PROCEDURE usp_GetPayoutMethodAnalysis
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        PayoutMethod,
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN Status = 'FAILED' THEN 1 ELSE 0 END) AS FailedTransfers,
        CAST(SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS DECIMAL) / 
            NULLIF(COUNT(*), 0) * 100 AS SuccessRate,
        SUM(SendAmount) AS TotalVolume,
        AVG(SendAmount) AS AvgTransferAmount,
        SUM(TotalFees) AS TotalFeeRevenue,
        AVG(DATEDIFF(MINUTE, CreatedAt, CompletedAt)) AS AvgCompletionMinutes
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY PayoutMethod
    ORDER BY TotalVolume DESC;
END;
GO

-- Customer acquisition report
CREATE PROCEDURE usp_GetCustomerAcquisitionReport
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    -- New registrations by day
    SELECT 
        CAST(CreatedAt AS DATE) AS RegistrationDate,
        COUNT(*) AS NewCustomers,
        SUM(CASE WHEN KYCStatus = 'VERIFIED' THEN 1 ELSE 0 END) AS VerifiedCustomers,
        SUM(CASE WHEN Status = 'ACTIVE' THEN 1 ELSE 0 END) AS ActiveCustomers
    FROM Customers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY CAST(CreatedAt AS DATE)
    ORDER BY RegistrationDate;
    
    -- By country
    SELECT 
        CountryOfResidence,
        COUNT(*) AS NewCustomers,
        SUM(CASE WHEN KYCStatus = 'VERIFIED' THEN 1 ELSE 0 END) AS VerifiedCustomers
    FROM Customers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY CountryOfResidence
    ORDER BY NewCustomers DESC;
    
    -- First transfer conversion
    SELECT 
        COUNT(DISTINCT c.CustomerId) AS TotalNewCustomers,
        COUNT(DISTINCT t.CustomerId) AS CustomersWithTransfer,
        CAST(COUNT(DISTINCT t.CustomerId) AS DECIMAL) / 
            NULLIF(COUNT(DISTINCT c.CustomerId), 0) * 100 AS ConversionRate
    FROM Customers c
    LEFT JOIN Transfers t ON c.CustomerId = t.CustomerId AND t.Status = 'COMPLETED'
    WHERE CAST(c.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate;
END;
GO

-- Customer lifetime value report
CREATE PROCEDURE usp_GetCustomerLifetimeValueReport
    @MinTransfers   INT = 1
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        c.CustomerId,
        c.Email,
        c.FirstName,
        c.LastName,
        c.CountryOfResidence,
        c.CreatedAt AS CustomerSince,
        COUNT(t.TransferId) AS TotalTransfers,
        SUM(t.SendAmount) AS TotalVolume,
        SUM(t.TotalFees) AS TotalFeesGenerated,
        SUM(t.FXMargin) AS TotalFXMarginGenerated,
        SUM(t.TotalFees) + SUM(t.FXMargin) AS TotalRevenueGenerated,
        AVG(t.SendAmount) AS AvgTransferAmount,
        MIN(t.CreatedAt) AS FirstTransferDate,
        MAX(t.CreatedAt) AS LastTransferDate,
        DATEDIFF(DAY, MIN(t.CreatedAt), MAX(t.CreatedAt)) AS ActiveDays
    FROM Customers c
    JOIN Transfers t ON c.CustomerId = t.CustomerId AND t.Status = 'COMPLETED'
    GROUP BY c.CustomerId, c.Email, c.FirstName, c.LastName, c.CountryOfResidence, c.CreatedAt
    HAVING COUNT(t.TransferId) >= @MinTransfers
    ORDER BY TotalRevenueGenerated DESC;
END;
GO

-- Compliance dashboard
CREATE PROCEDURE usp_GetComplianceDashboard
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Pending reviews
    SELECT 
        'Pending KYC Documents' AS Metric,
        COUNT(*) AS Count
    FROM CustomerDocuments WHERE VerificationStatus = 'PENDING'
    UNION ALL
    SELECT 
        'Flagged Transfers' AS Metric,
        COUNT(*) AS Count
    FROM Transfers WHERE ComplianceStatus = 'FLAGGED' AND Status NOT IN ('COMPLETED', 'CANCELLED', 'FAILED')
    UNION ALL
    SELECT 
        'Pending Compliance Screenings' AS Metric,
        COUNT(*) AS Count
    FROM ComplianceScreenings WHERE ResolutionStatus = 'PENDING_REVIEW'
    UNION ALL
    SELECT 
        'Draft SARs' AS Metric,
        COUNT(*) AS Count
    FROM SuspiciousActivityReports WHERE Status = 'DRAFT'
    UNION ALL
    SELECT 
        'High Risk Customers' AS Metric,
        COUNT(*) AS Count
    FROM Customers WHERE RiskLevel = 'HIGH' AND Status = 'ACTIVE';
    
    -- Recent flagged transfers
    SELECT TOP 10
        t.TransferId,
        t.TransferNumber,
        t.SendAmount,
        t.SendCurrency,
        t.Status,
        t.ComplianceStatus,
        t.RiskScore,
        t.CreatedAt,
        c.Email AS CustomerEmail,
        c.RiskLevel AS CustomerRiskLevel
    FROM Transfers t
    JOIN Customers c ON t.CustomerId = c.CustomerId
    WHERE t.ComplianceStatus = 'FLAGGED'
    ORDER BY t.CreatedAt DESC;
END;
GO

-- Revenue breakdown report
CREATE PROCEDURE usp_GetRevenueBreakdownReport
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    -- Overall revenue
    SELECT 
        SUM(TotalFees) AS TransferFeeRevenue,
        SUM(FXMargin) AS FXMarginRevenue,
        SUM(TotalFees) + SUM(FXMargin) AS TotalRevenue,
        SUM(PromoDiscount) AS TotalPromoDiscounts,
        COUNT(*) AS TotalTransfers,
        SUM(SendAmount) AS TotalVolume
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    AND Status = 'COMPLETED';
    
    -- By corridor
    SELECT 
        c.CorridorCode,
        c.DisplayName,
        SUM(t.TotalFees) AS TransferFeeRevenue,
        SUM(t.FXMargin) AS FXMarginRevenue,
        SUM(t.TotalFees) + SUM(t.FXMargin) AS TotalRevenue,
        COUNT(*) AS TransferCount
    FROM Transfers t
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    AND t.Status = 'COMPLETED'
    GROUP BY c.CorridorCode, c.DisplayName
    ORDER BY TotalRevenue DESC;
    
    -- By payout method
    SELECT 
        PayoutMethod,
        SUM(TotalFees) AS TransferFeeRevenue,
        SUM(FXMargin) AS FXMarginRevenue,
        SUM(TotalFees) + SUM(FXMargin) AS TotalRevenue,
        COUNT(*) AS TransferCount
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    AND Status = 'COMPLETED'
    GROUP BY PayoutMethod
    ORDER BY TotalRevenue DESC;
END;
GO

-- Partner performance report
CREATE PROCEDURE usp_GetPartnerPerformanceReport
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        p.PartnerId,
        p.PartnerCode,
        p.PartnerName,
        p.PartnerType,
        COUNT(t.TransferId) AS TotalTransfers,
        SUM(CASE WHEN t.Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN t.Status = 'FAILED' THEN 1 ELSE 0 END) AS FailedTransfers,
        CAST(SUM(CASE WHEN t.Status = 'COMPLETED' THEN 1 ELSE 0 END) AS DECIMAL) / 
            NULLIF(COUNT(t.TransferId), 0) * 100 AS SuccessRate,
        SUM(t.ReceiveAmount) AS TotalVolumePaidOut,
        AVG(DATEDIFF(MINUTE, t.SentToPartnerAt, t.CompletedAt)) AS AvgPayoutMinutes
    FROM PayoutPartners p
    LEFT JOIN Transfers t ON p.PartnerId = t.PayoutPartnerId
        AND CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    WHERE p.Status = 'ACTIVE'
    GROUP BY p.PartnerId, p.PartnerCode, p.PartnerName, p.PartnerType
    ORDER BY TotalVolumePaidOut DESC;
END;
GO

-- Transaction velocity report (for fraud detection)
CREATE PROCEDURE usp_GetTransactionVelocityReport
    @CustomerId BIGINT = NULL,
    @Hours      INT = 24
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CutoffTime DATETIME2 = DATEADD(HOUR, -@Hours, SYSUTCDATETIME());
    
    SELECT 
        c.CustomerId,
        c.Email,
        c.FirstName,
        c.LastName,
        c.RiskLevel,
        COUNT(t.TransferId) AS TransferCount,
        SUM(t.SendAmount) AS TotalVolume,
        COUNT(DISTINCT t.BeneficiaryId) AS UniqueBeneficiaries,
        COUNT(DISTINCT t.CorridorId) AS UniqueCorridors,
        MIN(t.CreatedAt) AS FirstTransferTime,
        MAX(t.CreatedAt) AS LastTransferTime
    FROM Customers c
    JOIN Transfers t ON c.CustomerId = t.CustomerId
    WHERE t.CreatedAt >= @CutoffTime
    AND (@CustomerId IS NULL OR c.CustomerId = @CustomerId)
    GROUP BY c.CustomerId, c.Email, c.FirstName, c.LastName, c.RiskLevel
    HAVING COUNT(t.TransferId) >= 3 OR SUM(t.SendAmount) >= 5000
    ORDER BY TotalVolume DESC;
END;
GO

-- Promo code usage report
CREATE PROCEDURE usp_GetPromoCodeUsageReport
    @StartDate  DATE = NULL,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @StartDate = COALESCE(@StartDate, DATEADD(MONTH, -1, CAST(SYSUTCDATETIME() AS DATE)));
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        p.PromoCodeId,
        p.Code,
        p.PromoName,
        p.DiscountType,
        p.DiscountValue,
        p.ValidFrom,
        p.ValidTo,
        p.MaxUsesTotal,
        p.CurrentUsageCount,
        COUNT(u.UsageId) AS UsageInPeriod,
        SUM(u.DiscountApplied) AS TotalDiscountGiven,
        SUM(t.SendAmount) AS TotalVolumeGenerated,
        SUM(t.TotalFees) AS FeeRevenueGenerated
    FROM PromoCodes p
    LEFT JOIN PromoCodeUsage u ON p.PromoCodeId = u.PromoCodeId
        AND CAST(u.UsedAt AS DATE) BETWEEN @StartDate AND @EndDate
    LEFT JOIN Transfers t ON u.TransferId = t.TransferId
    GROUP BY p.PromoCodeId, p.Code, p.PromoName, p.DiscountType, p.DiscountValue,
             p.ValidFrom, p.ValidTo, p.MaxUsesTotal, p.CurrentUsageCount
    ORDER BY UsageInPeriod DESC;
END;
GO

-- KYC funnel report
CREATE PROCEDURE usp_GetKYCFunnelReport
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        COUNT(*) AS TotalRegistrations,
        SUM(CASE WHEN KYCStatus = 'PENDING' THEN 1 ELSE 0 END) AS PendingKYC,
        SUM(CASE WHEN KYCStatus = 'VERIFIED' THEN 1 ELSE 0 END) AS VerifiedKYC,
        SUM(CASE WHEN KYCStatus = 'REJECTED' THEN 1 ELSE 0 END) AS RejectedKYC,
        CAST(SUM(CASE WHEN KYCStatus = 'VERIFIED' THEN 1 ELSE 0 END) AS DECIMAL) / 
            NULLIF(COUNT(*), 0) * 100 AS VerificationRate
    FROM Customers
    WHERE CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate;
    
    -- Documents submitted
    SELECT 
        DocumentType,
        COUNT(*) AS TotalSubmitted,
        SUM(CASE WHEN VerificationStatus = 'VERIFIED' THEN 1 ELSE 0 END) AS Approved,
        SUM(CASE WHEN VerificationStatus = 'REJECTED' THEN 1 ELSE 0 END) AS Rejected,
        SUM(CASE WHEN VerificationStatus = 'PENDING' THEN 1 ELSE 0 END) AS Pending,
        AVG(DATEDIFF(HOUR, SubmittedAt, VerifiedAt)) AS AvgReviewHours
    FROM CustomerDocuments
    WHERE CAST(SubmittedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY DocumentType
    ORDER BY TotalSubmitted DESC;
END;
GO

-- Geographic distribution report
CREATE PROCEDURE usp_GetGeographicDistributionReport
    @StartDate  DATE = NULL,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @StartDate = COALESCE(@StartDate, DATEADD(MONTH, -1, CAST(SYSUTCDATETIME() AS DATE)));
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    -- By origin country
    SELECT 
        c.OriginCountry,
        COUNT(t.TransferId) AS TransferCount,
        SUM(t.SendAmount) AS TotalVolume,
        SUM(t.TotalFees) AS TotalFees,
        COUNT(DISTINCT t.CustomerId) AS UniqueCustomers
    FROM Corridors c
    JOIN Transfers t ON c.CorridorId = t.CorridorId
    WHERE CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    AND t.Status = 'COMPLETED'
    GROUP BY c.OriginCountry
    ORDER BY TotalVolume DESC;
    
    -- By destination country
    SELECT 
        c.DestinationCountry,
        COUNT(t.TransferId) AS TransferCount,
        SUM(t.ReceiveAmount) AS TotalVolumePaid,
        COUNT(DISTINCT t.BeneficiaryId) AS UniqueBeneficiaries
    FROM Corridors c
    JOIN Transfers t ON c.CorridorId = t.CorridorId
    WHERE CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    AND t.Status = 'COMPLETED'
    GROUP BY c.DestinationCountry
    ORDER BY TotalVolumePaid DESC;
END;
GO

-- Hourly transaction pattern
CREATE PROCEDURE usp_GetHourlyTransactionPattern
    @Date   DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @Date = COALESCE(@Date, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        DATEPART(HOUR, CreatedAt) AS Hour,
        COUNT(*) AS TransferCount,
        SUM(SendAmount) AS Volume
    FROM Transfers
    WHERE CAST(CreatedAt AS DATE) = @Date
    GROUP BY DATEPART(HOUR, CreatedAt)
    ORDER BY Hour;
END;
GO

-- Failed transfer analysis
CREATE PROCEDURE usp_GetFailedTransferAnalysis
    @StartDate  DATE,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    -- By failure reason
    SELECT 
        StatusReason,
        COUNT(*) AS FailureCount,
        SUM(SendAmount) AS VolumeLost
    FROM Transfers
    WHERE Status = 'FAILED'
    AND CAST(CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY StatusReason
    ORDER BY FailureCount DESC;
    
    -- By corridor
    SELECT 
        c.CorridorCode,
        COUNT(*) AS FailureCount,
        CAST(COUNT(*) AS DECIMAL) / 
            NULLIF((SELECT COUNT(*) FROM Transfers t2 WHERE t2.CorridorId = c.CorridorId 
                    AND CAST(t2.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate), 0) * 100 AS FailureRate
    FROM Transfers t
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE t.Status = 'FAILED'
    AND CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY c.CorridorId, c.CorridorCode
    ORDER BY FailureCount DESC;
    
    -- By partner
    SELECT 
        p.PartnerCode,
        p.PartnerName,
        COUNT(*) AS FailureCount
    FROM Transfers t
    JOIN PayoutPartners p ON t.PayoutPartnerId = p.PartnerId
    WHERE t.Status = 'FAILED'
    AND CAST(t.CreatedAt AS DATE) BETWEEN @StartDate AND @EndDate
    GROUP BY p.PartnerId, p.PartnerCode, p.PartnerName
    ORDER BY FailureCount DESC;
END;
GO
