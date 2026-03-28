import * as vscode from 'vscode';

export type ViewNodeKind = 'group' | 'facet' | 'range' | 'file' | 'separator';

export interface ViewNode<TPayload = Record<string, unknown>> {
  id: string;
  kind: ViewNodeKind;
  label: string;
  count?: number;
  description?: string;
  tooltip?: string | vscode.MarkdownString;
  icon?: vscode.ThemeIcon;
  payload?: TPayload;
}

export interface FacetValuePayload {
  facet: string;
  value: string;
}

export interface FilePayload {
  relativePath: string;
}
