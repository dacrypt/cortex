# Integración de Go en Extensiones de VSCode

## Respuesta a tu pregunta

**Sí, una extensión de VSCode puede ejecutar código de Go además de TypeScript.**

Las extensiones de VSCode están escritas en TypeScript/JavaScript, pero pueden ejecutar código en cualquier lenguaje que pueda ejecutarse en el sistema operativo, incluyendo Go, Python, Rust, C++, etc.

## Cómo funciona

VSCode (y Node.js) proporciona el módulo `child_process` que permite ejecutar procesos externos. Tu extensión ya usa esto para ejecutar comandos como `mdls`, `osascript`, y `soffice`.

## Ejemplos creados

He creado dos archivos de ejemplo:

1. **`src/utils/goExecutor.ts`** - Utilidades para ejecutar código Go
2. **`src/commands/runGoExample.ts`** - Comandos de ejemplo que usan estas utilidades

## Formas de ejecutar Go

### 1. Ejecutar un binario compilado

```typescript
import { executeGoBinary } from './utils/goExecutor';

const result = await executeGoBinary('/path/to/my-go-program', ['arg1', 'arg2']);
console.log(result.stdout);
```

### 2. Ejecutar un archivo .go directamente con `go run`

```typescript
import { runGoFile } from './utils/goExecutor';

const result = await runGoFile('/path/to/program.go', ['arg1', 'arg2']);
console.log(result.stdout);
```

### 3. Compilar y ejecutar

```typescript
import { buildAndRunGoFile } from './utils/goExecutor';

const result = await buildAndRunGoFile('/path/to/program.go', undefined, ['arg1']);
console.log(result.stdout);
```

### 4. Ejecutar comandos de Go (go fmt, go test, etc.)

```typescript
import { executeGoCommand } from './utils/goExecutor';

// Ejecutar go test
const result = await executeGoCommand('test', '/workspace/path', ['-v']);

// Ejecutar go fmt
const fmtResult = await executeGoCommand('fmt', '/workspace/path');
```

## Integración en tu extensión

Para agregar estos comandos a tu extensión, necesitarías:

### 1. Registrar los comandos en `package.json`

```json
{
  "command": "cortex.runGoExample",
  "title": "Cortex: Run Go Example",
  "icon": "$(play)"
}
```

### 2. Registrar el comando en `extension.ts`

```typescript
import { runGoExampleCommand } from './commands/runGoExample';

// En la función activate():
vscode.commands.registerCommand('cortex.runGoExample', runGoExampleCommand);
```

## Casos de uso comunes

1. **Procesamiento de archivos**: Usar Go para procesar archivos grandes más rápido que TypeScript
2. **Herramientas CLI**: Ejecutar herramientas de Go desde la extensión
3. **Análisis de código**: Usar herramientas de análisis de Go para analizar código
4. **Compilación**: Compilar proyectos de Go desde la extensión
5. **Testing**: Ejecutar tests de Go

## Ventajas de usar Go desde TypeScript

- **Rendimiento**: Go es más rápido para ciertas tareas (procesamiento de archivos, parsing, etc.)
- **Herramientas existentes**: Puedes usar herramientas de Go ya existentes
- **Concurrencia**: Go tiene excelente soporte para concurrencia
- **Binarios estáticos**: Los binarios de Go son fáciles de distribuir

## Ejemplo completo: Procesar archivos con Go

Imagina que tienes un programa Go que procesa archivos:

```go
// fileprocessor.go
package main

import (
    "fmt"
    "os"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: fileprocessor <file>")
        os.Exit(1)
    }
    
    filePath := os.Args[1]
    // Procesar archivo...
    fmt.Println("Processed:", filePath)
}
```

Desde tu extensión TypeScript:

```typescript
import { buildAndRunGoFile } from './utils/goExecutor';

async function processFile(filePath: string) {
    const goScript = path.join(__dirname, '..', 'tools', 'fileprocessor.go');
    const result = await buildAndRunGoFile(goScript, undefined, [filePath]);
    
    if (result.exitCode === 0) {
        vscode.window.showInformationMessage(result.stdout);
    } else {
        vscode.window.showErrorMessage(result.stderr);
    }
}
```

## Notas importantes

1. **Go debe estar instalado**: El usuario necesita tener Go instalado en su sistema
2. **PATH**: Go debe estar en el PATH del sistema
3. **Permisos**: Asegúrate de tener permisos para ejecutar los binarios
4. **Manejo de errores**: Siempre verifica `exitCode` y maneja errores apropiadamente
5. **Seguridad**: Valida las rutas y argumentos antes de ejecutar

## Conclusión

Las extensiones de VSCode pueden ejecutar código en cualquier lenguaje, no solo TypeScript. Go es una excelente opción para tareas que requieren rendimiento o para integrar herramientas existentes escritas en Go.


