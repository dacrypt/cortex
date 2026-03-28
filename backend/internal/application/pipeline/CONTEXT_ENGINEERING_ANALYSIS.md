# Análisis de Context Engineering - Test E2E Pipeline

**Fecha**: 2025-12-25  
**Test**: `TestVerbosePipelineSingleFile`  
**Archivo procesado**: `Libros/Mis conversaciones con las Almas del Purgatorio - Eugenia von der Leyen.pdf`

## 📊 Resumen Ejecutivo

El test se ejecutó exitosamente en **24.14 segundos**, procesando un PDF de 804KB a través de 10 etapas del pipeline. Se generaron **4 traces de LLM** (summary, tags, project, category) y **80 embeddings** (promedio 84.9ms cada uno).

## 🔍 Problemas Identificados

### 1. **CRÍTICO: SuggestionStage retorna valores mock**

**Problema**: Los métodos `parseTagResponse` y `parseProjectResponse` en `SuggestionService` tienen TODOs y retornan valores hardcodeados:
- `parseTagResponse`: Retorna `[]entity.SuggestedTag{{Tag: "example", Confidence: 0.8}}`
- `parseProjectResponse`: Retorna `[]entity.SuggestedProject{{ProjectName: "example-project", Confidence: 0.7}}`

**Impacto**: El SuggestionStage no está generando sugerencias reales, solo valores de ejemplo.

**Solución**: Implementar parsing real de las respuestas JSON del LLM.

---

### 2. **Prompt de Tags en inglés mezclado con español**

**Problema**: El prompt base dice:
```
Analyze the following information and suggest up to 10 relevant tags in Spanish.
```

**Análisis**: 
- El prompt está en inglés pero pide tags en español
- Mezcla de idiomas puede confundir al modelo
- El contenido es completamente en español

**Mejora sugerida**:
```markdown
Analiza la siguiente información y sugiere hasta 10 tags relevantes en español.
Usa frases sustantivas concisas (1-3 palabras), evita duplicados y evita puntuación.
Responde SOLO con un array JSON de strings de tags, nada más.

Resumen:
%s

Descripción:
%s

Tags (array JSON):
```

---

### 3. **Respuesta de proyecto incluye punto final**

**Problema**: A pesar de que el prompt dice explícitamente:
```
3. Responde SOLO con el nombre del proyecto, sin explicaciones, sin comillas, sin puntos finales.
```

La respuesta fue: `"Conversaciones con las Almas del Purgatorio."` (con punto final)

**Análisis**: 
- El modelo no está siguiendo estrictamente las instrucciones
- El parsing no está limpiando el punto final

**Mejora sugerida**:
1. **Reforzar en el prompt**:
```markdown
REGLAS CRÍTICAS (OBLIGATORIAS):
1. El nombre del proyecto DEBE estar en ESPAÑOL, el mismo idioma que el contenido.
2. El nombre debe ser descriptivo y relevante al contenido.
3. Responde SOLO con el nombre del proyecto, sin explicaciones, sin comillas, sin puntos finales, sin espacios al inicio o final.
4. Si incluyes puntuación, la respuesta será rechazada.
```

2. **Mejorar el parsing** en `cleanString()`:
```go
func cleanString(s string) string {
    s = strings.TrimSpace(s)
    // Remover punto final si existe
    s = strings.TrimSuffix(s, ".")
    // Remover comillas si existen
    s = strings.Trim(s, `"'`)
    return s
}
```

---

### 4. **Categoría incorrecta: "Educación y Referencia" vs "Religión y Teología"**

**Problema**: El documento es claramente religioso (sobre almas del purgatorio, Princesa Eugenia, Director Espiritual, etc.) pero se clasificó como "Educación y Referencia".

**Análisis del prompt**:
- El prompt tiene "Religión y Teología" en la lista
- El resumen menciona: "Princesa Eugenia von der Leyen", "Almas del Purgatorio", "Director Espiritual", "Iglesia Sufriente"
- El modelo eligió incorrectamente

**Mejora sugerida**:

1. **Agregar ejemplos en el prompt**:
```markdown
Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

Basándote en la información proporcionada, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- Ciencia y Tecnología (ej: manuales técnicos, papers científicos)
- Arte y Diseño (ej: guías de diseño, catálogos de arte)
- Negocios y Finanzas (ej: reportes financieros, planes de negocio)
- Educación y Referencia (ej: enciclopedias, diccionarios, material educativo general)
- Literatura y Escritura (ej: novelas, poesía, guías de escritura)
- Documentación Técnica (ej: APIs, especificaciones técnicas)
- Recursos Humanos (ej: políticas de RRHH, manuales de empleados)
- Marketing y Comunicación (ej: estrategias de marketing, comunicados)
- Legal y Regulatorio (ej: contratos, regulaciones)
- Salud y Medicina (ej: estudios médicos, guías de salud)
- Religión y Teología (ej: textos religiosos, estudios teológicos, vidas de santos, experiencias místicas)
- Ingeniería y Construcción (ej: planos, especificaciones de construcción)
- Investigación y Análisis (ej: estudios de mercado, análisis de datos)
- Configuración y Administración (ej: guías de configuración, manuales de administración)
- Pruebas y Calidad (ej: planes de prueba, reportes de calidad)
- Sin Clasificar

Resumen del documento:
%s

Descripción:
%s

IMPORTANTE: Analiza cuidadosamente el tema principal del documento. Si menciona términos religiosos, teológicos, místicos, vidas de santos, o experiencias espirituales, la categoría correcta es "Religión y Teología".

Responde SOLO con el nombre exacto de la categoría de la lista (sin comillas, sin punto final, sin explicaciones).
```

2. **Usar few-shot examples**:
```markdown
Ejemplos:
- Documento sobre "vidas de santos y experiencias místicas" → Religión y Teología
- Documento sobre "manual de usuario de software" → Documentación Técnica
- Documento sobre "análisis de mercado" → Investigación y Análisis
```

---

### 5. **RAG no se está usando efectivamente en los prompts**

**Análisis del prompt de summary con RAG**:
- El prompt incluye contexto relacionado pero es muy genérico: `[1] Document`, `[2] Document`
- No se está aprovechando la información de similitud de los chunks encontrados
- Los snippets de RAG son muy cortos y no dan suficiente contexto

**Mejora sugerida**:

1. **Mejorar el formato del contexto RAG**:
```markdown
Contexto relacionado del workspace (documentos similares encontrados):

[1] Documento: "40 Conferencias.pdf"
   Relevancia: 0.85
   Fragmento: "Las almas del purgatorio necesitan nuestras oraciones..."
   
[2] Documento: "Vida de Santos.pdf"
   Relevancia: 0.78
   Fragmento: "La Princesa Eugenia tuvo experiencias místicas..."

Considera este contexto relacionado al generar el resumen. Si hay documentos similares, mantén consistencia en el estilo y terminología.
```

2. **Incluir metadatos de los documentos similares**:
```markdown
Documentos similares en el workspace:
- Tags comunes: ["religión", "misticismo", "santos"]
- Categoría común: "Religión y Teología"
- Proyectos relacionados: ["Literatura Religiosa", "Vidas de Santos"]
```

---

### 6. **Prompt de summary podría ser más específico**

**Análisis actual**:
- El prompt es genérico: "Resume el siguiente contenido en 80 palabras o menos"
- No especifica qué aspectos priorizar
- No menciona el tipo de documento

**Mejora sugerida**:
```markdown
Eres un experto en resumir documentos. Resume el siguiente contenido en 80 palabras o menos.

INSTRUCCIONES:
1. Sé conciso y captura los puntos principales
2. Identifica: quién, qué, cuándo, dónde, por qué
3. Si es un documento religioso/teológico, menciona el tema espiritual principal
4. Si es un documento técnico, menciona la tecnología o metodología principal
5. El resumen DEBE estar en español, el mismo idioma que el contenido

Tipo de documento detectado: PDF (documento de texto)
Tamaño: %d palabras

Contexto relacionado del workspace:
%s

Contenido:
%s

Resumen:
```

---

### 7. **Falta validación y post-procesamiento de respuestas**

**Problemas**:
- No se valida que las respuestas JSON sean válidas
- No se limpian caracteres especiales
- No se valida que las categorías estén en la lista permitida
- No se valida formato de tags

**Mejora sugerida**:

1. **Validar categorías**:
```go
func validateCategory(category string) (string, error) {
    validCategories := []string{
        "Ciencia y Tecnología", "Arte y Diseño", "Negocios y Finanzas",
        "Educación y Referencia", "Literatura y Escritura", "Documentación Técnica",
        "Recursos Humanos", "Marketing y Comunicación", "Legal y Regulatorio",
        "Salud y Medicina", "Religión y Teología", "Ingeniería y Construcción",
        "Investigación y Análisis", "Configuración y Administración", 
        "Pruebas y Calidad", "Sin Clasificar",
    }
    
    category = strings.TrimSpace(category)
    category = strings.TrimSuffix(category, ".")
    
    for _, valid := range validCategories {
        if strings.EqualFold(category, valid) {
            return valid, nil
        }
    }
    
    return "Sin Clasificar", fmt.Errorf("categoría '%s' no válida", category)
}
```

2. **Validar y limpiar tags JSON**:
```go
func parseAndValidateTags(response string) ([]string, error) {
    // Limpiar respuesta
    response = strings.TrimSpace(response)
    // Remover markdown code blocks si existen
    response = strings.TrimPrefix(response, "```json")
    response = strings.TrimPrefix(response, "```")
    response = strings.TrimSuffix(response, "```")
    response = strings.TrimSpace(response)
    
    var tags []string
    if err := json.Unmarshal([]byte(response), &tags); err != nil {
        return nil, fmt.Errorf("failed to parse JSON: %w", err)
    }
    
    // Validar y limpiar cada tag
    cleaned := make([]string, 0, len(tags))
    for _, tag := range tags {
        tag = strings.TrimSpace(tag)
        tag = strings.Trim(tag, `"'`)
        if tag != "" && len(tag) <= 50 {
            cleaned = append(cleaned, tag)
        }
    }
    
    return cleaned, nil
}
```

---

## 🎯 Recomendaciones Prioritarias

### Prioridad ALTA (Implementar inmediatamente)

1. **Implementar parsing real en SuggestionService**
   - Completar `parseTagResponse()` con parsing JSON real
   - Completar `parseProjectResponse()` con parsing JSON real
   - Agregar validación de respuestas

2. **Mejorar limpieza de respuestas**
   - Implementar `cleanString()` más robusto
   - Remover puntos finales, comillas, espacios
   - Validar formato esperado

3. **Unificar idioma en prompts**
   - Todos los prompts deben estar en el mismo idioma que el contenido
   - Detectar idioma automáticamente y usar prompts en ese idioma

### Prioridad MEDIA (Mejorar calidad)

4. **Mejorar prompt de categorización**
   - Agregar ejemplos few-shot
   - Incluir palabras clave para cada categoría
   - Reforzar análisis del tema principal

5. **Mejorar uso de RAG en prompts**
   - Incluir más contexto de documentos similares
   - Mostrar relevancia y snippets más largos
   - Incluir metadatos de documentos similares (tags, categorías)

6. **Agregar validación de respuestas**
   - Validar categorías contra lista permitida
   - Validar formato JSON de tags
   - Validar formato de proyectos

### Prioridad BAJA (Optimizaciones)

7. **Optimizar prompts para reducir tokens**
   - Usar versiones más concisas cuando sea posible
   - Cachear prompts comunes
   - Usar templates más eficientes

8. **Agregar métricas de calidad**
   - Trackear tasa de éxito de parsing
   - Trackear validación de respuestas
   - Trackear tiempo de respuesta por tipo de prompt

---

## 📈 Métricas del Test

- **Tiempo total**: 24.14 segundos
- **Embeddings generados**: 80
- **Tiempo promedio por embedding**: 84.9ms
- **Traces de LLM**: 4
  - Summary: 2492ms, 1283 tokens
  - Tags: 992ms, 334 tokens
  - Project: 499ms, 369 tokens
  - Category: 496ms, 436 tokens
- **Total tokens LLM**: ~2,422 tokens
- **Tiempo total LLM**: ~4.5 segundos

---

## 🔧 Archivos a Modificar

1. `backend/internal/infrastructure/llm/router.go`
   - Mejorar `SuggestTagsWithContextAndSummary()` - prompt en español
   - Mejorar `cleanString()` - limpieza más robusta
   - Agregar validación de categorías

2. `backend/internal/application/metadata/suggestion_service.go`
   - Implementar `parseTagResponse()` - parsing JSON real
   - Implementar `parseProjectResponse()` - parsing JSON real
   - Mejorar `buildTagPrompt()` - prompt en español
   - Mejorar `buildProjectPrompt()` - más específico

3. `backend/internal/application/pipeline/stages/ai.go`
   - Mejorar prompt de categorización con ejemplos
   - Mejorar uso de RAG en prompts de summary

---

## ✅ Checklist de Mejoras

- [ ] Implementar parsing real en SuggestionService
- [ ] Unificar idioma de prompts (español para contenido en español)
- [ ] Mejorar limpieza de respuestas (remover puntos, comillas, etc.)
- [ ] Agregar validación de categorías
- [ ] Mejorar prompt de categorización con ejemplos
- [ ] Mejorar uso de RAG en prompts (más contexto, mejor formato)
- [ ] Agregar validación de formato JSON
- [ ] Agregar métricas de calidad de respuestas

---

**Conclusión**: El pipeline funciona correctamente pero los prompts necesitan optimización para mejorar la calidad y consistencia de las respuestas del LLM. Las mejoras sugeridas aumentarán significativamente la precisión y utilidad de las sugerencias generadas.






