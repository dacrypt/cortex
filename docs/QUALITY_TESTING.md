# Tests de Calidad para Cortex

Este documento describe los tests de integración creados para evaluar la calidad de cada etapa del pipeline de Cortex y verificar que cumple con sus objetivos principales.

## Objetivo

Los tests de calidad evalúan:
1. **Precisión**: ¿Los resultados son correctos?
2. **Completitud**: ¿Se procesa toda la información necesaria?
3. **Consistencia**: ¿Los resultados son reproducibles?
4. **Rendimiento**: ¿El procesamiento es eficiente?
5. **Objetivos del Sistema**: ¿Cumple con los principios fundamentales?

## Estructura de Tests

### 1. Tests por Etapa del Pipeline

#### `TestQuality_BasicStage`
Evalúa la calidad de la extracción de metadatos básicos:
- ✅ Completitud de metadatos (tamaño, timestamp, extensión)
- ✅ Precisión de los valores almacenados
- ✅ Tasa de éxito (debe ser >= 95%)
- ✅ Tiempo de procesamiento (< 100ms por archivo)
- ✅ Calidad general (score >= 0.90)

**Métricas evaluadas:**
- `FilesProcessed`: Total de archivos procesados
- `FilesSucceeded`: Archivos procesados exitosamente
- `SuccessRate`: Tasa de éxito (succeeded / processed)
- `AverageProcessingTime`: Tiempo promedio de procesamiento
- `QualityScore`: Puntuación general (0-1)

#### `TestQuality_MimeStage`
Evalúa la calidad de la detección de tipos MIME:
- ✅ Precisión de detección MIME
- ✅ Categorización correcta (text, image, document, code)
- ✅ Manejo de archivos binarios
- ✅ Tasa de éxito (>= 90%)
- ✅ Calidad general (score >= 0.85)

#### `TestQuality_DocumentStage`
Evalúa la calidad del parsing y chunking de documentos:
- ✅ Extracción de frontmatter
- ✅ Creación de chunks apropiados
- ✅ Cobertura de contenido (>= 70%)
- ✅ Tamaños de chunks razonables (50-2000 caracteres)
- ✅ Generación de embeddings
- ✅ Calidad general (score >= 0.80)

#### `TestQuality_RelationshipStage`
Evalúa la calidad de la detección de relaciones:
- ✅ Detección de relaciones desde frontmatter
- ✅ Detección de relaciones desde enlaces Markdown
- ✅ Resolución correcta de paths a DocumentIDs
- ✅ Calidad general (score >= 0.80)

#### `TestQuality_StateStage`
Evalúa la calidad de la inferencia de estados:
- ✅ Inferencia correcta de estados (draft, active, replaced, archived)
- ✅ Transiciones de estado apropiadas
- ✅ Tasa de éxito (>= 95%)
- ✅ Calidad general (score >= 0.85)

#### `TestQuality_AIStage`
Evalúa la calidad de las sugerencias AI:
- ✅ Generación de categorías
- ✅ Generación de resúmenes
- ✅ Sugerencias de tags y proyectos
- ✅ Tasa de éxito (>= 80%, AI es opcional)
- ⚠️ Requiere LLM disponible

### 2. Tests de RAG

#### `TestQuality_RAG`
Evalúa la calidad del sistema RAG:
- ✅ Relevancia de resultados de búsqueda
- ✅ Precisión de embeddings
- ✅ Calidad de respuestas generadas
- ✅ Tiempo de respuesta (< 2 segundos)
- ✅ Tasa de éxito (>= 90%)
- ✅ Calidad general (score >= 0.75)

#### `TestQuality_EmbeddingConsistency`
Evalúa la consistencia de embeddings:
- ✅ Consistencia de embeddings para el mismo texto
- ✅ Dimensiones correctas
- ✅ Generación sin errores

#### `TestQuality_ChunkingStrategy`
Evalúa la estrategia de chunking:
- ✅ Número apropiado de chunks
- ✅ Tamaños de chunks razonables
- ✅ Cobertura de contenido
- ✅ Heading paths correctos
- ✅ Calidad general (score >= 0.80)

### 3. Tests de Objetivos del Sistema

#### `TestQuality_CoreObjectives`
Evalúa que Cortex cumple con sus objetivos principales:

**Objetivo 1: Files Stay in Place**
- ✅ Archivos no se mueven
- ✅ Contenido no se modifica
- ✅ Permisos no cambian

**Objetivo 2: Multiple Virtual Views**
- ✅ Vistas por tag funcionan
- ✅ Vistas por proyecto funcionan
- ✅ Vistas por tipo funcionan
- ✅ Múltiples vistas sobre los mismos archivos

**Objetivo 3: Multiple Projects Per File**
- ✅ Archivos pueden pertenecer a múltiples proyectos
- ✅ Archivo aparece en todas las vistas de proyecto
- ✅ Archivo sigue existiendo en una sola ubicación

**Objetivo 4: Local-First and Deterministic**
- ✅ Datos almacenados localmente
- ✅ Mismo input produce mismo output
- ✅ Comportamiento idempotente

#### `TestQuality_SemanticOrganization`
Evalúa la organización semántica:
- ✅ Consultas por tag
- ✅ Consultas por proyecto
- ✅ Consultas por múltiples tags
- ✅ Agrupación semántica correcta

#### `TestQuality_NonDestructive`
Evalúa que Cortex nunca modifica archivos fuente:
- ✅ Contenido de archivos no cambia
- ✅ Timestamps no cambian significativamente
- ✅ Permisos no cambian

#### `TestQuality_Consistency`
Evalúa la consistencia de datos:
- ✅ Tags se mantienen consistentes
- ✅ Proyectos se mantienen consistentes
- ✅ Múltiples consultas devuelven resultados consistentes

### 4. Tests End-to-End

#### `TestQuality_EndToEnd`
Evalúa el pipeline completo:
- ✅ Todos los stages se ejecutan correctamente
- ✅ Datos fluyen correctamente entre stages
- ✅ Resultado final es completo y correcto
- ✅ Tiempo total razonable (< 5 segundos)
- ✅ Calidad general (score >= 0.85)

## Métricas de Calidad

### QualityMetrics

Cada test genera un objeto `QualityMetrics` con:

```go
type QualityMetrics struct {
    StageName           string
    FilesProcessed      int
    FilesSucceeded      int
    FilesFailed         int
    SuccessRate         float64
    AverageProcessingTime time.Duration
    QualityScore         float64 // 0-1
    Issues              []QualityIssue
}
```

### QualityIssue

Cada problema encontrado se registra como:

```go
type QualityIssue struct {
    Severity    string // "error", "warning", "info"
    Stage       string
    FilePath    string
    Description string
    Expected    interface{}
    Actual      interface{}
}
```

### Cálculo de Quality Score

```
QualityScore = SuccessRate * (1.0 - errorCount*0.1 - warningCount*0.05 - infoCount*0.02)
```

- **SuccessRate**: Tasa de éxito del procesamiento
- **errorCount**: Número de errores encontrados
- **warningCount**: Número de advertencias
- **infoCount**: Número de problemas informativos

## Umbrales de Calidad

### Por Etapa

| Etapa | Success Rate | Quality Score | Processing Time |
|-------|--------------|---------------|-----------------|
| Basic | >= 95% | >= 0.90 | < 100ms |
| MIME | >= 90% | >= 0.85 | - |
| Document | >= 95% | >= 0.80 | - |
| Relationship | >= 95% | >= 0.80 | - |
| State | >= 95% | >= 0.85 | - |
| AI | >= 80% | - | - |
| RAG | >= 90% | >= 0.75 | < 2s |
| End-to-End | >= 95% | >= 0.85 | < 5s |

### Por Objetivo del Sistema

- **Files Stay in Place**: 100% (no se permite ningún cambio)
- **Multiple Views**: 100% (todas las vistas deben funcionar)
- **Multiple Projects**: 100% (archivos deben aparecer en todos los proyectos)
- **Local-First**: 100% (todo debe almacenarse localmente)
- **Non-Destructive**: 100% (ningún archivo debe modificarse)

## Ejecución de Tests

### Ejecutar Todos los Tests de Calidad

```bash
cd backend
go test -v ./internal/application/pipeline -run TestQuality
```

### Ejecutar Test Específico

```bash
go test -v ./internal/application/pipeline -run TestQuality_BasicStage
```

### Ejecutar con Cobertura

```bash
go test -v -cover ./internal/application/pipeline -run TestQuality
```

### Ejecutar con Perfil de Rendimiento

```bash
go test -v -bench=. -cpuprofile=cpu.prof ./internal/application/pipeline
```

## Interpretación de Resultados

### Success Rate
- **>= 95%**: Excelente
- **90-95%**: Bueno, revisar casos fallidos
- **< 90%**: Requiere atención inmediata

### Quality Score
- **>= 0.90**: Excelente calidad
- **0.80-0.90**: Buena calidad, mejoras menores
- **0.70-0.80**: Calidad aceptable, mejoras necesarias
- **< 0.70**: Requiere correcciones significativas

### Processing Time
- Debe ser consistente y predecible
- Aumentos significativos indican problemas de rendimiento
- Comparar con umbrales por etapa

### Issues
- **Errors**: Deben corregirse inmediatamente
- **Warnings**: Deben investigarse y corregirse
- **Info**: Mejoras opcionales

## Mejoras Continuas

### Monitoreo Regular
- Ejecutar tests de calidad en CI/CD
- Comparar métricas entre versiones
- Identificar regresiones temprano

### Análisis de Tendencias
- Trackear Quality Score a lo largo del tiempo
- Identificar etapas con degradación
- Priorizar mejoras basadas en métricas

### Optimización
- Identificar etapas lentas
- Optimizar algoritmos con bajo Quality Score
- Mejorar manejo de casos edge

## Notas de Implementación

⚠️ **Estado Actual**: Los tests están estructurados pero requieren correcciones de API para compilar correctamente. Las APIs reales de los repositorios difieren ligeramente de las asumidas en los tests.

**Próximos Pasos**:
1. Corregir llamadas a APIs para usar métodos reales
2. Ajustar estructuras para usar tipos correctos
3. Implementar mocks para dependencias externas (LLM)
4. Agregar tests de integración con bases de datos reales
5. Configurar CI/CD para ejecutar tests automáticamente

## Referencias

- [Pipeline Architecture](../docs/ARCHITECTURE.md)
- [Integration Tests](../backend/internal/application/pipeline/integration_test.go)
- [Quality Metrics Definition](../backend/internal/application/pipeline/quality_test.go)







