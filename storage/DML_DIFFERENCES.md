# DML Syntax Differences Across SQL Dialects

## Quick Reference Table

| Feature | T-SQL | PostgreSQL | MySQL | SQLite | Oracle |
|---------|-------|------------|-------|--------|--------|
| Table alias AS | Optional | Optional | Optional | Optional | **FORBIDDEN** |
| Placeholder | `@p1` | `$1` | `?` | `?` | `:p1` |
| Quote identifier | `[name]` | `"name"` | `` `name` `` | `"name"` | `"name"` |
| String concat | `+` | `\|\|` | `CONCAT()` | `\|\|` | `\|\|` |
| Boolean literal | `1/0` | `TRUE/FALSE` | `1/0` | `1/0` | `1/0` |
| LIMIT syntax | `TOP n` (in SELECT) | `LIMIT n` | `LIMIT n` | `LIMIT n` | `FETCH FIRST n` |
| RETURNING | `OUTPUT` clause | `RETURNING` | N/A | `RETURNING` (3.35+) | `RETURNING INTO` (PL/SQL) |
| Null-safe equal | Complex OR | `IS NOT DISTINCT FROM` | `<=>` | `IS` | `DECODE()` |
| UPDATE JOIN | `FROM...JOIN` | `FROM...WHERE` | Direct JOIN | Subquery/FROM (3.33+) | Subquery/MERGE |
| DELETE JOIN | `FROM...JOIN` | `USING` | `FROM...JOIN` | Subquery | Subquery |
| UPSERT | `MERGE` | `ON CONFLICT` | `ON DUPLICATE KEY` | `ON CONFLICT` (3.24+) | `MERGE` |
| SELECT w/o FROM | OK | OK | OK | OK | Needs `DUAL` |

## Table Aliases

### T-SQL (SQL Server)
```sql
-- AS keyword is optional
SELECT t.col FROM Table t
SELECT t.col FROM Table AS t

-- Can use = for column aliases (old style, avoid)
SELECT col = t.column FROM Table t

-- UPDATE with alias
UPDATE t SET col = 1 FROM Table t WHERE t.id = 1
UPDATE t SET col = 1 FROM Table AS t WHERE t.id = 1
```

### PostgreSQL
```sql
-- AS keyword is optional for table aliases
SELECT t.col FROM table_name t
SELECT t.col FROM table_name AS t

-- AS is REQUIRED for column aliases in some contexts
SELECT column AS alias FROM table_name

-- UPDATE with alias - uses FROM clause
UPDATE table_name AS t SET col = 1 FROM other_table o WHERE t.id = o.id
-- Or without alias on target:
UPDATE table_name SET col = 1 FROM table_name t WHERE table_name.id = t.id
```

### MySQL
```sql
-- AS keyword is optional
SELECT t.col FROM table_name t
SELECT t.col FROM table_name AS t

-- UPDATE with alias - direct
UPDATE table_name t SET t.col = 1 WHERE t.id = 1

-- UPDATE with JOIN
UPDATE table_name t 
JOIN other_table o ON t.id = o.id
SET t.col = 1
```

### SQLite
```sql
-- AS keyword is optional
SELECT t.col FROM table_name t
SELECT t.col FROM table_name AS t

-- UPDATE with alias - NOT SUPPORTED directly
-- Must use subquery or UPDATE FROM (v3.33+)
UPDATE table_name SET col = 1 WHERE id IN (SELECT id FROM ...)

-- SQLite 3.33+ supports UPDATE FROM
UPDATE table_name SET col = o.val
FROM other_table o WHERE table_name.id = o.id
```

### Oracle
```sql
-- AS keyword is FORBIDDEN for table aliases!
SELECT t.col FROM table_name t    -- OK
SELECT t.col FROM table_name AS t -- ERROR!

-- AS is optional for column aliases
SELECT column alias FROM table_name
SELECT column AS alias FROM table_name

-- UPDATE with alias
UPDATE table_name t SET t.col = 1 WHERE t.id = 1

-- UPDATE with subquery (no direct JOIN)
UPDATE table_name t 
SET col = (SELECT val FROM other_table o WHERE o.id = t.id)
WHERE EXISTS (SELECT 1 FROM other_table o WHERE o.id = t.id)
```

---

## UPDATE with JOIN

### T-SQL
```sql
-- UPDATE ... FROM ... JOIN
UPDATE t
SET t.col = o.val
FROM Table t
INNER JOIN OtherTable o ON t.id = o.id
WHERE o.status = 'active'

-- Also supports table aliases directly
UPDATE Table
SET col = o.val
FROM Table t
INNER JOIN OtherTable o ON t.id = o.id
```

### PostgreSQL
```sql
-- UPDATE ... FROM (no JOIN keyword)
UPDATE table_name
SET col = o.val
FROM other_table o
WHERE table_name.id = o.id AND o.status = 'active'

-- With alias on target table (PostgreSQL 9.0+)
UPDATE table_name AS t
SET col = o.val
FROM other_table o
WHERE t.id = o.id
```

### MySQL
```sql
-- UPDATE with explicit JOIN
UPDATE table_name t
INNER JOIN other_table o ON t.id = o.id
SET t.col = o.val
WHERE o.status = 'active'

-- Multi-table UPDATE
UPDATE table_name t, other_table o
SET t.col = o.val
WHERE t.id = o.id AND o.status = 'active'
```

### SQLite
```sql
-- No direct UPDATE JOIN until 3.33.0
-- Use subquery:
UPDATE table_name
SET col = (SELECT val FROM other_table WHERE other_table.id = table_name.id)
WHERE id IN (SELECT id FROM other_table WHERE status = 'active')

-- SQLite 3.33+ UPDATE FROM:
UPDATE table_name
SET col = o.val
FROM other_table o
WHERE table_name.id = o.id
```

### Oracle
```sql
-- No UPDATE JOIN. Use MERGE or correlated subquery:
UPDATE table_name t
SET col = (SELECT val FROM other_table o WHERE o.id = t.id)
WHERE EXISTS (SELECT 1 FROM other_table o WHERE o.id = t.id AND o.status = 'active')

-- Or use MERGE
MERGE INTO table_name t
USING other_table o ON (t.id = o.id)
WHEN MATCHED THEN UPDATE SET t.col = o.val
```

---

## DELETE with JOIN

### T-SQL
```sql
-- DELETE ... FROM ... JOIN
DELETE t
FROM Table t
INNER JOIN OtherTable o ON t.id = o.foreign_id
WHERE o.status = 'inactive'

-- Alternative: DELETE with alias
DELETE FROM Table
FROM Table t
INNER JOIN OtherTable o ON t.id = o.foreign_id
WHERE o.status = 'inactive'
```

### PostgreSQL
```sql
-- DELETE ... USING
DELETE FROM table_name
USING other_table o
WHERE table_name.id = o.foreign_id AND o.status = 'inactive'

-- With alias (PostgreSQL 9.0+)
DELETE FROM table_name AS t
USING other_table o
WHERE t.id = o.foreign_id
```

### MySQL
```sql
-- DELETE with JOIN
DELETE t
FROM table_name t
INNER JOIN other_table o ON t.id = o.foreign_id
WHERE o.status = 'inactive'

-- Multi-table DELETE
DELETE t FROM table_name t, other_table o
WHERE t.id = o.foreign_id AND o.status = 'inactive'
```

### SQLite
```sql
-- No DELETE JOIN. Use subquery:
DELETE FROM table_name
WHERE id IN (
    SELECT t.id FROM table_name t
    JOIN other_table o ON t.id = o.foreign_id
    WHERE o.status = 'inactive'
)
```

### Oracle
```sql
-- No DELETE JOIN. Use subquery:
DELETE FROM table_name
WHERE id IN (
    SELECT t.id FROM table_name t
    JOIN other_table o ON t.id = o.foreign_id
    WHERE o.status = 'inactive'
)

-- Or EXISTS:
DELETE FROM table_name t
WHERE EXISTS (
    SELECT 1 FROM other_table o 
    WHERE o.foreign_id = t.id AND o.status = 'inactive'
)
```

---

## INSERT ... RETURNING / OUTPUT

### T-SQL
```sql
-- OUTPUT clause
INSERT INTO Table (col1, col2)
OUTPUT INSERTED.id, INSERTED.created_at
VALUES ('a', 'b')

-- OUTPUT INTO table variable
DECLARE @InsertedRows TABLE (id INT, created_at DATETIME)
INSERT INTO Table (col1, col2)
OUTPUT INSERTED.id, INSERTED.created_at INTO @InsertedRows
VALUES ('a', 'b')
```

### PostgreSQL
```sql
-- RETURNING clause
INSERT INTO table_name (col1, col2)
VALUES ('a', 'b')
RETURNING id, created_at

-- RETURNING with alias
INSERT INTO table_name (col1, col2)
VALUES ('a', 'b')
RETURNING id AS new_id
```

### MySQL
```sql
-- No RETURNING. Use LAST_INSERT_ID()
INSERT INTO table_name (col1, col2) VALUES ('a', 'b');
SELECT LAST_INSERT_ID();

-- Or in a single query with a trick (not recommended):
INSERT INTO table_name (col1, col2) VALUES ('a', 'b');
SELECT * FROM table_name WHERE id = LAST_INSERT_ID();
```

### SQLite
```sql
-- RETURNING clause (SQLite 3.35+)
INSERT INTO table_name (col1, col2)
VALUES ('a', 'b')
RETURNING id, created_at

-- Pre-3.35: use last_insert_rowid()
INSERT INTO table_name (col1, col2) VALUES ('a', 'b');
SELECT last_insert_rowid();
```

### Oracle
```sql
-- RETURNING INTO (requires variable)
INSERT INTO table_name (col1, col2)
VALUES ('a', 'b')
RETURNING id INTO v_id

-- PL/SQL context only. In plain SQL, use:
INSERT INTO table_name (col1, col2) VALUES ('a', 'b');
SELECT table_name_seq.CURRVAL FROM DUAL;
```

---

## UPSERT / MERGE

### T-SQL
```sql
-- MERGE statement
MERGE INTO Table AS target
USING (SELECT @id AS id, @val AS val) AS source
ON target.id = source.id
WHEN MATCHED THEN
    UPDATE SET target.val = source.val
WHEN NOT MATCHED THEN
    INSERT (id, val) VALUES (source.id, source.val);
```

### PostgreSQL
```sql
-- INSERT ... ON CONFLICT (9.5+)
INSERT INTO table_name (id, val)
VALUES (1, 'a')
ON CONFLICT (id) DO UPDATE SET val = EXCLUDED.val

-- ON CONFLICT DO NOTHING
INSERT INTO table_name (id, val)
VALUES (1, 'a')
ON CONFLICT (id) DO NOTHING

-- MERGE (PostgreSQL 15+)
MERGE INTO table_name AS target
USING source_table AS source
ON target.id = source.id
WHEN MATCHED THEN UPDATE SET val = source.val
WHEN NOT MATCHED THEN INSERT (id, val) VALUES (source.id, source.val);
```

### MySQL
```sql
-- INSERT ... ON DUPLICATE KEY UPDATE
INSERT INTO table_name (id, val)
VALUES (1, 'a')
ON DUPLICATE KEY UPDATE val = VALUES(val)

-- MySQL 8.0.19+: VALUES() deprecated, use alias
INSERT INTO table_name (id, val)
VALUES (1, 'a') AS new
ON DUPLICATE KEY UPDATE val = new.val

-- REPLACE (deletes + inserts)
REPLACE INTO table_name (id, val) VALUES (1, 'a')
```

### SQLite
```sql
-- INSERT ... ON CONFLICT (3.24+)
INSERT INTO table_name (id, val)
VALUES (1, 'a')
ON CONFLICT(id) DO UPDATE SET val = excluded.val

-- REPLACE (deletes + inserts)
REPLACE INTO table_name (id, val) VALUES (1, 'a')

-- INSERT OR REPLACE
INSERT OR REPLACE INTO table_name (id, val) VALUES (1, 'a')
```

### Oracle
```sql
-- MERGE statement
MERGE INTO table_name target
USING (SELECT 1 AS id, 'a' AS val FROM DUAL) source
ON (target.id = source.id)
WHEN MATCHED THEN
    UPDATE SET target.val = source.val
WHEN NOT MATCHED THEN
    INSERT (id, val) VALUES (source.id, source.val);
```

---

## String Concatenation

| Dialect    | Operator | Function |
|------------|----------|----------|
| T-SQL      | `+`      | `CONCAT()` |
| PostgreSQL | `\|\|`   | `CONCAT()` |
| MySQL      | N/A*     | `CONCAT()` |
| SQLite     | `\|\|`   | N/A |
| Oracle     | `\|\|`   | `CONCAT()` (2 args only) |

*MySQL `||` is OR by default. Use `CONCAT()` or enable `PIPES_AS_CONCAT`.

---

## LIMIT / TOP / FETCH

### T-SQL
```sql
-- TOP (at SELECT)
SELECT TOP 10 * FROM Table
SELECT TOP 10 PERCENT * FROM Table
SELECT TOP 10 WITH TIES * FROM Table ORDER BY col

-- OFFSET FETCH (2012+)
SELECT * FROM Table ORDER BY col
OFFSET 10 ROWS FETCH NEXT 10 ROWS ONLY
```

### PostgreSQL
```sql
-- LIMIT OFFSET
SELECT * FROM table_name LIMIT 10
SELECT * FROM table_name LIMIT 10 OFFSET 20

-- FETCH (SQL standard)
SELECT * FROM table_name ORDER BY col
OFFSET 10 ROWS FETCH FIRST 10 ROWS ONLY
```

### MySQL
```sql
-- LIMIT [OFFSET]
SELECT * FROM table_name LIMIT 10
SELECT * FROM table_name LIMIT 10 OFFSET 20
SELECT * FROM table_name LIMIT 20, 10  -- offset, count
```

### SQLite
```sql
-- LIMIT OFFSET
SELECT * FROM table_name LIMIT 10
SELECT * FROM table_name LIMIT 10 OFFSET 20
```

### Oracle
```sql
-- FETCH (12c+)
SELECT * FROM table_name ORDER BY col
FETCH FIRST 10 ROWS ONLY

SELECT * FROM table_name ORDER BY col
OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY

-- Pre-12c: ROWNUM
SELECT * FROM (
    SELECT t.*, ROWNUM rn FROM table_name t WHERE ROWNUM <= 30
) WHERE rn > 20
```

---

## Boolean Handling

| Dialect    | TRUE | FALSE | Storage |
|------------|------|-------|---------|
| T-SQL      | 1    | 0     | BIT |
| PostgreSQL | TRUE | FALSE | BOOLEAN |
| MySQL      | TRUE/1 | FALSE/0 | TINYINT(1) or BOOLEAN |
| SQLite     | 1    | 0     | INTEGER |
| Oracle     | 1*   | 0*    | NUMBER(1) |

*Oracle has no boolean type in SQL. Use 1/0 or 'Y'/'N'.

---

## CASE Sensitivity in Identifiers

| Dialect    | Default | Quoted |
|------------|---------|--------|
| T-SQL      | Case-insensitive | Case-sensitive with `"` |
| PostgreSQL | Lowercased | Preserved with `"` |
| MySQL      | Depends on OS/config | Preserved with `` ` `` |
| SQLite     | Case-insensitive | Case-sensitive with `"` |
| Oracle     | Uppercased | Preserved with `"` |

---

## NULL Handling in Comparisons

### T-SQL
```sql
-- ANSI_NULLS ON (default): NULL = NULL returns NULL
-- Use IS NULL / IS NOT NULL
SET ANSI_NULLS OFF  -- Then NULL = NULL returns TRUE (legacy)
```

### PostgreSQL
```sql
-- IS NOT DISTINCT FROM (null-safe equality)
WHERE col IS NOT DISTINCT FROM @val
```

### MySQL
```sql
-- <=> operator (null-safe equality)
WHERE col <=> @val
```

### SQLite
```sql
-- IS operator (null-safe equality)
WHERE col IS @val
```

### Oracle
```sql
-- DECODE or NVL
WHERE DECODE(col, val, 1, 0) = 1
WHERE NVL(col, 'NULL') = NVL(val, 'NULL')
```

---

## Implications for Detector

1. **Table alias detection**: Must handle with/without AS, and Oracle's prohibition of AS
2. **UPDATE with JOIN**: Different patterns per dialect - detect T-SQL pattern, generate per target
3. **DELETE with JOIN**: T-SQL uses FROM..JOIN, others use USING or subqueries
4. **RETURNING/OUTPUT**: Must detect OUTPUT clause in T-SQL, map to RETURNING or separate query
5. **String concatenation**: Detect `+` in T-SQL, convert to `||` or `CONCAT()`
6. **TOP/LIMIT**: Detect TOP in T-SQL, convert to LIMIT or FETCH
7. **Boolean literals**: Detect BIT comparisons, convert appropriately
8. **NULL comparisons**: Detect ANSI NULL handling, use dialect-specific null-safe operators

---

## T-SQL Specific Patterns to Detect

### Variable Assignment in SELECT
```sql
-- T-SQL allows this (not standard SQL)
SELECT @var = column FROM Table WHERE id = @id

-- Must convert to:
-- PostgreSQL/MySQL: SELECT column INTO var FROM...
-- Or: var := (SELECT column FROM...)
```

### Multiple Assignments
```sql
-- T-SQL
SELECT @a = col1, @b = col2 FROM Table WHERE id = @id

-- Convert to single row fetch into struct
```

### SELECT without FROM
```sql
-- T-SQL
SELECT @a = @b + 1

-- PostgreSQL: No FROM needed
SELECT 1 + 1

-- Oracle: Needs DUAL
SELECT 1 + 1 FROM DUAL
```

### IF EXISTS Pattern
```sql
-- T-SQL
IF EXISTS (SELECT 1 FROM Table WHERE col = @val)
BEGIN
    ...
END

-- PostgreSQL: Same
-- MySQL: Same or use SELECT EXISTS(...)
-- Oracle: Different pattern with COUNT or ROWNUM
```
