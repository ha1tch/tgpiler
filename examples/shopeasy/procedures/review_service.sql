-- ============================================================================
-- ShopEasy Review Service Stored Procedures
-- ============================================================================

-- ============================================================================
-- usp_CreateReview
-- Creates a new product review
-- ============================================================================
CREATE PROCEDURE usp_CreateReview
    @ProductId BIGINT,
    @UserId BIGINT,
    @Rating INT,
    @Title NVARCHAR(255) = NULL,
    @Content NVARCHAR(MAX) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Validate rating
    IF @Rating < 1 OR @Rating > 5
    BEGIN
        RAISERROR('Rating must be between 1 and 5', 16, 1);
        RETURN;
    END
    
    -- Check if user already reviewed this product
    IF EXISTS (SELECT 1 FROM Reviews WHERE ProductId = @ProductId AND UserId = @UserId)
    BEGIN
        RAISERROR('You have already reviewed this product', 16, 1);
        RETURN;
    END
    
    -- Check if verified purchase
    DECLARE @VerifiedPurchase BIT = 0;
    IF EXISTS (
        SELECT 1 FROM Orders o
        INNER JOIN OrderItems oi ON o.Id = oi.OrderId
        WHERE o.UserId = @UserId 
          AND oi.ProductId = @ProductId
          AND o.Status = 'DELIVERED'
    )
        SET @VerifiedPurchase = 1;
    
    DECLARE @ReviewId BIGINT;
    
    INSERT INTO Reviews (ProductId, UserId, Rating, Title, Content, VerifiedPurchase)
    VALUES (@ProductId, @UserId, @Rating, @Title, @Content, @VerifiedPurchase);
    
    SET @ReviewId = SCOPE_IDENTITY();
    
    -- Update product rating (only approved reviews count, but we include pending for now)
    EXEC usp_UpdateProductRating @ProductId;
    
    -- Return the review
    EXEC usp_GetReviewById @ReviewId;
END
GO

-- ============================================================================
-- usp_GetReviewById
-- Retrieves a review by ID
-- ============================================================================
CREATE PROCEDURE usp_GetReviewById
    @ReviewId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        r.Id, r.ProductId, r.UserId,
        u.Username AS UserName,
        r.Rating, r.Title, r.Content, r.Status,
        r.VerifiedPurchase, r.HelpfulVotes, r.UnhelpfulVotes,
        r.CreatedAt, r.UpdatedAt
    FROM Reviews r
    INNER JOIN Users u ON r.UserId = u.Id
    WHERE r.Id = @ReviewId;
END
GO

-- ============================================================================
-- usp_ListReviews
-- Lists reviews for a product with filtering and pagination
-- ============================================================================
CREATE PROCEDURE usp_ListReviews
    @ProductId BIGINT,
    @MinRating INT = NULL,
    @MaxRating INT = NULL,
    @VerifiedOnly BIT = 0,
    @SortBy NVARCHAR(50) = 'date',
    @SortDescending BIT = 1,
    @PageSize INT = 20,
    @PageToken NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = 0;
    IF @PageToken IS NOT NULL
        SET @Offset = TRY_CAST(@PageToken AS INT);
    
    -- Get rating summary
    SELECT 
        @ProductId AS ProductId,
        AVG(CAST(Rating AS FLOAT)) AS AverageRating,
        COUNT(*) AS TotalReviews,
        SUM(CASE WHEN Rating = 5 THEN 1 ELSE 0 END) AS FiveStarCount,
        SUM(CASE WHEN Rating = 4 THEN 1 ELSE 0 END) AS FourStarCount,
        SUM(CASE WHEN Rating = 3 THEN 1 ELSE 0 END) AS ThreeStarCount,
        SUM(CASE WHEN Rating = 2 THEN 1 ELSE 0 END) AS TwoStarCount,
        SUM(CASE WHEN Rating = 1 THEN 1 ELSE 0 END) AS OneStarCount
    FROM Reviews
    WHERE ProductId = @ProductId AND Status = 'APPROVED';
    
    -- Get total count
    SELECT COUNT(*) AS TotalCount
    FROM Reviews
    WHERE ProductId = @ProductId
      AND Status = 'APPROVED'
      AND (@MinRating IS NULL OR Rating >= @MinRating)
      AND (@MaxRating IS NULL OR Rating <= @MaxRating)
      AND (@VerifiedOnly = 0 OR VerifiedPurchase = 1);
    
    -- Get reviews
    SELECT 
        r.Id, r.ProductId, r.UserId,
        u.Username AS UserName,
        r.Rating, r.Title, r.Content, r.Status,
        r.VerifiedPurchase, r.HelpfulVotes, r.UnhelpfulVotes,
        r.CreatedAt, r.UpdatedAt
    FROM Reviews r
    INNER JOIN Users u ON r.UserId = u.Id
    WHERE r.ProductId = @ProductId
      AND r.Status = 'APPROVED'
      AND (@MinRating IS NULL OR r.Rating >= @MinRating)
      AND (@MaxRating IS NULL OR r.Rating <= @MaxRating)
      AND (@VerifiedOnly = 0 OR r.VerifiedPurchase = 1)
    ORDER BY 
        CASE WHEN @SortBy = 'date' AND @SortDescending = 0 THEN r.CreatedAt END ASC,
        CASE WHEN @SortBy = 'date' AND @SortDescending = 1 THEN r.CreatedAt END DESC,
        CASE WHEN @SortBy = 'rating' AND @SortDescending = 0 THEN r.Rating END ASC,
        CASE WHEN @SortBy = 'rating' AND @SortDescending = 1 THEN r.Rating END DESC,
        CASE WHEN @SortBy = 'helpful' AND @SortDescending = 0 THEN r.HelpfulVotes END ASC,
        CASE WHEN @SortBy = 'helpful' AND @SortDescending = 1 THEN r.HelpfulVotes END DESC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_ListUserReviews
-- Lists reviews by a specific user
-- ============================================================================
CREATE PROCEDURE usp_ListUserReviews
    @UserId BIGINT,
    @PageSize INT = 20,
    @PageToken NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = 0;
    IF @PageToken IS NOT NULL
        SET @Offset = TRY_CAST(@PageToken AS INT);
    
    -- Get total count
    SELECT COUNT(*) AS TotalCount
    FROM Reviews
    WHERE UserId = @UserId;
    
    -- Get reviews
    SELECT 
        r.Id, r.ProductId, p.Name AS ProductName, r.UserId,
        u.Username AS UserName,
        r.Rating, r.Title, r.Content, r.Status,
        r.VerifiedPurchase, r.HelpfulVotes, r.UnhelpfulVotes,
        r.CreatedAt, r.UpdatedAt
    FROM Reviews r
    INNER JOIN Users u ON r.UserId = u.Id
    INNER JOIN Products p ON r.ProductId = p.Id
    WHERE r.UserId = @UserId
    ORDER BY r.CreatedAt DESC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_UpdateReview
-- Updates an existing review
-- ============================================================================
CREATE PROCEDURE usp_UpdateReview
    @ReviewId BIGINT,
    @UserId BIGINT,
    @Rating INT = NULL,
    @Title NVARCHAR(255) = NULL,
    @Content NVARCHAR(MAX) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Verify ownership
    IF NOT EXISTS (SELECT 1 FROM Reviews WHERE Id = @ReviewId AND UserId = @UserId)
    BEGIN
        RAISERROR('Review not found or unauthorized', 16, 1);
        RETURN;
    END
    
    -- Validate rating if provided
    IF @Rating IS NOT NULL AND (@Rating < 1 OR @Rating > 5)
    BEGIN
        RAISERROR('Rating must be between 1 and 5', 16, 1);
        RETURN;
    END
    
    DECLARE @ProductId BIGINT;
    SELECT @ProductId = ProductId FROM Reviews WHERE Id = @ReviewId;
    
    UPDATE Reviews
    SET Rating = COALESCE(@Rating, Rating),
        Title = COALESCE(@Title, Title),
        Content = COALESCE(@Content, Content),
        Status = 'PENDING',  -- Re-submit for moderation
        UpdatedAt = GETUTCDATE()
    WHERE Id = @ReviewId;
    
    -- Update product rating
    EXEC usp_UpdateProductRating @ProductId;
    
    -- Return updated review
    EXEC usp_GetReviewById @ReviewId;
END
GO

-- ============================================================================
-- usp_DeleteReview
-- Deletes a review
-- ============================================================================
CREATE PROCEDURE usp_DeleteReview
    @ReviewId BIGINT,
    @UserId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @ProductId BIGINT;
    DECLARE @ReviewUserId BIGINT;
    
    SELECT @ProductId = ProductId, @ReviewUserId = UserId
    FROM Reviews WHERE Id = @ReviewId;
    
    -- Allow deletion by owner only (admin check would be additional)
    IF @ReviewUserId <> @UserId
    BEGIN
        RAISERROR('Unauthorized to delete this review', 16, 1);
        RETURN;
    END
    
    DELETE FROM Reviews WHERE Id = @ReviewId;
    
    -- Update product rating
    EXEC usp_UpdateProductRating @ProductId;
    
    SELECT 1 AS Success;
END
GO

-- ============================================================================
-- usp_VoteReview
-- Votes a review as helpful or not helpful
-- ============================================================================
CREATE PROCEDURE usp_VoteReview
    @ReviewId BIGINT,
    @UserId BIGINT,
    @Helpful BIT
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Check if user already voted
    DECLARE @ExistingVote BIT;
    SELECT @ExistingVote = IsHelpful 
    FROM ReviewVotes 
    WHERE ReviewId = @ReviewId AND UserId = @UserId;
    
    IF @ExistingVote IS NOT NULL
    BEGIN
        IF @ExistingVote = @Helpful
        BEGIN
            -- Same vote, remove it
            DELETE FROM ReviewVotes WHERE ReviewId = @ReviewId AND UserId = @UserId;
            
            IF @Helpful = 1
                UPDATE Reviews SET HelpfulVotes = HelpfulVotes - 1 WHERE Id = @ReviewId;
            ELSE
                UPDATE Reviews SET UnhelpfulVotes = UnhelpfulVotes - 1 WHERE Id = @ReviewId;
        END
        ELSE
        BEGIN
            -- Different vote, change it
            UPDATE ReviewVotes SET IsHelpful = @Helpful WHERE ReviewId = @ReviewId AND UserId = @UserId;
            
            IF @Helpful = 1
            BEGIN
                UPDATE Reviews SET HelpfulVotes = HelpfulVotes + 1, UnhelpfulVotes = UnhelpfulVotes - 1 WHERE Id = @ReviewId;
            END
            ELSE
            BEGIN
                UPDATE Reviews SET HelpfulVotes = HelpfulVotes - 1, UnhelpfulVotes = UnhelpfulVotes + 1 WHERE Id = @ReviewId;
            END
        END
    END
    ELSE
    BEGIN
        -- New vote
        INSERT INTO ReviewVotes (ReviewId, UserId, IsHelpful)
        VALUES (@ReviewId, @UserId, @Helpful);
        
        IF @Helpful = 1
            UPDATE Reviews SET HelpfulVotes = HelpfulVotes + 1 WHERE Id = @ReviewId;
        ELSE
            UPDATE Reviews SET UnhelpfulVotes = UnhelpfulVotes + 1 WHERE Id = @ReviewId;
    END
    
    SELECT HelpfulVotes, UnhelpfulVotes
    FROM Reviews WHERE Id = @ReviewId;
END
GO

-- ============================================================================
-- usp_GetProductRatingSummary
-- Gets rating summary for a product
-- ============================================================================
CREATE PROCEDURE usp_GetProductRatingSummary
    @ProductId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        @ProductId AS ProductId,
        COALESCE(AVG(CAST(Rating AS FLOAT)), 0) AS AverageRating,
        COUNT(*) AS TotalReviews,
        SUM(CASE WHEN Rating = 5 THEN 1 ELSE 0 END) AS FiveStarCount,
        SUM(CASE WHEN Rating = 4 THEN 1 ELSE 0 END) AS FourStarCount,
        SUM(CASE WHEN Rating = 3 THEN 1 ELSE 0 END) AS ThreeStarCount,
        SUM(CASE WHEN Rating = 2 THEN 1 ELSE 0 END) AS TwoStarCount,
        SUM(CASE WHEN Rating = 1 THEN 1 ELSE 0 END) AS OneStarCount
    FROM Reviews
    WHERE ProductId = @ProductId AND Status = 'APPROVED';
END
GO

-- ============================================================================
-- usp_ModerateReview
-- Approves or rejects a review (admin operation)
-- ============================================================================
CREATE PROCEDURE usp_ModerateReview
    @ReviewId BIGINT,
    @Status NVARCHAR(50),
    @RejectionReason NVARCHAR(MAX) = NULL,
    @ModeratedBy BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF @Status NOT IN ('APPROVED', 'REJECTED')
    BEGIN
        RAISERROR('Status must be APPROVED or REJECTED', 16, 1);
        RETURN;
    END
    
    DECLARE @ProductId BIGINT;
    SELECT @ProductId = ProductId FROM Reviews WHERE Id = @ReviewId;
    
    UPDATE Reviews
    SET Status = @Status,
        RejectionReason = @RejectionReason,
        ModeratedBy = @ModeratedBy,
        ModeratedAt = GETUTCDATE(),
        UpdatedAt = GETUTCDATE()
    WHERE Id = @ReviewId;
    
    -- Update product rating
    EXEC usp_UpdateProductRating @ProductId;
    
    -- Return moderated review
    EXEC usp_GetReviewById @ReviewId;
END
GO

-- ============================================================================
-- usp_ListPendingReviews
-- Lists reviews pending moderation
-- ============================================================================
CREATE PROCEDURE usp_ListPendingReviews
    @PageSize INT = 20,
    @PageToken NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = 0;
    IF @PageToken IS NOT NULL
        SET @Offset = TRY_CAST(@PageToken AS INT);
    
    -- Get total count
    SELECT COUNT(*) AS TotalCount
    FROM Reviews
    WHERE Status = 'PENDING';
    
    -- Get reviews
    SELECT 
        r.Id, r.ProductId, p.Name AS ProductName, r.UserId,
        u.Username AS UserName,
        r.Rating, r.Title, r.Content, r.Status,
        r.VerifiedPurchase, r.CreatedAt
    FROM Reviews r
    INNER JOIN Users u ON r.UserId = u.Id
    INNER JOIN Products p ON r.ProductId = p.Id
    WHERE r.Status = 'PENDING'
    ORDER BY r.CreatedAt ASC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_UpdateProductRating
-- Helper procedure to update product's aggregate rating
-- ============================================================================
CREATE PROCEDURE usp_UpdateProductRating
    @ProductId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @AvgRating DECIMAL(3,2);
    DECLARE @ReviewCount INT;
    
    SELECT 
        @AvgRating = COALESCE(AVG(CAST(Rating AS DECIMAL(3,2))), 0),
        @ReviewCount = COUNT(*)
    FROM Reviews
    WHERE ProductId = @ProductId AND Status = 'APPROVED';
    
    UPDATE Products
    SET AverageRating = @AvgRating,
        ReviewCount = @ReviewCount,
        UpdatedAt = GETUTCDATE()
    WHERE Id = @ProductId;
END
GO
