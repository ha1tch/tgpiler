# MoneySend: Procedure to Service Mapping

This document maps stored procedures to their corresponding gRPC services and methods, useful for verifying tgpiler's mapping accuracy.

## CustomerService (customer.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_RegisterCustomer | RegisterCustomer | Creates customer with country validation |
| usp_AuthenticateCustomer | AuthenticateCustomer | Login with lockout |
| usp_GetCustomerById | GetCustomerById | |
| usp_GetCustomerByExternalId | GetCustomerByExternalId | UUID lookup |
| usp_GetCustomerByEmail | GetCustomerByEmail | |
| usp_UpdateCustomerProfile | UpdateCustomerProfile | |
| usp_UpdateCustomerAddress | UpdateCustomerAddress | |
| usp_SuspendCustomer | SuspendCustomer | |
| usp_ReactivateCustomer | ReactivateCustomer | |
| usp_CloseCustomerAccount | CloseCustomerAccount | |
| usp_LockCustomerAccount | LockCustomerAccount | Security lock |
| usp_UnlockCustomerAccount | UnlockCustomerAccount | |
| usp_UpgradeCustomerTier | UpgradeCustomerTier | |
| usp_UpdateCustomerLimits | UpdateCustomerLimits | |
| usp_GetCustomerRiskProfile | GetCustomerRiskProfile | |
| usp_GetCustomerVerificationHistory | GetCustomerVerificationHistory | |
| usp_SearchCustomers | SearchCustomers | |
| usp_ListCustomersByKYCStatus | ListCustomersByKYCStatus | |
| usp_ListHighRiskCustomers | ListHighRiskCustomers | |
| usp_ChangeCustomerPassword | ChangePassword | |
| usp_RequestPasswordReset | RequestPasswordReset | |
| usp_VerifyCustomerPhone | VerifyCustomerPhone | |

## BeneficiaryService (beneficiary.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_CreateBeneficiary | CreateBeneficiary | |
| usp_GetBeneficiaryById | GetBeneficiaryById | |
| usp_GetBeneficiaryByExternalId | GetBeneficiaryByExternalId | |
| usp_UpdateBeneficiary | UpdateBeneficiary | |
| usp_DeactivateBeneficiary | DeactivateBeneficiary | Soft delete |
| usp_ReactivateBeneficiary | ReactivateBeneficiary | |
| usp_SetFavoriteBeneficiary | SetFavoriteBeneficiary | |
| usp_ListBeneficiariesByCustomer | ListBeneficiariesByCustomer | |
| usp_SearchBeneficiaries | SearchBeneficiaries | |
| usp_AddBeneficiaryBankAccount | AddBeneficiaryBankAccount | |
| usp_UpdateBeneficiaryBankAccount | UpdateBeneficiaryBankAccount | |
| usp_RemoveBeneficiaryBankAccount | RemoveBeneficiaryBankAccount | |
| usp_SetPrimaryBankAccount | SetPrimaryBankAccount | |
| usp_GetBankAccountsByBeneficiary | GetBankAccountsByBeneficiary | |
| usp_AddBeneficiaryMobileWallet | AddBeneficiaryMobileWallet | |
| usp_RemoveBeneficiaryMobileWallet | RemoveBeneficiaryMobileWallet | |
| usp_GetMobileWalletsByBeneficiary | GetMobileWalletsByBeneficiary | |
| usp_ScreenBeneficiary | ScreenBeneficiary | |
| usp_GetBeneficiaryScreeningStatus | GetBeneficiaryScreeningStatus | |
| usp_ListFlaggedBeneficiaries | ListFlaggedBeneficiaries | |
| usp_BlockBeneficiary | BlockBeneficiary | |
| usp_UnblockBeneficiary | UnblockBeneficiary | |

## TransferService (transfer.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_GetTransferQuote | GetTransferQuote | Rate + fees |
| usp_GetCurrentExchangeRate | GetCurrentExchangeRate | |
| usp_GetExchangeRateByCorridorCode | GetExchangeRateByCorridorCode | |
| usp_CalculateFees | CalculateFees | |
| usp_ValidatePromoCode | ValidatePromoCode | |
| usp_GetCorridorByCode | GetCorridorByCode | |
| usp_GetCorridorByCountries | GetCorridorByCountries | |
| usp_ListActiveCorridors | ListActiveCorridors | |
| usp_ListCorridorsByDestination | ListCorridorsByDestination | |
| usp_InitiateTransfer | InitiateTransfer | Main create |
| usp_GetTransferById | GetTransferById | |
| usp_GetTransferByNumber | GetTransferByNumber | Human-readable |
| usp_GetTransferByExternalId | GetTransferByExternalId | UUID |
| usp_ListTransfersByCustomer | ListTransfersByCustomer | |
| usp_ListTransfersByStatus | ListTransfersByStatus | |
| usp_GetTransferStatusHistory | GetTransferStatusHistory | Audit trail |
| usp_ConfirmPaymentReceived | ConfirmPayment | |
| usp_ClearTransferForProcessing | ProcessTransfer | |
| usp_SendToPayoutPartner | SendToPartner | |
| usp_CompleteTransfer | CompleteTransfer | |
| usp_CancelTransfer | CancelTransfer | |
| usp_ProcessRefund | RefundTransfer | |
| usp_FlagTransferForInvestigation | HoldTransfer | |
| usp_ClearTransferForProcessing | ReleaseTransferHold | Dual use |

## ComplianceService (compliance.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_ScreenTransferForCompliance | ScreenTransfer | |
| usp_SubmitForComplianceReview | ReviewTransfer | |
| usp_ClearTransferForProcessing | ApproveTransfer | |
| usp_BlockTransfer | BlockTransfer | |
| usp_GetTransfersPendingCompliance | ListFlaggedTransfers | |
| usp_AssessCustomerRisk | AssessCustomerRisk | |
| usp_GetCustomerRiskAssessments | GetCustomerRiskAssessments | |
| usp_ScreenCustomerSanctions | RunComplianceScreening | |
| usp_ScreenCustomerPEP | RunComplianceScreening | Same RPC, diff type |
| usp_ResolveComplianceScreening | ResolveScreeningMatch | |
| usp_GetPendingScreenings | GetPendingScreenings | |
| usp_GetScreeningHistoryByEntity | GetScreeningHistory | |
| usp_CreateSAR | CreateSAR | |
| usp_UpdateSAR | UpdateSAR | |
| usp_FileSAR | FileSAR | |
| usp_GetSARById | GetSARById | |
| usp_ListSARs | ListSARs | |

## KYCService (kyc.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_SubmitDocument | SubmitDocument | |
| usp_GetDocumentsByCustomer | GetDocumentsByCustomer | |
| usp_GetDocumentById | GetDocumentById | |
| usp_ApproveDocument | ApproveDocument | |
| usp_RejectDocument | RejectDocument | |
| usp_ListPendingKYCReviews | GetPendingKYCReviews | |
| usp_ListExpiringDocuments | ListExpiringDocuments | |
| usp_RequestDocumentResubmission | RequestDocumentResubmission | |

## PartnerService (partner.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_CreatePayoutPartner | CreatePayoutPartner | |
| usp_GetPartnerById | GetPartnerById | |
| usp_GetPartnerByCode | GetPartnerByCode | |
| usp_ListPayoutPartners | ListPayoutPartners | |
| usp_UpdatePayoutPartner | UpdatePayoutPartner | |
| usp_SuspendPayoutPartner | SuspendPayoutPartner | |
| usp_ReactivatePayoutPartner | ReactivatePayoutPartner | |
| usp_UpdatePartnerBalance | UpdatePartnerBalance | |
| usp_AddCorridorPartnerMapping | AddCorridorPartnerMapping | |
| usp_GetPartnersForCorridor | GetPartnersForCorridor | |
| usp_GetBestPartnerForTransfer | GetBestPartnerForTransfer | Routing |
| usp_CreateSettlement | CreateSettlement | |
| usp_GetSettlementById | GetSettlementById | |
| usp_ListSettlementsByPartner | ListSettlementsByPartner | |
| usp_MarkSettlementPaid | MarkSettlementPaid | |
| usp_ReconcileSettlement | ReconcileSettlement | |
| usp_GetPendingSettlements | GetPendingSettlements | |
| usp_GenerateDailySettlements | GenerateDailySettlements | Batch |
| usp_CheckPartnerCreditAvailability | CheckPartnerCreditAvailability | |

## NotificationService (notification.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_CreateNotificationFromTemplate | CreateNotificationFromTemplate | |
| usp_SendTransferInitiatedNotification | SendTransferInitiatedNotification | |
| usp_SendTransferCompletedNotification | SendTransferCompletedNotification | |
| usp_SendTransferFailedNotification | SendTransferFailedNotification | |
| usp_SendKYCReminderNotification | SendKYCReminderNotification | |
| usp_GetPendingNotifications | GetPendingNotifications | |
| usp_MarkNotificationSent | MarkNotificationSent | |
| usp_MarkNotificationDelivered | MarkNotificationDelivered | |
| usp_MarkNotificationFailed | MarkNotificationFailed | |
| usp_GetNotificationsByCustomer | GetNotificationsByCustomer | |
| usp_CreateNotificationTemplate | CreateNotificationTemplate | |
| usp_UpdateNotificationTemplate | UpdateNotificationTemplate | |
| usp_ListNotificationTemplates | ListNotificationTemplates | |
| usp_GetNotificationStats | GetNotificationStats | |
| usp_RetryFailedNotifications | RetryFailedNotifications | |

## ReportingService (reporting.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_GetDailyTransferSummary | GetDailyTransferSummary | |
| usp_GetWeeklyTransferSummary | GetWeeklyTransferSummary | |
| usp_GetMonthlyTransferSummary | GetMonthlyTransferSummary | |
| usp_GetCorridorPerformanceReport | GetCorridorPerformanceReport | |
| usp_GetPayoutMethodAnalysis | GetPayoutMethodAnalysis | |
| usp_GetCustomerAcquisitionReport | GetCustomerAcquisitionReport | |
| usp_GetCustomerLifetimeValueReport | GetCustomerLifetimeValueReport | |
| usp_GetComplianceDashboard | GetComplianceDashboard | |
| usp_GetRevenueBreakdownReport | GetRevenueBreakdownReport | |
| usp_GetPartnerPerformanceReport | GetPartnerPerformanceReport | |
| usp_GetTransactionVelocityReport | GetTransactionVelocityReport | |
| usp_GetPromoCodeUsageReport | GetPromoCodeUsageReport | |
| usp_GetKYCFunnelReport | GetKYCFunnelReport | |
| usp_GetGeographicDistributionReport | GetGeographicDistributionReport | |
| usp_GetHourlyTransactionPattern | GetHourlyTransactionPattern | |
| usp_GetFailedTransferAnalysis | GetFailedTransferAnalysis | |

## AgentService (agent.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_CreateAgent | CreateAgent | |
| usp_GetAgentById | GetAgentById | |
| usp_GetAgentByEmployeeId | GetAgentByEmployeeId | |
| usp_ListAgents | ListAgents | |
| usp_UpdateAgent | UpdateAgent | |
| usp_UpdateAgentPermissions | UpdateAgentPermissions | |
| usp_SuspendAgent | SuspendAgent | |
| usp_ReactivateAgent | ReactivateAgent | |
| usp_RecordAgentLogin | RecordAgentLogin | |
| usp_LogAgentActivity | LogAgentActivity | |
| usp_GetAgentActivityLog | GetAgentActivityLog | |
| usp_GetAgentPerformanceStats | GetAgentPerformanceStats | |
| usp_CheckAgentPermission | CheckAgentPermission | |
| usp_GetAgentsByPermission | GetAgentsByPermission | |
| usp_GetAgentWorkload | GetAgentWorkload | |

## ConfigService (config.proto)

| Procedure | RPC Method | Notes |
|-----------|------------|-------|
| usp_GetConfigValue | GetConfigValue | |
| usp_GetAllConfiguration | GetAllConfiguration | |
| usp_SetConfigValue | SetConfigValue | |
| usp_DeleteConfigValue | DeleteConfigValue | |
| usp_GetCountryConfiguration | GetCountryConfiguration | |
| usp_ListCountryConfigurations | ListCountryConfigurations | |
| usp_UpdateCountryConfiguration | UpdateCountryConfiguration | |
| usp_EnableCountryForSending | EnableCountryForSending | |
| usp_DisableCountryForSending | DisableCountryForSending | |
| usp_CreatePromoCode | CreatePromoCode | |
| usp_GetPromoCodeByCode | GetPromoCodeByCode | |
| usp_ListPromoCodes | ListPromoCodes | |
| usp_UpdatePromoCode | UpdatePromoCode | |
| usp_DeactivatePromoCode | DeactivatePromoCode | |
| usp_GetPromoCodeUsage | GetPromoCodeUsage | |
| usp_CheckPromoEligibility | CheckPromoEligibility | |
| usp_GenerateBulkPromoCodes | GenerateBulkPromoCodes | |
| usp_GetActivePromotionsForCustomer | GetActivePromotionsForCustomer | |

## Unmapped Procedures (Internal/Ledger)

These procedures are internal or ledger-specific and may not need direct RPC exposure:

| Procedure | Purpose |
|-----------|---------|
| usp_UpdateExchangeRate | Internal rate management |
| usp_GetRateHistory | Internal analytics |
| usp_GetFeeSchedule | Internal fee lookup |
| usp_UpdateTransferStatus | Internal state machine |
| usp_RetryFailedTransfer | Internal retry logic |
| usp_AddFundingSource | Customer funding (could be CustomerService) |
| usp_RecordChargeback | Payment dispute handling |
| usp_CreateLedgerAccount | Accounting setup |
| usp_RecordLedgerEntry | Double-entry bookkeeping |
| usp_RecordTransferAccounting | Transfer journal entries |
| usp_RecordPayoutAccounting | Payout journal entries |
| usp_GetLedgerEntriesByReference | Ledger queries |
| usp_GetTrialBalance | Accounting reports |
| usp_RunDailyReconciliation | Batch reconciliation |
| usp_BatchRiskAssessment | Batch processing |
