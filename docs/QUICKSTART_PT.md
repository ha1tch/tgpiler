# Guia de Início Rápido

Coloque o tgpiler em funcionamento em 5 minutos.

## Instalação

```bash
go install github.com/ha1tch/tgpiler/cmd/tgpiler@latest
```

Ou compile a partir do código-fonte:

```bash
git clone https://github.com/ha1tch/tgpiler.git
cd tgpiler
make build
```

## Uso Básico

### Transpilar um Stored Procedure

```bash
# Lógica procedural simples
tgpiler input.sql -o output.go

# Com operações de banco de dados (SELECT, INSERT, UPDATE, DELETE)
tgpiler --dml input.sql -o output.go

# Especificar dialeto do banco de dados de destino
tgpiler --dml --dialect=postgres input.sql -o output.go
```

### Exemplo

**Entrada (T-SQL):**
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

**Saída (Go):**
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
        // Processar linha...
    }
    return rows.Err()
}
```

## Seleção de Backend

```bash
# Backend SQL (padrão) - gera chamadas database/sql
tgpiler --dml --backend=sql input.sql

# Backend gRPC - gera chamadas de cliente gRPC
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql

# Backend Mock - gera chamadas de mock store (para testes)
tgpiler --dml --backend=mock input.sql
```

## As Quatro Ações (Desenvolvimento Baseado em Proto)

Se você tem arquivos `.proto` definindo seus serviços gRPC:

```bash
# 1. Visualizar mapeamentos entre procedures e métodos RPC
tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures

# 2. Gerar implementações de repositório
tgpiler --gen-impl --proto-dir ./protos --sql-dir ./procedures -o repo.go

# 3. Gerar stubs de servidor gRPC
tgpiler --gen-server --proto-dir ./protos -o server.go

# 4. Gerar scaffolding de servidor mock
tgpiler --gen-mock --proto-dir ./protos -o mocks.go
```

## Opções Comuns

| Flag | Descrição |
|------|-----------|
| `--dml` | Habilitar modo DML (SELECT, INSERT, UPDATE, DELETE) |
| `--dialect` | Dialeto de destino: `postgres`, `mysql`, `sqlite`, `sqlserver` |
| `--backend` | Backend de saída: `sql`, `grpc`, `mock`, `inline` |
| `-p, --pkg` | Nome do pacote para código gerado |
| `-o, --output` | Arquivo de saída |
| `-f, --force` | Sobrescrever arquivos existentes |
| `--annotate` | Adicionar anotações ao código (none, minimal, standard, verbose) |

## Tratamento de Casos Especiais

### NEWID() (Geração de UUID)

```bash
# UUID no lado da aplicação (padrão)
tgpiler --dml --newid=app input.sql

# UUID no lado do banco de dados
tgpiler --dml --newid=db input.sql

# UUIDs mock para testes
tgpiler --dml --newid=mock input.sql
```

### Instruções DDL

```bash
# Pular DDL com avisos (padrão)
tgpiler --dml input.sql

# Extrair DDL para arquivo separado
tgpiler --dml --extract-ddl=migrations.sql input.sql

# Falhar em DDL
tgpiler --dml --strict-ddl input.sql
```

### Tabelas Temporárias com gRPC

```bash
# Tabelas temporárias automaticamente usam fallback para SQL
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql
# Exibe: info: Temp tables detected. Using --fallback-backend=sql (default).
```

## Próximos Passos

- [MANUAL.md](MANUAL.md) — Documentação de referência completa
- [DML.md](DML.md) — Operações de banco de dados e dialetos
- [GRPC.md](GRPC.md) — Backend gRPC e geração de proto
- [CHANGELOG.md](CHANGELOG.md) — Mudanças recentes e funcionalidades

## Exemplos

O diretório `examples/` contém exemplos completos funcionais:

```
examples/
├── moneysend/      # Serviços financeiros (222 procedures, 10 serviços)
│   ├── proto/      # Definições de serviço gRPC
│   ├── sql/        # Stored procedures T-SQL
│   └── generated/  # Código Go gerado
└── shopeasy/       # E-commerce (6 serviços)
    ├── protos/
    ├── procedures/
    └── generated/
```

Experimente:

```bash
cd examples/moneysend

# Visualizar todos os mapeamentos procedure-para-método
tgpiler --show-mappings --proto-dir proto --sql-dir sql

# Gerar implementações
tgpiler --gen-impl --proto-dir proto --sql-dir sql -o repo.go
```
