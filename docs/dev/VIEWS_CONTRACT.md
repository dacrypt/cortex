# Views Contract (Draft)

Goal: standardize structure, data sources, and behavior across all tree views.

## ViewNode

Every tree entry maps to a ViewNode with a stable id and a typed kind.

Fields:
- id: string (stable, not encoded in the label)
- kind: group | facet | range | file | separator
- label: string
- count: number (optional)
- description: string (optional)
- tooltip: string or MarkdownString (optional)
- icon: ThemeIcon (optional)
- payload: { facet, value } or { relativePath } or custom

## Conventions

- Language: use a single language in all labels, placeholders, and tooltips.
- Levels: root -> facet/group -> file (avoid mixing folders and files in the same root).
- Data source per facet:
  - tag: metadataStore only
  - type: normalized extension (shared helper)
  - content_type: mime_type.category with extension fallback (shared helper)
  - context: knowledgeClient only
- Sorting: define per facet (alpha or count desc) and keep consistent.
- contextValue: standardize by kind/facet to enable shared actions.
- Separators: use kind=separator (not encoded in labels).

## Migration Notes

Phase 1: move TagTreeProvider and TypeTreeProvider to shared facet helpers.
Phase 2: apply ViewNode payloads to TermsFacet and range facets.
Phase 3: align category-based views to the same levels and strings catalog.
