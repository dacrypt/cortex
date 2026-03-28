# Técnicas Forenses Adicionales para Extracción de Metadatos

## 📋 Técnicas Actualmente Implementadas

### ✅ Ya Implementadas
1. **pdfinfo** - Metadatos estándar del PDF (Author, Creator, Producer, Pages, etc.)
2. **exiftool** - Metadatos XMP y otros formatos
3. **strings** - Extracción de strings embebidos (URLs, emails, versiones)
4. **file** - Detección de tipo MIME
5. **stat** - Metadatos del sistema de archivos
6. **Hashes** - MD5 y SHA256
7. **Magic Bytes** - Firma del archivo

---

## 🚀 Técnicas Forenses Adicionales Recomendadas

### 1. Análisis de Estructura Interna del PDF

#### 1.1. Análisis de Objetos PDF
**Herramientas**: `qpdf`, `pdf-parser` (Didier Stevens), `mutool` (MuPDF)

**Información extraíble**:
- Estructura de objetos PDF (xref, trailer, catalog)
- Referencias cruzadas entre objetos
- Streams comprimidos y descomprimidos
- Objetos huérfanos o corruptos
- Versiones de objetos (historial de edición)

**Implementación sugerida**:
```go
// RunQPDFAnalysis analiza la estructura interna del PDF
func (ft *ForensicToolRunner) RunQPDFAnalysis(ctx context.Context, filePath string) (map[string]interface{}, error) {
    // qpdf --check --json file.pdf
    // Extrae: estructura de objetos, referencias, streams
}
```

#### 1.2. Análisis de Cross-Reference Table (xref)
**Información extraíble**:
- Tabla de referencias cruzadas
- Objetos eliminados (marcados como "free")
- Orden de objetos (puede revelar historial de edición)

---

### 2. Análisis de Fuentes Embebidas

#### 2.1. Extracción de Fuentes
**Herramientas**: `pdffonts`, `mutool`, análisis directo de objetos Font

**Información extraíble**:
- Lista de fuentes usadas (nombre, tipo, encoding)
- Fuentes embebidas vs. referenciadas
- Subsets de fuentes (caracteres incluidos)
- Información de licencia de fuentes
- Métricas de fuentes (ascender, descender, width)

**Implementación sugerida**:
```go
// RunPDFFontsAnalysis extrae información de fuentes
func (ft *ForensicToolRunner) RunPDFFontsAnalysis(ctx context.Context, filePath string) ([]FontInfo, error) {
    // pdffonts file.pdf
    // Extrae: nombre, tipo, encoding, embebida, subset
}
```

---

### 3. Análisis de Imágenes Embebidas

#### 3.1. Extracción de Imágenes
**Herramientas**: `pdfimages` (poppler), `mutool extract`, análisis de objetos Image

**Información extraíble**:
- Número de imágenes embebidas
- Dimensiones de cada imagen
- Formato de imagen (JPEG, PNG, TIFF, etc.)
- Compresión usada
- Metadatos EXIF de imágenes (si están embebidas)
- Ubicación de imágenes en el documento

**Implementación sugerida**:
```go
// RunPDFImagesAnalysis extrae información de imágenes
func (ft *ForensicToolRunner) RunPDFImagesAnalysis(ctx context.Context, filePath string) ([]ImageInfo, error) {
    // pdfimages -list file.pdf
    // Extrae: dimensiones, formato, compresión, posición
}
```

#### 3.2. Análisis de Metadatos EXIF en Imágenes
**Información extraíble**:
- Datos de cámara (si es foto escaneada)
- Fecha de creación de imagen original
- GPS coordinates (si aplica)
- Software usado para crear/editar imagen

---

### 4. Análisis de JavaScript y Acciones

#### 4.1. Detección de JavaScript
**Herramientas**: `pdf-parser`, análisis de objetos JavaScript

**Información extraíble**:
- Presencia de código JavaScript
- Acciones automáticas (onOpen, onClose, onPrint)
- URLs o comandos ejecutados
- Nivel de riesgo (malware potencial)

**Implementación sugerida**:
```go
// RunPDFJavaScriptAnalysis detecta JavaScript
func (ft *ForensicToolRunner) RunPDFJavaScriptAnalysis(ctx context.Context, filePath string) (JavaScriptInfo, error) {
    // Buscar objetos /JavaScript, /JS, /OpenAction
    // Extrae: código, acciones, URLs
}
```

---

### 5. Análisis de Formularios

#### 5.1. Campos de Formulario
**Herramientas**: `pdftk`, análisis de objetos AcroForm

**Información extraíble**:
- Número de campos de formulario
- Tipos de campos (text, checkbox, radio, etc.)
- Valores por defecto
- Validaciones y restricciones
- Nombres de campos (pueden revelar propósito del documento)

**Implementación sugerida**:
```go
// RunPDFFormAnalysis analiza formularios
func (ft *ForensicToolRunner) RunPDFFormAnalysis(ctx context.Context, filePath string) (FormInfo, error) {
    // pdftk file.pdf dump_data_fields
    // Extrae: campos, tipos, valores, validaciones
}
```

---

### 6. Análisis de Anotaciones y Comentarios

#### 6.1. Anotaciones Embebidas
**Herramientas**: Análisis de objetos Annotation

**Información extraíble**:
- Número de anotaciones
- Tipos de anotaciones (text, highlight, sticky note, etc.)
- Contenido de comentarios
- Autor de anotaciones
- Fechas de anotaciones
- Ubicación de anotaciones (páginas)

**Implementación sugerida**:
```go
// RunPDFAnnotationsAnalysis extrae anotaciones
func (ft *ForensicToolRunner) RunPDFAnnotationsAnalysis(ctx context.Context, filePath string) ([]Annotation, error) {
    // Buscar objetos /Annot
    // Extrae: tipo, contenido, autor, fecha, página
}
```

---

### 7. Análisis de Hipervínculos

#### 7.1. Enlaces Externos e Internos
**Herramientas**: Análisis de objetos Link, URI

**Información extraíble**:
- URLs externas (pueden revelar fuentes o referencias)
- Enlaces internos (destinos dentro del documento)
- Acciones de enlace (GoTo, URI, Launch)
- Texto visible del enlace vs. destino real

**Implementación sugerida**:
```go
// RunPDFLinksAnalysis extrae hipervínculos
func (ft *ForensicToolRunner) RunPDFLinksAnalysis(ctx context.Context, filePath string) ([]LinkInfo, error) {
    // Buscar objetos /Link, /URI
    // Extrae: URLs, destinos, acciones, texto visible
}
```

---

### 8. Análisis de Estructura del Documento

#### 8.1. Outline/Bookmarks
**Herramientas**: Análisis de objetos Outline

**Información extraíble**:
- Estructura jerárquica del documento
- Títulos de secciones
- Navegación interna
- Índice del documento

**Implementación sugerida**:
```go
// RunPDFOutlineAnalysis extrae estructura
func (ft *ForensicToolRunner) RunPDFOutlineAnalysis(ctx context.Context, filePath string) (OutlineInfo, error) {
    // Buscar objetos /Outlines
    // Extrae: jerarquía, títulos, destinos
}
```

#### 8.2. Páginas y Navegación
**Información extraíble**:
- Número de páginas
- Rotación de páginas
- Tamaño de cada página
- Orden de páginas (puede revelar reorganización)

---

### 9. Análisis de Seguridad

#### 9.1. Permisos y Restricciones
**Herramientas**: `pdfinfo`, análisis de objetos Encrypt

**Información extraíble**:
- Permisos del documento (print, copy, modify, etc.)
- Restricciones de seguridad
- Método de encriptación
- Nivel de protección
- Si requiere contraseña (user vs. owner)

**Implementación sugerida**:
```go
// RunPDFSecurityAnalysis analiza seguridad
func (ft *ForensicToolRunner) RunPDFSecurityAnalysis(ctx context.Context, filePath string) (SecurityInfo, error) {
    // pdfinfo -encrypted file.pdf
    // Extrae: permisos, restricciones, encriptación
}
```

#### 9.2. Firmas Digitales
**Herramientas**: `pdfsig` (poppler), análisis de objetos Signature

**Información extraíble**:
- Presencia de firmas digitales
- Certificados usados
- Validez de firmas
- Autor de firma
- Fecha de firma

**Implementación sugerida**:
```go
// RunPDFSignatureAnalysis analiza firmas
func (ft *ForensicToolRunner) RunPDFSignatureAnalysis(ctx context.Context, filePath string) ([]SignatureInfo, error) {
    // pdfsig -verify file.pdf
    // Extrae: certificados, validez, autor, fecha
}
```

---

### 10. Análisis de Versiones e Historial

#### 10.1. Versiones del Documento
**Herramientas**: Análisis de objetos Prev, análisis de xref

**Información extraíble**:
- Número de versiones guardadas
- Historial de modificaciones
- Objetos eliminados (pueden contener información borrada)
- Orden cronológico de ediciones

**Implementación sugerida**:
```go
// RunPDFVersionAnalysis analiza versiones
func (ft *ForensicToolRunner) RunPDFVersionAnalysis(ctx context.Context, filePath string) (VersionInfo, error) {
    // Buscar objetos /Prev, analizar xref
    // Extrae: versiones, historial, objetos eliminados
}
```

---

### 11. Análisis de Compresión

#### 11.1. Tipos de Compresión
**Herramientas**: Análisis de objetos Stream

**Información extraíble**:
- Tipos de compresión usados (FlateDecode, DCTDecode, etc.)
- Eficiencia de compresión
- Streams sin comprimir (pueden contener datos ocultos)
- Tamaño comprimido vs. descomprimido

**Implementación sugerida**:
```go
// RunPDFCompressionAnalysis analiza compresión
func (ft *ForensicToolRunner) RunPDFCompressionAnalysis(ctx context.Context, filePath string) (CompressionInfo, error) {
    // Analizar objetos Stream, /Filter
    // Extrae: tipos, ratios, streams sin comprimir
}
```

---

### 12. Análisis de Colores y Perfiles

#### 12.1. Espacios de Color
**Herramientas**: Análisis de objetos ColorSpace, ICC

**Información extraíble**:
- Espacios de color usados (RGB, CMYK, Grayscale, etc.)
- Perfiles ICC embebidos
- Información de calibración de color
- Dispositivos de destino (impresora, pantalla)

**Implementación sugerida**:
```go
// RunPDFColorAnalysis analiza colores
func (ft *ForensicToolRunner) RunPDFColorAnalysis(ctx context.Context, filePath string) (ColorInfo, error) {
    // Buscar objetos /ColorSpace, /ICCProfile
    // Extrae: espacios, perfiles, calibración
}
```

---

### 13. Análisis de Metadatos Ocultos

#### 13.1. Objetos No Estándar
**Herramientas**: Análisis exhaustivo de todos los objetos

**Información extraíble**:
- Metadatos en objetos personalizados
- Información en streams no estándar
- Datos en objetos /Metadata (XMP alternativo)
- Información en objetos /Info (adicional al estándar)

**Implementación sugerida**:
```go
// RunPDFHiddenMetadataAnalysis busca metadatos ocultos
func (ft *ForensicToolRunner) RunPDFHiddenMetadataAnalysis(ctx context.Context, filePath string) (map[string]interface{}, error) {
    // Analizar todos los objetos, buscar /Metadata, /Info
    // Extrae: metadatos en objetos no estándar
}
```

---

### 14. Análisis de Esteganografía

#### 14.1. Detección de Datos Ocultos
**Herramientas**: Análisis de patrones, estadísticas

**Información extraíble**:
- Datos ocultos en espacios en blanco
- Información en bits menos significativos
- Patrones anómalos que sugieren esteganografía
- Tamaño del archivo vs. contenido aparente

**Implementación sugerida**:
```go
// RunPDFSteganographyAnalysis detecta datos ocultos
func (ft *ForensicToolRunner) RunPDFSteganographyAnalysis(ctx context.Context, filePath string) (SteganographyInfo, error) {
    // Análisis estadístico, patrones, anomalías
    // Extrae: indicadores de esteganografía
}
```

---

### 15. Análisis de Watermarks y Marcas

#### 15.1. Marcas de Agua
**Herramientas**: Análisis visual y de objetos

**Información extraíble**:
- Presencia de watermarks
- Texto o imágenes de marca de agua
- Ubicación de marcas
- Información de copyright o identificación

---

### 16. Análisis de Metadatos de Aplicación

#### 16.1. Información de Software Creador
**Herramientas**: Análisis de strings, objetos /Producer

**Información extraíble**:
- Versión exacta del software usado
- Plugins o extensiones usadas
- Configuración del software
- Información de build o compilación

---

## 🎯 Priorización de Implementación

### Alta Prioridad (Mayor Valor)
1. **Análisis de Fuentes** - Información valiosa sobre el documento
2. **Análisis de Hipervínculos** - URLs y referencias externas
3. **Análisis de Imágenes** - Metadatos EXIF, dimensiones
4. **Análisis de Estructura (Outline)** - Navegación y organización

### Media Prioridad
5. **Análisis de Formularios** - Campos y validaciones
6. **Análisis de Anotaciones** - Comentarios y notas
7. **Análisis de Seguridad** - Permisos y restricciones
8. **Análisis de Compresión** - Eficiencia y tipos

### Baja Prioridad (Especializadas)
9. **Análisis de JavaScript** - Código embebido
10. **Análisis de Firmas Digitales** - Certificados
11. **Análisis de Versiones** - Historial de edición
12. **Análisis de Esteganografía** - Datos ocultos

---

## 🛠️ Herramientas Requeridas

### Herramientas Externas Necesarias
- **qpdf** - Análisis de estructura PDF
- **pdfimages** (poppler) - Extracción de imágenes
- **pdffonts** (poppler) - Análisis de fuentes
- **pdftk** - Análisis de formularios
- **pdfsig** (poppler) - Análisis de firmas
- **mutool** (MuPDF) - Herramienta todo-en-uno
- **pdf-parser** (Didier Stevens) - Análisis forense profundo

### Instalación
```bash
# macOS
brew install poppler qpdf mupdf

# Linux (Ubuntu/Debian)
sudo apt-get install poppler-utils qpdf mupdf-tools

# pdf-parser (Python)
pip install pdf-parser
```

---

## 📊 Estructura de Datos Sugerida

```go
type PDFForensicAnalysis struct {
    // Estructura
    Structure     *PDFStructureInfo
    Objects       []PDFObjectInfo
    
    // Contenido
    Fonts         []FontInfo
    Images        []ImageInfo
    Links         []LinkInfo
    Forms         *FormInfo
    Annotations   []Annotation
    
    // Navegación
    Outline       *OutlineInfo
    Pages         []PageInfo
    
    // Seguridad
    Security      *SecurityInfo
    Signatures    []SignatureInfo
    
    // Metadatos avanzados
    Compression   *CompressionInfo
    Colors        *ColorInfo
    Versions      *VersionInfo
    
    // Análisis especializado
    JavaScript    *JavaScriptInfo
    Steganography *SteganographyInfo
    HiddenData    map[string]interface{}
}
```

---

## 🔍 Ejemplo de Uso

```go
// Análisis forense completo
analysis, err := forensicRunner.RunCompletePDFAnalysis(ctx, filePath)
if err != nil {
    return err
}

// Acceder a información específica
fmt.Printf("Fuentes: %d\n", len(analysis.Fonts))
fmt.Printf("Imágenes: %d\n", len(analysis.Images))
fmt.Printf("Enlaces: %d\n", len(analysis.Links))
fmt.Printf("Firmas: %d\n", len(analysis.Signatures))
```

---

## 📝 Notas de Implementación

1. **Rendimiento**: Algunas técnicas son costosas computacionalmente. Considerar:
   - Ejecución asíncrona
   - Caché de resultados
   - Análisis incremental

2. **Errores**: Los PDFs pueden estar corruptos o mal formados. Manejar errores gracefully.

3. **Privacidad**: Algunas técnicas pueden extraer información sensible. Considerar:
   - Opciones de configuración
   - Filtrado de datos sensibles
   - Logging apropiado

4. **Compatibilidad**: Diferentes versiones de PDF tienen diferentes capacidades. Validar versiones.

---

## 🚀 Próximos Pasos

1. Implementar análisis de fuentes (alta prioridad)
2. Implementar análisis de hipervínculos (alta prioridad)
3. Implementar análisis de imágenes (alta prioridad)
4. Agregar tests para cada técnica
5. Documentar resultados en `DocumentMetrics.CustomProperties`
6. Integrar resultados en el pipeline de procesamiento






