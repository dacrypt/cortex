# Diseño: Metadatos del Sistema Operativo y Modelo de Personas/Usuarios

## Resumen Ejecutivo

Este documento propone una extensión del sistema de indexado de Cortex para capturar y clasificar metadatos del sistema operativo (permisos, usuarios, grupos, atributos) y establecer un modelo de personas/usuarios con relaciones hacia archivos y proyectos. La información se organiza mediante una taxonomía multi-jerárquica que permite múltiples clasificaciones contextuales.

## Objetivos

1. **Extracción Completa de Metadatos OS**: Capturar toda la información disponible del sistema operativo sobre archivos (permisos, propietarios, grupos, atributos extendidos, ACLs, etc.)

2. **Modelo de Personas y Usuarios**: Establecer entidades separadas para personas (identidad humana) y usuarios del sistema (cuentas OS), con relaciones hacia archivos y proyectos.

3. **Taxonomía Multi-jerárquica**: Clasificar la información de OS en múltiples dimensiones taxonómicas que permitan diferentes vistas y análisis.

4. **Contexto Enriquecido**: Utilizar esta información como contexto para sugerencias de proyectos, tags, y relaciones entre archivos.

## 1. Metadatos del Sistema Operativo

### 1.1 Información a Extraer

#### Información Básica del Archivo (ya parcialmente implementado)
- **Permisos**: Octal (0644), string (-rw-r--r--), y desglose por bits
- **Propietario**: UID y nombre de usuario
- **Grupo**: GID y nombre de grupo
- **Inodo**: Número de inodo
- **Enlaces duros**: Cantidad de enlaces
- **Tipo de archivo**: Regular, directorio, symlink, dispositivo, etc.
- **Tamaño en bloques**: Bloques asignados
- **Tamaño de bloque**: Tamaño del bloque del sistema de archivos

#### Atributos Extendidos (Extended Attributes)
- **macOS**: `xattr` - tags de Finder, quarantine, metadata
- **Linux**: `getfattr` - atributos de seguridad, metadata personalizada
- **Windows**: `attrib` - atributos de archivo (hidden, system, readonly, archive)

#### ACLs (Access Control Lists)
- **macOS/Linux**: ACLs POSIX (getfacl)
- **Windows**: ACLs NTFS (icacls)

#### Timestamps Extendidos
- **Creado**: Birth time (disponible en algunos sistemas de archivos)
- **Modificado**: Modification time
- **Accedido**: Access time
- **Cambiado**: Change time (metadata modificada)
- **Backup**: Último backup (si está disponible)

#### Información del Sistema de Archivos
- **Mount point**: Punto de montaje
- **Device ID**: ID del dispositivo
- **File system type**: Tipo de sistema de archivos (APFS, ext4, NTFS, etc.)
- **SELinux context**: Contexto de seguridad (Linux)

#### Atributos Específicos por OS

**macOS**:
- Finder tags (colores y etiquetas)
- Quarantine attributes
- Spotlight comments
- Resource fork metadata
- Extended attributes personalizados

**Linux**:
- SELinux context
- Capabilities (setcap)
- Immutable flag
- Append-only flag
- No dump flag
- Synchronous updates

**Windows**:
- File attributes (hidden, system, readonly, archive, compressed, encrypted)
- Alternate data streams (ADS)
- Security descriptors
- File ownership (SID)

### 1.2 Estructura de Datos Propuesta

```go
// OSMetadata contiene metadatos del sistema operativo
type OSMetadata struct {
    // Permisos y Propietario
    Permissions     *PermissionsInfo
    Owner           *UserInfo
    Group           *GroupInfo
    
    // Atributos del archivo
    FileAttributes  *FileAttributes
    ExtendedAttrs   map[string]string // xattr/ADS
    
    // ACLs
    ACLs            []ACLEntry
    
    // Timestamps
    Timestamps      *OSTimestamps
    
    // Sistema de archivos
    FileSystem      *FileSystemInfo
    
    // OS específico
    PlatformSpecific map[string]interface{}
}

type PermissionsInfo struct {
    Octal          string // "0644"
    String         string // "-rw-r--r--"
    OwnerRead      bool
    OwnerWrite     bool
    OwnerExecute   bool
    GroupRead      bool
    GroupWrite     bool
    GroupExecute   bool
    OtherRead      bool
    OtherWrite     bool
    OtherExecute   bool
    SetUID         bool
    SetGID         bool
    StickyBit     bool
}

type UserInfo struct {
    UID            int
    Username       string
    FullName       string // Si está disponible
    HomeDir        string // Si está disponible
    Shell          string // Si está disponible
}

type GroupInfo struct {
    GID            int
    GroupName      string
    Members        []string // Miembros del grupo
}

type FileAttributes struct {
    IsReadOnly     bool
    IsHidden       bool
    IsSystem       bool
    IsArchive      bool
    IsCompressed   bool
    IsEncrypted    bool
    IsImmutable    bool // Linux
    IsAppendOnly   bool // Linux
    IsNoDump       bool // Linux
}

type ACLEntry struct {
    Type           string // "user", "group", "mask", "other"
    Identity       string // Usuario o grupo
    Permissions    string // "rwx", "r--", etc.
    Flags          string // Opcional
}

type OSTimestamps struct {
    Created        *time.Time
    Modified       time.Time
    Accessed       time.Time
    Changed        time.Time // Metadata changed
    Backup         *time.Time
}

type FileSystemInfo struct {
    MountPoint     string
    DeviceID       string
    FileSystemType string
    BlockSize      int64
    Blocks         int64
    SELinuxContext *string // Linux
}
```

## 2. Modelo de Personas y Usuarios

### 2.1 Concepto

Separamos dos entidades:

1. **Persona**: Identidad humana (puede tener múltiples usuarios en diferentes sistemas)
2. **Usuario**: Cuenta del sistema operativo (pertenece a una persona)

### 2.2 Relaciones

```
Persona (1) ──< (N) Usuario
Usuario (1) ──< (N) FileOwnership (archivos que posee)
Usuario (1) ──< (N) FileAccess (archivos a los que tiene acceso)
Persona (1) ──< (N) ProjectMembership (proyectos en los que participa)
```

### 2.3 Estructura de Datos

```go
// Person representa una persona física
type Person struct {
    ID              string
    Name            string
    Email           *string
    DisplayName     string
    Notes           string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// SystemUser representa un usuario del sistema operativo
type SystemUser struct {
    ID              string
    PersonID        *string // FK a Person (opcional, puede ser usuario del sistema sin persona asociada)
    Username        string
    UID             int
    FullName        string
    HomeDir         string
    Shell           string
    System          bool // Si es usuario del sistema (root, daemon, etc.)
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// FileOwnership representa la propiedad de un archivo
type FileOwnership struct {
    FileID          string
    WorkspaceID     string
    UserID          string // FK a SystemUser
    OwnershipType  string // "owner", "group_member", "other"
    Permissions     string // Permisos específicos para este usuario
    DetectedAt      time.Time
}

// FileAccess representa acceso a un archivo (ACL, etc.)
type FileAccess struct {
    FileID          string
    WorkspaceID     string
    UserID          string // FK a SystemUser
    AccessType      string // "read", "write", "execute", "full"
    Source          string // "permissions", "acl", "group_membership"
    DetectedAt      time.Time
}

// ProjectMembership representa membresía en proyectos
type ProjectMembership struct {
    ProjectID       string
    WorkspaceID     string
    PersonID        string // FK a Person
    Role            string // "owner", "contributor", "viewer"
    JoinedAt        time.Time
}
```

## 3. Taxonomía Multi-jerárquica

### 3.1 Principios de Clasificación

La información de OS se clasifica en múltiples dimensiones taxonómicas que permiten diferentes vistas:

1. **Dimensión de Seguridad**: Permisos, ACLs, atributos de seguridad
2. **Dimensión de Propiedad**: Propietarios, grupos, relaciones de acceso
3. **Dimensión Temporal**: Timestamps, patrones de acceso
4. **Dimensión de Sistema**: Tipo de archivo, sistema de archivos, atributos del sistema
5. **Dimensión de Organización**: Agrupaciones por usuario, grupo, proyecto

### 3.2 Estructura Taxonómica

```go
// OSContextTaxonomy organiza metadatos OS en múltiples dimensiones
type OSContextTaxonomy struct {
    // Dimensión de Seguridad
    Security        *SecurityTaxonomy
    
    // Dimensión de Propiedad
    Ownership       *OwnershipTaxonomy
    
    // Dimensión Temporal
    Temporal        *TemporalTaxonomy
    
    // Dimensión de Sistema
    System          *SystemTaxonomy
    
    // Dimensión de Organización
    Organization    *OrganizationTaxonomy
}

type SecurityTaxonomy struct {
    // Nivel de permisos
    PermissionLevel string // "public", "group", "private", "restricted"
    
    // Categorías de seguridad
    SecurityCategory []string // ["readable_by_group", "writable_by_owner", "executable"]
    
    // Atributos de seguridad
    SecurityAttributes []string // ["encrypted", "immutable", "quarantined"]
    
    // ACLs presentes
    HasACLs         bool
    ACLComplexity   string // "simple", "complex"
}

type OwnershipTaxonomy struct {
    // Tipo de propietario
    OwnerType       string // "user", "system", "service", "unknown"
    
    // Categoría de grupo
    GroupCategory   string // "admin", "developer", "service", "custom"
    
    // Relaciones de acceso
    AccessRelations []string // ["owned_by_user", "accessible_by_group", "public_read"]
    
    // Patrones de propiedad
    OwnershipPattern string // "single_owner", "shared_group", "multi_user"
}

type TemporalTaxonomy struct {
    // Patrón temporal
    TemporalPattern string // "recent", "archived", "active", "stale"
    
    // Frecuencia de acceso
    AccessFrequency string // "frequent", "occasional", "rare", "never"
    
    // Categoría de tiempo
    TimeCategory    []string // ["created_recently", "modified_this_week", "accessed_today"]
    
    // Relaciones temporales
    TemporalRelations []string // ["newer_than", "older_than", "same_period"]
}

type SystemTaxonomy struct {
    // Tipo de archivo del sistema
    SystemFileType  string // "regular", "directory", "symlink", "device", "special"
    
    // Categoría del sistema de archivos
    FileSystemCategory string // "local", "network", "removable", "virtual"
    
    // Atributos del sistema
    SystemAttributes []string // ["hidden", "system", "archive", "compressed"]
    
    // Características del sistema
    SystemFeatures  []string // ["extended_attrs", "acls", "hard_links", "sparse"]
}

type OrganizationTaxonomy struct {
    // Agrupación por usuario
    UserGrouping    []string // ["files_by_david", "files_by_service"]
    
    // Agrupación por grupo
    GroupGrouping  []string // ["files_in_admin_group", "files_in_dev_group"]
    
    // Agrupación por proyecto (inferida)
    ProjectGrouping []string // ["project_a_files", "project_b_files"]
    
    // Patrones organizacionales
    OrgPatterns     []string // ["user_workspace", "shared_directory", "project_folder"]
}
```

### 3.3 Clasificadores Automáticos

Cada dimensión taxonómica tiene clasificadores que analizan los metadatos OS y asignan categorías:

```go
// SecurityClassifier analiza metis de seguridad
type SecurityClassifier struct{}

func (c *SecurityClassifier) Classify(meta *OSMetadata) *SecurityTaxonomy {
    // Analiza permisos, ACLs, atributos
    // Asigna PermissionLevel, SecurityCategory, etc.
}

// OwnershipClassifier analiza propiedad y acceso
type OwnershipClassifier struct{}

func (c *OwnershipClassifier) Classify(meta *OSMetadata, users map[string]*SystemUser) *OwnershipTaxonomy {
    // Analiza propietarios, grupos, relaciones
    // Asigna OwnerType, GroupCategory, etc.
}

// TemporalClassifier analiza patrones temporales
type TemporalClassifier struct{}

func (c *TemporalClassifier) Classify(meta *OSMetadata, history []FileEvent) *TemporalTaxonomy {
    // Analiza timestamps, eventos
    // Asigna TemporalPattern, AccessFrequency, etc.
}

// SystemClassifier analiza características del sistema
type SystemClassifier struct{}

func (c *SystemClassifier) Classify(meta *OSMetadata) *SystemTaxonomy {
    // Analiza tipo de archivo, FS, atributos
    // Asigna SystemFileType, FileSystemCategory, etc.
}

// OrganizationClassifier analiza organización
type OrganizationClassifier struct{}

func (c *OrganizationClassifier) Classify(
    meta *OSMetadata,
    users map[string]*SystemUser,
    projects map[string]*Project,
) *OrganizationTaxonomy {
    // Analiza agrupaciones, patrones
    // Asigna UserGrouping, ProjectGrouping, etc.
}
```

## 4. Integración en el Pipeline

### 4.1 Nuevo Stage: OSMetadataStage

```go
// OSMetadataStage extrae metadatos del sistema operativo
type OSMetadataStage struct {
    extractor      *OSMetadataExtractor
    classifier     *OSContextClassifier
    userRepo       *SystemUserRepository
    personRepo     *PersonRepository
    logger         zerolog.Logger
}

func (s *OSMetadataStage) Process(ctx context.Context, entry *entity.FileEntry) error {
    // 1. Extraer metadatos OS
    osMeta, err := s.extractor.Extract(ctx, entry.AbsolutePath)
    if err != nil {
        return err
    }
    
    // 2. Clasificar en taxonomía
    taxonomy := s.classifier.Classify(osMeta, entry)
    
    // 3. Actualizar/Crear usuarios del sistema
    if osMeta.Owner != nil {
        user, err := s.userRepo.Upsert(ctx, osMeta.Owner)
        if err != nil {
            s.logger.Warn().Err(err).Msg("Failed to upsert user")
        }
        
        // Crear relación de propiedad
        ownership := &FileOwnership{
            FileID: entry.ID.String(),
            UserID: user.ID,
            OwnershipType: "owner",
            Permissions: osMeta.Permissions.String,
        }
        // Guardar ownership
    }
    
    // 4. Almacenar metadatos OS en entry.Enhanced
    if entry.Enhanced == nil {
        entry.Enhanced = &entity.EnhancedMetadata{}
    }
    entry.Enhanced.OSMetadata = osMeta
    entry.Enhanced.OSContextTaxonomy = taxonomy
    
    return nil
}
```

### 4.2 Orden en el Pipeline

El `OSMetadataStage` debe ejecutarse temprano en el pipeline, después de `BasicStage`:

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

## 5. Esquema de Base de Datos

### 5.1 Nuevas Tablas

```sql
-- Tabla de personas
CREATE TABLE IF NOT EXISTS persons (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    email TEXT,
    display_name TEXT,
    notes TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_persons_workspace ON persons(workspace_id);
CREATE INDEX IF NOT EXISTS idx_persons_email ON persons(workspace_id, email);

-- Tabla de usuarios del sistema
CREATE TABLE IF NOT EXISTS system_users (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    person_id TEXT, -- FK a persons (opcional)
    username TEXT NOT NULL,
    uid INTEGER NOT NULL,
    full_name TEXT,
    home_dir TEXT,
    shell TEXT,
    is_system INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE SET NULL,
    UNIQUE(workspace_id, username, uid)
);

CREATE INDEX IF NOT EXISTS idx_system_users_workspace ON system_users(workspace_id);
CREATE INDEX IF NOT EXISTS idx_system_users_person ON system_users(person_id);
CREATE INDEX IF NOT EXISTS idx_system_users_username ON system_users(workspace_id, username);

-- Tabla de propiedad de archivos
CREATE TABLE IF NOT EXISTS file_ownership (
    file_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    ownership_type TEXT NOT NULL, -- "owner", "group_member", "other"
    permissions TEXT,
    detected_at INTEGER NOT NULL,
    PRIMARY KEY (workspace_id, file_id, user_id),
    FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES system_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_file_ownership_user ON file_ownership(workspace_id, user_id);
CREATE INDEX IF NOT EXISTS idx_file_ownership_type ON file_ownership(workspace_id, ownership_type);

-- Tabla de acceso a archivos (ACLs, etc.)
CREATE TABLE IF NOT EXISTS file_access (
    id TEXT PRIMARY KEY,
    file_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    access_type TEXT NOT NULL, -- "read", "write", "execute", "full"
    source TEXT NOT NULL, -- "permissions", "acl", "group_membership"
    detected_at INTEGER NOT NULL,
    FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES system_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_file_access_file ON file_access(workspace_id, file_id);
CREATE INDEX IF NOT EXISTS idx_file_access_user ON file_access(workspace_id, user_id);

-- Tabla de membresía en proyectos
CREATE TABLE IF NOT EXISTS project_memberships (
    project_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    person_id TEXT NOT NULL,
    role TEXT NOT NULL, -- "owner", "contributor", "viewer"
    joined_at INTEGER NOT NULL,
    PRIMARY KEY (workspace_id, project_id, person_id),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_memberships_person ON project_memberships(workspace_id, person_id);
CREATE INDEX IF NOT EXISTS idx_project_memberships_project ON project_memberships(workspace_id, project_id);

-- Extensión de la tabla files para almacenar metadatos OS
ALTER TABLE files ADD COLUMN os_metadata TEXT; -- JSON de OSMetadata
ALTER TABLE files ADD COLUMN os_taxonomy TEXT; -- JSON de OSContextTaxonomy
```

## 6. Uso como Contexto

### 6.1 Contexto para Sugerencias de Proyectos

Los metadatos OS pueden ayudar a sugerir proyectos:

- Archivos del mismo propietario → mismo proyecto
- Archivos en el mismo grupo → proyecto relacionado
- Patrones temporales similares → proyecto activo
- Permisos similares → proyecto compartido

### 6.2 Contexto para Tags

- Propietario → tag "owned_by:{username}"
- Grupo → tag "group:{groupname}"
- Permisos → tag "permission:{level}"
- Atributos → tag "attr:{attribute}"

### 6.3 Contexto para Relaciones

- Archivos del mismo propietario → relacionados
- Archivos accesibles por el mismo grupo → relacionados
- Archivos con patrones temporales similares → relacionados

## 7. Implementación por Plataforma

### 7.1 macOS

```go
// macOSOSExtractor extrae metadatos específicos de macOS
type macOSOSExtractor struct{}

func (e *macOSOSExtractor) Extract(ctx context.Context, path string) (*OSMetadata, error) {
    // Usar stat con formato macOS
    // Extraer xattr (Finder tags, quarantine, etc.)
    // Extraer ACLs con getfacl
    // Extraer información de usuarios con dscl
}
```

### 7.2 Linux

```go
// LinuxOSExtractor extrae metadatos específicos de Linux
type LinuxOSExtractor struct{}

func (e *LinuxOSExtractor) Extract(ctx context.Context, path string) (*OSMetadata, error) {
    // Usar stat con formato Linux
    // Extraer atributos extendidos con getfattr
    // Extraer ACLs con getfacl
    // Extraer contexto SELinux
    // Extraer capabilities
}
```

### 7.3 Windows

```go
// WindowsOSExtractor extrae metadatos específicos de Windows
type WindowsOSExtractor struct{}

func (e *WindowsOSExtractor) Extract(ctx context.Context, path string) (*OSMetadata, error) {
    // Usar GetFileAttributes
    // Extraer ACLs con icacls
    // Extraer SID del propietario
    // Extraer atributos de archivo
    // Extraer Alternate Data Streams
}
```

## 8. Consideraciones de Rendimiento

1. **Caché de Usuarios**: Los usuarios del sistema se cachean para evitar consultas repetidas
2. **Extracción Lazy**: Algunos metadatos costosos (ACLs, xattr) se extraen solo si están habilitados
3. **Indexación Incremental**: Solo se re-extraen metadatos OS si el archivo cambió
4. **Batch Processing**: Agrupar operaciones de base de datos

## 9. Seguridad y Privacidad

1. **Datos Sensibles**: Los metadatos OS pueden contener información sensible (usuarios, grupos, ACLs)
2. **Configuración**: Permitir deshabilitar extracción de ciertos metadatos
3. **Encriptación**: Considerar encriptar metadatos OS en reposo
4. **Acceso**: Controlar quién puede ver metadatos OS de otros usuarios

## 10. Próximos Pasos

1. Implementar `OSMetadataExtractor` con soporte multi-plataforma
2. Crear `OSMetadataStage` en el pipeline
3. Implementar clasificadores taxonómicos
4. Crear repositorios para Person y SystemUser
5. Agregar migraciones de base de datos
6. Integrar en sugerencias de proyectos/tags
7. Crear vistas UI para explorar metadatos OS
8. Documentar y probar






