# Carga de Configuración del Backend

## Archivos de Configuración

Hay 3 archivos de configuración en el proyecto:

1. **`backend/configs/cortexd.yaml.example`** - Template/ejemplo (NO se usa)
2. **`backend/cortexd.yaml`** - Configuración por defecto
3. **`backend/cortexd.local.yaml`** - Configuración local (requiere flag `-config`)

## Orden de Prioridad

### 1. Flag `-config` (Máxima Prioridad)
Si se ejecuta con `-config`, se usa ese archivo específico:
```bash
./cortexd -config cortexd.local.yaml
```

### 2. Búsqueda Automática (Sin flag)
Si NO se pasa `-config`, Viper busca archivos con nombre `cortexd` (sin extensión) en estos directorios (en orden):

1. **Directorio actual** (donde se ejecuta el comando)
   - Busca: `./cortexd.yaml`, `./cortexd.yml`, etc.

2. **`~/.cortex/`** (directorio de datos por defecto)
   - Busca: `~/.cortex/cortexd.yaml`, etc.

3. **`/etc/cortex/`** (configuración del sistema)
   - Busca: `/etc/cortex/cortexd.yaml`, etc.

### 3. Variables de Entorno
Las variables de entorno con prefijo `CORTEX_` pueden sobrescribir valores:
- `CORTEX_GRPC_ADDRESS` → `grpc_address`
- `CORTEX_LLM_ENABLED` → `llm.enabled`
- etc.

### 4. Valores por Defecto
Si no se encuentra ningún archivo, se usan los valores por defecto definidos en `DefaultConfig()`.

## Estado Actual

### `backend/cortexd.yaml`
- ✅ **Se usa automáticamente** si se ejecuta desde `backend/`
- Log level: `debug`
- Embeddings: **NO configurado** (usa defaults)

### `backend/cortexd.local.yaml`
- ❌ **NO se usa automáticamente** (nombre diferente)
- ✅ Se puede usar con: `./cortexd -config cortexd.local.yaml`
- Log level: `trace`
- Embeddings: **NO configurado** (usa defaults)

### `backend/configs/cortexd.yaml.example`
- ❌ **NO se usa** (es solo un template)
- Propósito: Documentación y referencia

## Problema Identificado

**Ninguno de los archivos de configuración tiene configurado `llm.embeddings`**, lo que significa que:

- `llm.embeddings.enabled` → Usa default: `true`
- `llm.embeddings.endpoint` → Usa default: `http://localhost:11434`
- `llm.embeddings.model` → Usa default: `nomic-embed-text`

Pero para que las mejoras RAG funcionen correctamente, deberíamos verificar/agregar esta configuración explícitamente.

## Recomendación

Agregar configuración de embeddings a los archivos activos:

```yaml
llm:
  # ... configuración existente ...
  embeddings:
    enabled: true
    endpoint: "http://localhost:11434"
    model: "nomic-embed-text"  # o el modelo que uses
```







