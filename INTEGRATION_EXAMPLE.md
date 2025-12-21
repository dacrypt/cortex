# Ejemplo Completo: TypeScript Frontend + Go Backend

## Resumen

Sí, **TypeScript puede ser el frontend y comunicarse con un backend Go empaquetado** con la extensión. Esta es una arquitectura muy común y potente.

## Arquitectura

```
┌─────────────────────────────────────────┐
│  VSCode Extension (TypeScript)          │
│  ┌───────────────────────────────────┐ │
│  │  UI / Commands / Tree Views       │ │
│  └──────────────┬────────────────────┘ │
│  ┌──────────────▼────────────────────┐ │
│  │  GoBackend Client (TypeScript)     │ │
│  └──────────────┬────────────────────┘ │
└─────────────────┼───────────────────────┘
                  │
                  │ stdin/stdout (JSON-RPC)
                  │ o HTTP
                  │
┌─────────────────▼───────────────────────┐
│  Backend Go (Binario empaquetado)       │
│  - Procesamiento pesado                 │
│  - Análisis de código                   │
│  - Operaciones de archivos              │
│  - Cálculos complejos                   │
└─────────────────────────────────────────┘
```

## Archivos Creados

### 1. Clientes TypeScript
- **`src/core/GoBackend.ts`** - Cliente para comunicación stdin/stdout (JSON-RPC)
- **`src/core/GoBackendHTTP.ts`** - Cliente para comunicación HTTP

### 2. Backend Go
- **`examples/go-backend/main.go`** - Backend usando stdin/stdout
- **`examples/go-backend/http-server.go`** - Backend usando HTTP

### 3. Utilidades
- **`build-go.sh`** - Script para compilar binarios para todas las plataformas
- **`src/commands/integrateGoBackend.ts`** - Ejemplo de integración

## Pasos para Implementar

### Paso 1: Compilar Binarios Go

```bash
# Hacer el script ejecutable
chmod +x build-go.sh

# Compilar para todas las plataformas
./build-go.sh
```

Esto creará binarios en `bin/`:
- `cortex-backend-linux-amd64`
- `cortex-backend-darwin-amd64`
- `cortex-backend-darwin-arm64`
- `cortex-backend-windows-amd64.exe`

### Paso 2: Actualizar package.json

Agrega los binarios a la lista de archivos a empaquetar:

```json
{
  "files": [
    "out",
    "resources",
    "bin/**/*"
  ]
}
```

### Paso 3: Integrar en extension.ts

```typescript
import { initializeGoBackend } from './commands/integrateGoBackend';

export async function activate(context: vscode.ExtensionContext) {
  // ... código existente ...

  // Inicializar backend Go
  await initializeGoBackend(context);

  // Registrar comandos que usan el backend
  vscode.commands.registerCommand(
    'cortex.processFileWithGo',
    () => processFileWithGo(context)
  );

  vscode.commands.registerCommand(
    'cortex.analyzeCodeWithGo',
    () => analyzeCodeWithGo(context)
  );
}
```

## Ejemplo de Uso

### Desde un Comando TypeScript:

```typescript
import { GoBackend } from './core/GoBackend';

const backend = new GoBackend(context, {
  binariesPath: 'bin',
  binaryName: 'cortex-backend',
});

// Iniciar backend
await backend.start();

// Llamar método del backend
const result = await backend.call('processFile', {
  filePath: '/path/to/file.txt',
});

console.log(result);
// { filePath: '/path/to/file.txt', size: 1024, processed: true }
```

## Comunicación

### Protocolo JSON-RPC (stdin/stdout)

**Request (TypeScript → Go):**
```json
{
  "id": "req_1",
  "method": "processFile",
  "params": {
    "filePath": "/path/to/file.txt"
  }
}
```

**Response (Go → TypeScript):**
```json
{
  "id": "req_1",
  "result": {
    "filePath": "/path/to/file.txt",
    "size": 1024,
    "processed": true
  }
}
```

### Protocolo HTTP

Si usas `GoBackendHTTP`, el backend expone un servidor HTTP:

```typescript
const backend = new GoBackendHTTP(context, {
  port: 8080,
});

await backend.start();
const result = await backend.call('processFile', {
  filePath: '/path/to/file.txt',
});
```

## Ventajas

1. **Rendimiento**: Go es más rápido para procesamiento pesado
2. **Concurrencia**: Goroutines para operaciones paralelas
3. **Binarios estáticos**: No requiere dependencias externas
4. **Multiplataforma**: Un binario por plataforma
5. **Separación**: UI en TypeScript, lógica pesada en Go

## Casos de Uso Reales

### 1. Indexación Rápida
```typescript
const index = await backend.call('indexWorkspace', {
  rootPath: workspaceRoot,
  patterns: ['**/*.ts', '**/*.tsx'],
  maxConcurrency: 10,
});
```

### 2. Análisis de Código Complejo
```typescript
const analysis = await backend.call('analyzeCode', {
  code: editor.document.getText(),
  language: 'typescript',
  options: {
    includeAST: true,
    includeMetrics: true,
  },
});
```

### 3. Procesamiento de Archivos Grandes
```typescript
const result = await backend.call('processLargeFile', {
  filePath: '/path/to/huge-file.txt',
  chunkSize: 1024 * 1024, // 1MB chunks
  parallel: true,
});
```

## Distribución

Cuando empaquetas con `vsce package`:

1. Los binarios en `bin/` se incluyen automáticamente
2. Los usuarios reciben los binarios precompilados
3. No necesitan tener Go instalado
4. El backend se inicia automáticamente

## Testing

### Probar localmente:

1. Compila los binarios: `./build-go.sh`
2. Ejecuta la extensión en modo desarrollo (F5)
3. El backend se iniciará desde `bin/`
4. Prueba los comandos que usan el backend

### Debugging:

Los logs del backend aparecen en la consola de VSCode:
- `[GoBackend]` - Logs del cliente TypeScript
- `[GoBackend] stderr:` - Logs del proceso Go

## Notas Importantes

1. **Tamaño**: Los binarios pueden ser grandes (5-20MB cada uno)
2. **Permisos**: En Unix, los binarios deben ser ejecutables
3. **Actualizaciones**: Si cambias el backend Go, recompila y redistribuye
4. **Plataformas**: Compila para todas las plataformas que quieras soportar

## Ejemplo Completo

Ver:
- `src/core/GoBackend.ts` - Implementación completa del cliente
- `examples/go-backend/main.go` - Backend Go de ejemplo
- `src/commands/integrateGoBackend.ts` - Integración práctica

## Conclusión

Esta arquitectura te permite:
- ✅ Usar TypeScript para la UI y lógica de VSCode
- ✅ Usar Go para procesamiento pesado y rendimiento
- ✅ Empaquetar todo junto en una sola extensión
- ✅ Distribuir sin requerir Go instalado en el usuario

¡Es la mejor de ambos mundos!


