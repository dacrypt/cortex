export class TreeAccordionState {
  private accordionEnabled = false;
  private expandedKeys = new Set<string>();
  private lastExpandedKey: string | null = null;

  constructor(
    private getRootKeys: () => string[],
    private refresh: () => void
  ) {}

  isExpanded(key: string): boolean {
    return this.expandedKeys.has(key);
  }

  setAccordionEnabled(enabled: boolean): void {
    if (this.accordionEnabled === enabled) {
      return;
    }
    this.accordionEnabled = enabled;

    if (!enabled || this.expandedKeys.size <= 1) {
      return;
    }

    const rootKeys = this.getRootKeys();
    let keep =
      this.lastExpandedKey && rootKeys.includes(this.lastExpandedKey)
        ? this.lastExpandedKey
        : null;

    if (!keep) {
      for (const key of this.expandedKeys) {
        if (rootKeys.includes(key)) {
          keep = key;
          break;
        }
      }
    }

    this.expandedKeys.clear();
    if (keep) {
      this.expandedKeys.add(keep);
    }
    this.refresh();
  }

  expandAll(): void {
    const rootKeys = this.getRootKeys();
    if (rootKeys.length === 0) {
      return;
    }
    this.expandedKeys = new Set(rootKeys);
    this.refresh();
  }

  collapseAll(): void {
    if (this.expandedKeys.size === 0) {
      return;
    }
    this.expandedKeys.clear();
    this.refresh();
  }

  handleDidExpand(key?: string): void {
    if (!this.isRootKey(key)) {
      return;
    }

    this.lastExpandedKey = key;
    if (this.accordionEnabled) {
      const needsRefresh =
        this.expandedKeys.size !== 1 || !this.expandedKeys.has(key);
      this.expandedKeys.clear();
      this.expandedKeys.add(key);
      if (needsRefresh) {
        this.refresh();
      }
      return;
    }

    this.expandedKeys.add(key);
  }

  handleDidCollapse(key?: string): void {
    if (!this.isRootKey(key)) {
      return;
    }
    this.expandedKeys.delete(key);
  }

  private isRootKey(key?: string): key is string {
    if (!key) {
      return false;
    }
    return this.getRootKeys().includes(key);
  }
}
