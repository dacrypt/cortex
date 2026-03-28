/**
 * Project Nature utilities and definitions
 * Provides UI-friendly labels and organization for project types
 */

import * as vscode from 'vscode';

export interface ProjectNatureCategory {
  id: string;
  label: string;
  icon: string;
  natures: ProjectNatureOption[];
}

export interface ProjectNatureOption {
  value: string;
  label: string;
  description: string;
  icon?: string;
}

export const PROJECT_NATURE_CATEGORIES: ProjectNatureCategory[] = [
  {
    id: 'writing',
    label: 'Escritura y Creación',
    icon: '$(edit)',
    natures: [
      {
        value: 'writing.book',
        label: 'Libro',
        description: 'Libro en proceso de escritura',
        icon: '$(book)',
      },
      {
        value: 'writing.thesis',
        label: 'Tesis',
        description: 'Tesis de grado o académica',
        icon: '$(mortar-board)',
      },
      {
        value: 'writing.article',
        label: 'Artículo',
        description: 'Artículo académico o profesional',
        icon: '$(file-text)',
      },
      {
        value: 'writing.documentation',
        label: 'Documentación',
        description: 'Documentación técnica',
        icon: '$(bookmark)',
      },
      {
        value: 'writing.blog',
        label: 'Blog',
        description: 'Blog o publicación periódica',
        icon: '$(rss)',
      },
      {
        value: 'writing.poetry',
        label: 'Poesía',
        description: 'Colección de poemas o poesía',
        icon: '$(book)',
      },
      {
        value: 'writing.screenplay',
        label: 'Guion',
        description: 'Guion de cine, teatro o televisión',
        icon: '$(device-camera-video)',
      },
      {
        value: 'writing.manual',
        label: 'Manual',
        description: 'Manual de usuario o instrucciones',
        icon: '$(bookmark)',
      },
      {
        value: 'writing.report',
        label: 'Informe',
        description: 'Informe técnico o de investigación',
        icon: '$(file-text)',
      },
      {
        value: 'writing.newsletter',
        label: 'Boletín',
        description: 'Boletín informativo o newsletter',
        icon: '$(mail)',
      },
      {
        value: 'writing.presentation',
        label: 'Presentación',
        description: 'Presentación o slides',
        icon: '$(file-media)',
      },
    ],
  },
  {
    id: 'collection',
    label: 'Colecciones',
    icon: '$(library)',
    natures: [
      {
        value: 'collection.library',
        label: 'Librería',
        description: 'Conjunto de libros/documentos por tema',
        icon: '$(library)',
      },
      {
        value: 'collection.archive',
        label: 'Archivo',
        description: 'Archivo histórico o de referencia',
        icon: '$(archive)',
      },
      {
        value: 'collection.reference',
        label: 'Referencia',
        description: 'Material de consulta',
        icon: '$(references)',
      },
      {
        value: 'collection.playlist',
        label: 'Playlist',
        description: 'Lista de reproducción o colección multimedia',
        icon: '$(list-unordered)',
      },
      {
        value: 'collection.gallery',
        label: 'Galería',
        description: 'Galería de imágenes o medios',
        icon: '$(file-media)',
      },
      {
        value: 'collection.dataset',
        label: 'Dataset',
        description: 'Conjunto de datos o dataset',
        icon: '$(database)',
      },
    ],
  },
  {
    id: 'development',
    label: 'Desarrollo',
    icon: '$(code)',
    natures: [
      {
        value: 'development.software',
        label: 'Software',
        description: 'Proyecto de desarrollo de software',
        icon: '$(code)',
      },
      {
        value: 'development.erp',
        label: 'ERP',
        description: 'Sistema ERP empresarial',
        icon: '$(server)',
      },
      {
        value: 'development.website',
        label: 'Sitio Web',
        description: 'Desarrollo de sitio web',
        icon: '$(globe)',
      },
      {
        value: 'development.api',
        label: 'API',
        description: 'API o servicio backend',
        icon: '$(plug)',
      },
      {
        value: 'development.mobile',
        label: 'Aplicación Móvil',
        description: 'Aplicación móvil (iOS, Android)',
        icon: '$(device-mobile)',
      },
      {
        value: 'development.game',
        label: 'Videojuego',
        description: 'Desarrollo de videojuego',
        icon: '$(game)',
      },
      {
        value: 'development.data-science',
        label: 'Ciencia de Datos',
        description: 'Proyecto de análisis de datos o ML',
        icon: '$(graph)',
      },
      {
        value: 'development.devops',
        label: 'DevOps',
        description: 'Infraestructura y automatización',
        icon: '$(server-process)',
      },
      {
        value: 'development.database',
        label: 'Base de Datos',
        description: 'Diseño y gestión de base de datos',
        icon: '$(database)',
      },
      {
        value: 'development.blockchain',
        label: 'Blockchain',
        description: 'Proyecto blockchain o cripto',
        icon: '$(link)',
      },
      {
        value: 'development.embedded',
        label: 'Sistemas Embebidos',
        description: 'Desarrollo de sistemas embebidos',
        icon: '$(circuit-board)',
      },
    ],
  },
  {
    id: 'management',
    label: 'Gestión',
    icon: '$(briefcase)',
    natures: [
      {
        value: 'management.business',
        label: 'Empresarial',
        description: 'Proyecto empresarial',
        icon: '$(briefcase)',
      },
      {
        value: 'management.personal',
        label: 'Personal',
        description: 'Proyecto personal',
        icon: '$(person)',
      },
      {
        value: 'management.family',
        label: 'Familiar',
        description: 'Proyecto familiar',
        icon: '$(home)',
      },
    ],
  },
  {
    id: 'hierarchical',
    label: 'Jerárquico',
    icon: '$(list-tree)',
    natures: [
      {
        value: 'hierarchical.parent',
        label: 'Proyecto Padre',
        description: 'Proyecto que contiene subproyectos',
        icon: '$(folder-opened)',
      },
      {
        value: 'hierarchical.child',
        label: 'Subproyecto',
        description: 'Subproyecto de un proyecto padre',
        icon: '$(folder)',
      },
      {
        value: 'hierarchical.portfolio',
        label: 'Portafolio',
        description: 'Portafolio de proyectos relacionados',
        icon: '$(folder-library)',
      },
    ],
  },
  {
    id: 'purchase',
    label: 'Compras',
    icon: '$(shopping-cart)',
    natures: [
      {
        value: 'purchase.vehicle',
        label: 'Vehículo',
        description: 'Compra de vehículo',
        icon: '$(car)',
      },
      {
        value: 'purchase.property',
        label: 'Propiedad',
        description: 'Compra de propiedad',
        icon: '$(home)',
      },
      {
        value: 'purchase.equipment',
        label: 'Equipo',
        description: 'Compra de equipo',
        icon: '$(tools)',
      },
      {
        value: 'purchase.service',
        label: 'Servicio',
        description: 'Contratación de servicio',
        icon: '$(briefcase)',
      },
      {
        value: 'purchase.insurance',
        label: 'Seguro',
        description: 'Contratación de seguro',
        icon: '$(shield)',
      },
      {
        value: 'purchase.investment',
        label: 'Inversión',
        description: 'Inversión financiera',
        icon: '$(graph-line)',
      },
      {
        value: 'purchase.subscription',
        label: 'Suscripción',
        description: 'Suscripción a servicio',
        icon: '$(sync)',
      },
    ],
  },
  {
    id: 'education',
    label: 'Educación',
    icon: '$(mortar-board)',
    natures: [
      {
        value: 'education.course',
        label: 'Curso',
        description: 'Curso o programa educativo',
        icon: '$(mortar-board)',
      },
      {
        value: 'education.research',
        label: 'Investigación',
        description: 'Investigación académica',
        icon: '$(search)',
      },
      {
        value: 'education.school',
        label: 'Colegio',
        description: 'Gestión escolar (matrículas, etc.)',
        icon: '$(school)',
      },
      {
        value: 'education.training',
        label: 'Capacitación',
        description: 'Programa de capacitación profesional',
        icon: '$(mortar-board)',
      },
      {
        value: 'education.certification',
        label: 'Certificación',
        description: 'Preparación para certificación',
        icon: '$(verified)',
      },
      {
        value: 'education.workshop',
        label: 'Taller',
        description: 'Taller o workshop',
        icon: '$(tools)',
      },
      {
        value: 'education.online-course',
        label: 'Curso Online',
        description: 'Curso en línea o MOOC',
        icon: '$(globe)',
      },
    ],
  },
  {
    id: 'event',
    label: 'Eventos',
    icon: '$(calendar)',
    natures: [
      {
        value: 'event.wedding',
        label: 'Boda',
        description: 'Planificación de boda',
        icon: '$(heart)',
      },
      {
        value: 'event.travel',
        label: 'Viaje',
        description: 'Planificación de viaje',
        icon: '$(globe)',
      },
      {
        value: 'event.conference',
        label: 'Conferencia',
        description: 'Conferencia o evento profesional',
        icon: '$(megaphone)',
      },
      {
        value: 'event.meeting',
        label: 'Reunión',
        description: 'Reunión o encuentro',
        icon: '$(people)',
      },
      {
        value: 'event.party',
        label: 'Fiesta',
        description: 'Fiesta o celebración',
        icon: '$(gift)',
      },
      {
        value: 'event.exhibition',
        label: 'Exposición',
        description: 'Exposición o muestra',
        icon: '$(file-media)',
      },
      {
        value: 'event.seminar',
        label: 'Seminario',
        description: 'Seminario o charla',
        icon: '$(megaphone)',
      },
    ],
  },
  {
    id: 'reference',
    label: 'Referencia',
    icon: '$(bookmark)',
    natures: [
      {
        value: 'reference.knowledge_base',
        label: 'Base de Conocimiento',
        description: 'Base de conocimiento',
        icon: '$(database)',
      },
      {
        value: 'reference.template',
        label: 'Plantilla',
        description: 'Plantillas y recursos reutilizables',
        icon: '$(file-symlink-file)',
      },
      {
        value: 'reference.archive',
        label: 'Archivo',
        description: 'Archivo de referencia',
        icon: '$(archive)',
      },
    ],
  },
  // PMI: Tipo de Industria (basado en GICS - Global Industry Classification Standard)
  {
    id: 'industry',
    label: 'Industria (PMI/GICS)',
    icon: '$(briefcase)',
    natures: [
      {
        value: 'industry.energy',
        label: 'Energía',
        description: 'Proyecto de energía (petróleo, gas, renovables)',
        icon: '$(zap)',
      },
      {
        value: 'industry.materials',
        label: 'Materiales',
        description: 'Proyecto de materiales (químicos, construcción, minería)',
        icon: '$(package)',
      },
      {
        value: 'industry.industrials',
        label: 'Industriales',
        description: 'Proyecto industrial (aeroespacial, maquinaria, transporte)',
        icon: '$(gear)',
      },
      {
        value: 'industry.consumer-discretionary',
        label: 'Consumo Discrecional',
        description: 'Proyecto de consumo discrecional (automóviles, medios, retail)',
        icon: '$(shopping-cart)',
      },
      {
        value: 'industry.consumer-staples',
        label: 'Consumo Básico',
        description: 'Proyecto de consumo básico (alimentos, bebidas, productos personales)',
        icon: '$(package)',
      },
      {
        value: 'industry.healthcare',
        label: 'Salud',
        description: 'Proyecto de salud (equipos médicos, farmacéuticos, biotecnología)',
        icon: '$(heart)',
      },
      {
        value: 'industry.financial',
        label: 'Finanzas',
        description: 'Proyecto financiero (bancos, seguros, inversiones)',
        icon: '$(graph-line)',
      },
      {
        value: 'industry.it',
        label: 'Tecnología de la Información',
        description: 'Proyecto de TI (software, hardware, semiconductores)',
        icon: '$(code)',
      },
      {
        value: 'industry.telecommunications',
        label: 'Telecomunicaciones',
        description: 'Proyecto de telecomunicaciones y medios',
        icon: '$(radio-tower)',
      },
      {
        value: 'industry.utilities',
        label: 'Servicios Públicos',
        description: 'Proyecto de servicios públicos (electricidad, gas, agua)',
        icon: '$(zap)',
      },
      {
        value: 'industry.real-estate',
        label: 'Bienes Raíces',
        description: 'Proyecto inmobiliario (desarrollo, gestión, REITs)',
        icon: '$(home)',
      },
      {
        value: 'industry.construction',
        label: 'Construcción',
        description: 'Proyecto de construcción e infraestructura',
        icon: '$(tools)',
      },
      {
        value: 'industry.manufacturing',
        label: 'Manufactura',
        description: 'Proyecto de manufactura y producción',
        icon: '$(gear)',
      },
      {
        value: 'industry.retail',
        label: 'Retail',
        description: 'Proyecto de venta al por menor',
        icon: '$(store)',
      },
      {
        value: 'industry.hospitality',
        label: 'Hospitalidad',
        description: 'Proyecto de hotelería y turismo',
        icon: '$(home)',
      },
      {
        value: 'industry.agriculture',
        label: 'Agricultura',
        description: 'Proyecto agrícola o agropecuario',
        icon: '$(leaf)',
      },
      {
        value: 'industry.transportation',
        label: 'Transporte',
        description: 'Proyecto de transporte y logística',
        icon: '$(car)',
      },
      {
        value: 'industry.consulting',
        label: 'Consultoría',
        description: 'Proyecto de consultoría profesional',
        icon: '$(briefcase)',
      },
    ],
  },
  // PKM: Naturaleza del Contenido
  {
    id: 'content',
    label: 'Naturaleza del Contenido (PKM)',
    icon: '$(file-text)',
    natures: [
      {
        value: 'content.research',
        label: 'Investigación',
        description: 'Proyecto de investigación',
        icon: '$(search)',
      },
      {
        value: 'content.learning',
        label: 'Aprendizaje',
        description: 'Proyecto de aprendizaje',
        icon: '$(mortar-board)',
      },
      {
        value: 'content.creative',
        label: 'Creativo',
        description: 'Proyecto creativo',
        icon: '$(paintbrush)',
      },
      {
        value: 'content.analytical',
        label: 'Analítico',
        description: 'Proyecto analítico',
        icon: '$(graph)',
      },
      {
        value: 'content.administrative',
        label: 'Administrativo',
        description: 'Proyecto administrativo',
        icon: '$(file)',
      },
    ],
  },
  // GTD: Área de Responsabilidad
  {
    id: 'responsibility',
    label: 'Área de Responsabilidad (GTD)',
    icon: '$(person)',
    natures: [
      {
        value: 'responsibility.personal',
        label: 'Personal',
        description: 'Proyecto personal',
        icon: '$(person)',
      },
      {
        value: 'responsibility.professional',
        label: 'Profesional',
        description: 'Proyecto profesional',
        icon: '$(briefcase)',
      },
      {
        value: 'responsibility.family',
        label: 'Familiar',
        description: 'Proyecto familiar',
        icon: '$(home)',
      },
    ],
  },
  // Ontologías: Propósito
  {
    id: 'purpose',
    label: 'Propósito (Ontologías)',
    icon: '$(target)',
    natures: [
      {
        value: 'purpose.creation',
        label: 'Creación',
        description: 'Proyecto de creación',
        icon: '$(add)',
      },
      {
        value: 'purpose.research',
        label: 'Investigación',
        description: 'Proyecto de investigación',
        icon: '$(search)',
      },
      {
        value: 'purpose.management',
        label: 'Gestión',
        description: 'Proyecto de gestión',
        icon: '$(list-unordered)',
      },
      {
        value: 'purpose.learning',
        label: 'Aprendizaje',
        description: 'Proyecto de aprendizaje',
        icon: '$(mortar-board)',
      },
    ],
  },
];

export const PROJECT_NATURE_OPTIONS: ProjectNatureOption[] = PROJECT_NATURE_CATEGORIES.flatMap(
  (cat) => cat.natures
);

export const DEFAULT_NATURE = 'generic';

export function getNatureLabel(value: string): string {
  const option = PROJECT_NATURE_OPTIONS.find((opt) => opt.value === value);
  return option?.label || value;
}

export function getNatureDescription(value: string): string {
  const option = PROJECT_NATURE_OPTIONS.find((opt) => opt.value === value);
  return option?.description || '';
}

export function getNatureIcon(value: string): string {
  const option = PROJECT_NATURE_OPTIONS.find((opt) => opt.value === value);
  return option?.icon || '$(folder)';
}

export function getNatureCategory(value: string): ProjectNatureCategory | undefined {
  return PROJECT_NATURE_CATEGORIES.find((cat) =>
    cat.natures.some((opt) => opt.value === value)
  );
}

/**
 * Quick pick item for nature selection
 */
export function createNatureQuickPickItems(): vscode.QuickPickItem[] {
  const items: vscode.QuickPickItem[] = [];
  
  for (const category of PROJECT_NATURE_CATEGORIES) {
    // Add category header
    items.push({
      label: `$(folder) ${category.label}`,
      kind: vscode.QuickPickItemKind.Separator,
    });
    
    // Add nature options
    for (const nature of category.natures) {
      items.push({
        label: `${nature.icon || '$(circle-outline)'} ${nature.label}`,
        description: nature.description,
        detail: nature.value,
      });
    }
  }
  
  // Add generic option at the end
  items.push({
    label: '$(circle-outline) Genérico',
    description: 'Tipo de proyecto no especificado',
    detail: 'generic',
  });
  
  return items;
}

