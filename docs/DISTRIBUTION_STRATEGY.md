# Estrategia de Distribución: Cortex como Extensión VS Code Autocontenida

## Objetivo

Distribuir Cortex como una **sola extensión VS Code** donde:
- ✅ Todo se instala automáticamente
- ✅ Backend se inicia automáticamente
- ✅ Dependencias (Tika, Ollama) se gestionan automáticamente
- ✅ Sin configuración manual del usuario
- ✅ Funciona "out of the box"

## Arquitectura Actual

```
VS Code Extension (TypeScript)
    ↓ gRPC
Go Backend Daemon (cortexd)
    ↓ HTTP
Tika Server (Java)
    ↓ HTTP
Ollama (LLM)
```

## Estrategias de Distribución

### Estrategia 1: Bundling Completo (Recomendada)

**Incluir todos los binarios en el .vsix**

#### Estructura del Package

```
cortex-extension/
├── package.json
├── out/                    # TypeScript compilado
├── resources/
│   └── cortex-icon.svg
├── bin/                    # Binarios por plataforma
│   ├── darwin/
│   │   ├── cortexd         # Backend Go (macOS)
│   │   └── tika-server.jar # Tika Server JAR
│   ├── linux/
│   │   ├── cortexd         # Backend Go (Linux)
│   │   └── tika-server.jar
│   └── win32/
│       ├── cortexd.exe     # Backend Go (Windows)
│       └── tika-server.jar
└── scripts/
    ├── postinstall.js      # Verificación de dependencias
    └── start-backend.js     # Iniciar backend
```

#### Ventajas

- ✅ **Zero-config**: Usuario solo instala la extensión
- ✅ **Funciona offline**: Todo está incluido
- ✅ **Versionado consistente**: Mismo backend que extensión
- ✅ **Sin dependencias externas**: No requiere Go, Java, etc. instalados

#### Desventajas

- ❌ **Tamaño grande**: .vsix puede ser 50-100MB
- ❌ **Múltiples builds**: Necesita compilar para cada plataforma
- ❌ **Actualizaciones**: Requiere nueva versión de extensión

#### Implementación

**1. Build Script Multi-plataforma**

```json
// package.json
{
  "scripts": {
    "build:backend": "node scripts/build-backend.js",
    "package": "vsce package",
    "prepackage": "npm run build:backend && npm run compile"
  }
}
```

**2. Backend Manager en Extension**

```typescript
// src/core/BackendManager.ts
export class BackendManager {
  private process?: ChildProcess;
  private configPath: string;
  
  async start(): Promise<void> {
    const platform = process.platform;
    const arch = process.arch;
    const backendPath = this.getBackendPath(platform, arch);
    
    // Verificar que el binario existe
    if (!await fs.pathExists(backendPath)) {
      throw new Error(`Backend binary not found for ${platform}-${arch}`);
    }
    
    // Iniciar proceso
    this.process = spawn(backendPath, [
      '--config', this.configPath,
      '--data-dir', this.getDataDir()
    ]);
    
    // Esperar a que esté listo
    await this.waitForReady();
  }
  
  private getBackendPath(platform: string, arch: string): string {
    const ext = platform === 'win32' ? '.exe' : '';
    return path.join(
      vscode.extensions.getExtension('your-publisher.cortex')!.extensionPath,
      'bin',
      platform,
      `cortexd${ext}`
    );
  }
}
```

**3. Tika Manager Integrado**

```typescript
// src/core/TikaManager.ts
export class TikaManager {
  async start(): Promise<void> {
    const jarPath = this.getTikaJarPath();
    const javaPath = await this.findJava();
    
    this.process = spawn(javaPath, [
      '-jar', jarPath,
      '--host', '0.0.0.0',
      '--port', '9998'
    ]);
  }
  
  private async findJava(): Promise<string> {
    // Buscar Java en PATH
    // Si no está, ofrecer descargar JRE portable
  }
}
```

### Estrategia 2: Auto-descarga (Híbrida)

**Extensión ligera + descarga automática de dependencias**

#### Flujo

1. Usuario instala extensión (pequeña, ~5MB)
2. Primera activación:
   - Descarga backend binario para su plataforma
   - Descarga Tika JAR
   - Verifica/instala Java si falta
   - Configura todo automáticamente

#### Ventajas

- ✅ **Extensión pequeña**: Descarga inicial rápida
- ✅ **Actualizaciones independientes**: Puede actualizar backend sin nueva extensión
- ✅ **Flexible**: Puede descargar versiones específicas por plataforma

#### Desventajas

- ❌ **Requiere internet**: Primera instalación necesita conexión
- ❌ **Complejidad**: Manejo de descargas, checksums, errores
- ❌ **Permisos**: Necesita permisos de escritura para descargar

#### Implementación

```typescript
// src/core/DownloadManager.ts
export class DownloadManager {
  private readonly baseUrl = 'https://releases.cortex.dev';
  private readonly storagePath: string;
  
  async ensureBackend(): Promise<string> {
    const platform = process.platform;
    const arch = process.arch;
    const version = await this.getLatestVersion();
    const backendPath = path.join(this.storagePath, `cortexd-${version}${ext}`);
    
    if (await fs.pathExists(backendPath)) {
      return backendPath;
    }
    
    // Mostrar progress
    await vscode.window.withProgress({
      location: vscode.ProgressLocation.Notification,
      title: 'Downloading Cortex backend...'
    }, async (progress) => {
      const url = `${this.baseUrl}/v${version}/cortexd-${platform}-${arch}${ext}`;
      await this.download(url, backendPath, progress);
    });
    
    // Verificar checksum
    await this.verifyChecksum(backendPath);
    
    // Hacer ejecutable (Unix)
    if (platform !== 'win32') {
      await fs.chmod(backendPath, 0o755);
    }
    
    return backendPath;
  }
}
```

### Estrategia 3: Native Dependencies (npm)

**Usar npm native modules para gestionar procesos**

#### Estructura

```json
// package.json
{
  "dependencies": {
    "cortex-backend-native": "^1.0.0"  // Native module con binarios
  },
  "scripts": {
    "postinstall": "node scripts/postinstall.js"
  }
}
```

#### Ventajas

- ✅ **Gestión automática**: npm maneja instalación
- ✅ **Versionado**: Semver para dependencias
- ✅ **Familiar**: Mismo patrón que otras extensiones

#### Desventajas

- ❌ **Requiere rebuild**: Native modules necesitan compilar
- ❌ **Complejidad**: Crear y mantener paquetes npm
- ❌ **Tamaño**: npm packages pueden ser grandes

### Estrategia 4: Docker-in-Docker (No Recomendada)

**Usar Docker para contenedores**

#### Problemas

- ❌ VS Code extensions no pueden ejecutar Docker directamente
- ❌ Requiere Docker instalado (dependencia externa)
- ❌ Complejidad de gestión de contenedores
- ❌ Overhead de recursos

## Recomendación: Estrategia Híbrida Optimizada

### Fase 1: Bundling Esencial

**Incluir en .vsix:**
- ✅ Backend Go binario (por plataforma)
- ✅ Tika Server JAR (único, multiplataforma)
- ✅ Scripts de gestión

**No incluir:**
- ❌ Java JRE (demasiado grande, ~200MB)
- ❌ Ollama (opcional, usuario puede instalarlo)

### Fase 2: Auto-detección y Gestión

**Al activar extensión:**

1. **Iniciar Backend**
   ```typescript
   const backend = await BackendManager.start({
     binary: getBundledBackend(),
     config: generateDefaultConfig(),
     dataDir: getExtensionDataDir()
   });
   ```

2. **Backend gestiona Tika automáticamente**
   - El backend Go ya tiene `TikaManager` integrado
   - Si `tika.auto_download: true`, descarga JAR automáticamente
   - Si `tika.manage_process: true`, inicia Tika como subproceso
   - No necesita gestión desde TypeScript

3. **Verificar Java (opcional, solo si Tika habilitado)**
   ```typescript
   if (config.tika.enabled) {
     if (!await checkJava()) {
       showNotification({
         message: 'Java required for Tika. Install from https://adoptium.net',
         action: 'Open Website'
       });
       // Backend usará extractores nativos como fallback
     }
   }
   ```

4. **Verificar Ollama (opcional)**
   ```typescript
   if (config.llm.enabled) {
     if (!await checkOllama()) {
       showNotification({
         message: 'Ollama not found. Install from https://ollama.ai',
         action: 'Open Website'
       });
     }
   }
   ```

### Estructura de Archivos

```
cortex-extension/
├── package.json
├── out/                    # TypeScript compilado
├── bin/
│   ├── darwin-x64/
│   │   └── cortexd
│   ├── darwin-arm64/
│   │   └── cortexd
│   ├── linux-x64/
│   │   └── cortexd
│   ├── linux-arm64/
│   │   └── cortexd
│   ├── win32-x64/
│   │   └── cortexd.exe
│   └── win32-arm64/
│       └── cortexd.exe
└── resources/
    └── cortex-icon.svg

# Nota: Tika JAR NO se incluye en el package
# El backend lo descarga automáticamente si tika.auto_download: true
# Se almacena en {data_dir}/tika/tika-server-standard.jar
```

### Implementación Detallada

#### 1. Backend Manager

```typescript
// src/core/BackendManager.ts
import * as vscode from 'vscode';
import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as fs from 'fs-extra';

export class BackendManager {
  private process?: ChildProcess;
  private readonly extensionPath: string;
  private readonly dataDir: string;
  private readonly configPath: string;
  
  constructor(context: vscode.ExtensionContext) {
    this.extensionPath = context.extensionPath;
    this.dataDir = path.join(context.globalStorageUri.fsPath, 'cortex');
    this.configPath = path.join(this.dataDir, 'cortexd.yaml');
  }
  
  async start(): Promise<void> {
    // Asegurar directorios
    await fs.ensureDir(this.dataDir);
    
    // Generar configuración por defecto si no existe
    if (!await fs.pathExists(this.configPath)) {
      await this.generateDefaultConfig();
    }
    
    // Obtener binario del backend
    const backendPath = await this.getBackendBinary();
    
    // Iniciar proceso
    this.process = spawn(backendPath, [
      '--config', this.configPath,
      '--data-dir', this.dataDir
    ], {
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: false
    });
    
    // Logging
    this.process.stdout?.on('data', (data) => {
      console.log(`[Backend] ${data}`);
    });
    
    this.process.stderr?.on('data', (data) => {
      console.error(`[Backend] ${data}`);
    });
    
    // Esperar a que esté listo
    await this.waitForReady();
    
    vscode.window.showInformationMessage('Cortex backend started');
  }
  
  private async getBackendBinary(): Promise<string> {
    const platform = process.platform;
    const arch = process.arch;
    const ext = platform === 'win32' ? '.exe' : '';
    
    // Mapear arquitecturas
    const archMap: Record<string, string> = {
      'x64': 'x64',
      'x32': 'x64',  // 32-bit usa 64-bit
      'arm64': 'arm64',
      'arm': 'arm64'
    };
    
    const mappedArch = archMap[arch] || 'x64';
    const platformMap: Record<string, string> = {
      'darwin': 'darwin',
      'linux': 'linux',
      'win32': 'win32'
    };
    
    const mappedPlatform = platformMap[platform] || platform;
    const binDir = path.join(this.extensionPath, 'bin', `${mappedPlatform}-${mappedArch}`);
    const binary = path.join(binDir, `cortexd${ext}`);
    
    if (!await fs.pathExists(binary)) {
      throw new Error(
        `Backend binary not found for ${platform}-${arch}. ` +
        `Expected: ${binary}. ` +
        `Please report this issue.`
      );
    }
    
    // Hacer ejecutable (Unix)
    if (platform !== 'win32') {
      await fs.chmod(binary, 0o755);
    }
    
    return binary;
  }
  
  private async waitForReady(timeout = 30000): Promise<void> {
    const startTime = Date.now();
    const checkInterval = 500;
    
    while (Date.now() - startTime < timeout) {
      try {
        // Intentar conectar vía gRPC
        const client = new GrpcAdminClient();
        await client.healthCheck();
        return; // ¡Listo!
      } catch (error) {
        // Aún no está listo, esperar
        await new Promise(resolve => setTimeout(resolve, checkInterval));
      }
    }
    
    throw new Error('Backend failed to start within timeout');
  }
  
  async stop(): Promise<void> {
    if (this.process) {
      this.process.kill('SIGTERM');
      // Esperar graceful shutdown
      await new Promise(resolve => {
        this.process?.on('exit', resolve);
        setTimeout(() => {
          if (this.process) {
            this.process.kill('SIGKILL');
            resolve(null);
          }
        }, 5000);
      });
      this.process = undefined;
    }
  }
  
  private async generateDefaultConfig(): Promise<void> {
    const defaultConfig = {
      grpc_address: '127.0.0.1:50051',
      http_address: '127.0.0.1:8081',
      data_dir: this.dataDir,
      log_level: 'info',
      tika: {
        enabled: true,
        manage_process: true,
        endpoint: 'http://localhost:9998',
        port: 9998
      },
      llm: {
        enabled: true,
        default_provider: 'ollama',
        default_model: 'llama3.2',
        endpoint: 'http://localhost:11434'
      }
    };
    
    await fs.writeJSON(this.configPath, defaultConfig, { spaces: 2 });
  }
}
```

#### 2. Tika Manager (Gestionado por Backend Go)

**Nota**: Tika se gestiona completamente desde el backend Go, no desde TypeScript.

El backend Go ya incluye `TikaManager` que:
- Descarga el JAR automáticamente si `tika.auto_download: true`
- Inicia Tika como subproceso si `tika.manage_process: true`
- Monitorea salud y reinicia si falla
- Se detiene limpiamente al cerrar Cortex

**No se necesita código TypeScript para Tika** - el backend lo gestiona todo.

Si quieres verificar el estado desde la extensión:

```typescript
// src/core/BackendHealth.ts
export async function checkTikaStatus(adminClient: GrpcAdminClient): Promise<boolean> {
  try {
    const status = await adminClient.getStatus();
    return status.tika?.running === true;
  } catch {
    return false;
  }
}
```

#### 3. Build Script Multi-plataforma

```javascript
// scripts/build-backend.js
const { execSync } = require('child_process');
const fs = require('fs-extra');
const path = require('path');

const platforms = [
  { os: 'darwin', arch: 'amd64', name: 'darwin-x64' },
  { os: 'darwin', arch: 'arm64', name: 'darwin-arm64' },
  { os: 'linux', arch: 'amd64', name: 'linux-x64' },
  { os: 'linux', arch: 'arm64', name: 'linux-arm64' },
  { os: 'windows', arch: 'amd64', name: 'win32-x64' },
  { os: 'windows', arch: 'arm64', name: 'win32-arm64' }
];

async function buildBackend() {
  const binDir = path.join(__dirname, '..', 'bin');
  await fs.ensureDir(binDir);
  
  for (const platform of platforms) {
    console.log(`Building for ${platform.name}...`);
    
    const outputDir = path.join(binDir, platform.name);
    await fs.ensureDir(outputDir);
    
    const ext = platform.os === 'windows' ? '.exe' : '';
    const outputPath = path.join(outputDir, `cortexd${ext}`);
    
    const env = {
      ...process.env,
      GOOS: platform.os,
      GOARCH: platform.arch,
      CGO_ENABLED: '0' // Static binary
    };
    
    execSync(
      `go build -ldflags="-s -w" -o "${outputPath}" ./cmd/cortexd`,
      {
        cwd: path.join(__dirname, '..', 'backend'),
        env,
        stdio: 'inherit'
      }
    );
    
    console.log(`✓ Built ${platform.name}`);
  }
}

buildBackend().catch(console.error);
```

#### 4. Package Scripts

```json
// package.json
{
  "scripts": {
    "build:backend": "node scripts/build-backend.js",
    "download:tika": "node scripts/download-tika.js",
    "prepackage": "npm run build:backend && npm run download:tika && npm run compile",
    "package": "vsce package",
    "package:all": "npm run prepackage && npm run package"
  }
}
```

### Gestión del Ciclo de Vida

```typescript
// src/extension.ts
export async function activate(context: vscode.ExtensionContext) {
  const backendManager = new BackendManager(context);
  
  // Iniciar backend (Tika se gestiona automáticamente por el backend)
  try {
    await backendManager.start();
    // Backend iniciará Tika automáticamente si está configurado
  } catch (error) {
    vscode.window.showErrorMessage(`Failed to start Cortex: ${error}`);
    return;
  }
  
  // Registrar cleanup
  context.subscriptions.push({
    dispose: async () => {
      // Backend detendrá Tika automáticamente
      await backendManager.stop();
    }
  });
  
  // Resto de la inicialización...
}
```

## Tamaño Estimado del Package

| Componente | Tamaño | Notas |
|------------|--------|-------|
| TypeScript compilado | ~2MB | Código de extensión |
| Backend Go (6 plataformas) | ~30MB | ~5MB cada uno |
| **Total .vsix** | **~32MB** | ✅ Muy razonable |

### Descargas Automáticas (Post-instalación)

| Componente | Tamaño | Cuándo se descarga |
|------------|--------|-------------------|
| Tika Server JAR | ~80MB | Primera vez que se habilita Tika |
| Ollama (opcional) | Variable | Usuario lo instala manualmente |

**Ventajas**:
- ✅ Package inicial pequeño (~32MB)
- ✅ Tika solo se descarga si se necesita
- ✅ Backend gestiona descarga automáticamente
- ✅ No requiere Docker

## Checklist de Implementación

- [ ] Build script multi-plataforma para backend
- [x] Script de descarga de Tika JAR (implementado en backend Go)
- [x] BackendManager en extensión (implementado)
- [x] TikaManager en extensión (gestionado por backend Go)
- [ ] Java detection y opción de JRE portable (opcional, backend usa extractores nativos si falta Java)
- [x] Configuración automática por defecto (implementado)
- [x] Gestión de ciclo de vida (start/stop) (implementado)
- [x] Manejo de errores y fallbacks (implementado)
- [x] Notificaciones al usuario (implementado)
- [x] Documentación de instalación (TIKA_SETUP.md actualizado)

## Recomendación Final

**Usar Estrategia Híbrida Optimizada con Auto-descarga de Tika:**

1. **Bundle esencial**: Solo Backend Go en .vsix (~32MB)
2. **Auto-descarga de Tika**: Backend descarga JAR automáticamente si se habilita
3. **Auto-detección**: Java, Ollama (opcional)
4. **Gestión automática**: Backend gestiona Tika completamente
5. **Fallbacks**: Si falta Java, usar extractores nativos

**Resultado**: 
- Usuario instala extensión (~32MB)
- Backend se inicia automáticamente
- Si Tika está habilitado, backend descarga JAR automáticamente
- Todo funciona sin configuración manual

**Ventajas sobre Docker**:
- ✅ No requiere Docker instalado
- ✅ Más ligero (solo JAR, no contenedor completo)
- ✅ Gestión más simple (proceso directo)
- ✅ Mejor integración con el ciclo de vida de Cortex

