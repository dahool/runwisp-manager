import { describe, expect, it } from 'vitest';
import { diffOps, setAt, toSaveOps } from './ops.js';
describe('editor operations', () => {
	it('returns no operations when the form is untouched', () => { expect(diffOps({ schedule: '* * * * *' }, { schedule: '* * * * *' })).toEqual([]); });
	it('creates surgical set and delete operations', () => {
		const original = { schedule: 'daily', enabled: true, nested: { keep: 'x' } };
		const current = setAt(setAt(original, 'schedule', 'hourly'), 'enabled');
		expect(toSaveOps(diffOps(original, current))).toEqual([{ type: 'deleteKey', path: ['enabled'] }, { type: 'setValue', path: ['schedule'], value: 'hourly' }]);
	});
});
