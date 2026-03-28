# Propuestas de Investigación y Mejoras Basadas en Literatura Académica

Este documento propone mejoras futuras para Cortex basadas en conceptos y técnicas de la literatura académica sobre organización de información, clasificación multi-jerárquica y sistemas modernos de gestión de documentos.

## 1. Clasificación Facetada (Faceted Classification)

### Concepto
La clasificación facetada permite organizar documentos usando múltiples dimensiones (facetas) independientes. Cada faceta representa un aspecto diferente del documento (tema, tipo, autor, fecha, etc.).

### Referencias Académicas
- **Ranganathan, S.R. (1933)**: "Colon Classification" - Sistema de clasificación facetada original
- **Svenonius, E. (2000)**: "The Intellectual Foundation of Information Organization"
- **Broughton, V. (2004)**: "Essential Classification" - Clasificación facetada moderna

### Implementación Propuesta para Cortex

```typescript
interface Facet {
  id: string;
  name: string;
  type: 'hierarchical' | 'flat' | 'numeric' | 'date' | 'boolean';
  values: FacetValue[];
}

interface FacetValue {
  id: string;
  label: string;
  parentId?: string; // Para facetas jerárquicas
}

// Ejemplo de facetas para un documento
const documentFacets = {
  topic: ['software', 'ai', 'machine-learning'], // Jerárquico
  documentType: ['article', 'tutorial', 'reference'], // Flat
  status: ['draft', 'review', 'published'], // Flat
  priority: [1, 2, 3, 4, 5], // Numérico
  createdDate: '2024-01-15', // Fecha
  isPublic: true // Boolean
};
```

### Beneficios
- ✅ Múltiples formas de organizar el mismo documento
- ✅ Búsqueda combinando múltiples facetas
- ✅ Escalable a grandes volúmenes
- ✅ Flexible para diferentes contextos

### Casos de Uso
- Organizar documentos por: tema + tipo + estado + fecha
- Filtrar por múltiples criterios simultáneamente
- Crear vistas personalizadas combinando facetas

---

## 2. Polijerarquías (Polyhierarchies)

### Concepto
Permite que un documento pertenezca a múltiples jerarquías simultáneamente. A diferencia de una jerarquía única, las polijerarquías reconocen que los documentos pueden tener múltiples relaciones de "pertenencia".

### Referencias Académicas
- **Svenonius, E. (2000)**: "The Intellectual Foundation of Information Organization" - Discute polijerarquías en sistemas de clasificación
- **Hjørland, B. (2013)**: "Theories of Knowledge Organization" - Múltiples perspectivas de organización
- **Dahlberg, I. (2006)**: "Knowledge Organization: A New Science?" - Organización multi-dimensional

### Implementación Propuesta

```typescript
interface PolyHierarchy {
  id: string;
  name: string;
  rootNodes: HierarchyNode[];
}

interface HierarchyNode {
  id: string;
  label: string;
  parentId?: string;
  children: HierarchyNode[];
  documents: string[]; // Document IDs
}

// Un documento puede pertenecer a múltiples jerarquías
const documentHierarchies = {
  'project-structure': ['client-acme', 'frontend', 'react'],
  'technology-stack': ['javascript', 'react', 'typescript'],
  'team-ownership': ['team-frontend', 'squad-ui'],
  'lifecycle': ['active', 'in-development', 'needs-review']
};
```

### Beneficios
- ✅ Representa la realidad: documentos tienen múltiples contextos
- ✅ No fuerza una única estructura organizacional
- ✅ Permite exploración desde diferentes perspectivas

### Casos de Uso
- Proyecto puede estar en: jerarquía de cliente + jerarquía técnica + jerarquía de equipo
- Documento puede ser: tutorial + referencia + ejemplo

---

## 3. Sistemas de Gestión de Información Personal (PIM)

### Concepto
Los sistemas PIM se enfocan en cómo las personas organizan y recuperan información personal. Incluyen conceptos como "information scraps", "keeping found things found", y "activity-based organization".

### Referencias Académicas
- **Jones, W. (2007)**: "Keeping Found Things Found: The Study and Practice of Personal Information Management"
- **Whittaker, S. & Sidner, C. (1996)**: "Email Overload: Exploring Personal Information Management of Email"
- **Bergman, O. et al. (2013)**: "The User-Subjective Approach to Personal Information Management Systems Design"

### Conceptos Clave

#### 3.1 Activity-Based Organization
Organizar archivos por actividad/proyecto en lugar de por tipo o fecha.

**Implementación Propuesta**:
```typescript
interface Activity {
  id: string;
  name: string;
  startDate: Date;
  endDate?: Date;
  status: 'active' | 'completed' | 'paused';
  documents: string[];
  relatedActivities: string[]; // Actividades relacionadas
}
```

#### 3.2 Information Scraps
Fragmentos de información que no encajan en estructuras formales (notas rápidas, URLs, ideas).

**Implementación Propuesta**:
```typescript
interface InformationScrap {
  id: string;
  type: 'note' | 'url' | 'idea' | 'todo' | 'reminder';
  content: string;
  createdAt: Date;
  relatedDocuments: string[];
  tags: string[];
}
```

#### 3.3 Keeping Found Things Found
Mecanismos para recordar dónde se encontró información importante.

**Implementación Propuesta**:
```typescript
interface SearchHistory {
  query: string;
  results: string[]; // Document IDs
  timestamp: Date;
  saved: boolean; // Si el usuario guardó esta búsqueda
  notes?: string; // Notas sobre por qué fue útil
}
```

---

## 4. Knowledge Graphs para Organización de Documentos

### Concepto
Usar grafos de conocimiento para representar relaciones semánticas entre documentos, conceptos y entidades.

### Referencias Académicas
- **Ehrlinger, L. & Wöß, W. (2016)**: "Towards a Definition of Knowledge Graphs"
- **Paulheim, H. (2017)**: "Knowledge Graph Refinement: A Survey of Approaches and Evaluation Methods"
- **Färber, M. et al. (2018)**: "The Microsoft Academic Knowledge Graph: A Knowledge Graph for the Academic World"

### Implementación Propuesta

```typescript
interface KnowledgeGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

interface GraphNode {
  id: string;
  type: 'document' | 'concept' | 'person' | 'project' | 'topic';
  label: string;
  properties: Record<string, any>;
}

interface GraphEdge {
  source: string;
  target: string;
  type: 'references' | 'depends_on' | 'related_to' | 'authored_by' | 'belongs_to';
  weight: number; // 0-1, fuerza de la relación
  metadata?: Record<string, any>;
}
```

### Beneficios
- ✅ Descubrimiento de relaciones implícitas
- ✅ Navegación semántica entre documentos
- ✅ Visualización de conocimiento
- ✅ Búsqueda por proximidad en el grafo

### Casos de Uso
- Encontrar documentos relacionados por conceptos compartidos
- Visualizar la red de conocimiento del workspace
- Sugerir documentos relevantes basados en posición en el grafo

---

## 5. Clasificación Automática con Machine Learning

### Concepto
Usar técnicas de ML para clasificar automáticamente documentos en categorías o jerarquías.

### Referencias Académicas
- **Sebastiani, F. (2002)**: "Machine Learning in Automated Text Categorization"
- **Aggarwal, C.C. & Zhai, C. (2012)**: "Mining Text Data" - Capítulo sobre clasificación
- **Zhang, L. et al. (2015)**: "Character-level Convolutional Networks for Text Classification"

### Técnicas Propuestas

#### 5.1 Clasificación Jerárquica
Clasificar en múltiples niveles de una jerarquía simultáneamente.

```typescript
interface HierarchicalClassifier {
  predict(document: Document): {
    level1: string; // Categoría principal
    level2: string; // Subcategoría
    level3?: string; // Sub-subcategoría
    confidence: number[];
  };
}
```

#### 5.2 Clustering Semántico
Agrupar documentos similares automáticamente.

```typescript
interface DocumentCluster {
  id: string;
  centroid: number[]; // Embedding del centroide
  documents: string[];
  label?: string; // Etiqueta generada automáticamente
  keywords: string[]; // Palabras clave del cluster
}
```

#### 5.3 Topic Modeling
Descubrir temas latentes en colecciones de documentos.

```typescript
interface Topic {
  id: string;
  name: string;
  keywords: { word: string; weight: number }[];
  documents: { id: string; relevance: number }[];
}
```

---

## 6. Sistemas de Etiquetado Colaborativo (Folksonomies)

### Concepto
Permitir que múltiples usuarios etiqueten documentos, creando una taxonomía emergente (folksonomía).

### Referencias Académicas
- **Vander Wal, T. (2007)**: "Folksonomy" - Término original
- **Marlow, C. et al. (2006)**: "Position Paper, Tagging, Taxonomy, Flickr, Article, ToRead"
- **Golder, S.A. & Huberman, B.A. (2006)**: "Usage Patterns of Collaborative Tagging Systems"

### Implementación Propuesta

```typescript
interface Tag {
  id: string;
  label: string;
  usageCount: number; // Cuántas veces se usa
  users: string[]; // Usuarios que lo han usado
  documents: string[]; // Documentos etiquetados
  relatedTags: { tagId: string; coOccurrence: number }[];
  suggestedTags?: string[]; // Tags sugeridos basados en uso
}

interface TaggingEvent {
  documentId: string;
  tagId: string;
  userId: string;
  timestamp: Date;
  context?: string; // Contexto del etiquetado
}
```

### Beneficios
- ✅ Vocabulario emergente del equipo
- ✅ Descubrimiento de tags populares
- ✅ Sugerencias basadas en uso colectivo
- ✅ Detección de sinónimos y variantes

---

## 7. Organización Temporal y Contextual

### Concepto
Organizar documentos basándose en tiempo y contexto de uso, no solo en contenido.

### Referencias Académicas
- **Dumais, S. et al. (2003)**: "Stuff I've Seen: A System for Personal Information Retrieval and Re-use"
- **Teevan, J. et al. (2004)**: "The Perfect Search Engine is Not Enough: A Study of Orienteering Behavior in Directed Search"
- **Morris, D. et al. (2008)**: "SearchTogether: An Interface for Collaborative Web Search"

### Conceptos Clave

#### 7.1 Temporal Context
Recordar cuándo y en qué contexto se usó un documento.

```typescript
interface TemporalContext {
  documentId: string;
  accessHistory: {
    timestamp: Date;
    context: string; // "working on project X", "researching Y"
    duration: number; // Tiempo de acceso en segundos
    actions: string[]; // "read", "edited", "referenced"
  }[];
  patterns: {
    timeOfDay: 'morning' | 'afternoon' | 'evening';
    dayOfWeek: number;
    frequency: number; // Accesos por semana
  };
}
```

#### 7.2 Contextual Retrieval
Recuperar documentos basándose en contexto actual.

```typescript
interface ContextualQuery {
  currentActivity: string;
  recentDocuments: string[];
  timeContext: 'today' | 'this-week' | 'this-month';
  semanticContext: string[]; // Conceptos relacionados
}
```

---

## 8. Sistemas de Recomendación para Documentos

### Concepto
Recomendar documentos relevantes basándose en patrones de uso, contenido y contexto.

### Referencias Académicas
- **Ricci, F. et al. (2011)**: "Recommender Systems Handbook"
- **Gunawardana, A. & Shani, G. (2015)**: "Evaluating Recommender Systems"
- **Aggarwal, C.C. (2016)**: "Recommender Systems: The Textbook"

### Técnicas Propuestas

#### 8.1 Collaborative Filtering
Recomendar basándose en qué documentos usan usuarios similares.

```typescript
interface UserSimilarity {
  userId1: string;
  userId2: string;
  similarity: number; // 0-1
  commonDocuments: string[];
}

interface Recommendation {
  documentId: string;
  score: number; // 0-1, relevancia
  reason: string; // "Users with similar interests also viewed"
  source: 'collaborative' | 'content' | 'hybrid';
}
```

#### 8.2 Content-Based Filtering
Recomendar basándose en similitud de contenido.

```typescript
interface ContentBasedRecommendation {
  documentId: string;
  score: number;
  similarDocuments: {
    id: string;
    similarity: number;
    sharedConcepts: string[];
  }[];
}
```

---

## 9. Visualización de Espacios de Información

### Concepto
Visualizar documentos en espacios multidimensionales para exploración y descubrimiento.

### Referencias Académicas
- **Card, S.K. et al. (1999)**: "Readings in Information Visualization: Using Vision to Think"
- **Shneiderman, B. (1996)**: "The Eyes Have It: A Task by Data Type Taxonomy for Information Visualizations"
- **Keim, D.A. (2002)**: "Information Visualization and Visual Data Mining"

### Visualizaciones Propuestas

#### 9.1 Document Map (2D/3D)
Mapear documentos en espacio 2D/3D basado en similitud semántica.

```typescript
interface DocumentMap {
  documents: {
    id: string;
    position: { x: number; y: number; z?: number };
    cluster?: string;
    neighbors: string[]; // Documentos cercanos
  }[];
  clusters: {
    id: string;
    centroid: { x: number; y: number; z?: number };
    documents: string[];
    label: string;
  }[];
}
```

#### 9.2 Timeline Visualization
Visualizar documentos en línea de tiempo con contexto.

```typescript
interface TimelineView {
  events: {
    documentId: string;
    timestamp: Date;
    type: 'created' | 'modified' | 'accessed' | 'tagged';
    context?: string;
  }[];
  periods: {
    start: Date;
    end: Date;
    label: string;
    documents: string[];
  }[];
}
```

#### 9.3 Network Graph
Visualizar relaciones entre documentos como grafo.

```typescript
interface NetworkGraph {
  nodes: {
    id: string;
    type: 'document' | 'concept' | 'project';
    position: { x: number; y: number };
    size: number; // Basado en importancia
    color: string; // Basado en categoría
  }[];
  edges: {
    source: string;
    target: string;
    type: string;
    weight: number;
    visible: boolean; // Para filtrado
  }[];
}
```

---

## 10. Metadatos Enriquecidos y Extracción Automática

### Concepto
Extraer y usar metadatos ricos de documentos para mejor organización.

### Referencias Académicas
- **Dublin Core Metadata Initiative**: Estándar de metadatos
- **ISO 15836**: Dublin Core Metadata Element Set
- **NISO Z39.85**: Dublin Core Metadata Element Set

### Metadatos Propuestos

```typescript
interface RichMetadata {
  // Metadatos básicos
  title?: string;
  author?: string[];
  subject?: string[];
  description?: string;
  publisher?: string;
  date?: Date;
  
  // Metadatos técnicos
  format: string;
  language: string;
  identifier: string;
  
  // Metadatos semánticos
  keywords: string[];
  concepts: string[]; // Conceptos extraídos
  entities: { // Entidades nombradas
    type: 'person' | 'organization' | 'location' | 'date';
    value: string;
    confidence: number;
  }[];
  
  // Metadatos de relación
  references: string[]; // Documentos referenciados
  citedBy: string[]; // Documentos que citan este
  related: string[]; // Documentos relacionados
  
  // Metadatos de uso
  accessCount: number;
  lastAccessed: Date;
  averageReadTime?: number;
}
```

---

## Priorización de Implementación

### Fase 1: Fundamentos (Alto Impacto, Media Complejidad)
1. **Clasificación Facetada** - Base para organización multi-dimensional
2. **Polijerarquías** - Extender sistema de proyectos actual
3. **Knowledge Graph** - Mejorar sistema de relaciones existente

### Fase 2: Inteligencia (Alto Impacto, Alta Complejidad)
4. **Clasificación Automática con ML** - Mejorar sugerencias AI actuales
5. **Sistemas de Recomendación** - Descubrimiento de documentos
6. **Metadatos Enriquecidos** - Mejorar extracción actual

### Fase 3: UX Avanzada (Medio Impacto, Media Complejidad)
7. **Visualización de Espacios** - Interfaces de exploración
8. **Organización Temporal/Contextual** - Mejorar memoria temporal actual
9. **Folksonomies** - Etiquetado colaborativo

### Fase 4: PIM Completo (Medio Impacto, Alta Complejidad)
10. **Activity-Based Organization** - Nuevo paradigma de organización
11. **Information Scraps** - Gestión de fragmentos de información

---

## Referencias Completas

### Libros Fundamentales
- Svenonius, E. (2000). *The Intellectual Foundation of Information Organization*. MIT Press.
- Jones, W. (2007). *Keeping Found Things Found: The Study and Practice of Personal Information Management*. Morgan Kaufmann.
- Card, S.K., Mackinlay, J.D., & Shneiderman, B. (1999). *Readings in Information Visualization: Using Vision to Think*. Morgan Kaufmann.

### Papers Clave
- Ranganathan, S.R. (1933). "Colon Classification". *Library Science with a Slant to Documentation*.
- Dumais, S., Cutrell, E., Cadiz, J.J., Jancke, G., Sarin, R., & Robbins, D.C. (2003). "Stuff I've Seen: A System for Personal Information Retrieval and Re-use". *SIGIR*.
- Ehrlinger, L., & Wöß, W. (2016). "Towards a Definition of Knowledge Graphs". *SEMANTiCS*.

### Estándares
- ISO/IEC 15489: Information and documentation - Records management
- Dublin Core Metadata Initiative (DCMI)
- Encoded Archival Description (EAD)

---

**Nota**: Estas propuestas están basadas en literatura académica establecida y pueden implementarse incrementalmente en Cortex, aprovechando la arquitectura backend-first existente y el sistema de Knowledge Engine ya implementado.







