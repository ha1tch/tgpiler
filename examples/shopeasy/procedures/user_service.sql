-- ============================================================================
-- ShopEasy User Service Stored Procedures
-- ============================================================================

-- ============================================================================
-- usp_RegisterUser
-- Creates a new user account
-- ============================================================================
CREATE PROCEDURE usp_RegisterUser
    @Email NVARCHAR(255),
    @Username NVARCHAR(100),
    @PasswordHash NVARCHAR(255),
    @Salt NVARCHAR(100),
    @FirstName NVARCHAR(100) = NULL,
    @LastName NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Check if email already exists
    IF EXISTS (SELECT 1 FROM Users WHERE Email = @Email)
    BEGIN
        RAISERROR('Email already registered', 16, 1);
        RETURN;
    END
    
    -- Check if username already exists
    IF EXISTS (SELECT 1 FROM Users WHERE Username = @Username)
    BEGIN
        RAISERROR('Username already taken', 16, 1);
        RETURN;
    END
    
    DECLARE @UserId BIGINT;
    DECLARE @VerificationToken NVARCHAR(255) = NEWID();
    
    INSERT INTO Users (
        Email, Username, PasswordHash, Salt, 
        FirstName, LastName, EmailVerificationToken
    )
    VALUES (
        @Email, @Username, @PasswordHash, @Salt,
        @FirstName, @LastName, @VerificationToken
    );
    
    SET @UserId = SCOPE_IDENTITY();
    
    SELECT 
        u.Id, u.Email, u.Username, u.FirstName, u.LastName,
        u.Phone, u.IsActive, u.EmailVerified, u.CreatedAt, u.UpdatedAt,
        @VerificationToken AS ConfirmationToken
    FROM Users u
    WHERE u.Id = @UserId;
END
GO

-- ============================================================================
-- usp_Login
-- Authenticates a user and returns user data
-- ============================================================================
CREATE PROCEDURE usp_Login
    @Email NVARCHAR(255),
    @PasswordHash NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @UserId BIGINT;
    DECLARE @StoredHash NVARCHAR(255);
    DECLARE @IsActive BIT;
    DECLARE @LockoutEnd DATETIME2;
    DECLARE @FailedAttempts INT;
    
    SELECT 
        @UserId = Id,
        @StoredHash = PasswordHash,
        @IsActive = IsActive,
        @LockoutEnd = LockoutEnd,
        @FailedAttempts = FailedLoginAttempts
    FROM Users
    WHERE Email = @Email;
    
    -- User not found
    IF @UserId IS NULL
    BEGIN
        RAISERROR('Invalid email or password', 16, 1);
        RETURN;
    END
    
    -- Account locked
    IF @LockoutEnd IS NOT NULL AND @LockoutEnd > GETUTCDATE()
    BEGIN
        RAISERROR('Account is locked. Try again later.', 16, 1);
        RETURN;
    END
    
    -- Account inactive
    IF @IsActive = 0
    BEGIN
        RAISERROR('Account is deactivated', 16, 1);
        RETURN;
    END
    
    -- Password mismatch
    IF @StoredHash <> @PasswordHash
    BEGIN
        UPDATE Users
        SET FailedLoginAttempts = FailedLoginAttempts + 1,
            LockoutEnd = CASE 
                WHEN FailedLoginAttempts >= 4 THEN DATEADD(MINUTE, 15, GETUTCDATE())
                ELSE NULL
            END
        WHERE Id = @UserId;
        
        RAISERROR('Invalid email or password', 16, 1);
        RETURN;
    END
    
    -- Successful login
    UPDATE Users
    SET FailedLoginAttempts = 0,
        LockoutEnd = NULL,
        LastLoginAt = GETUTCDATE()
    WHERE Id = @UserId;
    
    SELECT 
        Id, Email, Username, FirstName, LastName, Phone,
        BillingAddressLine1, BillingAddressLine2, BillingCity, 
        BillingState, BillingPostalCode, BillingCountry,
        ShippingAddressLine1, ShippingAddressLine2, ShippingCity,
        ShippingState, ShippingPostalCode, ShippingCountry,
        IsActive, EmailVerified, CreatedAt, UpdatedAt
    FROM Users
    WHERE Id = @UserId;
END
GO

-- ============================================================================
-- usp_VerifyEmail
-- Verifies a user's email address
-- ============================================================================
CREATE PROCEDURE usp_VerifyEmail
    @Token NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @UserId BIGINT;
    
    SELECT @UserId = Id
    FROM Users
    WHERE EmailVerificationToken = @Token AND EmailVerified = 0;
    
    IF @UserId IS NULL
    BEGIN
        RAISERROR('Invalid or expired verification token', 16, 1);
        RETURN;
    END
    
    UPDATE Users
    SET EmailVerified = 1,
        EmailVerificationToken = NULL,
        UpdatedAt = GETUTCDATE()
    WHERE Id = @UserId;
    
    SELECT 1 AS Success;
END
GO

-- ============================================================================
-- usp_GetUserById
-- Retrieves a user by ID
-- ============================================================================
CREATE PROCEDURE usp_GetUserById
    @UserId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        Id, Email, Username, FirstName, LastName, Phone,
        BillingAddressLine1, BillingAddressLine2, BillingCity, 
        BillingState, BillingPostalCode, BillingCountry,
        ShippingAddressLine1, ShippingAddressLine2, ShippingCity,
        ShippingState, ShippingPostalCode, ShippingCountry,
        IsActive, EmailVerified, CreatedAt, UpdatedAt
    FROM Users
    WHERE Id = @UserId;
END
GO

-- ============================================================================
-- usp_GetUserByEmail
-- Retrieves a user by email
-- ============================================================================
CREATE PROCEDURE usp_GetUserByEmail
    @Email NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        Id, Email, Username, FirstName, LastName, Phone,
        BillingAddressLine1, BillingAddressLine2, BillingCity, 
        BillingState, BillingPostalCode, BillingCountry,
        ShippingAddressLine1, ShippingAddressLine2, ShippingCity,
        ShippingState, ShippingPostalCode, ShippingCountry,
        IsActive, EmailVerified, CreatedAt, UpdatedAt
    FROM Users
    WHERE Email = @Email;
END
GO

-- ============================================================================
-- usp_UpdateUser
-- Updates user profile information
-- ============================================================================
CREATE PROCEDURE usp_UpdateUser
    @UserId BIGINT,
    @FirstName NVARCHAR(100) = NULL,
    @LastName NVARCHAR(100) = NULL,
    @Phone NVARCHAR(50) = NULL,
    @BillingAddressLine1 NVARCHAR(255) = NULL,
    @BillingAddressLine2 NVARCHAR(255) = NULL,
    @BillingCity NVARCHAR(100) = NULL,
    @BillingState NVARCHAR(100) = NULL,
    @BillingPostalCode NVARCHAR(20) = NULL,
    @BillingCountry NVARCHAR(100) = NULL,
    @ShippingAddressLine1 NVARCHAR(255) = NULL,
    @ShippingAddressLine2 NVARCHAR(255) = NULL,
    @ShippingCity NVARCHAR(100) = NULL,
    @ShippingState NVARCHAR(100) = NULL,
    @ShippingPostalCode NVARCHAR(20) = NULL,
    @ShippingCountry NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Users
    SET FirstName = COALESCE(@FirstName, FirstName),
        LastName = COALESCE(@LastName, LastName),
        Phone = COALESCE(@Phone, Phone),
        BillingAddressLine1 = COALESCE(@BillingAddressLine1, BillingAddressLine1),
        BillingAddressLine2 = COALESCE(@BillingAddressLine2, BillingAddressLine2),
        BillingCity = COALESCE(@BillingCity, BillingCity),
        BillingState = COALESCE(@BillingState, BillingState),
        BillingPostalCode = COALESCE(@BillingPostalCode, BillingPostalCode),
        BillingCountry = COALESCE(@BillingCountry, BillingCountry),
        ShippingAddressLine1 = COALESCE(@ShippingAddressLine1, ShippingAddressLine1),
        ShippingAddressLine2 = COALESCE(@ShippingAddressLine2, ShippingAddressLine2),
        ShippingCity = COALESCE(@ShippingCity, ShippingCity),
        ShippingState = COALESCE(@ShippingState, ShippingState),
        ShippingPostalCode = COALESCE(@ShippingPostalCode, ShippingPostalCode),
        ShippingCountry = COALESCE(@ShippingCountry, ShippingCountry),
        UpdatedAt = GETUTCDATE()
    WHERE Id = @UserId;
    
    SELECT 
        Id, Email, Username, FirstName, LastName, Phone,
        BillingAddressLine1, BillingAddressLine2, BillingCity, 
        BillingState, BillingPostalCode, BillingCountry,
        ShippingAddressLine1, ShippingAddressLine2, ShippingCity,
        ShippingState, ShippingPostalCode, ShippingCountry,
        IsActive, EmailVerified, CreatedAt, UpdatedAt
    FROM Users
    WHERE Id = @UserId;
END
GO

-- ============================================================================
-- usp_ChangePassword
-- Changes a user's password
-- ============================================================================
CREATE PROCEDURE usp_ChangePassword
    @UserId BIGINT,
    @CurrentPasswordHash NVARCHAR(255),
    @NewPasswordHash NVARCHAR(255),
    @NewSalt NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @StoredHash NVARCHAR(255);
    
    SELECT @StoredHash = PasswordHash
    FROM Users
    WHERE Id = @UserId;
    
    IF @StoredHash IS NULL
    BEGIN
        RAISERROR('User not found', 16, 1);
        RETURN;
    END
    
    IF @StoredHash <> @CurrentPasswordHash
    BEGIN
        RAISERROR('Current password is incorrect', 16, 1);
        RETURN;
    END
    
    UPDATE Users
    SET PasswordHash = @NewPasswordHash,
        Salt = @NewSalt,
        UpdatedAt = GETUTCDATE()
    WHERE Id = @UserId;
    
    SELECT 1 AS Success;
END
GO

-- ============================================================================
-- usp_DeactivateUser
-- Deactivates a user account
-- ============================================================================
CREATE PROCEDURE usp_DeactivateUser
    @UserId BIGINT,
    @Reason NVARCHAR(MAX) = NULL,
    @DeactivatedBy BIGINT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Users
    SET IsActive = 0,
        UpdatedAt = GETUTCDATE()
    WHERE Id = @UserId;
    
    -- Log the deactivation
    INSERT INTO AuditLog (EntityType, EntityId, Action, NewValues, PerformedBy)
    VALUES ('User', @UserId, 'DEACTIVATE', @Reason, @DeactivatedBy);
    
    SELECT 1 AS Success;
END
GO

-- ============================================================================
-- usp_CreateRefreshToken
-- Creates a new refresh token for a user
-- ============================================================================
CREATE PROCEDURE usp_CreateRefreshToken
    @UserId BIGINT,
    @Token NVARCHAR(500),
    @ExpiresAt DATETIME2
AS
BEGIN
    SET NOCOUNT ON;
    
    INSERT INTO RefreshTokens (UserId, Token, ExpiresAt)
    VALUES (@UserId, @Token, @ExpiresAt);
    
    SELECT SCOPE_IDENTITY() AS Id;
END
GO

-- ============================================================================
-- usp_ValidateRefreshToken
-- Validates and returns refresh token data
-- ============================================================================
CREATE PROCEDURE usp_ValidateRefreshToken
    @Token NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT rt.Id, rt.UserId, rt.Token, rt.ExpiresAt, rt.CreatedAt, rt.RevokedAt,
           u.Email, u.Username, u.IsActive
    FROM RefreshTokens rt
    INNER JOIN Users u ON rt.UserId = u.Id
    WHERE rt.Token = @Token
      AND rt.RevokedAt IS NULL
      AND rt.ExpiresAt > GETUTCDATE()
      AND u.IsActive = 1;
END
GO

-- ============================================================================
-- usp_RevokeRefreshToken
-- Revokes a refresh token
-- ============================================================================
CREATE PROCEDURE usp_RevokeRefreshToken
    @Token NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE RefreshTokens
    SET RevokedAt = GETUTCDATE()
    WHERE Token = @Token AND RevokedAt IS NULL;
    
    SELECT @@ROWCOUNT AS RowsAffected;
END
GO
