import * as vscode from 'vscode';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { MetricsDashboard, DashboardMetrics, ModelDriftReport } from '../frontend/MetricsDashboard';

/**
 * Opens the AI Metadata Metrics Dashboard webview.
 * Fetches dashboard metrics and model drift report from the backend.
 */
export async function openMetricsDashboardCommand(
  context: vscode.ExtensionContext,
  adminClient: GrpcAdminClient,
  workspaceId: string
): Promise<void> {
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Loading Cortex Metrics Dashboard...',
      cancellable: false,
    },
    async (progress) => {
      try {
        progress.report({ increment: 0, message: 'Fetching metrics...' });

        // Fetch dashboard metrics and drift report in parallel
        const [metricsResponse, driftResponse] = await Promise.all([
          adminClient.getDashboardMetrics(workspaceId, 24),
          adminClient.getModelDriftReport(workspaceId),
        ]);

        progress.report({ increment: 50, message: 'Processing data...' });

        // Convert proto response to TypeScript types
        const metrics = convertProtoToDashboardMetrics(metricsResponse);
        const driftReport = convertProtoToDriftReport(driftResponse);

        progress.report({ increment: 75, message: 'Rendering dashboard...' });

        // Create and show the dashboard
        const dashboard = new MetricsDashboard(context.extensionUri);
        await dashboard.show(metrics, driftReport);

        progress.report({ increment: 100, message: 'Done' });
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        vscode.window.showErrorMessage(
          `Failed to load metrics dashboard: ${errorMessage}`
        );
      }
    }
  );
}

/**
 * Converts proto DashboardMetrics to TypeScript interface.
 */
function convertProtoToDashboardMetrics(proto: any): DashboardMetrics {
  return {
    extractionSuccessRate: proto.extraction_success_rate ?? 0,
    extractionFailureCount: proto.extraction_failure_count ?? 0,
    avgExtractionLatencyMs: proto.avg_extraction_latency_ms ?? 0,
    confidenceHistogram: new Map(Object.entries(proto.confidence_histogram ?? {})),
    lowConfidenceCount: proto.low_confidence_count ?? 0,
    modelVersions: new Map(Object.entries(proto.model_versions ?? {})),
    tokenUsageByModel: new Map(Object.entries(proto.token_usage_by_model ?? {})),
    costByModel: new Map(Object.entries(proto.cost_by_model ?? {})),
    missingMetadataCount: proto.missing_metadata_count ?? 0,
    staleMetadataCount: proto.stale_metadata_count ?? 0,
    orphanedRecordsCount: proto.orphaned_records_count ?? 0,
    hourlyExtractionCounts: proto.hourly_extraction_counts ?? [],
    dailyErrorRates: proto.daily_error_rates ?? [],
    generatedAt: Number(proto.generated_at ?? 0),
    periodStart: Number(proto.period_start ?? 0),
    periodEnd: Number(proto.period_end ?? 0),
  };
}

/**
 * Converts proto ModelDriftReport to TypeScript interface.
 */
function convertProtoToDriftReport(proto: any): ModelDriftReport | undefined {
  if (!proto) {
    return undefined;
  }

  return {
    workspaceId: proto.workspace_id ?? '',
    periodHours: Number(proto.period_hours ?? 0),
    driftDetected: proto.drift_detected ?? false,
    driftSeverity: (proto.drift_severity as ModelDriftReport['driftSeverity']) ?? 'none',
    indicators: (proto.indicators ?? []).map((ind: any) => ({
      metric: ind.metric ?? '',
      baselineValue: ind.baseline_value ?? 0,
      currentValue: ind.current_value ?? 0,
      changePercent: ind.change_percent ?? 0,
      isSignificant: ind.is_significant ?? false,
    })),
    recommendations: proto.recommendations ?? [],
    generatedAt: Number(proto.generated_at ?? 0),
  };
}

/**
 * Creates a VS Code Disposable for the metrics dashboard command.
 */
export function openMetricsDashboardDisposable(context: vscode.ExtensionContext): vscode.Disposable {
  return vscode.commands.registerCommand('cortex.openMetricsDashboard', async () => {
    // This will be called from extension.ts with proper context
    vscode.window.showErrorMessage('Please use the Cortex panel to open the metrics dashboard');
  });
}
