# Cortex - Semantic Cognition Layer for Your Workspace

**Cortex** es una extensión de VS Code con un daemon backend en Go que proporciona una capa semántica de organización sobre tu sistema de archivos. Permite organizar archivos por proyectos, tags, tipos y otros atributos sin mover archivos ni crear duplicados. La arquitectura está basada en un daemon Go con gRPC que maneja todo el procesamiento pesado, mientras que la extensión VS Code proporciona la interfaz de usuario.

## 🎯 Principio Base

- **Los archivos se quedan donde están** - sin duplicados, sin mover
- **Vistas virtuales múltiples** - distintos agrupamientos sobre los mismos archivos
- **Un archivo puede pertenecer a varios proyectos** - tags y contextos combinables
- **Local-first y determinístico** - todo se almacena localmente, Markdown es la fuente de verdad

## ✨ Características Principales

### 1. Indexación Backend con SQLite

- **Arquitectura Backend-First**: Todo el procesamiento se realiza en el daemon Go
- Persistencia en SQLite embebido del daemon (ubicación configurable)
- Índice rápido con metadatos enriquecidos
- Escaneo incremental y comportamiento determinístico
- Soporte para 10,000+ archivos con rendimiento sub-segundo
- Actualizaciones en tiempo real vía gRPC

### 2. Vistas Semánticas (Extensión VS Code)

Cortex proporciona múltiples vistas virtuales para organizar tus archivos:

- **Por Proyecto** - agrupa por clientes, casos o iniciativas
- **Por Tag** - filtra por etiquetas como "urgent", "review", "bug-fix"
- **Por Tipo** - agrupa por extensión o tipo de archivo
- **Por Fecha** - agrupa por fecha de modificación (automático)
- **Por Tamaño** - categoriza por tamaño de archivo (automático)
- **Por Carpeta** - estructura jerárquica de carpetas (automático)
- **Por Tipo de Contenido** - agrupa por MIME type real (automático)
- **Métricas de Código** - análisis de LOC, comentarios, complejidad
- **Métricas de Documentos** - estadísticas de documentos
- **Issues** - visualización de TODOs y problemas
- **Info de Archivo** - panel de detalles de archivo
- **Biblioteca** - vista de categorización

### 3. Daemon en Go con gRPC (✅ Implementado)

- **6 Servicios gRPC completos**:
  - `AdminService` - Gestión de workspaces, escaneo, progreso
  - `FileService` - Operaciones sobre archivos
  - `MetadataService` - Tags, proyectos, metadata
  - `LLMService` - Integración con LLMs locales
  - `RAGService` - Búsqueda semántica y consultas
  - `KnowledgeService` - Knowledge Engine completo
- Arquitectura hexagonal / clean architecture
- Repositorios SQLite, colas de tareas, pipeline de indexación
- LLM router con soporte para múltiples proveedores (Ollama, LM Studio, etc.)
- Knowledge Engine completo con:
  - ✅ Gestión de estados de documentos (draft, active, replaced, archived)
  - ✅ Relaciones entre documentos (replaces, depends_on, belongs_to)
  - ✅ Jerarquía de proyectos (proyectos pueden tener subproyectos)
  - ✅ Memoria temporal (uso, frecuencia, co-ocurrencias)
  - ✅ Búsqueda semántica con RAG (Retrieval Augmented Generation)
  - ✅ Consultas declarativas
- Servicios de actualización en tiempo real
- Panel de administración web integrado

### 4. Semantic File System (LSFS) - NUEVO

Comandos en lenguaje natural para organizar archivos, inspirado en el paper LSFS:

- **Comandos Naturales** - "agrupa todos los PDFs por autor", "etiqueta estos archivos como importantes"
- **Preview Mode** - vista previa de cambios antes de ejecutar
- **Undo Support** - comandos de deshacer generados automáticamente
- **Sugerencias Inteligentes** - autocompletado basado en contexto

**Ejemplos de comandos:**
```
"group files by extension"
"tag these files as review-needed"
"assign files to project Client-X"
"find all PDFs modified today"
"create project for invoices"
"merge project A into project B"
```

### 5. Preference Learning (MemGPT-inspired) - NUEVO

El sistema aprende de tus decisiones para mejorar futuras sugerencias:

- **Feedback Tracking** - registra aceptaciones, rechazos y correcciones
- **Pattern Learning** - aprende patrones basados en tipo de archivo, carpeta, etc.
- **Adaptive Suggestions** - ajusta confianza basada en historial
- **Auto-Apply** - aplica preferencias de alta confianza automáticamente

### 6. Document Clustering (D2CS-inspired) - NUEVO

Agrupación inteligente de documentos usando grafos y comunidades:

- **Document Graph** - construye grafo basado en similitud semántica, entidades, temporal
- **Community Detection** - algoritmo Louvain para detectar clusters
- **LLM Supervision** - valida y nombra clusters con LLM
- **GraphRAG Summaries** - genera resúmenes por cluster

### 7. Dynamic Taxonomy (Chain-of-Layer) - NUEVO

Generación automática de jerarquías de categorías:

- **LLM-Driven Induction** - genera taxonomía capa por capa con LLM
- **Auto-Merge** - combina categorías similares automáticamente
- **Poly-hierarchy** - un archivo puede pertenecer a múltiples categorías
- **Adaptive Evolution** - la taxonomía evoluciona con nuevos datos

### 8. Características AI (Activadas por defecto)

Estas características vienen habilitadas por defecto en la extensión y en el daemon; requieren un LLM local (Ollama, LM Studio, etc.) para ejecutarse.

- **Sugerencias de Tags AI** - analiza contenido y sugiere tags relevantes
- **Asignación de Proyectos AI** - recomienda proyectos basados en contexto
- **Resúmenes de Archivos AI** - genera resúmenes concisos
- **Indexación Automática AI** - genera tags y proyectos durante la indexación
- **RAG Mejorado** - usa embeddings para mejor precisión en sugerencias
- Integración con LLMs locales (Ollama, LM Studio, etc.)

### 5. Extracción de Metadatos Profunda

- **MIME Types** - detección por magic bytes (no solo extensión)
- **Análisis de Texto** - conteo de líneas, palabras, encoding
- **Métricas de Código** - LOC, comentarios, imports, exports, funciones, clases
- **Propiedades de Imágenes** - dimensiones, formato, aspect ratio
- **Metadatos Git** - autor, commit, branch (opcional)

### 6. Pipeline de Indexación

El daemon Go procesa archivos a través de un pipeline multi-etapa:

1. **BasicStage** - metadatos básicos (tamaño, fechas)
2. **MimeStage** - detección de tipo MIME
3. **MirrorStage** - extracción de contenido (PDF, Office docs)
4. **CodeStage** - análisis de código
5. **DocumentStage** - parsing de Markdown, chunking, embeddings
6. **RelationshipStage** - detección de relaciones entre documentos
7. **StateStage** - inferencia de estados de documentos
8. **AIStage** - procesamiento AI (tags, proyectos, resúmenes)

## 🚀 Instalación

**⚠️ Importante**: Cortex requiere que el daemon backend esté ejecutándose. La extensión VS Code se conecta al backend vía gRPC.

### Paso 1: Iniciar el Backend Daemon (Requerido)

1. **Build del daemon**:
   ```bash
   cd backend
   go build -o cortexd ./cmd/cortexd
   ```

2. **Configuración básica**:
   ```bash
   cp backend/configs/cortexd.yaml.example backend/cortexd.local.yaml
   # Edita cortexd.local.yaml según tus necesidades
   ```

3. **Ejecutar el daemon**:
   ```bash
   ./cortexd --config cortexd.local.yaml
   ```

   El daemon iniciará el servidor gRPC en `127.0.0.1:50051` (por defecto).

### Paso 2: Instalar y Ejecutar la Extensión VS Code

1. **Instalar dependencias**:
   ```bash
   npm install
   ```

2. **Compilar TypeScript**:
   ```bash
   npm run compile
   ```

3. **Ejecutar en modo desarrollo**:
   - Abre el proyecto en VS Code
   - Presiona `F5` para lanzar el Extension Development Host
   - Abre un workspace en la nueva ventana
   - La extensión se conectará automáticamente al backend

### Verificar la Conexión

- La extensión mostrará un mensaje de bienvenida si se conecta correctamente
- Si hay problemas, verifica que el daemon esté ejecutándose
- Usa el comando "Backend Admin" para abrir el panel de administración

## ⚙️ Configuración

### Configuración de la Extensión VS Code

Las opciones de configuración están disponibles en VS Code Settings (`Cmd+,` / `Ctrl+,`):

#### Auto-Asignación de Proyectos
```json
{
  "cortex.projectAutoAssign.enabled": true,
  "cortex.projectAutoAssign.windowHours": 6,
  "cortex.projectAutoAssign.minClusterSize": 2,
  "cortex.projectAutoAssign.dominanceThreshold": 0.6,
  "cortex.projectAutoAssign.suggestionThreshold": 0.3
}
```

#### Configuración LLM/AI
Valores por defecto (AI activa):
```json
{
  "cortex.llm.endpoint": "http://localhost:11434",
  "cortex.llm.model": "llama3.2",
  "cortex.llm.maxContextTokens": 2000,
  "cortex.llm.autoSummary.enabled": true,
  "cortex.llm.autoIndex.enabled": true,
  "cortex.llm.autoIndex.applyTags": true,
  "cortex.llm.autoIndex.applyProjects": true
}
```

#### Configuración RAG
```json
{
  "cortex.rag.similarityThreshold": 0.5,
  "cortex.rag.maxSuggestions": 10,
  "cortex.rag.enableSemanticGrouping": true
}
```

#### Configuración de Mirroring
```json
{
  "cortex.mirror.maxConcurrency": 1,
  "cortex.mirror.maxFileSizeMB": 25
}
```

### Configuración del Daemon Go

El daemon usa archivos YAML para configuración:

```yaml
# backend/cortexd.local.yaml
workspace:
  root: "/path/to/workspace"
  excludes:
    - ".git"
    - "node_modules"
    - ".vscode"
    - ".cortex"

database:
  path: "tmp/cortex-test-data/cortex.sqlite"

llm:
  enabled: true
  endpoint: "http://localhost:11434"
  model: "llama3.2"
  embeddings:
    enabled: true
    endpoint: "http://localhost:11434"
    model: "nomic-embed-text"

grpc:
  address: "127.0.0.1:50051"
```

## 📖 Uso Básico

### Flujo Básico

1. **Iniciar el backend daemon** (requerido):
   ```bash
   cd backend
   ./cortexd --config cortexd.local.yaml
   ```

2. **Abrir el workspace en VS Code** - La extensión se conecta automáticamente al backend

3. **El backend indexa automáticamente** - El pipeline procesa todos los archivos en segundo plano

4. **Asignar proyectos** - Right-click en un archivo → "Cortex: Assign project to current file"

5. **Agregar tags** - Right-click en un archivo → "Cortex: Add tag to current file"

6. **Explorar vistas** - Click en el icono de Cortex en Activity Bar para ver las 12 vistas disponibles

7. **Monitorear progreso** - Usa "Pipeline Progress" para ver el estado de indexación en tiempo real

### Comandos Disponibles

#### Gestión de Metadata
- `Cortex: Add tag to current file` - Etiquetar el archivo activo
- `Cortex: Assign project to current file` - Asignar proyecto al archivo activo
- `Cortex: Sync Backend` - Sincronizar con el backend

#### Navegación y Vistas
- `Cortex: Open Cortex View` - Enfocar la sidebar de Cortex
- `Cortex: Enable Accordion Mode` - Activar modo acordeón (auto-colapsar)
- `Cortex: Disable Accordion Mode` - Desactivar modo acordeón
- `Cortex: Expand All` - Expandir todos los nodos
- `Cortex: Collapse All` - Colapsar todos los nodos

#### Indexación
- `Cortex: Rebuild Index` - Rescan completo del workspace (re-procesa todo)
- `Cortex: Auto Index (AI)` - Indexación automática con AI

#### Características AI
- `Cortex: Suggest Tags (AI)` - Obtener sugerencias de tags con AI
- `Cortex: Suggest Project (AI)` - Obtener sugerencias de proyectos con AI
- `Cortex: Generate File Summary (AI)` - Generar resumen AI del archivo
- `Cortex: Ask AI` - Hacer preguntas sobre documentos usando RAG

#### Backend y Administración
- `Backend Admin` - Abrir panel de administración web del backend
- `Pipeline Progress` - Ver progreso del pipeline de indexación en tiempo real

### Ejemplo de Uso

#### Organizando un Proyecto de Cliente
```
Proyecto: "client-acme"
Archivos:
  - contracts/acme-contract.pdf
  - src/acme-integration.ts
  - emails/acme-kickoff.eml
  - designs/acme-mockups.fig
```

#### Rastreando Code Reviews
```
Tag: "needs-review"
Archivos:
  - src/auth/login.ts
  - src/api/endpoints.ts
  - tests/auth.test.ts
```

#### Encontrando Todos los Archivos TypeScript
```
Tipo: "typescript"
Archivos:
  - (todos los archivos .ts y .tsx)
```

## 🏗️ Arquitectura

### Arquitectura Backend-First

```
┌─────────────────────────────────────────────────────────────┐
│  VS Code Extension (TypeScript) - Frontend UI               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  TreeView Providers (12 vistas)                     │   │
│  │  - ContextTreeProvider, TagTreeProvider, etc.       │   │
│  └──────────────────┬─────────────────────────────────┘   │
│                       │                                       │
│  ┌────────────────────▼─────────────────────────────────┐   │
│  │  gRPC Clients                                         │   │
│  │  - GrpcAdminClient, GrpcMetadataClient,              │   │
│  │    GrpcRAGClient, GrpcKnowledgeClient                │   │
│  └──────────────────┬───────────────────────────────────┘   │
└──────────────────────┼───────────────────────────────────────┘
                       │ gRPC (127.0.0.1:50051)
                       │
┌──────────────────────▼───────────────────────────────────────┐
│  Backend Daemon (Go) - Procesamiento Completo                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  gRPC Server (6 Services)                            │   │
│  │  - AdminService, FileService, MetadataService,       │   │
│  │    LLMService, RAGService, KnowledgeService          │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                       │                                       │
│  ┌────────────────────▼─────────────────────────────────┐   │
│  │  Application Layer                                   │   │
│  │  - Pipeline Orchestrator (8 stages)                 │   │
│  │  - Project Service, RAG Service, etc.                │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                       │                                       │
│  ┌────────────────────▼─────────────────────────────────┐   │
│  │  Infrastructure Layer                                │   │
│  │  - SQLite Repositories                               │   │
│  │  - LLM Router, Embedding Service                     │   │
│  │  - File System Watcher                               │   │
│  └──────────────────┬────────────────────────────────────┘   │
│                      │                                        │
│  ┌───────────────────▼──────────────────────────────────┐   │
│  │  SQLite Database                                      │   │
│  │  - files, file_metadata, documents, chunks,          │   │
│  │    projects, relationships, usage_events, etc.      │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────┘
                       ▲
                       │
┌──────────────────────┴───────────────────────────────────────┐
│  File System (Workspace)                                       │
│  - Archivos reales (nunca modificados por Cortex)             │
└───────────────────────────────────────────────────────────────┘
```

### Componentes Principales

#### Extensión VS Code (TypeScript)

- **FileScanner** - Descubre todos los archivos en el workspace
- **IndexStore** - Índice en memoria para búsquedas rápidas
- **MetadataStore** - Persistencia SQLite para tags/proyectos
- **TreeDataProviders** - Proveedores de vistas virtuales
- **Commands** - Comandos de VS Code para operaciones

#### Backend Go

- **Pipeline Orchestrator** - Orquesta el procesamiento de archivos
- **Repositories** - Acceso a datos (SQLite)
- **Services** - Lógica de negocio (Project, Relationship, Usage, Query, RAG)
- **gRPC Handlers** - API gRPC para integraciones
- **Knowledge Engine** - Motor de conocimiento completo

### Modelo de Datos

#### FileIndexEntry (In-Memory)
```typescript
{
  absolutePath: string;      // Ruta completa del sistema
  relativePath: string;      // Ruta relativa al workspace
  filename: string;          // Nombre del archivo
  extension: string;         // Extensión (e.g., ".ts")
  lastModified: number;      // Timestamp (ms)
  fileSize: number;          // Tamaño en bytes
  enhanced?: EnhancedMetadata; // Metadatos enriquecidos
}
```

#### FileMetadata (Persisted in SQLite)
```typescript
{
  file_id: string;           // SHA-256 hash de relative path
  relativePath: string;
  tags: string[];
  contexts: string[];       // Proyectos
  type: string;              // Inferido de extensión
  notes?: string;
  created_at: number;
  updated_at: number;
}
```

### Esquema de Base de Datos

**Tablas Principales**:
- `workspaces` - Workspaces registrados
- `files` - Índice de archivos con flags de indexación
- `file_metadata` - Metadata semántica (tags, contexts, AI summaries)
- `file_tags` - Tags asignados (many-to-many)
- `file_contexts` - Proyectos/contextos asignados (many-to-many)
- `documents` - Documentos parseados (Markdown)
- `chunks` - Chunks de documentos
- `chunk_embeddings` - Vectores de embeddings para RAG
- `projects` - Jerarquía de proyectos
- `document_relationships` - Relaciones entre documentos
- `document_states` - Estados de documentos
- `document_usage_events` - Eventos de uso temporal

## 🤖 Características AI

### Configuración de Ollama

1. **Instalar Ollama**:
   ```bash
   # macOS
   brew install ollama
   
   # Linux
   curl -fsSL https://ollama.com/install.sh | sh
   ```

2. **Iniciar Ollama**:
   ```bash
   ollama serve
   ```

3. **Descargar modelo**:
   ```bash
   ollama pull llama3.2
   ollama pull nomic-embed-text  # Para embeddings
   ```

4. **Configurar en VS Code**:
   - Settings → Buscar "Cortex LLM"
   - Verificar endpoint: `http://localhost:11434`
   - Verificar modelo: `llama3.2`

### Uso de Características AI

#### Sugerencias de Tags
1. Abre un archivo
2. Command Palette → "Cortex: Suggest Tags (AI)"
3. Selecciona tags para aplicar

#### Sugerencias de Proyectos
1. Abre un archivo
2. Command Palette → "Cortex: Suggest Project (AI)"
3. Acepta, edita o rechaza la sugerencia

#### Resúmenes de Archivos
1. Abre un archivo
2. Command Palette → "Cortex: Generate File Summary (AI)"
3. Elige guardar como notas, agregar a notas existentes, o copiar

## 🔍 Knowledge Engine

El Knowledge Engine es un sistema completo de gestión de conocimiento que incluye:

### Estados de Documentos
- **Draft** - Borrador
- **Active** - Activo/actual
- **Replaced** - Reemplazado por otro documento
- **Archived** - Archivado

### Relaciones entre Documentos
- **Replaces** - Este documento reemplaza a otro
- **DependsOn** - Este documento depende de otro
- **BelongsTo** - Este documento pertenece a otro
- **References** - Este documento referencia a otro
- **ParentOf** - Este documento es padre de otro

### Jerarquía de Proyectos
- Proyectos pueden tener subproyectos
- Path jerárquico automático: "parent/child/grandchild"
- Consultas que incluyen subproyectos

### Memoria Temporal
- Eventos de uso (Opened, Edited, Searched, Referenced, Indexed)
- Estadísticas de uso (frecuencia, co-ocurrencias)
- Análisis de clusters de conocimiento

### Búsqueda Semántica (RAG)
- Embeddings de documentos
- Búsqueda por similitud semántica
- Sugerencias mejoradas usando contexto

### Consultas Declarativas
```go
Query(workspaceID).
    Filter(ProjectFilter{ProjectID: "x", IncludeSubprojects: true}).
    Filter(StateFilter{States: []DocumentState{DocumentStateActive}}).
    OrderBy("updated_at", true).
    Limit(10).
    Execute(ctx)
```

## 📁 Estructura del Proyecto

```
cortex/
├── src/                          # Código fuente TypeScript
│   ├── extension.ts             # Punto de entrada principal
│   ├── core/                     # Componentes core
│   │   ├── FileScanner.ts        # Escaneo de workspace
│   │   ├── IndexStore.ts         # Índice en memoria
│   │   ├── MetadataStore.ts      # Persistencia SQLite
│   │   └── ...
│   ├── views/                     # Proveedores de vistas
│   │   ├── ContextTreeProvider.ts # Vista por proyecto
│   │   ├── TagTreeProvider.ts     # Vista por tag
│   │   └── ...
│   ├── commands/                  # Comandos VS Code
│   │   ├── addTag.ts
│   │   ├── assignContext.ts
│   │   └── ...
│   ├── services/                  # Servicios
│   │   └── LLMService.ts          # Integración LLM
│   └── utils/                     # Utilidades
├── backend/                       # Backend Go
│   ├── cmd/
│   │   └── cortexd/               # Daemon principal
│   ├── internal/
│   │   ├── domain/                # Entidades de dominio
│   │   ├── application/           # Lógica de aplicación
│   │   │   ├── pipeline/          # Pipeline de indexación
│   │   │   ├── project/           # Servicio de proyectos
│   │   │   ├── relationship/      # Servicio de relaciones
│   │   │   ├── usage/             # Servicio de uso
│   │   │   ├── query/             # Servicio de consultas
│   │   │   └── rag/               # Servicio RAG
│   │   ├── infrastructure/        # Infraestructura
│   │   │   ├── persistence/       # Repositorios SQLite
│   │   │   ├── llm/               # Integración LLM
│   │   │   └── filesystem/        # Sistema de archivos
│   │   └── interfaces/            # Interfaces
│   │       └── grpc/              # Handlers gRPC
│   ├── api/
│   │   └── proto/                 # Definiciones gRPC
│   └── configs/                   # Archivos de configuración
├── docs/                          # Documentación
├── resources/                     # Recursos (iconos, etc.)
├── package.json                   # Manifest de extensión
└── tsconfig.json                  # Configuración TypeScript
```

## 🛠️ Desarrollo

### Build & Run

```bash
# Instalar dependencias
npm install

# Compilar TypeScript
npm run compile

# Modo watch (auto-compile)
npm run watch

# Ejecutar tests
npm test

# Lint
npm run lint
```

### Debugging

1. Abre el proyecto en VS Code
2. Presiona `F5` para lanzar Extension Development Host
3. Abre un workspace en la nueva ventana
4. Usa Debug Console para ver logs

### Testing

```bash
# Tests unitarios
npm test

# Tests de integración (backend)
cd backend
go test ./...
```

### Empaquetar Extensión

```bash
# Instalar vsce
npm install -g @vscode/vsce

# Crear paquete .vsix
vsce package
```

## 📊 Estado del Proyecto

### ✅ Completado (Core Features - ~90%)

#### Backend (Go)
- ✅ Daemon completo con 6 servicios gRPC
- ✅ Pipeline de indexación multi-etapa (8 stages)
- ✅ Persistencia SQLite con migraciones
- ✅ Knowledge Engine completo
- ✅ RAG con embeddings (nomic-embed-text)
- ✅ LLM Router con múltiples proveedores
- ✅ Extracción de contenido (PDF, Office docs)
- ✅ Análisis de código y métricas
- ✅ Detección de relaciones entre documentos
- ✅ Gestión de estados de documentos
- ✅ Jerarquía de proyectos
- ✅ Memoria temporal y analytics
- ✅ Panel de administración web
- ✅ Actualizaciones en tiempo real
- ✅ Progreso de indexación en tiempo real

#### Frontend (VS Code Extension)
- ✅ 12 vistas virtuales implementadas
- ✅ Comandos completos (tags, proyectos, AI, etc.)
- ✅ Integración gRPC con backend
- ✅ Actualizaciones en tiempo real
- ✅ Panel de progreso del pipeline
- ✅ Modo acordeón para vistas
- ✅ Comando "Ask AI" para consultas RAG

### 🔄 Funcionalidades Mejorables

- ⚠️ **Creación automática de proyectos**: Actualmente solo sugiere proyectos, no los crea automáticamente. Se puede mejorar para crear proyectos cuando la confianza es alta.
- ⚠️ **Integración MCP para Cursor**: La API gRPC existe pero falta un servidor MCP que exponga Cortex como herramienta para Cursor.

### 🔮 Propuesto (Futuro)

#### Visualización
- 🔮 **Generación de estructuras de visualización**: Grafos de relaciones, heatmaps de co-ocurrencias, visualización de jerarquías de proyectos
- 🔮 **Vista de grafo interactiva**: Visualización de relaciones entre documentos en un grafo navegable
- 🔮 **Tag cloud visual**: Representación visual de tags y su frecuencia

#### Integraciones
- 🔮 **Servidor MCP para Cursor**: Exponer Cortex como herramienta MCP para que Cursor pueda usar Cortex como memoria de largo plazo
- 🔮 **Endpoints específicos para AI writing tools**: APIs especializadas para herramientas de escritura asistida por IA

#### UX Mejorada
- 🔮 **Multi-selección**: Operaciones en múltiples archivos a la vez
- 🔮 **Quick pick suggestions**: Autocompletado inteligente para tags y proyectos
- 🔮 **Badges inline**: Mostrar tags/proyectos directamente en el File Explorer
- 🔮 **Búsqueda/filtro en vistas**: Filtrar contenido dentro de las vistas

#### Metadata Avanzada
- 🔮 **Notas de archivo**: Editor de notas enriquecido
- 🔮 **Campos personalizados**: Metadata definida por el usuario
- 🔮 **Relaciones entre archivos**: UI para gestionar relaciones manualmente

#### Indexación Inteligente
- 🔮 **Búsqueda de contenido**: Búsqueda full-text en contenido de archivos
- 🔮 **Tracking de imports/exports**: Análisis de dependencias de código
- 🔮 **Integración Git avanzada**: Historial de cambios, blame, etc.

## 🗺️ Roadmap

### ✅ Fase 1: MVP Core (Completo)
- ✅ Indexación backend completa
- ✅ 12 vistas virtuales
- ✅ Comandos esenciales
- ✅ API gRPC completa
- ✅ Knowledge Engine
- ✅ RAG con embeddings
- ✅ Características AI

### 🔄 Fase 2: Mejoras de UX (Propuesto)
- 🔮 Multi-selección de archivos
- 🔮 Quick pick suggestions con autocompletado
- 🔮 Badges inline en File Explorer
- 🔮 Búsqueda/filtro dentro de vistas
- 🔮 Mejores visualizaciones de progreso

### 🔮 Fase 3: Visualización (Propuesto)
- 🔮 Vista de grafo de relaciones
- 🔮 Tag cloud visual
- 🔮 Timeline view interactiva
- 🔮 Heatmaps de co-ocurrencias
- 🔮 Visualización de jerarquías de proyectos

### 🔮 Fase 4: Integraciones (Propuesto)
- 🔮 Servidor MCP para Cursor
- 🔮 Endpoints especializados para AI writing tools
- 🔮 Integración con más editores (Neovim, etc.)
- 🔮 API REST adicional (además de gRPC)

### 🔮 Fase 5: Metadata Avanzada (Propuesto)
- 🔮 Editor de notas enriquecido
- 🔮 Campos personalizados definidos por usuario
- 🔮 UI para gestionar relaciones manualmente
- 🔮 Templates de metadata

### 🔮 Fase 6: Indexación Inteligente (Propuesto)
- 🔮 Búsqueda full-text en contenido
- 🔮 Tracking de imports/exports de código
- 🔮 Análisis de dependencias
- 🔮 Integración Git avanzada (historial, blame, etc.)

### 🔮 Fase 7: Colaboración (Propuesto)
- 🔮 Proyectos compartidos vía Git
- 🔮 Templates de proyectos compartibles
- 🔮 Export/import de metadata
- 🔮 Sincronización multi-dispositivo (opcional)

## 🤝 Contribuir

Ver [CONTRIBUTING.md](docs/CONTRIBUTING.md) para guías de contribución.

## 📝 Documentación Adicional

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - Arquitectura técnica detallada
- [SETUP.md](docs/SETUP.md) - Guía de instalación y configuración
- [EXAMPLES.md](docs/EXAMPLES.md) - Ejemplos de uso real
- [ROADMAP.md](docs/ROADMAP.md) - Plan de desarrollo futuro
- [LLM_SETUP.md](docs/LLM_SETUP.md) - Configuración de características AI
- [DEEP_METADATA.md](docs/DEEP_METADATA.md) - Extracción de metadatos profunda
- [KNOWLEDGE_ENGINE_ARCHITECTURE.md](docs/KNOWLEDGE_ENGINE_ARCHITECTURE.md) - Arquitectura del Knowledge Engine
- [RAG_IMPLEMENTATION_SUMMARY.md](docs/RAG_IMPLEMENTATION_SUMMARY.md) - Implementación RAG
- [FUTURE_RESEARCH_PROPOSALS.md](docs/FUTURE_RESEARCH_PROPOSALS.md) - Propuestas basadas en investigación académica
- [QUALITY_TESTING.md](docs/QUALITY_TESTING.md) - Tests de calidad y métricas de evaluación

## 📄 Licencia

MIT

---

**Cortex** - Tu workspace, organizado semánticamente. 🧠✨
