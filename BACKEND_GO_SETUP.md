# Backend Go Empaquetado con Extensión VSCode

## Arquitectura

```
┌─────────────────────────────────────┐
│   VSCode Extension (TypeScript)    │
│   ┌─────────────────────────────┐  │
│   │  GoBackend / GoBackendHTTP  │  │
│   └──────────┬──────────────────┘  │
└──────────────┼─────────────────────┘
               │
               │ stdin/stdout o HTTP
               │
┌──────────────▼─────────────────────┐
│   Backend Go (Binario empaquetado)  │
│   - Procesamiento pesado            │
│   - Análisis de código              │
│   - Operaciones de archivos         │
└─────────────────────────────────────┘
```

## Estructura de Directorios

```
cortex/
├── src/
│   └── core/
│       ├── GoBackend.ts          # Cliente stdin/stdout
│       └── GoBackendHTTP.ts      # Cliente HTTP
├── bin/                           # Binarios Go (empaquetados)
│   ├── cortex-backend-darwin-amd64
│   ├── cortex-backend-darwin-arm64
│   ├── cortex-backend-linux-amd64
│   └── cortex-backend-windows-amd64.exe
├── examples/
│   └── go-backend/
│       ├── main.go               # Backend stdin/stdout
│       └── http-server.go         # Backend HTTP
└── build-go.sh                    # Script para compilar binarios
```

## Paso 1: Compilar Binarios Go

Crea un script `build-go.sh` para compilar para todas las plataformas:

```bash
#!/bin/bash

# build-go.sh - Compila binarios Go para todas las plataformas

BINARY_NAME="cortex-backend"
OUTPUT_DIR="bin"

mkdir -p $OUTPUT_DIR

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o $OUTPUT_DIR/$BINARY_NAME-linux-amd64 ./examples/go-backend/main.go

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o $OUTPUT_DIR/$BINARY_NAME-darwin-amd64 ./examples/go-backend/main.go

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o $OUTPUT_DIR/$BINARY_NAME-darwin-arm64 ./examples/go-backend/main.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o $OUTPUT_DIR/$BINARY_NAME-windows-amd64.exe ./examples/go-backend/main.go

echo "Binarios compilados en $OUTPUT_DIR/"
```

## Paso 2: Incluir Binarios en package.json

Agrega los binarios al `package.json` para que se incluyan en el empaquetado:

```json
{
  "files": [
    "out",
    "resources",
    "bin/**/*"
  ]
}
```

## Paso 3: Usar el Backend en TypeScript

### Ejemplo con stdin/stdout:

```typescript
import { GoBackend } from './core/GoBackend';
import * as vscode from 'vscode';

export async function activate(context: vscode.ExtensionContext) {
  // Inicializar backend
  const backend = new GoBackend(context, {
    binariesPath: 'bin',
    binaryName: 'cortex-backend',
  });

  // Iniciar backend
  try {
    await backend.start();
    console.log('[Cortex] Go backend started');
  } catch (error) {
    console.error('[Cortex] Failed to start Go backend:', error);
    vscode.window.showErrorMessage('Failed to start backend');
    return;
  }

  // Usar el backend
  try {
    const result = await backend.call('processFile', {
      filePath: '/path/to/file.txt',
    });
    console.log('Result:', result);
  } catch (error) {
    console.error('Error:', error);
  }

  // Detener backend al desactivar
  context.subscriptions.push({
    dispose: async () => {
      await backend.stop();
    },
  });
}
```

### Ejemplo con HTTP:

```typescript
import { GoBackendHTTP } from './core/GoBackendHTTP';

const backend = new GoBackendHTTP(context, {
  binariesPath: 'bin',
  binaryName: 'cortex-backend-http',
  port: 8080,
});

await backend.start();
const result = await backend.call('processFile', {
  filePath: '/path/to/file.txt',
});
```

## Paso 4: Comunicación JSON-RPC

El protocolo de comunicación usa JSON-RPC style:

### Request (TypeScript → Go):
```json
{
  "id": "req_1",
  "method": "processFile",
  "params": {
    "filePath": "/path/to/file.txt"
  }
}
```

### Response (Go → TypeScript):
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

## Ventajas de esta Arquitectura

1. **Rendimiento**: Go es más rápido para procesamiento pesado
2. **Concurrencia**: Go tiene excelente soporte para goroutines
3. **Binarios estáticos**: No requiere dependencias externas
4. **Multiplataforma**: Un binario por plataforma, fácil de distribuir
5. **Separación de responsabilidades**: UI en TypeScript, lógica pesada en Go

## Casos de Uso

### 1. Procesamiento de Archivos Grandes
```typescript
const result = await backend.call('processLargeFile', {
  filePath: '/path/to/huge-file.txt',
  options: {
    chunkSize: 1024 * 1024,
    parallel: true,
  },
});
```

### 2. Análisis de Código
```typescript
const analysis = await backend.call('analyzeCode', {
  code: editor.document.getText(),
  language: 'typescript',
});
```

### 3. Indexación Rápida
```typescript
const index = await backend.call('indexWorkspace', {
  rootPath: workspaceRoot,
  patterns: ['**/*.ts', '**/*.tsx'],
});
```

## Distribución

Cuando empaquetas la extensión con `vsce package`, los binarios se incluyen automáticamente si están en el directorio `bin/` y están listados en `package.json` bajo `files`.

Los usuarios no necesitan tener Go instalado - solo reciben los binarios precompilados.

## Testing Local

Para probar localmente antes de empaquetar:

1. Compila los binarios: `./build-go.sh`
2. Ejecuta la extensión en modo desarrollo
3. El backend se iniciará automáticamente desde `bin/`

## Notas Importantes

- **Tamaño**: Los binarios pueden ser grandes (5-20MB cada uno). Considera comprimirlos o usar compresión UPX.
- **Permisos**: En Unix, asegúrate de hacer los binarios ejecutables (`chmod +x`)
- **Actualizaciones**: Si actualizas el backend Go, necesitas recompilar y redistribuir
- **Debugging**: Puedes ver los logs del backend en la consola de VSCode

## Ejemplo Completo de Integración

Ver `src/core/GoBackend.ts` y `examples/go-backend/main.go` para ejemplos completos.


