# Garantía de Calidad con AI/LLM - Implementación

## Principio Fundamental

**Cortex garantiza calidad mediante el uso obligatorio de AI/LLM en todas las operaciones críticas.** No se permite ninguna operación que no esté validada por AI.

## Implementación

### 1. Servicio de Calidad AI (`src/services/AIQualityService.ts`)

Servicio centralizado que garantiza:
- ✅ Validación obligatoria de disponibilidad LLM antes de cualquier operación
- ✅ Sugerencias AI para naturaleza de proyectos
- ✅ Generación AI de descripciones
- ✅ Validación AI de nombres de proyectos
- ✅ Sugerencias AI de atributos (status, priority, etc.)
- ✅ Sugerencias AI de proyectos basadas en contenido de archivos

**Métodos principales:**
- `validateLLMAvailable()`: Verifica que LLM esté disponible
- `requireLLM()`: **BLOQUEA** operaciones si LLM no está disponible
- `suggestProjectNature()`: AI sugiere naturaleza basada en nombre/contexto
- `generateProjectDescription()`: AI genera descripción profesional
- `validateProjectName()`: AI valida y sugiere mejoras al nombre
- `suggestProjectForFile()`: AI sugiere proyecto basado en contenido

### 2. Comandos con Validación AI Obligatoria

#### `createProjectCommand`
**Flujo con AI obligatorio:**
1. ✅ **Validación LLM**: Verifica que LLM esté disponible (BLOQUEA si no)
2. ✅ **Validación de nombre**: AI valida el nombre del proyecto
3. ✅ **Sugerencia de naturaleza**: AI sugiere naturaleza basada en nombre
4. ✅ **Generación de descripción**: AI genera descripción si no se proporciona
5. ✅ **Sugerencia de atributos**: AI sugiere status, priority, etc.

**Bloqueos:**
- ❌ No se puede crear proyecto sin LLM disponible
- ❌ No se puede crear proyecto sin validación AI

#### `assignProjectCommand`
**Flujo con AI obligatorio:**
1. ✅ **Validación LLM**: Verifica que LLM esté disponible (BLOQUEA si no)
2. ✅ **Análisis de contenido**: AI analiza el contenido del archivo
3. ✅ **Sugerencia de proyecto**: AI sugiere proyecto basado en contenido
4. ✅ **Validación**: Usuario puede aceptar o modificar sugerencia AI

**Bloqueos:**
- ❌ No se puede asignar proyecto sin LLM disponible
- ❌ No se puede asignar proyecto sin análisis AI del contenido

#### `editProjectCommand`
**Flujo con AI obligatorio:**
1. ✅ **Validación LLM**: Verifica que LLM esté disponible antes de editar
2. ✅ **Validación de cambios**: AI valida cambios propuestos

### 3. Mensajes de Error Claros

Cuando LLM no está disponible, Cortex muestra mensajes claros:

```
❌ CRITICAL: LLM is not available. [detalles del error]

Cortex requires AI/LLM to be available for quality assurance.

Please:
1. Ensure Ollama or your LLM service is running
2. Check your Cortex backend configuration
3. Verify LLM providers are properly configured

Operations cannot proceed without AI validation.
```

### 4. Integración con Backend

El backend también valida LLM en operaciones críticas:

**Backend AI Stage (`backend/internal/application/pipeline/stages/ai.go`):**
```go
if !s.llmRouter.IsAvailable(ctx) {
    err := fmt.Errorf("CRITICAL: LLM not available - AI stage requires LLM to be available")
    // Logs error and returns
    return err
}
```

**Backend Main (`backend/cmd/cortexd/main.go`):**
- Valida que al menos un proveedor LLM esté registrado
- Verifica que el proveedor activo esté disponible
- **FATAL ERROR** si no hay proveedores disponibles (en modo LLM habilitado)

## Flujos de Usuario con AI

### Crear Proyecto con AI

1. Usuario ejecuta "Create New Project"
2. **Sistema valida LLM** → Si no disponible, BLOQUEA con mensaje claro
3. Usuario ingresa nombre
4. **AI valida nombre** → Sugiere mejoras si necesario
5. **AI sugiere naturaleza** → Muestra sugerencia con confianza
6. Usuario puede aceptar o seleccionar otra naturaleza
7. **AI genera descripción** → Usuario puede editar
8. **AI sugiere atributos** → Status, priority, etc.
9. Proyecto creado con validación AI completa

### Asignar Proyecto con AI

1. Usuario ejecuta "Assign Project" en archivo abierto
2. **Sistema valida LLM** → Si no disponible, BLOQUEA
3. **AI analiza contenido** del archivo
4. **AI sugiere proyecto** basado en:
   - Contenido del archivo
   - Proyectos existentes
   - Contexto del workspace
5. Usuario ve sugerencia con:
   - Nombre del proyecto sugerido
   - Nivel de confianza (%)
   - Razón de la sugerencia
6. Usuario puede aceptar o modificar
7. Proyecto asignado con validación AI

## Configuración Requerida

### Backend (`cortexd.yaml`)
```yaml
llm:
  enabled: true  # MANDATORY: Must be true
  default_provider: "ollama"
  default_model: "llama3.2"
  providers:
    - id: "ollama"
      type: "ollama"
      endpoint: "http://localhost:11434"
```

### Verificación de Disponibilidad

El sistema verifica LLM en múltiples niveles:
1. **Frontend**: Antes de permitir comandos
2. **Backend**: Antes de procesar archivos
3. **Pipeline**: En cada etapa AI

## Garantías de Calidad

### ✅ Garantizado
- Todos los proyectos tienen naturaleza validada por AI
- Todas las descripciones son generadas o validadas por AI
- Todos los nombres son validados por AI
- Todas las asignaciones de proyectos usan análisis AI del contenido
- No se permite ninguna operación sin LLM disponible

### ❌ Bloqueado
- Crear proyecto sin LLM
- Asignar proyecto sin análisis AI
- Operaciones que no pasen validación AI
- Continuar con LLM no disponible

## Mensajes de Usuario

### Éxito
```
✓ Created project "Mi Libro" (Libro) with AI validation
✓ Project "Mi Libro" assigned to documento.md (AI-validated)
```

### Error (LLM no disponible)
```
❌ CRITICAL: LLM is not available. [detalles]

Cortex requires AI/LLM to be available for quality assurance.
Please ensure your LLM service is running and configured.

Operations cannot proceed without AI validation.
```

### Progreso
```
Validating AI availability...
AI analyzing project...
AI suggesting project nature... (85% confidence)
AI generating description...
```

## Arquitectura

```
User Command
    ↓
AIQualityService.requireLLM()  ← BLOQUEA si no disponible
    ↓
AI Analysis/Validation
    ↓
User Confirmation (puede modificar)
    ↓
Operation with AI-validated data
```

## Testing

Para probar sin LLM (debería fallar):
1. Detener Ollama/LM Studio
2. Intentar crear proyecto → Debe mostrar error claro
3. Intentar asignar proyecto → Debe mostrar error claro

Para probar con LLM:
1. Asegurar Ollama corriendo
2. Verificar configuración backend
3. Crear proyecto → Debe mostrar sugerencias AI
4. Asignar proyecto → Debe analizar contenido y sugerir

## Próximas Mejoras

1. **Cache de validaciones AI**: Cachear validaciones para mejorar performance
2. **Métricas de calidad**: Tracking de confianza AI en operaciones
3. **Aprendizaje continuo**: Mejorar sugerencias basadas en feedback
4. **Validación multi-modelo**: Usar múltiples modelos para mayor confianza
5. **Validación incremental**: Validar cambios incrementales con AI

## Conclusión

Cortex ahora **garantiza calidad mediante AI obligatorio**. Todas las operaciones críticas requieren validación AI, y el sistema bloquea operaciones si LLM no está disponible. Esto asegura que:

- ✅ Todos los proyectos tienen naturaleza apropiada
- ✅ Todas las descripciones son profesionales
- ✅ Todas las asignaciones son contextualmente relevantes
- ✅ La calidad es consistente y garantizada







