# Cortex Test Suite

## Estructura de Tests

Los tests están organizados de forma clara y práctica:

```
src/test/
├── helpers/
│   └── testHelpers.ts          # Helpers compartidos (mocks, utilidades)
├── suite/
│   ├── index.ts                # Punto de entrada - carga todos los tests
│   ├── commands.test.ts        # Tests de comandos VS Code
│   ├── utils.test.ts           # Tests de utilidades (fileHash, osTags, etc.)
│   ├── projectDeduplication.test.ts  # Tests de deduplicación de proyectos
│   ├── treeProviders.unit.test.ts   # Tests unitarios de tree providers
│   └── treeProvidersIntegration.test.ts  # Tests de integración de tree providers
└── fixtures/
    └── workspace/              # Datos de prueba
```

## Categorías de Tests

### 1. **Unit Tests** (`*.unit.test.ts`)
- Tests rápidos y aislados
- Usan mocks para dependencias
- Validan comportamientos específicos
- Ejemplos: `treeProviders.unit.test.ts`, `utils.test.ts`

### 2. **Integration Tests** (`*.integration.test.ts`)
- Tests más completos con datos reales
- Validan flujos end-to-end
- Verifican que los componentes trabajan juntos
- Ejemplos: `treeProvidersIntegration.test.ts`

### 3. **Feature Tests** (`*.test.ts`)
- Tests de funcionalidades completas
- Ejemplos: `commands.test.ts`, `projectDeduplication.test.ts`

## Ejecutar Tests

```bash
# Compilar primero
npm run compile

# Ejecutar todos los tests
npm test

# Ejecutar tests en modo watch (si está configurado)
npm run test:watch
```

## Helpers Compartidos

Todos los helpers comunes están en `test/helpers/testHelpers.ts`:

- `createMockContext()` - Crea un ExtensionContext mock
- `withMockedFileCache()` - Mockea FileCacheService
- `getChildrenItems()` - Obtiene hijos de tree providers
- `createMockMetadataStore()` - Crea un IMetadataStore mock
- `comprehensiveTestData` - Datos de prueba completos

## Agregar Nuevos Tests

1. **Para tests unitarios**: Agregar a archivo `*.unit.test.ts` existente o crear uno nuevo
2. **Para tests de integración**: Agregar a archivo `*.integration.test.ts` existente
3. **Usar helpers compartidos**: Importar desde `../helpers/testHelpers`
4. **Evitar duplicación**: Revisar helpers existentes antes de crear nuevos

## Convenciones

- **Nombres descriptivos**: `should return files sorted by activity`
- **Agrupar por funcionalidad**: Usar `describe()` para agrupar tests relacionados
- **Tests independientes**: Cada test debe poder ejecutarse solo
- **Limpieza**: Usar `finally` para restaurar mocks

## Mantenimiento

- **Sin redundancias**: Helpers compartidos evitan duplicación
- **Fácil de extender**: Agregar nuevos tests es simple
- **Bien documentado**: Cada archivo tiene comentarios claros



