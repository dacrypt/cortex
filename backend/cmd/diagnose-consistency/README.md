# Diagnóstico de Consistencia: Documentos vs Archivos

Esta herramienta verifica la consistencia entre la base de datos de documentos y la base de datos de archivos, identificando:

1. **Documentos sin archivos correspondientes**: Documentos que existen en la base de datos pero no tienen un archivo correspondiente en el índice de archivos
2. **Archivos sin documentos correspondientes**: Archivos que están en el índice pero no tienen un documento correspondiente
3. **Problemas de normalización de rutas**: Casos donde las rutas no coinciden exactamente pero son equivalentes (diferentes separadores de ruta)

## Uso

### Compilar

```bash
cd backend
make diagnose-build
```

O directamente:

```bash
go build -o diagnose-consistency ./cmd/diagnose-consistency
```

### Ejecutar

```bash
# Verificar todos los workspaces
./diagnose-consistency -db /path/to/.cortex/index.sqlite

# Verificar un workspace específico
./diagnose-consistency -db /path/to/.cortex/index.sqlite -workspace <workspace-id>
```

### Usando Make

```bash
# Verificar todos los workspaces
make diagnose DB=/path/to/.cortex/index.sqlite

# Verificar un workspace específico
make diagnose DB=/path/to/.cortex/index.sqlite WORKSPACE=<workspace-id>
```

## Ejemplo de Salida

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 DIAGNÓSTICO DE CONSISTENCIA: Documentos vs Archivos
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Verificando documentos sin archivos correspondientes...
⚠️  Encontrados 3 documentos sin archivos correspondientes:
   - Libros/documento1.pdf (workspace: abc123)
   - Libros/documento2.pdf (workspace: abc123)
   - docs/README.md (workspace: def456)

📁 Verificando archivos sin documentos correspondientes...
✅ Todos los archivos tienen documentos correspondientes

🔄 Verificando problemas de normalización de rutas...
⚠️  Encontrados 2 problemas de normalización de rutas:
   - Documento: Libros\documento3.pdf
     Archivo:   Libros/documento3.pdf
     Workspace: abc123

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 RESUMEN
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Documentos sin archivos: 3
Archivos sin documentos: 0
Problemas de normalización: 2
```

## Interpretación de Resultados

### Documentos sin archivos
Esto puede indicar:
- Archivos que fueron eliminados del sistema de archivos pero sus documentos aún existen
- Archivos que fueron movidos/renombrados
- Problemas durante el proceso de indexación

### Archivos sin documentos
Esto puede indicar:
- Archivos que aún no han sido procesados por el DocumentStage
- Archivos que fallaron durante el procesamiento de documentos
- Archivos que no son procesables (no pasan `CanProcess`)

### Problemas de normalización
Esto indica:
- Rutas almacenadas con diferentes separadores (Windows `\` vs Unix `/`)
- Estos casos deberían ser manejados automáticamente por `GetByPath` con normalización

## Soluciones

Si encuentras inconsistencias:

1. **Documentos sin archivos**: Considera limpiar documentos huérfanos o reindexar los archivos
2. **Archivos sin documentos**: Ejecuta un reindex completo o verifica por qué el DocumentStage no procesa esos archivos
3. **Normalización**: El código actual debería manejar esto automáticamente, pero puedes ejecutar una migración para normalizar todas las rutas


