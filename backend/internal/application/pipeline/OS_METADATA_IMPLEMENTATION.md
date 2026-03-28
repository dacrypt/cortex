# Implementación: Metadatos del Sistema Operativo

## Resumen

Se ha implementado un sistema completo para extraer, clasificar y almacenar metadatos del sistema operativo (permisos, usuarios, grupos, atributos) y un modelo de personas/usuarios con relaciones hacia archivos y proyectos. La información se organiza mediante una taxonomía multi-jerárquica que permite múltiples clasificaciones contextuales.

## Componentes Implementados

### 1. Entidades de Dominio (`backend/internal/domain/entity/os_metadata.go`)

- **OSMetadata**: Contiene metadatos del sistema operativo (permisos, propietario, grupo, ACLs, timestamps, etc.)
- **OSContextTaxonomy**: Organiza metadatos OS en 5 dimensiones taxonómicas:
  - **SecurityTaxonomy**: Clasificación de seguridad (niveles de permisos, atributos)
  - **OwnershipTaxonomy**: Clasificación de propiedad (tipos de propietario, grupos, patrones)
  - **TemporalTaxonomy**: Patrones temporales (frecuencia de acceso, categorías de tiempo)
  - **SystemTaxonomy**: Características del sistema (tipo de archivo, sistema de archivos, atributos)
  - **OrganizationTaxonomy**: Patrones organizacionales (agrupaciones por usuario, grupo, proyecto)

- **Person**: Representa una persona física (puede tener múltiples usuarios del sistema)
- **SystemUser**: Representa un usuario del sistema operativo
- **FileOwnership**: Relación de propiedad de archivos
- **FileAccess**: Relación de acceso a archivos (ACLs, etc.)
- **ProjectMembership**: Membresía en proyectos

### 2. Extractor de Metadatos OS (`backend/internal/infrastructure/metadata/os_extractor.go`)

Extractor multi-plataforma que extrae:

- **Permisos**: Octal, string, y desglose por bits (read, write, execute para owner/group/other)
- **Propietario**: UID, nombre de usuario, nombre completo
- **Grupo**: GID, nombre de grupo
- **Timestamps**: Creado, modificado, accedido, cambiado
- **Atributos de archivo**: Read-only, hidden, system, archive, etc.
- **ACLs**: Listas de control de acceso (soporte para macOS, Linux, Windows)
- **Atributos extendidos**: xattr (macOS), getfattr (Linux), ADS (Windows)
- **Información del sistema de archivos**: Mount point, device ID, tipo de FS

**Soporte por plataforma**:
- **macOS**: xattr, Finder tags, quarantine, ACLs
- **Linux**: getfattr, getfacl, SELinux context, lsattr (immutable, append-only)
- **Windows**: attrib, icacls, ACLs NTFS

### 3. Clasificador Taxonómico (`backend/internal/infrastructure/metadata/os_classifier.go`)

Clasifica automáticamente los metadatos OS en las 5 dimensiones taxonómicas:

- **SecurityClassifier**: Analiza permisos y asigna niveles (public, group, private, restricted)
- **OwnershipClassifier**: Analiza propiedad y determina tipos (user, system, service)
- **TemporalClassifier**: Analiza patrones temporales (recent, active, stale, archived)
- **SystemClassifier**: Analiza características del sistema (regular, directory, symlink, etc.)
- **OrganizationClassifier**: Analiza patrones organizacionales (user_workspace, shared_directory, etc.)

### 4. Stage del Pipeline (`backend/internal/application/pipeline/stages/os_metadata.go`)

**OSMetadataStage** que:
- Extrae metadatos OS usando `OSExtractor`
- Clasifica en taxonomía usando `OSContextClassifier`
- Almacena en `entry.Enhanced.OSMetadata` y `entry.Enhanced.OSContextTaxonomy`
- No falla el pipeline si hay errores (solo registra warnings)

### 5. Migraciones de Base de Datos (`migrations.go` - Versión 12)

Nuevas tablas creadas:

- **persons**: Personas físicas
- **system_users**: Usuarios del sistema operativo
- **file_ownership**: Relaciones de propiedad de archivos
- **file_access**: Relaciones de acceso a archivos (ACLs)
- **project_memberships**: Membresías en proyectos

Columnas agregadas a `files`:
- **os_metadata**: JSON de OSMetadata
- **os_taxonomy**: JSON de OSContextTaxonomy

## Integración en el Pipeline

### Agregar el Stage

El `OSMetadataStage` debe agregarse temprano en el pipeline, después de `BasicStage`:

```go
// En cmd/cortexd/main.go o donde se configure el pipeline
orchestrator := pipeline.NewOrchestrator(publisher, logger)

// Agregar OSMetadataStage después de BasicStage
osMetadataStage := stages.NewOSMetadataStage(logger)
orchestrator.InsertStage(1, osMetadataStage) // Después de BasicStage (índice 0)
```

### Orden Recomendado del Pipeline

```
1. BasicStage          → Información básica (tamaño, fechas)
2. OSMetadataStage     → Metadatos OS (permisos, usuarios, atributos) ✨ NUEVO
3. MimeStage           → Tipo MIME
4. MetadataStage       → Metadatos del contenido
5. CodeStage           → Análisis de código
6. DocumentStage       → Análisis de documentos
7. MirrorStage         → Extracción de texto
8. AIStage             → Sugerencias AI (usa OS metadata como contexto)
```

## Uso como Contexto para AI

Los metadatos OS pueden usarse como contexto en el `AIStage` para:

### 1. Sugerencias de Proyectos

- Archivos del mismo propietario → mismo proyecto
- Archivos en el mismo grupo → proyecto relacionado
- Patrones temporales similares → proyecto activo
- Permisos similares → proyecto compartido

**Ejemplo de uso**:
```go
// En AIStage, al sugerir proyectos
if entry.Enhanced.OSMetadata != nil && entry.Enhanced.OSMetadata.Owner != nil {
    owner := entry.Enhanced.OSMetadata.Owner.Username
    // Buscar otros archivos del mismo propietario
    // Agrupar en proyectos sugeridos
}
```

### 2. Sugerencias de Tags

- Propietario → tag "owned_by:{username}"
- Grupo → tag "group:{groupname}"
- Permisos → tag "permission:{level}"
- Atributos → tag "attr:{attribute}"

**Ejemplo de uso**:
```go
// En AIStage, al sugerir tags
if entry.Enhanced.OSContextTaxonomy != nil {
    security := entry.Enhanced.OSContextTaxonomy.Security
    if security != nil {
        // Agregar tags basados en nivel de permisos
        tags = append(tags, "permission:"+security.PermissionLevel)
    }
}
```

### 3. Relaciones entre Archivos

- Archivos del mismo propietario → relacionados
- Archivos accesibles por el mismo grupo → relacionados
- Archivos con patrones temporales similares → relacionados

**Ejemplo de uso**:
```go
// En AIStage, al sugerir archivos relacionados
if entry.Enhanced.OSMetadata != nil && entry.Enhanced.OSMetadata.Owner != nil {
    ownerID := entry.Enhanced.OSMetadata.Owner.UID
    // Buscar otros archivos con mismo ownerID
    // Calcular similitud basada en propietario + temporal patterns
}
```

## Próximos Pasos

### Pendientes

1. **Repositorios**: Crear repositorios para `Person` y `SystemUser` para persistir y consultar usuarios
2. **Integración en Pipeline**: Agregar `OSMetadataStage` al pipeline principal en `cmd/cortexd/main.go`
3. **Upsert de Usuarios**: En `OSMetadataStage`, actualizar/crear usuarios del sistema cuando se detecten
4. **Relaciones de Propiedad**: Crear relaciones `FileOwnership` cuando se detecten propietarios
5. **Vistas UI**: Crear vistas para explorar metadatos OS en la UI
6. **Documentación**: Documentar cómo usar metadatos OS en sugerencias de proyectos/tags

### Mejoras Futuras

1. **Extracción de Miembros de Grupo**: Implementar extracción de miembros de grupos (requiere comandos del sistema)
2. **Timestamps Avanzados**: Mejorar extracción de timestamps con soporte específico por plataforma
3. **Caché de Usuarios**: Implementar caché de usuarios para evitar consultas repetidas
4. **Batch Processing**: Agrupar operaciones de base de datos para mejor rendimiento
5. **Configuración**: Permitir deshabilitar extracción de ciertos metadatos por privacidad

## Ejemplo de Uso

```go
// El stage se ejecuta automáticamente en el pipeline
// Los metadatos OS están disponibles en entry.Enhanced.OSMetadata

entry := &entity.FileEntry{
    // ... campos básicos
}

// Después de procesar por OSMetadataStage
if entry.Enhanced != nil && entry.Enhanced.OSMetadata != nil {
    osMeta := entry.Enhanced.OSMetadata
    
    // Acceder a permisos
    if osMeta.Permissions != nil {
        fmt.Printf("Permisos: %s\n", osMeta.Permissions.String)
    }
    
    // Acceder a propietario
    if osMeta.Owner != nil {
        fmt.Printf("Propietario: %s (UID: %d)\n", osMeta.Owner.Username, osMeta.Owner.UID)
    }
    
    // Acceder a taxonomía
    if entry.Enhanced.OSContextTaxonomy != nil {
        security := entry.Enhanced.OSContextTaxonomy.Security
        if security != nil {
            fmt.Printf("Nivel de permisos: %s\n", security.PermissionLevel)
        }
    }
}
```

## Consideraciones de Seguridad y Privacidad

1. **Datos Sensibles**: Los metadatos OS pueden contener información sensible (usuarios, grupos, ACLs)
2. **Configuración**: Considerar permitir deshabilitar extracción de ciertos metadatos
3. **Encriptación**: Considerar encriptar metadatos OS en reposo si contienen información sensible
4. **Acceso**: Controlar quién puede ver metadatos OS de otros usuarios

## Referencias

- Documento de diseño: `OS_METADATA_DESIGN.md`
- Entidades: `backend/internal/domain/entity/os_metadata.go`
- Extractor: `backend/internal/infrastructure/metadata/os_extractor.go`
- Clasificador: `backend/internal/infrastructure/metadata/os_classifier.go`
- Stage: `backend/internal/application/pipeline/stages/os_metadata.go`






