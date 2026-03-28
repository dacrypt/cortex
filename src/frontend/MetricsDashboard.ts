import * as vscode from 'vscode';

/**
 * Dashboard metrics from the backend.
 */
export interface DashboardMetrics {
    extractionSuccessRate: number;
    extractionFailureCount: number;
    avgExtractionLatencyMs: number;
    confidenceHistogram: Map<string, number>;
    lowConfidenceCount: number;
    modelVersions: Map<string, number>;
    tokenUsageByModel: Map<string, number>;
    costByModel: Map<string, number>;
    missingMetadataCount: number;
    staleMetadataCount: number;
    orphanedRecordsCount: number;
    hourlyExtractionCounts: number[];
    dailyErrorRates: number[];
    generatedAt: number;
    periodStart: number;
    periodEnd: number;
}

/**
 * Confidence distribution for a category.
 */
export interface ConfidenceDistribution {
    category: string;
    distribution: Map<string, number>;
    mean: number;
    median: number;
    stdDev: number;
}

/**
 * Drift indicator for model performance.
 */
export interface DriftIndicator {
    metric: string;
    baselineValue: number;
    currentValue: number;
    changePercent: number;
    isSignificant: boolean;
}

/**
 * Model drift report.
 */
export interface ModelDriftReport {
    workspaceId: string;
    periodHours: number;
    driftDetected: boolean;
    driftSeverity: 'none' | 'minor' | 'major' | 'critical';
    indicators: DriftIndicator[];
    recommendations: string[];
    generatedAt: number;
}

/**
 * MetricsDashboard provides a webview for displaying AI metadata quality metrics.
 */
export class MetricsDashboard {
    private panel: vscode.WebviewPanel | undefined;
    private readonly extensionUri: vscode.Uri;
    private metrics: DashboardMetrics | undefined;
    private driftReport: ModelDriftReport | undefined;

    constructor(extensionUri: vscode.Uri) {
        this.extensionUri = extensionUri;
    }

    /**
     * Shows the metrics dashboard in a webview panel.
     */
    public async show(metrics: DashboardMetrics, driftReport?: ModelDriftReport): Promise<void> {
        this.metrics = metrics;
        this.driftReport = driftReport;

        if (this.panel) {
            this.panel.reveal(vscode.ViewColumn.One);
        } else {
            this.panel = vscode.window.createWebviewPanel(
                'cortexMetricsDashboard',
                'Cortex Metrics Dashboard',
                vscode.ViewColumn.One,
                {
                    enableScripts: true,
                    retainContextWhenHidden: true,
                }
            );

            this.panel.onDidDispose(() => {
                this.panel = undefined;
            });
        }

        this.panel.webview.html = this.getWebviewContent();
    }

    /**
     * Updates the dashboard with new metrics.
     */
    public update(metrics: DashboardMetrics, driftReport?: ModelDriftReport): void {
        this.metrics = metrics;
        this.driftReport = driftReport;
        if (this.panel) {
            this.panel.webview.html = this.getWebviewContent();
        }
    }

    /**
     * Disposes the dashboard panel.
     */
    public dispose(): void {
        if (this.panel) {
            this.panel.dispose();
            this.panel = undefined;
        }
    }

    private getWebviewContent(): string {
        const metrics = this.metrics;
        const drift = this.driftReport;

        if (!metrics) {
            return `<!DOCTYPE html>
            <html>
            <head><title>Cortex Metrics</title></head>
            <body><p>No metrics available.</p></body>
            </html>`;
        }

        const successRateColor = metrics.extractionSuccessRate > 0.9 ? '#4caf50' :
                                 metrics.extractionSuccessRate > 0.7 ? '#ff9800' : '#f44336';
        const driftSeverityColor = !drift ? '#4caf50' :
                                   drift.driftSeverity === 'none' ? '#4caf50' :
                                   drift.driftSeverity === 'minor' ? '#ff9800' :
                                   drift.driftSeverity === 'major' ? '#ff5722' : '#f44336';

        return `<!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>Cortex Metrics Dashboard</title>
            <style>
                body {
                    font-family: var(--vscode-font-family);
                    color: var(--vscode-foreground);
                    background-color: var(--vscode-editor-background);
                    padding: 20px;
                    margin: 0;
                }
                .dashboard {
                    display: grid;
                    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
                    gap: 20px;
                }
                .card {
                    background-color: var(--vscode-editor-background);
                    border: 1px solid var(--vscode-panel-border);
                    border-radius: 8px;
                    padding: 16px;
                }
                .card h3 {
                    margin-top: 0;
                    color: var(--vscode-textLink-foreground);
                    font-size: 14px;
                    text-transform: uppercase;
                    letter-spacing: 0.5px;
                }
                .metric-value {
                    font-size: 32px;
                    font-weight: bold;
                    margin: 8px 0;
                }
                .metric-label {
                    font-size: 12px;
                    color: var(--vscode-descriptionForeground);
                }
                .bar-chart {
                    display: flex;
                    flex-direction: column;
                    gap: 4px;
                }
                .bar-row {
                    display: flex;
                    align-items: center;
                    gap: 8px;
                }
                .bar-label {
                    width: 60px;
                    font-size: 11px;
                }
                .bar-container {
                    flex: 1;
                    height: 16px;
                    background: var(--vscode-progressBar-background);
                    border-radius: 4px;
                    overflow: hidden;
                }
                .bar-fill {
                    height: 100%;
                    background: var(--vscode-progressBar-foreground);
                    border-radius: 4px;
                }
                .bar-value {
                    width: 40px;
                    text-align: right;
                    font-size: 11px;
                }
                .status-indicator {
                    display: inline-block;
                    width: 8px;
                    height: 8px;
                    border-radius: 50%;
                    margin-right: 8px;
                }
                .alert-card {
                    background-color: var(--vscode-inputValidation-warningBackground);
                    border-color: var(--vscode-inputValidation-warningBorder);
                }
                .recommendations {
                    list-style-type: none;
                    padding: 0;
                    margin: 0;
                }
                .recommendations li {
                    padding: 8px 0;
                    border-bottom: 1px solid var(--vscode-panel-border);
                    font-size: 12px;
                }
                .recommendations li:last-child {
                    border-bottom: none;
                }
            </style>
        </head>
        <body>
            <h1>Cortex Metrics Dashboard</h1>
            <p class="metric-label">Generated: ${new Date(metrics.generatedAt).toLocaleString()}</p>

            <div class="dashboard">
                <!-- Extraction Health -->
                <div class="card">
                    <h3>Extraction Health</h3>
                    <div class="metric-value" style="color: ${successRateColor}">
                        ${(metrics.extractionSuccessRate * 100).toFixed(1)}%
                    </div>
                    <div class="metric-label">Success Rate</div>
                    <br>
                    <div class="metric-label">
                        <strong>${metrics.extractionFailureCount}</strong> failures |
                        <strong>${metrics.avgExtractionLatencyMs.toFixed(0)}ms</strong> avg latency
                    </div>
                </div>

                <!-- Model Drift -->
                <div class="card ${drift?.driftDetected ? 'alert-card' : ''}">
                    <h3>Model Drift Status</h3>
                    <div style="display: flex; align-items: center;">
                        <span class="status-indicator" style="background: ${driftSeverityColor}"></span>
                        <span class="metric-value" style="font-size: 24px;">
                            ${drift?.driftSeverity?.toUpperCase() || 'UNKNOWN'}
                        </span>
                    </div>
                    ${drift?.recommendations?.length ? `
                    <h4 style="margin-bottom: 8px;">Recommendations</h4>
                    <ul class="recommendations">
                        ${drift.recommendations.map(r => `<li>${r}</li>`).join('')}
                    </ul>
                    ` : ''}
                </div>

                <!-- Data Quality -->
                <div class="card">
                    <h3>Data Quality</h3>
                    <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 16px; text-align: center;">
                        <div>
                            <div class="metric-value" style="font-size: 24px;">${metrics.missingMetadataCount}</div>
                            <div class="metric-label">Missing</div>
                        </div>
                        <div>
                            <div class="metric-value" style="font-size: 24px;">${metrics.staleMetadataCount}</div>
                            <div class="metric-label">Stale</div>
                        </div>
                        <div>
                            <div class="metric-value" style="font-size: 24px;">${metrics.orphanedRecordsCount}</div>
                            <div class="metric-label">Orphaned</div>
                        </div>
                    </div>
                </div>

                <!-- Confidence Distribution -->
                <div class="card">
                    <h3>Confidence Distribution</h3>
                    <div class="bar-chart">
                        ${this.renderConfidenceChart(metrics.confidenceHistogram)}
                    </div>
                    <div class="metric-label" style="margin-top: 8px;">
                        <strong>${metrics.lowConfidenceCount}</strong> files with low confidence (&lt;40%)
                    </div>
                </div>

                <!-- Model Usage -->
                <div class="card">
                    <h3>Model Usage</h3>
                    <div class="bar-chart">
                        ${this.renderModelUsageChart(metrics.modelVersions)}
                    </div>
                </div>

                <!-- Cost Summary -->
                <div class="card">
                    <h3>Cost Summary</h3>
                    ${this.renderCostSummary(metrics.costByModel, metrics.tokenUsageByModel)}
                </div>
            </div>

            ${drift?.indicators?.length ? `
            <div class="card" style="margin-top: 20px;">
                <h3>Drift Indicators</h3>
                <table style="width: 100%; border-collapse: collapse; font-size: 12px;">
                    <tr style="border-bottom: 1px solid var(--vscode-panel-border);">
                        <th style="text-align: left; padding: 8px;">Metric</th>
                        <th style="text-align: right; padding: 8px;">Baseline</th>
                        <th style="text-align: right; padding: 8px;">Current</th>
                        <th style="text-align: right; padding: 8px;">Change</th>
                        <th style="text-align: center; padding: 8px;">Significant</th>
                    </tr>
                    ${drift.indicators.map(ind => `
                    <tr style="border-bottom: 1px solid var(--vscode-panel-border);">
                        <td style="padding: 8px;">${ind.metric}</td>
                        <td style="text-align: right; padding: 8px;">${ind.baselineValue.toFixed(3)}</td>
                        <td style="text-align: right; padding: 8px;">${ind.currentValue.toFixed(3)}</td>
                        <td style="text-align: right; padding: 8px; color: ${ind.changePercent < 0 ? '#f44336' : '#4caf50'}">
                            ${ind.changePercent > 0 ? '+' : ''}${ind.changePercent.toFixed(1)}%
                        </td>
                        <td style="text-align: center; padding: 8px;">
                            ${ind.isSignificant ? '⚠️' : '✓'}
                        </td>
                    </tr>
                    `).join('')}
                </table>
            </div>
            ` : ''}
        </body>
        </html>`;
    }

    private renderConfidenceChart(histogram: Map<string, number>): string {
        const buckets = ['0.0-0.2', '0.2-0.4', '0.4-0.6', '0.6-0.8', '0.8-1.0'];
        const total = Array.from(histogram.values()).reduce((a, b) => a + b, 0) || 1;

        return buckets.map(bucket => {
            const count = histogram.get(bucket) || 0;
            const pct = (count / total) * 100;
            return `
            <div class="bar-row">
                <span class="bar-label">${bucket}</span>
                <div class="bar-container">
                    <div class="bar-fill" style="width: ${pct}%"></div>
                </div>
                <span class="bar-value">${count}</span>
            </div>`;
        }).join('');
    }

    private renderModelUsageChart(modelVersions: Map<string, number>): string {
        const total = Array.from(modelVersions.values()).reduce((a, b) => a + b, 0) || 1;

        return Array.from(modelVersions.entries()).map(([model, count]) => {
            const pct = (count / total) * 100;
            return `
            <div class="bar-row">
                <span class="bar-label" title="${model}">${model.substring(0, 10)}</span>
                <div class="bar-container">
                    <div class="bar-fill" style="width: ${pct}%"></div>
                </div>
                <span class="bar-value">${count}</span>
            </div>`;
        }).join('');
    }

    private renderCostSummary(costByModel: Map<string, number>, tokensByModel: Map<string, number>): string {
        const totalCost = Array.from(costByModel.values()).reduce((a, b) => a + b, 0);
        const totalTokens = Array.from(tokensByModel.values()).reduce((a, b) => a + b, 0);

        let modelBreakdown = '';
        if (costByModel.size > 0) {
            modelBreakdown = Array.from(costByModel.entries())
                .map(([model, cost]) => `
                    <div style="display: flex; justify-content: space-between; padding: 4px 0;">
                        <span>${model}</span>
                        <span>$${cost.toFixed(4)}</span>
                    </div>
                `).join('');
        }

        return `
        <div style="text-align: center; margin-bottom: 16px;">
            <div class="metric-value" style="font-size: 24px;">$${totalCost.toFixed(2)}</div>
            <div class="metric-label">Total Estimated Cost</div>
            <div class="metric-label" style="margin-top: 8px;">
                ${(totalTokens / 1000).toFixed(1)}K tokens used
            </div>
        </div>
        ${modelBreakdown}`;
    }
}
