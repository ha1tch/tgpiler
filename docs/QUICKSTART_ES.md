# Guía de Inicio Rápido

Pon tgpiler en funcionamiento en 5 minutos.

## Instalación

```bash
go install github.com/ha1tch/tgpiler/cmd/tgpiler@latest
```

O compila desde el código fuente:

```bash
git clone https://github.com/ha1tch/tgpiler.git
cd tgpiler
make build
```

## Uso Básico

### Transpilar un Stored Procedure

```bash
# Lógica procedural simple
tgpiler input.sql -o output.go

# Con operaciones de base de datos (SELECT, INSERT, UPDATE, DELETE)
tgpiler --dml input.sql -o output.go

# Especificar el dialecto de base de datos destino
tgpiler --dml --dialect=postgres input.sql -o output.go
```

### Ejemplo

**Input (T-SQL):**
```sql
CREATE PROCEDURE dbo.GetCustomerOrders
    @CustomerID INT,
    @Status VARCHAR(20) = 'Active'
AS
BEGIN
    SET NOCOUNT ON
    
    SELECT OrderID, Amount, OrderDate
    FROM Orders
    WHERE CustomerID = @CustomerID AND Status = @Status
    ORDER BY OrderDate DESC
END
```

**Output (Go):**
```go
func (r *Repository) GetCustomerOrders(ctx context.Context, customerID int32, status string) error {
    rows, err := r.db.QueryContext(ctx,
        "SELECT OrderID, Amount, OrderDate FROM Orders WHERE CustomerID = $1 AND Status = $2 ORDER BY OrderDate DESC",
        customerID, status)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var orderID int64
        var amount decimal.Decimal
        var orderDate time.Time
        if err := rows.Scan(&orderID, &amount, &orderDate); err != nil {
            return err
        }
        // Procesar row...
    }
    return rows.Err()
}
```

## Selección de Backend

```bash
# Backend SQL (default) - genera llamadas database/sql
tgpiler --dml --backend=sql input.sql

# Backend gRPC - genera llamadas de cliente gRPC
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql

# Backend mock - genera llamadas mock store (para testing)
tgpiler --dml --backend=mock input.sql
```

## Las Cuatro Acciones (Desarrollo Basado en Proto)

Si tienes archivos `.proto` que definen tus servicios gRPC:

```bash
# 1. Ver mappings entre procedures y métodos RPC
tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures

# 2. Generar implementaciones de repository
tgpiler --gen-impl --proto-dir ./protos --sql-dir ./procedures -o repo.go

# 3. Generar server stubs gRPC
tgpiler --gen-server --proto-dir ./protos -o server.go

# 4. Generar scaffolding de mock server
tgpiler --gen-mock --proto-dir ./protos -o mocks.go
```

## Opciones Comunes

| Flag | Descripción |
|------|-------------|
| `--dml` | Habilitar modo DML (SELECT, INSERT, UPDATE, DELETE) |
| `--dialect` | Dialecto destino: `postgres`, `mysql`, `sqlite`, `sqlserver` |
| `--backend` | Backend de output: `sql`, `grpc`, `mock`, `inline` |
| `-p, --pkg` | Nombre del package para el código generado |
| `-o, --output` | Archivo de output |
| `-f, --force` | Sobreescribir archivos existentes |
| `--annotate` | Agregar anotaciones al código (none, minimal, standard, verbose) |

## Manejo de Casos Especiales

### NEWID() (Generación de UUID)

```bash
# UUID en la aplicación (default)
tgpiler --dml --newid=app input.sql

# UUID en la base de datos
tgpiler --dml --newid=db input.sql

# Mock UUIDs para testing
tgpiler --dml --newid=mock input.sql
```

### Statements DDL

```bash
# Skip DDL con warnings (default)
tgpiler --dml input.sql

# Extraer DDL a archivo separado
tgpiler --dml --extract-ddl=migrations.sql input.sql

# Fallar en DDL
tgpiler --dml --strict-ddl input.sql
```

### Tablas Temporales con gRPC

```bash
# Las tablas temporales automáticamente hacen fallback a SQL
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql
# Muestra: info: Temp tables detected. Using --fallback-backend=sql (default).
```

## Siguientes Pasos

- [MANUAL.md](MANUAL.md) — Documentación de referencia completa
- [DML.md](DML.md) — Operaciones de base de datos y dialectos
- [GRPC.md](GRPC.md) — Backend gRPC y generación de proto
- [CHANGELOG.md](CHANGELOG.md) — Cambios recientes y features

## Ejemplos

El directorio `examples/` contiene ejemplos completos funcionales:

```
examples/
├── moneysend/      # Servicios financieros (222 procedures, 10 servicios)
│   ├── proto/      # Definiciones de servicio gRPC
│   ├── sql/        # Stored procedures T-SQL
│   └── generated/  # Código Go generado
└── shopeasy/       # E-commerce (6 servicios)
    ├── protos/
    ├── procedures/
    └── generated/
```

Para probarlos:

```bash
cd examples/moneysend

# Ver todos los mappings procedure-a-método
tgpiler --show-mappings --proto-dir proto --sql-dir sql

# Generar implementaciones
tgpiler --gen-impl --proto-dir proto --sql-dir sql -o repo.go
```
