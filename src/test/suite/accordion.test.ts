import * as assert from 'assert';
import { TreeAccordionState } from '../../views/TreeAccordionState';

describe('TreeAccordionState', () => {
  it('tracks expanded keys and accordion behavior', () => {
    const rootKeys = ['a', 'b'];
    let refreshed = 0;
    const state = new TreeAccordionState(
      () => rootKeys,
      () => {
        refreshed += 1;
      }
    );

    state.handleDidExpand('a');
    assert.strictEqual(state.isExpanded('a'), true);

    state.setAccordionEnabled(true);
    state.handleDidExpand('b');
    assert.strictEqual(state.isExpanded('a'), false);
    assert.strictEqual(state.isExpanded('b'), true);
    assert.ok(refreshed > 0);

    state.collapseAll();
    assert.strictEqual(state.isExpanded('b'), false);
  });
});
