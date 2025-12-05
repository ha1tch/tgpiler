-- Shred JSON array to rows using OPENJSON
CREATE PROCEDURE dbo.ShredJsonArray
    @JsonArray NVARCHAR(MAX)
AS
BEGIN
    -- Basic OPENJSON returns key, value, type columns
    SELECT 
        [key] AS ArrayIndex,
        [value] AS ItemValue,
        [type] AS JsonType
    FROM OPENJSON(@JsonArray)
    ORDER BY CAST([key] AS INT)
END
