# Análisis de Calidad de Prompts y Respuestas LLM

**Fecha**: 2025-12-25  
**Test**: `TestVerbosePipelineSingleFile`  
**Archivo procesado**: "40 Conferencias.pdf"  
**Modelo LLM**: llama3.2  
**Análisis realizado por**: Experto en Ingeniería de Prompting y Contexto

---

## Resumen Ejecutivo

### Métricas Generales
- ✅ **Cobertura de metadata**: 100% (5/5 campos)
- ⚠️ **Tiempo total**: 85.88 segundos (aceptable pero mejorable)
- ✅ **Trazabilidad**: 5 traces completos registrados
- ⚠️ **Calidad de respuestas**: Variable (ver análisis detallado)

### Problemas Críticos Identificados

1. **PROYECTO**: Nombre truncado (máximo 50 caracteres violado)
2. **CONTEXTUAL_INFO**: Valores "null" como strings en lugar de null JSON
3. **CATEGORY**: Clasificación incorrecta (debería ser "Religión y Teología")
4. **SUMMARY**: Falta estructura en párrafos según instrucciones

---

## Análisis Detallado por Operación

### 1. SUMMARY (Resumen)

#### Prompt Analizado
```
Eres un experto en resumir documentos. Resume el siguiente contenido en 80 palabras o menos.

INSTRUCCIONES:
1. Sé conciso y captura los puntos principales
2. Identifica: quién, qué, cuándo, dónde, por qué
3. Si es un documento religioso/teológico, menciona el tema espiritual principal
4. Si es un documento técnico, menciona la tecnología o metodología principal
5. El resumen DEBE estar en español, el mismo idioma que el contenido

Contexto relacionado del workspace (documentos similares encontrados):
[1] Documento similar: ...
[2] Documento similar: ...
[3] Documento similar: ...

Considera este contexto relacionado al generar el resumen...
```

**Calidad del Prompt**: ⭐⭐⭐⭐ (4/5)

**Fortalezas**:
- ✅ Instrucciones claras y estructuradas
- ✅ Integración de RAG (contexto de documentos similares)
- ✅ Especificación de idioma
- ✅ Instrucciones específicas para tipos de documentos

**Debilidades**:
- ⚠️ **FALTA**: La instrucción #6 del template original sobre estructura en párrafos no aparece en el prompt real
- ⚠️ El contexto RAG está presente pero podría ser más específico sobre cómo usarlo

#### Respuesta del LLM
```
El documento es un índice de una conferencia teológica que aborda temas relacionados con la ciencia y la fe. El autor analiza la existencia de Dios, la conquista del espacio, la historia de los evangelios, la divinidad de Cristo y su relación con la autenticidad de la Sábana Santa. También explora el tema del carbono-14 en relación con la falsificación de la Sábana Santa. El documento presenta un debate entre la ciencia y la fe, buscando encontrar un equilibrio entre ambas perspectivas.
```

**Calidad de la Respuesta**: ⭐⭐⭐ (3/5)

**Fortalezas**:
- ✅ Está en español
- ✅ Captura los temas principales
- ✅ Menciona elementos clave (Dios, ciencia, fe, Sábana Santa)

**Debilidades**:
- ⚠️ **NO sigue la estructura de 2-3 párrafos** especificada en el template (aunque no aparece en el prompt real)
- ⚠️ No identifica explícitamente "quién" (autor: Rodante)
- ⚠️ No menciona "cuándo" (año 2015, aunque está en metadata)
- ⚠️ No menciona "dónde" (Zaragoza, España)
- ⚠️ El resumen es un solo párrafo largo, difícil de escanear

**Recomendaciones**:
1. Asegurar que el prompt incluya la instrucción de estructura en párrafos
2. Enfatizar la extracción de metadatos básicos (quién, cuándo, dónde)
3. Considerar usar few-shot examples para mostrar formato esperado

---

### 2. CONTEXTUAL_INFO (Información Contextual)

#### Prompt Analizado
```
Eres un experto bibliotecario y archivista. Analiza el siguiente documento y extrae TODA la información contextual relevante en formato JSON.

INSTRUCCIONES:
1-10. [Lista completa de instrucciones]

FORMATO DE RESPUESTA (JSON estricto):
{
  "authors": [...],
  "editors": [...],
  ...
}

IMPORTANTE:
- Responde SOLO con JSON válido, sin texto adicional
- Usa null para campos no disponibles
- NUNCA uses placeholders genéricos...
- Para organizaciones: si todos los campos son "null", usa array vacío...
```

**Calidad del Prompt**: ⭐⭐⭐⭐⭐ (5/5)

**Fortalezas**:
- ✅ Instrucciones extremadamente detalladas
- ✅ Ejemplos de formato JSON claros
- ✅ Reglas explícitas sobre qué NO hacer (placeholders, duplicados)
- ✅ Integración de contexto RAG
- ✅ Manejo de casos edge (arrays vacíos vs null)

**Debilidades**:
- Ninguna crítica significativa

#### Respuesta del LLM
```json
{
  "authors": [{"name": "Rodante", "role": "autor", "affiliation": null}],
  "editors": [],
  "translators": [],
  "contributors": [],
  "publisher": "null",  // ❌ PROBLEMA: debería ser null, no string "null"
  "publication_year": 2024,  // ❌ PROBLEMA: debería ser 2015 según metadata
  "publication_place": "Zaragoza, España",
  "isbn": "null",  // ❌ PROBLEMA: debería ser null, no string "null"
  "issn": "null",  // ❌ PROBLEMA: debería ser null, no string "null"
  "historical_period": ["Medieval"],  // ⚠️ DUDOSO: el documento es de 2015, no medieval
  "locations": [...],  // ✅ Correcto
  "people_mentioned": [...],  // ✅ Correcto
  "organizations": [{"name": "null", "type": "null", "context": "null"}],  // ❌ PROBLEMA: debería ser []
  "historical_events": [...],  // ✅ Correcto
  "references": [{"title": "null", ...}],  // ❌ PROBLEMA: debería ser []
  "original_language": "null",  // ❌ PROBLEMA: debería ser null
  "genre": "teología",
  "subject": "ciencia y fe",
  "audience": "investigadores, académicos"
}
```

**Calidad de la Respuesta**: ⭐⭐ (2/5)

**Problemas Críticos**:
1. ❌ **Strings "null" en lugar de null JSON**: `"publisher": "null"` debería ser `"publisher": null`
2. ❌ **Año incorrecto**: `2024` en lugar de `2015` (está en los metadatos del archivo)
3. ❌ **Período histórico incorrecto**: `["Medieval"]` cuando el documento es de 2015
4. ❌ **Arrays con objetos null**: `organizations` y `references` tienen objetos con todos los campos "null" en lugar de arrays vacíos
5. ⚠️ **Falta información**: No extrajo el año de publicación correcto (2015) que está en los metadatos del archivo

**Fortalezas**:
- ✅ Estructura JSON válida
- ✅ Extrajo autores, ubicaciones, personas correctamente
- ✅ No usó placeholders genéricos prohibidos
- ✅ Género y tema correctos

**Recomendaciones**:
1. **CRÍTICO**: Mejorar el parser JSON para convertir strings "null" a null real
2. Incluir metadatos del archivo (fecha de modificación) en el prompt para validación
3. Agregar validación post-procesamiento para:
   - Convertir `"null"` → `null`
   - Convertir arrays con objetos null → arrays vacíos
   - Validar coherencia de fechas (año de publicación vs período histórico)
4. Agregar few-shot examples mostrando el formato correcto de null vs "null"

---

### 3. TAGS (Etiquetas)

#### Prompt Analizado
```
Analiza la siguiente información y sugiere hasta 10 tags relevantes en español.
Usa frases sustantivas concisas (1-3 palabras), evita duplicados y evita puntuación.
Responde SOLO con un array JSON de strings de tags, nada más.

Resumen:
[Resumen del documento]

Descripción:
[Descripción generada]

Tags (array JSON):
```

**Calidad del Prompt**: ⭐⭐⭐⭐ (4/5)

**Fortalezas**:
- ✅ Instrucciones claras y concisas
- ✅ Especificación de formato (array JSON)
- ✅ Reglas específicas (1-3 palabras, sin puntuación)
- ✅ Límite claro (hasta 10 tags)

**Debilidades**:
- ⚠️ No especifica si los tags deben ser en singular/plural
- ⚠️ No menciona evitar tags genéricos o demasiado amplios

#### Respuesta del LLM
```json
["Teología", "Ciencia", "Dios existente", "Conquista espacial", "Evangelios", "Divinidad Cristiana", "Sábana Santa auténtica", "Carbono-14", "Falsificación histórica", "Debate científico-teológico"]
```

**Calidad de la Respuesta**: ⭐⭐⭐⭐ (4/5)

**Fortalezas**:
- ✅ Formato JSON válido
- ✅ Todos los tags en español
- ✅ Tags relevantes y específicos
- ✅ Sin duplicados
- ✅ Respeta el límite de 10 tags
- ✅ Mayoría en 1-3 palabras

**Debilidades**:
- ⚠️ "Dios existente" podría ser más conciso ("Existencia de Dios")
- ⚠️ "Divinidad Cristiana" podría ser "Divinidad de Cristo" (más específico)
- ⚠️ "Falsificación histórica" es un poco genérico

**Recomendaciones**:
1. Agregar ejemplo de tags buenos vs malos en el prompt
2. Especificar preferencia por sustantivos simples sobre frases descriptivas
3. Considerar normalización post-procesamiento (capitalización, singular/plural)

---

### 4. PROJECT (Proyecto)

#### Prompt Analizado
```
Eres un asistente experto en organización de documentos.

Basándote en la siguiente información, sugiere el proyecto/contexto más apropiado de la lista a continuación.
Si ninguno de los proyectos existentes encaja, sugiere un nuevo nombre de proyecto.

REGLAS CRÍTICAS:
1. El nombre del proyecto DEBE estar en ESPAÑOL, el mismo idioma que el contenido.
2. El nombre debe ser descriptivo y relevante al contenido.
3. El nombre DEBE ser CONCISO: máximo 50 caracteres (preferiblemente 30-40).
4. Evita usar dos puntos (:) en el nombre.
5. Responde SOLO con el nombre del proyecto, sin explicaciones, sin comillas, sin puntos finales.

Proyectos existentes:
None available

[Resumen y descripción]

Proyecto sugerido:
```

**Calidad del Prompt**: ⭐⭐⭐⭐ (4/5)

**Fortalezas**:
- ✅ Reglas críticas claramente marcadas
- ✅ Límite de caracteres específico (50, preferiblemente 30-40)
- ✅ Instrucciones sobre formato de respuesta
- ✅ Manejo de proyectos existentes

**Debilidades**:
- ⚠️ No hay ejemplos de nombres buenos vs malos
- ⚠️ No especifica qué hacer si el nombre naturalmente excede 50 caracteres

#### Respuesta del LLM
```
Debate Científico y Teológico sobre la Existencia de Dios y la Autenticidad de la Sábana Santa.
```

**Calidad de la Respuesta**: ⭐⭐ (2/5)

**Problemas Críticos**:
1. ❌ **VIOLA límite de 50 caracteres**: El nombre tiene **89 caracteres** (casi el doble del máximo)
2. ❌ **Incluye punto final**: Tiene "." al final, violando la regla #5
3. ⚠️ Demasiado descriptivo y largo

**Fortalezas**:
- ✅ Está en español
- ✅ Es descriptivo y relevante
- ✅ No usa dos puntos

**Recomendaciones**:
1. **CRÍTICO**: Agregar validación post-procesamiento para truncar nombres > 50 caracteres
2. **CRÍTICO**: Agregar limpieza de puntuación final (puntos, comas, etc.)
3. Agregar few-shot examples mostrando nombres buenos (30-40 chars) vs malos (>50 chars)
4. Considerar usar técnicas de resumen para nombres largos (ej: "Debate Ciencia-Fe y Sábana Santa")
5. Agregar instrucción explícita: "Si el nombre natural excede 50 caracteres, acórtalo manteniendo las palabras clave más importantes"

---

### 5. CATEGORY (Categoría)

#### Prompt Analizado
```
Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

Basándote en la información proporcionada, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- Ciencia y Tecnología
- Arte y Diseño
- Negocios y Finanzas
- Educación y Referencia
- Literatura y Escritura
- Documentación Técnica
- Recursos Humanos
- Marketing y Comunicación
- Legal y Regulatorio
- Salud y Medicina
- Religión y Teología
- Ingeniería y Construcción
- Investigación y Análisis
- Configuración y Administración
- Pruebas y Calidad
- Sin Clasificar

[Resumen y descripción]

IMPORTANTE: Analiza cuidadosamente el tema principal del documento. Si menciona términos religiosos, teológicos, místicos, vidas de santos, o experiencias espirituales, la categoría correcta es "Religión y Teología".

Ejemplos de clasificación:
- Documento sobre "vidas de santos y experiencias místicas" → Religión y Teología
- Documento sobre "manual de usuario de software" → Documentación Técnica
- Documento sobre "análisis de mercado" → Investigación y Análisis
- Documento sobre "enciclopedia o diccionario" → Educación y Referencia

Responde SOLO con el nombre exacto de la categoría de la lista (sin comillas, sin punto final, sin explicaciones).
```

**Calidad del Prompt**: ⭐⭐⭐⭐⭐ (5/5)

**Fortalezas**:
- ✅ Lista clara de categorías
- ✅ Instrucción IMPORTANTE destacada sobre documentos religiosos/teológicos
- ✅ Ejemplos específicos de clasificación
- ✅ Instrucciones claras sobre formato de respuesta

**Debilidades**:
- Ninguna crítica significativa

#### Respuesta del LLM
```
Educación y Referencia.
```

**Calidad de la Respuesta**: ⭐ (1/5)

**Problemas Críticos**:
1. ❌ **CLASIFICACIÓN INCORRECTA**: El documento es claramente "Religión y Teología"
   - El documento trata sobre: teología, ciencia y fe, divinidad de Cristo, Sábana Santa
   - El prompt tenía una instrucción IMPORTANTE específica sobre esto
   - El resumen menciona explícitamente "conferencia teológica"
   - Los tags incluyen "Teología" como primer tag
2. ❌ **Incluye punto final**: Tiene "." al final

**Análisis del Error**:
El LLM probablemente se confundió porque:
- El documento es un "índice" (estructura de referencia)
- El prompt menciona "enciclopedia o diccionario" → "Educación y Referencia"
- No priorizó la instrucción IMPORTANTE sobre términos religiosos/teológicos

**Fortalezas**:
- ✅ Formato correcto (nombre de categoría de la lista)
- ✅ Está en español

**Recomendaciones**:
1. **CRÍTICO**: Reforzar la instrucción IMPORTANTE al inicio del prompt
2. **CRÍTICO**: Agregar validación post-procesamiento basada en tags/resumen
   - Si tags contienen "Teología", "Religión", etc. → forzar "Religión y Teología"
3. Agregar few-shot examples específicos:
   - "Índice de conferencias teológicas sobre ciencia y fe" → Religión y Teología
   - "Índice de manual técnico" → Documentación Técnica
4. Considerar usar chain-of-thought: "Primero identifica el tema principal, luego clasifica"
5. Agregar limpieza de puntuación final

---

## Análisis de Integración RAG

### Uso de RAG en los Prompts

**Summary**: ✅ Usa contexto RAG (3 documentos similares)  
**Contextual Info**: ✅ Usa contexto RAG (3 documentos similares)  
**Tags**: ⚠️ No usa RAG directamente (pero tiene contexto de documentos similares en el stage)  
**Project**: ⚠️ No usa RAG directamente  
**Category**: ❌ No usa RAG

**Observaciones**:
- El RAG está bien integrado donde se usa
- Los snippets de contexto son relevantes pero podrían ser más largos
- Category podría beneficiarse de RAG para ver cómo se clasificaron documentos similares

---

## Análisis de Parsing y Post-procesamiento

### Parsers Actuales

1. **JSONParser**: ⭐⭐⭐⭐ (4/5)
   - Maneja markdown code blocks
   - Limpia artefactos comunes
   - Tiene retry logic
   - **FALTA**: Conversión de strings "null" a null JSON

2. **ArrayParser**: ⭐⭐⭐⭐ (4/5)
   - Múltiples estrategias de fallback
   - Limpieza adecuada
   - **FALTA**: Normalización de capitalización

3. **StringParser**: ⭐⭐⭐⭐ (4/5)
   - Limpieza básica adecuada
   - **FALTA**: Limpieza de puntuación final más agresiva

### Recomendaciones de Post-procesamiento

1. **Contextual Info**:
   ```go
   // Convertir strings "null" a null
   if publisher == "null" { publisher = nil }
   // Validar coherencia de fechas
   if publicationYear < fileModYear - 10 { /* warning */ }
   // Limpiar arrays con objetos null
   if org.name == "null" && org.type == "null" { /* remove */ }
   ```

2. **Project**:
   ```go
   // Truncar a 50 caracteres
   if len(projectName) > 50 {
       projectName = truncateIntelligently(projectName, 50)
   }
   // Limpiar puntuación final
   projectName = strings.TrimRight(projectName, ".,;:")
   ```

3. **Category**:
   ```go
   // Validar contra tags/resumen
   if containsReligiousTerms(tags, summary) && category != "Religión y Teología" {
       category = "Religión y Teología" // override
   }
   // Limpiar puntuación final
   category = strings.TrimRight(category, ".,;:")
   ```

---

## Métricas de Calidad por Operación

| Operación | Prompt Quality | Response Quality | Parsing Quality | Overall |
|-----------|---------------|------------------|-----------------|---------|
| Summary | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ |
| Contextual Info | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| Tags | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| Project | ⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| Category | ⭐⭐⭐⭐⭐ | ⭐ | ⭐⭐⭐ | ⭐⭐ |

**Promedio General**: ⭐⭐⭐ (3/5)

---

## Recomendaciones Prioritarias

### 🔴 CRÍTICO (Implementar Inmediatamente)

1. **Post-procesamiento de Project**:
   - Truncar nombres > 50 caracteres
   - Limpiar puntuación final
   - Validar longitud antes de persistir

2. **Post-procesamiento de Contextual Info**:
   - Convertir strings "null" a null JSON
   - Limpiar arrays con objetos null
   - Validar coherencia de fechas

3. **Post-procesamiento de Category**:
   - Validar contra tags/resumen para documentos religiosos
   - Limpiar puntuación final
   - Override si hay conflicto claro

### 🟡 ALTA PRIORIDAD (Implementar Pronto)

4. **Mejorar Prompt de Summary**:
   - Incluir instrucción de estructura en párrafos
   - Enfatizar extracción de metadatos básicos (quién, cuándo, dónde)
   - Agregar few-shot examples

5. **Mejorar Prompt de Project**:
   - Agregar few-shot examples (buenos vs malos)
   - Instrucción explícita sobre truncamiento si excede 50 chars

6. **Mejorar Prompt de Category**:
   - Reforzar instrucción IMPORTANTE al inicio
   - Agregar few-shot examples específicos para índices teológicos

### 🟢 MEDIA PRIORIDAD (Mejoras Incrementales)

7. **Mejorar RAG Integration**:
   - Usar RAG en Category
   - Aumentar longitud de snippets de contexto
   - Mejorar relevancia de documentos similares

8. **Normalización de Tags**:
   - Capitalización consistente
   - Singular/plural consistente
   - Validación de duplicados semánticos

---

## Conclusión

El sistema de prompts está **bien diseñado en general**, con instrucciones claras y estructura adecuada. Sin embargo, hay **problemas críticos en el post-procesamiento** que causan:

1. **Datos incorrectos persistidos** (año 2024 en lugar de 2015, strings "null")
2. **Violaciones de reglas** (proyecto > 50 caracteres, categoría incorrecta)
3. **Falta de validación** (coherencia de fechas, validación semántica)

**La solución no es solo mejorar los prompts**, sino también **agregar validación y post-procesamiento robusto** que:
- Corrija errores comunes del LLM
- Valide coherencia de datos
- Aplique reglas de negocio (límites de caracteres, etc.)
- Use metadatos del archivo para validación cruzada

**Prioridad de implementación**: Post-procesamiento > Mejoras de prompts






