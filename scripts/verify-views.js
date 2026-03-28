"use strict";
/**
 * Script de verificación de vistas
 *
 * Compara los datos mostrados por cada vista con la información
 * en la base de datos del índice del backend.
 *
 * Uso:
 *   npx ts-node scripts/verify-views.ts
 */
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.verifyViews = verifyViews;
const vscode = __importStar(require("vscode"));
const GrpcAdminClient_1 = require("../src/core/GrpcAdminClient");
const GrpcKnowledgeClient_1 = require("../src/core/GrpcKnowledgeClient");
const FileCacheService_1 = require("../src/core/FileCacheService");
const BackendMetadataStore_1 = require("../src/core/BackendMetadataStore");
const GrpcMetadataClient_1 = require("../src/core/GrpcMetadataClient");
class ViewVerifier {
    constructor(adminClient, knowledgeClient, metadataClient, fileCacheService, metadataStore, workspaceId, workspaceRoot) {
        this.adminClient = adminClient;
        this.knowledgeClient = knowledgeClient;
        this.metadataClient = metadataClient;
        this.fileCacheService = fileCacheService;
        this.metadataStore = metadataStore;
        this.workspaceId = workspaceId;
        this.workspaceRoot = workspaceRoot;
        this.results = [];
    }
    async verifyAll() {
        console.log('🔍 Iniciando verificación de vistas...\n');
        // Forzar refresh del cache
        await this.fileCacheService.refresh();
        // Verificar cada vista
        await this.verifyContextView();
        await this.verifyTagView();
        await this.verifyTypeView();
        await this.verifyDateView();
        await this.verifySizeView();
        await this.verifyFolderView();
        await this.verifyContentView();
        await this.verifyCodeMetricsView();
        await this.verifyDocumentMetricsView();
        await this.verifyIssuesView();
        return this.results;
    }
    async verifyContextView() {
        console.log('📁 Verificando vista de Proyectos (Context)...');
        try {
            // Obtener proyectos del backend
            const projects = await this.knowledgeClient.listProjects(this.workspaceId);
            const projectCount = projects.length;
            // Contar archivos totales en proyectos
            let totalFilesInProjects = 0;
            const fileSets = new Set();
            for (const project of projects) {
                const docInfos = await this.knowledgeClient.queryDocuments(this.workspaceId, project.id, false);
                docInfos.forEach(doc => fileSets.add(doc.path));
                totalFilesInProjects = fileSets.size;
            }
            // Obtener todos los archivos del backend
            const allFiles = await this.adminClient.listFiles(this.workspaceId);
            const totalFiles = allFiles?.length || 0;
            const issues = [];
            if (projectCount === 0) {
                issues.push('No hay proyectos en el backend');
            }
            if (totalFilesInProjects === 0 && totalFiles > 0) {
                issues.push('Hay archivos en el backend pero ningún proyecto tiene archivos asignados');
            }
            this.results.push({
                view: 'ContextTreeProvider (Por Proyecto)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: projectCount,
                    fromDatabase: projectCount,
                },
            });
            console.log(`   ✅ Proyectos: ${projectCount}, Archivos en proyectos: ${totalFilesInProjects}, Total archivos: ${totalFiles}`);
        }
        catch (error) {
            this.results.push({
                view: 'ContextTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyTagView() {
        console.log('🏷️  Verificando vista de Tags...');
        try {
            // Obtener tags del metadataStore
            const tags = this.metadataStore.getAllTags();
            const tagCount = tags.length;
            // Contar archivos con tags
            let totalTaggedFiles = 0;
            const taggedFileSets = new Set();
            for (const tag of tags) {
                const files = this.metadataStore.getFilesByTag(tag);
                files.forEach(f => taggedFileSets.add(f));
            }
            totalTaggedFiles = taggedFileSets.size;
            // Obtener todos los archivos del backend
            const allFiles = await this.adminClient.listFiles(this.workspaceId);
            const totalFiles = allFiles?.length || 0;
            const issues = [];
            if (tagCount === 0 && totalFiles > 0) {
                issues.push('No hay tags pero hay archivos en el backend');
            }
            this.results.push({
                view: 'TagTreeProvider (Por Tag)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: tagCount,
                    fromDatabase: tagCount,
                },
            });
            console.log(`   ✅ Tags: ${tagCount}, Archivos etiquetados: ${totalTaggedFiles}, Total archivos: ${totalFiles}`);
        }
        catch (error) {
            this.results.push({
                view: 'TagTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyTypeView() {
        console.log('📄 Verificando vista de Tipos...');
        try {
            // Obtener tipos del metadataStore
            const types = this.metadataStore.getAllTypes();
            const typeCount = types.length;
            // Contar archivos por tipo
            let totalTypedFiles = 0;
            const typedFileSets = new Set();
            for (const type of types) {
                const files = this.metadataStore.getFilesByType(type);
                files.forEach(f => typedFileSets.add(f));
            }
            totalTypedFiles = typedFileSets.size;
            // Obtener todos los archivos del backend y contar extensiones
            const allFiles = await this.adminClient.listFiles(this.workspaceId);
            const backendExtensions = new Set();
            allFiles?.forEach(file => {
                if (file.extension) {
                    backendExtensions.add(file.extension);
                }
            });
            const issues = [];
            if (typeCount === 0 && allFiles && allFiles.length > 0) {
                issues.push('No hay tipos pero hay archivos en el backend');
            }
            if (typeCount !== backendExtensions.size) {
                issues.push(`Diferencia en conteo de tipos: vista=${typeCount}, backend=${backendExtensions.size}`);
            }
            this.results.push({
                view: 'TypeTreeProvider (Por Tipo)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: typeCount,
                    fromDatabase: backendExtensions.size,
                    difference: Math.abs(typeCount - backendExtensions.size),
                },
            });
            console.log(`   ✅ Tipos en vista: ${typeCount}, Extensiones en backend: ${backendExtensions.size}`);
        }
        catch (error) {
            this.results.push({
                view: 'TypeTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyDateView() {
        console.log('📅 Verificando vista de Fechas...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Verificar que todos los archivos tengan fecha
            const filesWithoutDate = files.filter(f => {
                const lastModified = f.last_modified || f.enhanced?.stats?.modified || 0;
                return lastModified === 0;
            });
            const issues = [];
            if (filesWithoutDate.length > 0) {
                issues.push(`${filesWithoutDate.length} archivos sin fecha de modificación`);
            }
            this.results.push({
                view: 'DateTreeProvider (Por Fecha)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: fileCount,
                    fromDatabase: fileCount,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Sin fecha: ${filesWithoutDate.length}`);
        }
        catch (error) {
            this.results.push({
                view: 'DateTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifySizeView() {
        console.log('💾 Verificando vista de Tamaños...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Verificar que todos los archivos tengan tamaño
            const filesWithoutSize = files.filter(f => {
                const fileSize = f.file_size || f.enhanced?.stats?.size || 0;
                return fileSize === 0;
            });
            const issues = [];
            if (filesWithoutSize.length > 0) {
                issues.push(`${filesWithoutSize.length} archivos sin tamaño`);
            }
            this.results.push({
                view: 'SizeTreeProvider (Por Tamaño)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: fileCount,
                    fromDatabase: fileCount,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Sin tamaño: ${filesWithoutSize.length}`);
        }
        catch (error) {
            this.results.push({
                view: 'SizeTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyFolderView() {
        console.log('📂 Verificando vista de Carpetas...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Verificar que todos los archivos tengan ruta
            const filesWithoutPath = files.filter(f => !f.relative_path);
            const issues = [];
            if (filesWithoutPath.length > 0) {
                issues.push(`${filesWithoutPath.length} archivos sin ruta relativa`);
            }
            this.results.push({
                view: 'FolderTreeProvider (Por Carpeta)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: fileCount,
                    fromDatabase: fileCount,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Sin ruta: ${filesWithoutPath.length}`);
        }
        catch (error) {
            this.results.push({
                view: 'FolderTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyContentView() {
        console.log('🔖 Verificando vista de Tipos de Contenido...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Contar archivos con MIME type
            const filesWithMime = files.filter(f => f.enhanced?.mime_type?.category);
            const filesWithoutMime = fileCount - filesWithMime.length;
            const issues = [];
            if (filesWithoutMime > 0 && fileCount > 0) {
                issues.push(`${filesWithoutMime} archivos sin tipo MIME (de ${fileCount} total)`);
            }
            this.results.push({
                view: 'ContentTypeTreeProvider (Por Tipo de Contenido)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: filesWithMime.length,
                    fromDatabase: fileCount,
                    difference: filesWithoutMime,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Con MIME: ${filesWithMime.length}, Sin MIME: ${filesWithoutMime}`);
        }
        catch (error) {
            this.results.push({
                view: 'ContentTypeTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyCodeMetricsView() {
        console.log('📊 Verificando vista de Métricas de Código...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Contar archivos con métricas de código
            const filesWithMetrics = files.filter(f => f.enhanced?.code_metrics);
            const filesWithoutMetrics = fileCount - filesWithMetrics.length;
            const issues = [];
            // No es un error si no hay métricas, solo información
            if (filesWithoutMetrics === fileCount && fileCount > 0) {
                issues.push(`Ningún archivo tiene métricas de código (puede ser normal si no hay archivos de código)`);
            }
            this.results.push({
                view: 'CodeMetricsTreeProvider (Métricas de Código)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: filesWithMetrics.length,
                    fromDatabase: fileCount,
                    difference: filesWithoutMetrics,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Con métricas: ${filesWithMetrics.length}`);
        }
        catch (error) {
            this.results.push({
                view: 'CodeMetricsTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyDocumentMetricsView() {
        console.log('📑 Verificando vista de Métricas de Documentos...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Contar archivos con métricas de documento
            const filesWithMetrics = files.filter(f => f.enhanced?.document_metrics);
            const filesWithoutMetrics = fileCount - filesWithMetrics.length;
            const issues = [];
            // No es un error si no hay métricas, solo información
            if (filesWithoutMetrics === fileCount && fileCount > 0) {
                issues.push(`Ningún archivo tiene métricas de documento (puede ser normal si no hay documentos)`);
            }
            this.results.push({
                view: 'DocumentMetricsTreeProvider (Métricas de Documentos)',
                status: issues.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: filesWithMetrics.length,
                    fromDatabase: fileCount,
                    difference: filesWithoutMetrics,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Con métricas: ${filesWithMetrics.length}`);
        }
        catch (error) {
            this.results.push({
                view: 'DocumentMetricsTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    async verifyIssuesView() {
        console.log('⚠️  Verificando vista de Problemas...');
        try {
            const files = await this.fileCacheService.getFiles();
            const fileCount = files.length;
            // Contar archivos con errores
            const filesWithErrors = files.filter(f => f.enhanced?.error);
            const filesWithoutErrors = fileCount - filesWithErrors.length;
            const issues = [];
            if (filesWithErrors.length > 0) {
                issues.push(`${filesWithErrors.length} archivos con errores durante el indexado`);
            }
            this.results.push({
                view: 'IssuesTreeProvider (Problemas)',
                status: filesWithErrors.length > 0 ? 'warning' : 'ok',
                issues,
                data: {
                    fromView: filesWithErrors.length,
                    fromDatabase: fileCount,
                    difference: filesWithoutErrors,
                },
            });
            console.log(`   ✅ Archivos: ${fileCount}, Con errores: ${filesWithErrors.length}`);
        }
        catch (error) {
            this.results.push({
                view: 'IssuesTreeProvider',
                status: 'error',
                issues: [`Error al verificar: ${error instanceof Error ? error.message : String(error)}`],
                data: {
                    fromView: 0,
                    fromDatabase: 0,
                },
            });
            console.log(`   ❌ Error: ${error}`);
        }
    }
    generateReport() {
        let report = '\n' + '='.repeat(80) + '\n';
        report += 'REPORTE DE VERIFICACIÓN DE VISTAS\n';
        report += '='.repeat(80) + '\n\n';
        const ok = this.results.filter(r => r.status === 'ok').length;
        const warnings = this.results.filter(r => r.status === 'warning').length;
        const errors = this.results.filter(r => r.status === 'error').length;
        report += `Resumen:\n`;
        report += `  ✅ OK: ${ok}\n`;
        report += `  ⚠️  Warnings: ${warnings}\n`;
        report += `  ❌ Errores: ${errors}\n\n`;
        report += 'Detalles por vista:\n';
        report += '-'.repeat(80) + '\n';
        for (const result of this.results) {
            const icon = result.status === 'ok' ? '✅' : result.status === 'warning' ? '⚠️' : '❌';
            report += `\n${icon} ${result.view}\n`;
            report += `   Estado: ${result.status.toUpperCase()}\n`;
            report += `   Datos en vista: ${result.data.fromView}\n`;
            report += `   Datos en BD: ${result.data.fromDatabase}\n`;
            if (result.data.difference !== undefined) {
                report += `   Diferencia: ${result.data.difference}\n`;
            }
            if (result.issues.length > 0) {
                report += `   Problemas:\n`;
                result.issues.forEach(issue => {
                    report += `     - ${issue}\n`;
                });
            }
        }
        report += '\n' + '='.repeat(80) + '\n';
        return report;
    }
}
// Función principal (solo para testing fuera de VS Code)
// En producción, esto se ejecutaría como comando de VS Code
async function verifyViews(context, workspaceId, workspaceRoot) {
    const adminClient = new GrpcAdminClient_1.GrpcAdminClient(context);
    const knowledgeClient = new GrpcKnowledgeClient_1.GrpcKnowledgeClient(context);
    const metadataClient = new GrpcMetadataClient_1.GrpcMetadataClient(context);
    const fileCacheService = FileCacheService_1.FileCacheService.getInstance(adminClient);
    fileCacheService.setWorkspaceId(workspaceId);
    const metadataStore = new BackendMetadataStore_1.BackendMetadataStore(metadataClient, adminClient, workspaceId);
    await metadataStore.initialize();
    const verifier = new ViewVerifier(adminClient, knowledgeClient, metadataClient, fileCacheService, metadataStore, workspaceId, workspaceRoot);
    const results = await verifier.verifyAll();
    const report = verifier.generateReport();
    console.log(report);
    // Mostrar en output channel de VS Code
    const outputChannel = vscode.window.createOutputChannel('Cortex View Verification');
    outputChannel.appendLine(report);
    outputChannel.show();
}
//# sourceMappingURL=verify-views.js.map