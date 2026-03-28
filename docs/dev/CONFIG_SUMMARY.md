# Resumen de Configuración

## Archivos de Configuración

### 1. `backend/configs/cortexd.yaml.example`
- **Propósito**: Template/ejemplo de configuración
- **Se usa**: ❌ NO (solo referencia)
- **Embeddings**: ✅ Configurado (ejemplo)

### 2. `backend/cortexd.yaml`
- **Propósito**: Configuración por defecto
- **Se usa**: ✅ SÍ (si se ejecuta desde `backend/` sin flag `-config`)
- **Log level**: `debug`
- **Embeddings**: ✅ Configurado (agregado)

### 3. `backend/cortexd.local.yaml`
- **Propósito**: Configuración local para desarrollo
- **Se usa**: ✅ SÍ (con flag `-config cortexd.local.yaml`)
- **Log level**: `trace` (más verboso)
- **Embeddings**: ✅ Configurado (agregado)

## Cómo se Carga

### Opción 1: Sin flag (automático)
```bash
cd backend
./cortexd
```
→ Busca `cortexd.yaml` en el directorio actual

### Opción 2: Con flag (explícito)
```bash
cd backend
./cortexd -config cortexd.local.yaml
```
→ Usa el archivo especificado

### Opción 3: Desde otro directorio
```bash
./cortexd -config /path/to/backend/cortexd.yaml
```

## Configuración de Embeddings (RAG)

Todos los archivos ahora incluyen:

```yaml
llm:
  embeddings:
    enabled: true
    endpoint: "http://localhost:11434"
    model: "nomic-embed-text"
```

**Importante**: Asegúrate de tener el modelo de embeddings instalado en Ollama:
```bash
ollama pull nomic-embed-text
```

## Diferencias Clave

| Archivo | Log Level | Embeddings | Uso |
|---------|-----------|------------|-----|
| `cortexd.yaml` | `debug` | ✅ | Desarrollo normal |
| `cortexd.local.yaml` | `trace` | ✅ | Debugging detallado |
| `cortexd.yaml.example` | `info` | ✅ | Solo referencia |

## Verificación

Para verificar qué archivo se está usando, revisa los logs al iniciar:
```
Starting Cortex daemon
  config_path: /path/to/config.yaml
```







