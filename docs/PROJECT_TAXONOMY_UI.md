# UI de Taxonomía de Proyectos - Implementación

## Resumen

Se ha implementado una interfaz de usuario completa para la taxonomía de proyectos en Cortex, permitiendo a los usuarios crear, editar y visualizar proyectos con sus naturalezas y atributos específicos.

## Componentes Implementados

### 1. Utilidades de Naturaleza de Proyecto (`src/utils/projectNature.ts`)

**Funcionalidades:**
- Definición de 9 categorías principales con 30+ tipos de proyectos
- Funciones helper para obtener labels, descripciones e iconos
- Generador de QuickPick items para selección de naturaleza

**Categorías:**
- Escritura y Creación (5 tipos)
- Colecciones (3 tipos)
- Desarrollo (4 tipos)
- Gestión (3 tipos)
- Jerárquico (3 tipos)
- Compras (4 tipos)
- Educación (3 tipos)
- Eventos (3 tipos)
- Referencia (3 tipos)

### 2. Comandos de Proyecto (`src/commands/createProject.ts`)

#### `createProjectCommand`
Flujo completo para crear un nuevo proyecto:
1. Solicita nombre del proyecto
2. Verifica si ya existe (ofrece editar si existe)
3. Muestra selector de naturaleza con categorías organizadas
4. Solicita descripción opcional
5. Crea el proyecto con naturaleza seleccionada

#### `editProjectCommand`
Permite editar proyectos existentes:
- **Edit Name**: Cambiar nombre
- **Edit Description**: Cambiar descripción
- **Change Nature**: Cambiar tipo/naturaleza
- **Edit Attributes**: Editar atributos (status, priority, temporality, collaboration, visibility)

#### `editProjectAttributes`
Editor interactivo de atributos:
- Status: planning, active, on-hold, completed, archived
- Priority: low, medium, high, critical
- Temporality: temporary, ongoing
- Collaboration: individual, team, organization
- Visibility: private, shared, public

### 3. Cliente gRPC Actualizado (`src/core/GrpcKnowledgeClient.ts`)

**Nuevos métodos:**
- `createProjectWithNature()`: Crea proyecto con naturaleza específica
- `updateProject()`: Actualiza proyecto (nombre, descripción, naturaleza, atributos)

**Interfaz Project actualizada:**
```typescript
interface Project {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  nature?: string;        // NUEVO
  attributes?: string;    // NUEVO (JSON)
  parent_id?: string;
  created_at: number;
  updated_at: number;
}
```

### 4. Vista de Proyectos Mejorada (`src/views/ContextTreeProvider.ts`)

**Mejoras visuales:**
- Muestra icono y label de naturaleza junto al nombre del proyecto
- Tooltip con información completa (nombre, tipo, descripción, atributos)
- Formato: `Nombre (archivos) • 📖 Libro`

**Ejemplo de visualización:**
```
📁 Mi Libro (15) • 📖 Libro
📁 Compra Vehículo (8) • 🚗 Vehículo
📁 ERP Empresa (42) • 🖥️ ERP
```

### 5. Comandos Registrados (`package.json` y `extension.ts`)

**Nuevos comandos:**
- `cortex.createProject`: Crear nuevo proyecto con naturaleza
- `cortex.editProject`: Editar proyecto existente

**Menús contextuales:**
- Click derecho en proyecto → "Edit Project"
- Vista de proyectos → Botón "Create New Project"

**Command Palette:**
- "Cortex: Create New Project"
- "Cortex: Edit Project"

## Flujo de Usuario

### Crear Proyecto

1. Usuario ejecuta `Cortex: Create New Project` (Command Palette o menú)
2. Ingresa nombre del proyecto
3. Selecciona naturaleza de una lista organizada por categorías:
   ```
   📁 Escritura y Creación
     📖 Libro
     🎓 Tesis
     📄 Artículo
     ...
   📁 Desarrollo
     💻 Software
     🖥️ ERP
     ...
   ```
4. (Opcional) Ingresa descripción
5. Proyecto creado con naturaleza asignada

### Editar Proyecto

1. Click derecho en proyecto → "Edit Project"
2. Selecciona qué editar:
   - Nombre
   - Descripción
   - Naturaleza
   - Atributos
3. Realiza cambios
4. Vista se actualiza automáticamente

### Visualizar Naturaleza

- En la vista "By Project", cada proyecto muestra:
  - Nombre
  - Cantidad de archivos
  - Icono y label de naturaleza
- Tooltip muestra información completa al pasar el mouse

## Ejemplos de Uso

### Crear un libro
```
Nombre: "Mi Novela"
Naturaleza: writing.book
Descripción: "Novela de ciencia ficción"
Atributos: { status: "active", priority: "high" }
```

### Crear proyecto jerárquico
```
Proyecto Padre:
  Nombre: "Colegio de los Hijos"
  Naturaleza: hierarchical.parent

Subproyecto:
  Nombre: "Matrícula Felipe"
  Naturaleza: education.school
  Parent: "Colegio de los Hijos"
```

### Crear compra
```
Nombre: "Compra Vehículo Ford 2019"
Naturaleza: purchase.vehicle
Atributos: { 
  status: "active",
  temporality: "temporary",
  priority: "high"
}
```

## Integración con Backend

La UI se integra completamente con el backend Go:
- Usa gRPC para todas las operaciones
- Sincroniza con la base de datos SQLite
- Respeta la migración versión 7 (campos nature y attributes)
- Compatible con la taxonomía definida en el backend

## Próximas Mejoras Sugeridas

1. **Filtros por Naturaleza**: Agregar filtro en la vista para mostrar solo proyectos de cierto tipo
2. **Vista por Naturaleza**: Nueva vista que agrupe proyectos por naturaleza
3. **Sugerencias Inteligentes**: AI sugiere naturaleza basada en nombre/descripción
4. **Plantillas**: Plantillas predefinidas por naturaleza con atributos comunes
5. **Validación**: Validar que atributos sean consistentes con naturaleza
6. **Métricas por Naturaleza**: Estadísticas específicas según tipo de proyecto

## Archivos Modificados/Creados

### Nuevos
- `src/utils/projectNature.ts` - Utilidades de naturaleza
- `src/commands/createProject.ts` - Comandos de creación/edición
- `docs/PROJECT_TAXONOMY_UI.md` - Esta documentación

### Modificados
- `src/core/GrpcKnowledgeClient.ts` - Métodos nuevos
- `src/views/ContextTreeProvider.ts` - Visualización mejorada
- `src/extension.ts` - Registro de comandos
- `package.json` - Definición de comandos y menús

## Estado

✅ **Completado:**
- Selector de naturaleza al crear proyecto
- Editor de naturaleza y atributos
- Visualización de naturaleza en tree view
- Comandos registrados y funcionando
- Integración completa con backend

⏳ **Pendiente (opcional):**
- Filtros por naturaleza
- Vista especializada por naturaleza
- Sugerencias AI de naturaleza
- Validación de atributos







